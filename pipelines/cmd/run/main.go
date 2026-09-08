// stroppy-run is the Graphene pipeline that executes one benchmark run
// from a fully resolved RunSpec (spec.run@1) and returns a RunResult
// (spec.result.run@1). The server starts it; a person can too:
//
//	stroppy-run run --params @runspec.json --watch
//
// The binary manages its own pipeline record (push/run/plan) and serves as
// the run worker or as the machine executor depending on GRAPHENE_ROLE.
package main

import (
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/run"
)

func main() {
	// Runs are independent: the server bounds concurrency through tenant
	// limits, not through the pipeline's own queue.
	pipeline.Main(run.PipelineID, run.Run, pipeline.WithConcurrency(pipeline.Parallel))
}
