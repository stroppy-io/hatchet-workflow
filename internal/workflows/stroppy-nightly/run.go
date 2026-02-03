package stroppy_nightly

import (
	"fmt"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install"
)

func NightlyCloudStroppyRunPostgresTask(runId string, c *hatchetLib.Client) *hatchetLib.StandaloneTask {
	return c.NewStandaloneTask(
		fmt.Sprintf("run-postgres-%s", runId),
		func(ctx hatchetLib.Context, input hatchet.InstallPostgresParams) (hatchet.InstallPostgresParams, error) {
			installer := install.New(install.DefaultConfig())
			if err := installer.InstallPostgres(ctx, &input); err != nil {
				return hatchet.InstallPostgresParams{}, err
			}
			return input, nil
		},
	)
}

func stroppyInstallTaskName(runId string) string {
	return fmt.Sprintf("install-stroppy-%s", runId)
}

func stroppyRunTaskName(runId string) string {
	return fmt.Sprintf("run-stroppy-%s", runId)
}

func NightlyCloudStroppyRunWorkflow(
	runId string,
	c *hatchetLib.Client,
) *hatchetLib.Workflow {
	w := c.NewWorkflow(
		fmt.Sprintf("nightly-cloud-stroppy-run-%s", runId),
		hatchetLib.WithWorkflowDescription("Nightly Cloud Stroppy Run Workflow"),
	)
	installStroppyTask := w.NewTask(
		stroppyInstallTaskName(runId),
		func(ctx hatchetLib.Context, input hatchet.RunStroppyParams) (hatchet.RunStroppyParams, error) {
			installer := install.New(install.DefaultConfig())
			if err := installer.InstallStroppy(ctx, &input); err != nil {
				return hatchet.RunStroppyParams{}, err
			}
			return input, nil
		},
	)
	w.NewTask(
		stroppyRunTaskName(runId),
		func(ctx hatchetLib.Context, input hatchet.RunStroppyParams) (hatchet.RunStroppyResponse, error) {
			ctx.Log(fmt.Sprintf("Running Stroppy, Postgres URL: %s", input.GetConnectionString()))
			return hatchet.RunStroppyResponse{
				Output:       "Stroppy output",
				WorkloadName: input.GetWorkloadName(),
				GrafanaUrl:   fmt.Sprintf("http://grafana.com/runId=%s", runId),
			}, nil
		},
		hatchetLib.WithParents(installStroppyTask),
	)
	return w
}
