package edge

import (
	"fmt"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	edgeDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"
)

func StartContainersTask(c *hatchetLib.Client, identifier *edge.Task_Identifier) *hatchetLib.StandaloneTask {
	return c.NewStandaloneTask(
		edgeDomain.TaskIdToString(identifier),
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *workflows.Tasks_StartDockerContainers_Input,
		) (*workflows.Tasks_StartDockerContainers_Output, error) {
			if err := input.Validate(); err != nil {
				return nil, err
			}

			networkName, err := resolveContainerNetworkName(input)
			if err != nil {
				return nil, err
			}

			runner, err := NewContainerRunner(networkName)
			if err != nil {
				return nil, err
			}
			defer runner.Close()

			if err := runner.DeployContainersForTarget(
				ctx.GetContext(),
				input.GetContext(),
				input.GetWorkerInternalIp().GetValue(),
				input.GetContainers(),
			); err != nil {
				return nil, err
			}

			return &workflows.Tasks_StartDockerContainers_Output{}, nil
		}),
	)
}

func resolveContainerNetworkName(input *workflows.Tasks_StartDockerContainers_Input) (string, error) {
	if dockerSettings := input.GetContext().GetSelectedTarget().GetDockerSettings(); dockerSettings != nil {
		if dockerSettings.GetNetworkName() != "" {
			return dockerSettings.GetNetworkName(), nil
		}
		return "", fmt.Errorf("docker target selected but docker network name is empty")
	}

	cidr := input.GetWorkerInternalCidr().GetValue()
	if cidr == "" {
		return "", fmt.Errorf("worker internal cidr is empty")
	}
	runID := sanitizeDockerNamePart(input.GetContext().GetRunId())
	return fmt.Sprintf("edge-%s-%s", runID, sanitizeDockerNamePart(cidr)), nil
}
