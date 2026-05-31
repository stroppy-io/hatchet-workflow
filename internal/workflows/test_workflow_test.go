package workflows

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	"github.com/stroppy-io/stroppy-cloud/internal/deploy/dockerprov"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	gen "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// fakeInfra is a stand-in for *dockerprov.Deployer that returns canned IPs so
// the server activities can run without a real docker daemon.
type fakeInfra struct {
	deployed  bool
	tornDown  bool
	deployErr error
}

func (f *fakeInfra) Deploy(_ context.Context, _ *topology.Topology, _, _ string) (*dockerprov.Deployed, error) {
	if f.deployErr != nil {
		return nil, f.deployErr
	}
	f.deployed = true
	return &dockerprov.Deployed{
		Network: "stroppy-test",
		Instances: map[string]dockerprov.InstanceDeployment{
			"db": {ContainerID: "cid-db", IP: "10.0.0.5", Name: "stroppy-test-db"},
			"wl": {ContainerID: "cid-wl", IP: "10.0.0.6", Name: "stroppy-test-wl"},
		},
	}, nil
}

func (f *fakeInfra) Teardown(_ context.Context, _ string) error {
	f.tornDown = true
	return nil
}

func testRun() *domain.TestRun {
	return &domain.TestRun{
		Id: "run-1",
		Topology: &topology.Topology{
			Instances: []*topology.Topology_Instance{
				{Id: "db", Components: []*topology.Component{{Kind: topology.Component_KIND_DATABASE}}},
				{Id: "wl", Components: []*topology.Component{{Kind: topology.Component_KIND_WORKLOAD}}},
			},
		},
		Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES},
		Workload: &domain.Workload{},
	}
}

func TestTestWorkflow_HappyPath(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	// Worker sessions are used to pin the agent activities to one worker; the
	// test env must run a session worker for CreateSession to resolve.
	env.SetWorkerOptions(worker.Options{EnableSessionWorker: true})

	fi := &fakeInfra{}
	sa := &ServerActivities{
		Deployer:            fi,
		StroppyArtifactPath: "/artifacts/stroppy",
		StroppyChecksum:     "",
		DefaultServerAddr:   "http://server.local:8080",
	}

	// Register the real server activities on the env (they call the fake infra
	// and the pure recipe).
	env.RegisterActivity(sa.DeployActivity)
	env.RegisterActivity(sa.BuildRecipeActivity)
	env.RegisterActivity(sa.TeardownActivity)

	// Register stub agent activities under the generated names so OnActivity can
	// mock them (the real ones run on the separate agent worker / container).
	env.RegisterActivityWithOptions(
		func(ctx context.Context) error { return nil },
		activity.RegisterOptions{Name: gen.EnsureAgentOnlineActivityActivityName},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, f *common.File) error { return nil },
		activity.RegisterOptions{Name: gen.WriteFileActivityActivityName},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, c *common.Cmd) (*common.Cmd_Result, error) {
			return &common.Cmd_Result{ExitCode: 0}, nil
		},
		activity.RegisterOptions{Name: gen.CallCmdActivityActivityName},
	)
	env.RegisterActivityWithOptions(
		func(ctx context.Context, f *common.File) error { return nil },
		activity.RegisterOptions{Name: gen.FetchFileActivityActivityName},
	)

	// Mock the agent activities (which would otherwise run on the agent worker).
	env.OnActivity(gen.EnsureAgentOnlineActivityActivityName, mock.Anything).
		Return(nil)
	env.OnActivity(gen.WriteFileActivityActivityName, mock.Anything, mock.Anything).
		Return(nil)
	env.OnActivity(gen.FetchFileActivityActivityName, mock.Anything, mock.Anything).
		Return(nil)
	env.OnActivity(gen.CallCmdActivityActivityName, mock.Anything, mock.Anything).
		Return(&common.Cmd_Result{ExitCode: 0}, nil)

	client := gen.NewTestTestServiceClient(env, Workflows{}, nil)

	run, err := client.TestWorkflowAsync(context.Background(),
		&gen.TestWorkflowRequest{TestRun: testRun()},
		gen.NewTestWorkflowOptions().WithID("test-run/run-1"))
	require.NoError(t, err)

	_, err = run.Get(context.Background())
	require.NoError(t, err)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	require.True(t, fi.deployed, "deployer should have deployed")
	require.True(t, fi.tornDown, "deployer should have torn down")

	// Query the live run state: overall completed + all stages completed.
	state, err := run.GetRunState(context.Background())
	require.NoError(t, err)
	require.Equal(t, common.Status_STATUS_COMPLETED, state.GetStatus())

	// Every node (deploy, build_recipe, the granular recipe steps, teardown) must
	// be COMPLETED with timestamps.
	seen := map[string]bool{}
	for _, s := range state.GetStages() {
		seen[s.GetName()] = true
		require.Equal(t, common.Status_STATUS_COMPLETED, s.GetStatus(),
			"stage %q not completed", s.GetName())
		require.NotNil(t, s.GetStartedAt(), "stage %q missing started_at", s.GetName())
		require.NotNil(t, s.GetFinishedAt(), "stage %q missing finished_at", s.GetName())
	}
	// The pipeline must include the framing nodes AND per-step granular nodes,
	// each namespaced by the instance it ran on: the database steps on "db", the
	// workload steps on "wl" — proving routing put each component's steps on its
	// own machine.
	for _, name := range []string{
		stageDeploy, stageBuildRecipe, stageTeardown,
		"db/apt update", "db/install postgresql-16", "db/start cluster", "db/seed database",
		"wl/fetch stroppy", "wl/run stroppy",
	} {
		require.True(t, seen[name], "expected pipeline node %q", name)
	}
	// No database step ever appears under the workload instance (no misroute).
	require.False(t, seen["wl/install postgresql-16"], "postgres install must NOT run on the workload node")
	require.False(t, seen["db/run stroppy"], "stroppy must NOT run on the database node")
	// Granular: many more nodes than the old 5 coarse stages.
	require.Greater(t, len(state.GetStages()), 8, "expected granular per-step nodes")
}
