package run

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
)

// Phase names — the server's RunPhase enum.
const (
	PhaseProvisioning = "provisioning"
	PhaseDeploying    = "deploying"
	PhaseWorkload     = "workload"
	PhaseCollecting   = "collecting"
	PhaseTeardown     = "teardown"
)

// phase runs one named step of the run with milestones around it. A panic
// from a Ready inside the body is recovered into the error so the
// phase.failed milestone is emitted before the run fails — the SDK's
// resource failure is re-panicked afterwards so Main still reports it.
func phase[T any](ctx pipeline.Context, name string, body func() (T, error)) (out T, err error) {
	start := workflow.Now(ctx)
	events.Emit(ctx, events.PhaseStarted, events.Payload{"phase": name})
	defer func() {
		if p := recover(); p != nil {
			perr, ok := p.(error)
			if !ok {
				perr = fmt.Errorf("%v", p)
			}
			events.Emit(ctx, events.PhaseFailed, events.Payload{"phase": name, "error": perr.Error(), "elapsed": elapsed(ctx, start)})
			panic(p)
		}
		if err != nil {
			events.Emit(ctx, events.PhaseFailed, events.Payload{"phase": name, "error": err.Error(), "elapsed": elapsed(ctx, start)})
			return
		}
		events.Emit(ctx, events.PhaseFinished, events.Payload{"phase": name, "elapsed": elapsed(ctx, start)})
	}()
	return body()
}

func elapsed(ctx pipeline.Context, start time.Time) string {
	return workflow.Now(ctx).Sub(start).Round(time.Millisecond).String()
}

// parallel runs fns concurrently as workflow goroutines and joins their
// errors, naming each failed unit. Deterministic: results are collected in
// input order.
func parallel(ctx pipeline.Context, names []string, fns []func(gctx pipeline.Context) error) error {
	if len(names) != len(fns) {
		panic("parallel: names and fns differ in length")
	}
	errs := make([]error, len(fns))
	wg := workflow.NewWaitGroup(ctx)
	for i := range fns {
		wg.Add(1)
		workflow.Go(ctx, func(gctx workflow.Context) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					if e, ok := p.(error); ok {
						errs[i] = e
						return
					}
					errs[i] = fmt.Errorf("%v", p)
				}
			}()
			errs[i] = fns[i](pipeline.Context{Context: gctx})
		})
	}
	wg.Wait(ctx)
	var joined []error
	for i, err := range errs {
		if err != nil {
			joined = append(joined, fmt.Errorf("%s: %w", names[i], err))
		}
	}
	return joinErrors(joined)
}

func joinErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}
	msg := make([]string, 0, len(errs))
	for _, e := range errs {
		msg = append(msg, e.Error())
	}
	return fmt.Errorf("%d failures: %s", len(errs), joinStrings(msg))
}

func joinStrings(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "; "
		}
		out += s
	}
	return out
}
