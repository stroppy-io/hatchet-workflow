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
)

const (
	// segmentMargin is added to a segment's own bound: load steps and
	// container start are not counted by k6.
	segmentMargin = 30 * time.Minute
	// segmentIterationsBound caps an iteration-bounded segment.
	segmentIterationsBound = 24 * time.Hour
	segmentHeartbeat       = 5 * time.Minute
)

// workloadOutcome is what the workload phase produced.
type workloadOutcome struct {
	Segments []spec.SegmentResult
	// Files on the runner machine to publish as artifacts.
	Files []artifactFile
	// Runner is the agent the files live on.
	Runner pipeline.Agent
}

type artifactFile struct {
	Name string
	Path string
}

// runWorkload executes the segments in order on the runner machine. Every
// segment is one-shot (AtMostOnce): an undeterminable outcome fails the run
// rather than loading data twice. The first failed segment stops the
// sequence — later segments depend on the earlier ones (schema, data).
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
	env, err := addrs.ExpandMap(run.Workload.Env)
	if err != nil {
		return workloadOutcome{}, fmt.Errorf("workload env: %w", err)
	}
	out := workloadOutcome{Runner: runner.Agent}
	for i, seg := range segments {
		events.Emit(ctx, events.SegmentStarted, events.Payload{"segment": seg.Name, "index": i, "script": seg.Script})
		req := activities.RunSegmentRequest{
			RunID: run.RunID, Segment: seg, Index: i, Image: run.Workload.StroppyImage, Env: env,
			OTLPEndpoint: run.Observability.OTLPEndpoint, Labels: run.Observability.Labels,
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
			failed := spec.SegmentResult{Name: seg.Name, Status: spec.SegmentFailed, Error: err.Error()}
			out.Segments = append(out.Segments, failed)
			events.Emit(ctx, events.SegmentFailed, events.Payload{"segment": seg.Name, "index": i, "error": err.Error()})
			return out, fmt.Errorf("segment %s: %w", seg.Name, err)
		}
		out.Segments = append(out.Segments, res.Result)
		if res.SummaryPath != "" {
			out.Files = append(out.Files, artifactFile{Name: "stroppy-" + seg.Name + "-summary", Path: res.SummaryPath})
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

// segmentBound is the activity timeout of a segment.
func segmentBound(seg spec.Segment) time.Duration {
	if seg.Execution.Limit.Kind == "iterations" {
		return segmentIterationsBound
	}
	d := seg.Execution.Limit.Duration.Std()
	return d + d/2 + seg.Warmup.Std() + segmentMargin
}

// headlineOf picks the few numbers worth a milestone payload.
func headlineOf(r spec.SegmentResult) map[string]float64 {
	out := map[string]float64{}
	for _, k := range []string{"iterations_rate", "iterations_count", "iteration_duration_p95", "iteration_duration_p99"} {
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
