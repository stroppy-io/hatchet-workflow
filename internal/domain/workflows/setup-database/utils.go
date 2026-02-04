package setup_database

import (
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/scripting"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

var ErrUnsupportedDatabaseTopology = fmt.Errorf("unsupported database topology")

const UbuntuBaseImageId = "fd82pkek8uu0ejjkh4vn"

func singlePostgresInstance(
	runId string,
	cloud crossplane.SupportedCloud,
	database *database.Database,
) *crossplane.Deployment_Request {
	return &crossplane.Deployment_Request{
		Name:           database.GetName(),
		SupportedCloud: cloud,
		MachineInfo: &crossplane.MachineInfo{
			Cores:       database.GetStandalone().GetVmSpec().GetCpu(),
			Memory:      database.GetStandalone().GetVmSpec().GetMemory(),
			Disk:        database.GetStandalone().GetVmSpec().GetDisk(),
			BaseImageId: UbuntuBaseImageId,
		},
		// TODO: Add CloudInit
		CloudInit: scripting.InstallEdgeWorkerCloudInit(),
		Labels:    map[string]string{"database": database.GetName(), "run_id": runId},
	}
}

func buildDatabaseProvisionInput(
	input *hatchet.Tasks_SetupDatabase_Input,
) (*hatchet.Tasks_Provision_Input, error) {
	if input == nil {
		return nil, fmt.Errorf("input is nil")
	}
	err := input.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate input: %w", err)
	}
	switch input.GetDatabase().GetTopology().(type) {
	case *database.Database_Standalone:
		return &hatchet.Tasks_Provision_Input{
			Request: &crossplane.DeploymentSet_Request{
				Requests: []*crossplane.Deployment_Request{
					singlePostgresInstance(input.GetRunId(), input.GetSupportedCloud(), input.GetDatabase()),
				},
			},
		}, nil
	default:
		return nil, ErrUnsupportedDatabaseTopology
	}
}
