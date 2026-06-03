package run

import (
	"errors"

	"google.golang.org/protobuf/proto"

	databasebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/database"
	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
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
	spec, err = workloadbuilder.ExtendTopologySpec(spec, options.Workload)
	if err != nil {
		return nil, err
	}

	infrastructureOptions := options.Infrastructure
	infrastructureOptions.MachineSizing = cloneMachineSizing(options.Infrastructure.MachineSizing)
	infrastructureOptions.MachineOverrides = cloneMachineOverrides(options.Infrastructure.MachineOverrides)
	workloadbuilder.ApplyRunnerSizing(&infrastructureOptions, options.Workload)

	infrastructurePlan, err := infrastructurebuilder.BuildPlan(spec, options.Provider, infrastructureOptions)
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

func cloneMachineSizing(input map[string]infrastructurebuilder.MachineSizing) map[string]infrastructurebuilder.MachineSizing {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]infrastructurebuilder.MachineSizing, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneMachineOverrides(input []*deployment.MachinePlan) []*deployment.MachinePlan {
	if len(input) == 0 {
		return nil
	}
	output := make([]*deployment.MachinePlan, 0, len(input))
	for _, machine := range input {
		if machine == nil {
			continue
		}
		output = append(output, proto.Clone(machine).(*deployment.MachinePlan))
	}
	return output
}
