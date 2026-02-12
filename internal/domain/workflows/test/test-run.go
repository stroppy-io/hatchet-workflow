package test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/install"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/provision"
	stroppy2 "github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/stroppy"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"
)

const (
	RunWorkflowName hatchet_ext.WorkflowName = "stroppy-test-run"

	ValidateInputTaskName    hatchet_ext.TaskName = "validate-input"
	BuildWorkersTaskName     hatchet_ext.TaskName = "build-workers"
	ProvisionWorkersTaskName hatchet_ext.TaskName = "provision-workers"
	InstallSoftwareTaskName  hatchet_ext.TaskName = "install-software"
	RunStroppyTaskName       hatchet_ext.TaskName = "run-stroppy"
)

func TestRunWorkflow(
	c *hatchetLib.Client,
) *hatchetLib.Workflow {
	workflow := c.NewWorkflow(
		RunWorkflowName,
		hatchetLib.WithWorkflowDescription("Stroppy Test Run Workflow"),
	)
	/*
		Validate input
	*/
	validateInputTask := workflow.NewTask(
		ValidateInputTaskName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
		) (*hatchet.Workflows_StroppyTest_Input, error) {
			err := input.Validate()
			if err != nil {
				return nil, err
			}
			return input, nil
		}),
	)

	/*
		Build workers by requested test
	*/
	buildWorkersTask := workflow.NewTask(
		BuildWorkersTaskName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
		) (*hatchet.EdgeWorkersSet, error) {
			return stroppy2.NewTestWorkers(
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
		ProvisionWorkersTaskName,
		hatchet_ext.PTask(buildWorkersTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *hatchet.EdgeWorkersSet,
		) (*hatchet.DeployedEdgeWorkersSet, error) {
			wf, err := provision.ProvisionWorkflow(c)
			if err != nil {
				return nil, err
			}
			wfResult, err := wf.Run(ctx, &workflows.Workflows_Provision_Input{
				Common:         input.GetCommon(),
				EdgeWorkersSet: parentOutput,
			})
			if err != nil {
				return nil, err
			}
			var provisionOutput *workflows.Workflows_Provision_Output
			if err := wfResult.TaskOutput(provision.WaitWorkerInHatchet).Into(&provisionOutput); err != nil {
				return nil, fmt.Errorf("failed to get %s output: %w", provision.WaitWorkerInHatchet, err)
			}
			return provisionOutput.GetDeployedEdgeWorkers(), nil
		}),
		hatchetLib.WithParents(buildWorkersTask),
	)

	/*
		Install software on edge workers
	*/
	installSoftwareTask := workflow.NewTask(
		InstallSoftwareTaskName,
		hatchet_ext.PTask(provisionWorkersTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *hatchet.DeployedEdgeWorkersSet,
		) (*hatchet.DeployedEdgeWorkersSet, error) {
			// Filter workers with software to skip workers without software
			workersWithSoftware := lo.Filter(parentOutput.GetDeployedEdgeWorkers(),
				func(w *hatchet.DeployedEdgeWorker, _ int) bool {
					return len(w.GetWorker().GetSoftware()) > 0
				})
			installPool := pool.New().
				WithContext(ctx.GetContext()).WithFailFast().
				WithCancelOnError().
				WithFirstError()
			for _, worker := range workersWithSoftware {
				installSoftwareTasks := lo.Filter(worker.GetWorker().GetAcceptableTasks(),
					func(t *hatchet.EdgeTasks_Identifier, _ int) bool {
						return t.GetKind() == hatchet.EdgeTasks_SETUP_SOFTWARE
					})
				installPool.Go(func(ctx context.Context) error {
					for _, task := range installSoftwareTasks {
						_, err := edge.InstallSoftwareTask(c, task).
							Run(ctx,
								hatchet.EdgeTasks_InstallSoftware_Input{
									Common:   input.GetCommon(),
									Software: worker.GetWorker().GetSoftware(),
								},
							)
						if err != nil {
							return fmt.Errorf("failed to run install software task: %w", err)
						}

					}
					return nil
				})

			}
			return parentOutput, installPool.Wait()
		}),
		hatchetLib.WithExecutionTimeout(10*time.Minute),
		hatchetLib.WithParents(provisionWorkersTask),
	)

	/*
		Run Stroppy Test
	*/
	_ = workflow.NewTask(
		RunStroppyTaskName,
		hatchet_ext.PTask(installSoftwareTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_StroppyTest_Input,
			parentOutput *hatchet.DeployedEdgeWorkersSet,
		) (*hatchet.Workflows_StroppyTest_Output, error) {
			postgresWorker, err := stroppy2.SelectDeployedEdgeWorker(parentOutput.GetDeployedEdgeWorkers(), map[string]string{
				stroppy2.MetadataRoleKey: stroppy2.WorkerRoleDatabase,
				stroppy2.MetadataTypeKey: stroppy2.WorkerTypePostgresMaster,
			})
			if err != nil {
				return nil, err
			}
			masterIp := postgresWorker.Deployment.Template.GetInternalIp()
			if masterIp == nil || masterIp.GetValue() == "" {
				return nil, fmt.Errorf("postgres worker internal ip is empty")
			}

			stroppyWorker, err := stroppy2.SelectDeployedEdgeWorker(parentOutput.GetDeployedEdgeWorkers(), map[string]string{
				stroppy2.MetadataRoleKey: stroppy2.WorkerRoleStroppy,
			})
			if err != nil {
				return nil, err
			}
			stroppyTask, err := stroppy2.GetWorkerTask(stroppyWorker, hatchet.EdgeTasks_RUN_STROPPY)
			if err != nil {
				return nil, err
			}
			runStroppyResult, err := edge.RunStroppyTask(c, stroppyTask).
				Run(ctx,
					hatchet.EdgeTasks_RunStroppy_Input{
						Common: input.GetCommon(),
						StroppyCliCall: &stroppy.StroppyCli{
							Version: input.GetTest().GetStroppyCli().GetVersion(),
							BinaryPath: defaults.StringPtrOrDefaultPtr(
								input.GetTest().GetStroppyCli().BinaryPath,
								stroppy2.DefaultStroppyInstallPath,
							),
							Workdir: defaults.StringPtrOrDefaultPtr(
								input.GetTest().GetStroppyCli().Workdir,
								filepath.Join(stroppy2.DefaultOptStroppyWorkdir, input.GetCommon().GetRunId()),
							),
							Workload:   input.GetTest().GetStroppyCli().GetWorkload(),
							StroppyEnv: input.GetTest().GetStroppyCli().GetStroppyEnv(),
							ConnectionString: install.PostgresConnectionString(
								input.GetTest().GetDatabase().GetStandalone().GetPostgres(),
								masterIp.GetValue(),
							),
						},
					},
				)
			if err != nil {
				return nil, fmt.Errorf("failed to run stroppy task: %w", err)
			}
			var runStroppyOutput *hatchet.EdgeTasks_RunStroppy_Output
			if err := runStroppyResult.Into(&runStroppyOutput); err != nil {
				return nil, fmt.Errorf("failed to get stroppy output: %w", err)
			}
			// how we do not use runStroppyOutput for simplification
			return &hatchet.Workflows_StroppyTest_Output{
				Result: &stroppy.TestResult{
					RunId: input.GetCommon().GetRunId(),
					Test:  input.GetTest(),
					GrafanaUrl: lo.ToPtr(fmt.Sprintf(
						"http://some-grafana-url?runId=%s",
						input.GetCommon().GetRunId(),
					)),
				},
			}, nil
		}),
		hatchetLib.WithParents(installSoftwareTask),
		hatchetLib.WithExecutionTimeout(1*time.Hour),
	)

	/*
		Destroy deployments on failure (if provision succeeded)
	*/
	workflow.OnFailure(func(
		ctx hatchetLib.Context,
		input hatchet.Workflows_StroppyTest_Input,
	) (provision.FailureHandlerOutput, error) {
		stepErrors := ctx.StepRunErrors()
		var errorDetails string
		for stepName, errorMsg := range stepErrors {
			ctx.Log(fmt.Sprintf("Multi-step: Step '%s' failed with error: %s", stepName, errorMsg))
			errorDetails += stepName + ": " + errorMsg + "; "
		}
		retErr := func(handled bool, err error) (provision.FailureHandlerOutput, error) {
			return provision.FailureHandlerOutput{
				FailureHandled: handled,
				ErrorDetails:   "Failed to handle deployments: " + err.Error(),
			}, nil
		}

		var provisionOutput *hatchet.DeployedEdgeWorkersSet
		if err := ctx.StepOutput(ProvisionWorkersTaskName, &provisionOutput); err != nil {
			return retErr(false, fmt.Errorf("failed to get %s output: %w", ProvisionWorkersTaskName, err))
		}
		if provisionOutput == nil || provisionOutput.GetDeployment() == nil {
			return retErr(false, fmt.Errorf("provision output is empty"))
		}

		deps, err := provision.NewProvisionDeps()
		if err != nil {
			return retErr(false, err)
		}
		if err := deps.FallbackDestroyDeployment(ctx, input.GetCommon().GetSupportedCloud(), provisionOutput.GetDeployment()); err != nil {
			return retErr(false, err)
		}
		if errorDetails != "" {
			return retErr(true, fmt.Errorf("original failure: %s", errorDetails))
		}
		return provision.FailureHandlerOutput{
			FailureHandled: true,
			ErrorDetails:   "",
		}, nil
	})

	return workflow
}
