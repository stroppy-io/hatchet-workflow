package stroppy

import (
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/types"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/scripting"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

var ErrUnsupportedDatabaseTopology = fmt.Errorf("unsupported database topology")
var ErrClusterNotImplemented = fmt.Errorf("cluster not implemented now")

const UbuntuBaseImageId = "fd82pkek8uu0ejjkh4vn"

func buildProvisionInput(
	hatchetRunId ids.HatchetRunId,
	runId ids.RunId,
	deploymentName string,
	workerName types.WorkerName,
	cloud crossplane.SupportedCloud,
	machineInfo *crossplane.MachineInfo,
) (*hatchet.Tasks_Provision_Input, error) {
	return &hatchet.Tasks_Provision_Input{
		RunId:               runId.String(),
		HatchetWorkersNames: []string{workerName.String()},
		Request: &crossplane.DeploymentSet_Request{
			Requests: []*crossplane.Deployment_Request{
				{
					Name:           deploymentName,
					SupportedCloud: cloud,
					MachineInfo:    machineInfo,
					CloudInit: scripting.InstallEdgeWorkerCloudInit(
						scripting.WithEnv(map[string]string{
							"HATCHET_PARENT_STEP_RUN_ID": hatchetRunId.String(),
							"HATCHET_WORKER_NAME":        workerName.String(),
						}),
					),
					Labels: map[string]string{"run_id": runId.String()},
				},
			},
		},
	}, nil
}
