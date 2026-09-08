// stroppy-provider-verify checks a tenant's cloud credentials and rights
// (spec.provider_verify@1 → spec.result.provider_verify@1) without creating
// anything billable.
package main

import (
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/probe"
)

func main() {
	pipeline.Main(probe.VerifyPipelineID, probe.Verify, pipeline.WithConcurrency(pipeline.Parallel))
}
