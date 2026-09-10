package run

import (
	"fmt"
	"time"

	"github.com/graphene-ci/pipeline/pkg/activity"
	"github.com/graphene-ci/pipeline/pkg/artifact"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

const (
	// segmentMargin is added to a segment's own bound: schema and data load
	// steps, image pull and container start are not part of the scenario
	// duration.
	segmentMargin = 30 * time.Minute
	// segmentIterationsBound caps an iteration-bounded segment, whose wall
	// time stroppy cannot know in advance.
	segmentIterationsBound = 24 * time.Hour
	segmentHeartbeat       = 5 * time.Minute
	// baselineBound covers a full two-tier baseline plus the pg-noop
	// download on a slow machine.
	baselineBound = 20 * time.Minute
)

// workloadOutcome is what the workload phase produced.
type workloadOutcome struct {
	Segments []spec.SegmentResult
	Baseline *spec.BaselineResult
	// Files on the runner machine to publish as artifacts.
	Files []artifactFile
	// Runner is the agent the files live on.
	Runner pipeline.Agent
}

type artifactFile struct {
	Name string
	Path string
}

// runWorkload executes the machine baseline (when asked) and then the
// segments in order on the runner machine. Every segment is one-shot
// (AtMostOnce): an undeterminable outcome fails the run rather than loading
// data twice. The first failed segment stops the sequence — later segments
// depend on the earlier ones (schema, data).
func runWorkload(ctx pipeline.Context, run spec.Run, infra provision.Infra, addrs Addresses) (workloadOutcome, error) {
	runners := infra.ByRole(run.Workload.RunnerRole)
	if len(runners) == 0 {
		return workloadOutcome{}, fmt.Errorf("workload: no machine of runner role %q", run.Workload.RunnerRole)
	}
	runner := runners[0]
	segments, err := spec.DecodeSegments(run.Workload.Segments)
	if err != nil {
		return workloadOutcome{}, fmt.Errorf("workload: %w", err)
	}
	url, err := addrs.Expand(run.Workload.URL)
	if err != nil {
		return workloadOutcome{}, fmt.Errorf("workload url: %w", err)
	}
	out := workloadOutcome{Runner: runner.Agent}

	if b := run.Workload.Baseline; b != nil && b.Enabled {
		out.Baseline = runBaseline(ctx, run, runner, &out)
	}

	for i, seg := range segments {
		events.Emit(ctx, events.SegmentStarted, events.Payload{"segment": seg.Name, "index": i, "script": seg.Workload.Script})
		req := activities.RunSegmentRequest{
			RunID: run.RunID, Segment: seg, Index: i, Workload: run.Workload, URL: url,
			OTLPEndpoint: run.Observability.OTLPEndpoint, OTLPHeaders: run.Observability.OTLPHeaders,
			Labels:         run.Observability.Labels,
			RegistrySecret: run.Provider.RegistrySecret,
		}
		res, err := activity.Activity(ctx, runner.Agent,
			activity.Fn(activities.NameRunSegment, activities.RunSegment, req),
			activity.WithGuarantee(activity.AtMostOnce),
			activity.WithTimeout(segmentBound(seg)),
			activity.WithHeartbeat(segmentHeartbeat),
		)
		if err != nil {
			// The activity itself failed (worker died, unknown outcome):
			// record the segment as failed and stop.
			failed := spec.SegmentResult{Name: seg.Name, Status: spec.SegmentFailed, Error: err.Error(), ExitCode: -1}
			out.Segments = append(out.Segments, failed)
			events.Emit(ctx, events.SegmentFailed, events.Payload{"segment": seg.Name, "index": i, "error": err.Error()})
			return out, fmt.Errorf("segment %s: %w", seg.Name, err)
		}
		out.Segments = append(out.Segments, res.Result)
		if res.ConfigPath != "" {
			out.Files = append(out.Files, artifactFile{Name: "stroppy-" + seg.Name + "-config", Path: res.ConfigPath})
		}
		if res.LogPath != "" {
			out.Files = append(out.Files, artifactFile{Name: "stroppy-" + seg.Name + "-log", Path: res.LogPath})
		}
		if res.Result.Status != spec.SegmentCompleted {
			events.Emit(ctx, events.SegmentFailed, events.Payload{"segment": seg.Name, "index": i, "status": string(res.Result.Status), "error": res.Result.Error})
			return out, fmt.Errorf("segment %s %s: %s", seg.Name, res.Result.Status, res.Result.Error)
		}
		events.Emit(ctx, events.SegmentFinished, events.Payload{
			"segment": seg.Name, "index": i, "status": string(res.Result.Status),
			"started_at": res.Result.StartedAt, "finished_at": res.Result.FinishedAt,
			"metrics": headlineOf(res.Result),
		})
	}
	return out, nil
}

// runBaseline measures the runner with `stroppy baseline`. Its failure is
// recorded in the result, never fatal: the run is about the database, the
// baseline is the context to read it in.
func runBaseline(ctx pipeline.Context, run spec.Run, runner *provision.Machine, out *workloadOutcome) *spec.BaselineResult {
	events.Emit(ctx, events.BaselineStarted, events.Payload{"tiers": run.Workload.Baseline.Tiers})
	res, err := activity.Activity(ctx, runner.Agent,
		activity.Fn(activities.NameRunBaseline, activities.RunBaseline, activities.RunBaselineRequest{
			RunID: run.RunID, Image: run.Workload.StroppyImage, Baseline: *run.Workload.Baseline,
			RegistrySecret: run.Provider.RegistrySecret,
		}),
		activity.WithGuarantee(activity.AtMostOnce),
		activity.WithTimeout(baselineBound),
		activity.WithHeartbeat(segmentHeartbeat),
	)
	if err != nil {
		events.Emit(ctx, events.BaselineFinished, events.Payload{"ok": false, "error": err.Error()})
		return &spec.BaselineResult{Error: err.Error()}
	}
	if res.LogPath != "" {
		out.Files = append(out.Files, artifactFile{Name: "stroppy-baseline-log", Path: res.LogPath})
	}
	events.Emit(ctx, events.BaselineFinished, events.Payload{"ok": res.Result.OK, "error": res.Result.Error, "verdicts": res.Result.Verdicts})
	return &res.Result
}

// segmentBound is the activity timeout of a segment.
func segmentBound(seg spec.Segment) time.Duration {
	return stroppycfg.Bound(seg, segmentMargin, segmentIterationsBound)
}

// headlineOf picks the few numbers worth a milestone payload.
func headlineOf(r spec.SegmentResult) map[string]float64 {
	out := map[string]float64{}
	for _, k := range []string{"iterations_total", "iteration_duration_p50", "iteration_duration_p95", "iteration_duration_p99", "failed_iterations_total"} {
		if v, ok := r.Metrics[k]; ok {
			out[k] = v.Value
		}
	}
	return out
}

// publishArtifacts declares one artifact per workload file on the runner and
// waits for the uploads. A failed upload is a warning, not a failed run:
// the benchmark happened, the metrics are in Victoria.
func publishArtifacts(ctx pipeline.Context, run spec.Run, wl workloadOutcome) []string {
	var names []string
	for _, f := range wl.Files {
		name := run.RunID[:8] + "-" + f.Name
		h := pipeline.NewArtifact(ctx, name, artifact.FromAgentFile(wl.Runner, f.Path))
		if _, err := h.TryReady(ctx); err != nil {
			ctx.Logger().Warn("artifact upload failed", "artifact", name, "error", err)
			continue
		}
		names = append(names, "artifact/"+name)
	}
	return names
}
