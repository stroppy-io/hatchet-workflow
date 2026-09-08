// stroppy-quotas reads a tenant's cloud quota limits and usage
// (spec.quotas@1 → spec.result.quotas@1).
package main

import (
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/probe"
)

func main() {
	pipeline.Main(probe.QuotasPipelineID, probe.Quotas, pipeline.WithConcurrency(pipeline.Parallel))
}
