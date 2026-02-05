package stroppy

import (
	"fmt"

	"github.com/hatchet-dev/hatchet/pkg/worker/condition"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/test"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

const (
	TestRunWorkflowName                 = "stroppy-test-run"
	TestRunValidateInputWorkflowName    = "stroppy-test-run-validate-input"
	TestRunBuildWorkersWorkflowName     = "stroppy-test-run-build-workers"
	TestRunProvisionWorkersWorkflowName = "stroppy-test-run-provision-workers"
	TestRunInstallSoftwareWorkflowName  = "stroppy-test-run-install-software"
	TestRunStroppyWorkflowName          = "stroppy-test-run-stroppy"
)

func TestRunWorkflow(
	c *hatchetLib.Client,
) *hatchetLib.Workflow {
	workflow := c.NewWorkflow(
		TestRunWorkflowName,
		hatchetLib.WithWorkflowDescription("Stroppy Test Run Workflow"),
	)
	/*
		Validate input
	*/
	validateInputTask := workflow.NewTask(
		TestRunValidateInputWorkflowName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_StroppyTest_Input,
		) (*hatchet.Workflows_StroppyTest_Input, error) {
			err := input.Validate()
			if err != nil {
				return nil, err
			}
			return input, nil
		}),
		hatchetLib.WithSkipIf(condition.Conditions()),
	)

	/*
		Build workers by requested test
	*/
	buildWorkersTask := workflow.NewTask(
		TestRunBuildWorkersWorkflowName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_StroppyTest_Input,
		) (*hatchet.EdgeWorkersSet, error) {
			return test.NewTestWorkers(
				ids.ParseRunId(input.GetCommon().GetRunId()),
				input.GetTest(),
			)
		}),
		hatchetLib.WithParents(validateInputTask),
	)

	/*
		Provision workers
	*/
	provisionWorkersTask := workflow.NewTask(
		TestRunProvisionWorkersWorkflowName,
		hatchet_ext.PTask(buildWorkersTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_StroppyTest_Input,
			parentOutput *hatchet.EdgeWorkersSet,
		) (*hatchet.DeployedEdgeWorkersSet, error) {
			wf, err := provision.ProvisionWorkflow(c)
			if err != nil {
				return nil, err
			}
			wfResult, err := wf.Run(ctx, &hatchet.Workflows_Provision_Input{
				Common:         input.GetCommon(),
				EdgeWorkersSet: parentOutput,
			})
			if err != nil {
				return nil, err
			}
			var provisionOutput *hatchet.Workflows_Provision_Output
			if err := wfResult.TaskOutput(provision.WaitWorkerInHatchet).Into(&provisionOutput); err != nil {
				return nil, fmt.Errorf("failed to get %s output: %w", provision.WaitWorkerInHatchet, err)
			}
			return provisionOutput.GetDeployedEdgeWorkers(), nil
		}),
		hatchetLib.WithParents(validateInputTask),
	)

	/*
		Install software on edge workers
	*/
	installSoftwareTask := workflow.NewTask(
		TestRunInstallSoftwareWorkflowName,
		hatchet_ext.PTask(provisionWorkersTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_StroppyTest_Input,
			parentOutput *hatchet.DeployedEdgeWorkersSet,
		) (*hatchet.DeployedEdgeWorker, error) {
			return nil, nil
		}),
		hatchetLib.WithParents(provisionWorkersTask),
	)

	/*
		Run Stroppy Test
	*/
	runStroppyTask := workflow.NewTask(
		TestRunStroppyWorkflowName,
		hatchet_ext.PTask(installSoftwareTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_StroppyTest_Input,
			parentOutput *hatchet.DeployedEdgeWorkersSet,
		) (*stroppy.TestResult, error) {
			return nil, nil
		}),
		hatchetLib.WithParents(installSoftwareTask, provisionStroppyTask),
	)

	return workflow
}
