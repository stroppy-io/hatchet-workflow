package setup_database

import (
	"time"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/provisioning"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

const (
	WorkflowName                          = "setup-database"
	BuildDatabaseProvisionRequestTaskName = "build-database-provision-request"
	ProvisionDatabaseTaskName             = "provision-database-deployment"
	InstallDatabaseTaskName               = "install-database"
)

func SetupDatabaseWorkflow(
	runId string,
	c *hatchetLib.Client,
) *hatchetLib.Workflow {
	workflow := c.NewWorkflow(
		WorkflowName,
		hatchetLib.WithWorkflowDescription("Setup Database Workflow"),
	)

	buildDeploymentTask := workflow.NewTask(
		BuildDatabaseProvisionRequestTaskName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_SetupDatabase_Input,
		) (*hatchet.Tasks_Provision_Input, error) {
			ctx.Log("Building Database Deployment")
			err := input.Validate()
			if err != nil {
				return nil, err
			}
			provisionInput, err := buildDatabaseProvisionInput(input)
			if err != nil {
				return nil, err
			}
			return provisionInput, nil
		}),
	)

	provisionDatabaseTask := workflow.NewTask(
		ProvisionDatabaseTaskName,
		hatchet_ext.Ptask(buildDeploymentTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_SetupDatabase_Input,
			parentOutput *hatchet.Tasks_Provision_Input,
		) (*hatchet.Tasks_SetupDatabase_Output, error) {
			ctx.Log("Creating Database Deployment")
			provisionWf, err := provisioning.ProvisionWorkflow(c)
			if err != nil {
				return nil, err
			}
			provisionRunRes, err := provisionWf.Run(ctx, &hatchet.Tasks_Provision_Input{
				RunId: runId,
			})
			if err != nil {
				return nil, err
			}
			var provisionOutput *hatchet.Tasks_Provision_Output
			if err := provisionRunRes.TaskOutput(provisioning.WaitWorkerInHatchet).Into(&provisionOutput); err != nil {
				return nil, err
			}
			return &hatchet.Tasks_SetupDatabase_Output{
				DeploymentSet: provisionOutput.GetDeploymentSet(),
			}, nil
		}),
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(buildDeploymentTask),
	)
	/*
		This task start installing database on the provisioned deployment worker.
		That means worker on the deployment must be active in hatchet. (this insurance in ProvisionWorkflow)
	*/
	_ = workflow.NewTask(
		InstallDatabaseTaskName,
		hatchet_ext.Ptask(provisionDatabaseTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_SetupDatabase_Input,
			parentOutput *hatchet.Tasks_Provision_Output,
		) (*hatchet.Tasks_SetupDatabase_Output, error) {
			ctx.Log("Installing Database")
			return nil, nil
		}),
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(provisionDatabaseTask),
	)

	return workflow
}
