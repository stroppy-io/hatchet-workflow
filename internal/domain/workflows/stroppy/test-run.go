package stroppy

import (
	"errors"
	"fmt"

	"github.com/hatchet-dev/hatchet/pkg/worker/condition"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/types"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

const (
	TestRunWorkflowName                 = "stroppy-test-run"
	TestRunValidateInputWorkflowName    = "stroppy-test-run-validate-input"
	TestRunSetupDatabaseWorkflowName    = "stroppy-test-run-provision-database"
	TestRunProvisionStroppyWorkflowName = "stroppy-test-run-provision-stroppy"
	TestRunRunStroppyWorkflowName       = "stroppy-test-run-run-stroppy"
)

func TestRunWorkflow(
	c *hatchetLib.Client,
) *hatchetLib.Workflow {
	workflow := c.NewWorkflow(
		TestRunWorkflowName,
		hatchetLib.WithWorkflowDescription("Stroppy Test Run Workflow"),
	)
	validateInputTask := workflow.NewTask(
		TestRunValidateInputWorkflowName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_StroppyTest_Input,
		) (*hatchet.Tasks_StroppyTest_Input, error) {
			err := input.Validate()
			if err != nil {
				return nil, err
			}
			return input, nil
		}),
		hatchetLib.WithSkipIf(condition.Conditions()),
	)

	provisionDatabaseTask := workflow.NewTask(
		TestRunSetupDatabaseWorkflowName,
		hatchet_ext.Ptask(validateInputTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_StroppyTest_Input,
			parentOutput *hatchet.Tasks_StroppyTest_Input,
		) (*hatchet.Tasks_StroppyTest_Output, error) {
			var provisionInput *hatchet.Tasks_Provision_Input
			switch input.GetTest().GetDatabase().GetTopology().(type) {
			case *database.Database_Standalone:
				databaseProvisionInput, err := buildProvisionInput(
					ids.HatchetRunId(ctx.StepRunId()),
					ids.RunId(input.RunId),
					"database-deployment-"+ids.HatchetRunId(ctx.StepRunId()).String(),
					types.WorkerName("database-worker-"+ids.HatchetRunId(ctx.StepRunId()).String()),
					input.GetSupportedCloud(),
					&crossplane.MachineInfo{
						Cores:       input.GetTest().GetDatabase().GetStandalone().GetVmSpec().GetCpu(),
						Memory:      input.GetTest().GetDatabase().GetStandalone().GetVmSpec().GetMemory(),
						Disk:        input.GetTest().GetDatabase().GetStandalone().GetVmSpec().GetDisk(),
						BaseImageId: UbuntuBaseImageId,
					},
				)
				if err != nil {
					return nil, fmt.Errorf("failed to build provision database input: %w", err)
				}
				provisionInput = databaseProvisionInput
			case *database.Database_Cluster:
				return nil, errors.Join(ErrUnsupportedDatabaseTopology, ErrClusterNotImplemented)
			default:
				return nil, ErrUnsupportedDatabaseTopology
			}
			return nil, ErrUnsupportedDatabaseTopology
		}),
		hatchetLib.WithParents(validateInputTask),
		hatchetLib.WithSkipIf(condition.ParentCondition(
			validateInputTask,
			"output.connection_string != \"\"",
		)),
	)

	provisionStroppyTask := workflow.NewTask(
		TestRunProvisionStroppyWorkflowName,
		hatchet_ext.Ptask(validateInputTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_StroppyTest_Input,
			parentOutput *hatchet.Tasks_StroppyTest_Input,
		) (*hatchet.Tasks_StroppyTest_Output, error) {

			return nil, nil
		}),
		hatchetLib.WithParents(validateInputTask),
	)

	runStroppyTask := workflow.NewTask(
		TestRunRunStroppyWorkflowName,
		hatchet_ext.Ptask(provisionStroppyTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_StroppyTest_Input,
			parentOutput *hatchet.Tasks_StroppyTest_Output,
		) (*hatchet.Tasks_StroppyTest_Output, error) {
			return nil, nil
		}),
		hatchetLib.WithParents(provisionStroppyTask, provisionDatabaseTask),
	)

	return workflow
}
