package run

import (
	"errors"

	databasebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/database"
	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

type BuildOptions struct {
	ID      string
	SuiteID string

	Database *domain.Database
	Workload *domain.Workload

	Provider        deployment.Provider
	Infrastructure  infrastructurebuilder.BuildOptions
	RenderOverrides *deployment.RenderOverrideSet

	Tags *common.Tags
}

func BuildTestRun(options BuildOptions) (*domain.TestRun, error) {
	if options.Database == nil {
		return nil, errors.New("database is required")
	}
	if options.Workload == nil {
		return nil, errors.New("workload is required")
	}
	if err := options.Workload.Validate(); err != nil {
		return nil, err
	}

	spec, err := databasebuilder.BuildTopologySpec(options.Database)
	if err != nil {
		return nil, err
	}

	infrastructurePlan, err := infrastructurebuilder.BuildPlan(spec, options.Provider, options.Infrastructure)
	if err != nil {
		return nil, err
	}

	run := &domain.TestRun{
		Id:                 options.ID,
		SuiteId:            options.SuiteID,
		Database:           options.Database,
		Workload:           options.Workload,
		TopologySpec:       spec,
		InfrastructurePlan: infrastructurePlan,
		RenderOverrides:    options.RenderOverrides,
		Tags:               options.Tags,
	}
	if err := run.Validate(); err != nil {
		return nil, err
	}
	return run, nil
}
