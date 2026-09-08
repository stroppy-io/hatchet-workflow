// stroppy-suite is the Graphene pipeline that fans a SuiteSpec
// (spec.suite@1) out into stroppy-run child runs and returns a
// SuiteResult with every cell's outcome.
package main

import (
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/suite"
)

func main() {
	pipeline.Main(suite.PipelineID, suite.Run, pipeline.WithConcurrency(pipeline.Parallel))
}
