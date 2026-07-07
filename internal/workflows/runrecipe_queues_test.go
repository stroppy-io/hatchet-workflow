package workflows

import (
	"testing"

	"github.com/stretchr/testify/require"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// TestBootstrapWithAgentQueues locks the fix for "agent task queue for node
// ... is missing": every provisioned node must land in bootstrap.
// AgentTaskQueues under its deterministic TaskQueue(nodeID), the same string
// the docker provider renders into the agent's AGENT_TASK_QUEUE env.
func TestBootstrapWithAgentQueues(t *testing.T) {
	bootstrap := &workflowpb.AgentBootstrap{ServerAddr: "http://gateway:8080"}
	machines := map[string][]*deploymentpb.MachineState{
		"db":     {{NodeId: "db-0"}, {NodeId: "db-1"}},
		"runner": {{NodeId: "runner-0"}},
	}

	out := bootstrapWithAgentQueues(bootstrap, machines)

	require.Equal(t, agentdomain.TaskQueue("db-0"), out.GetAgentTaskQueues()["db-0"])
	require.Equal(t, agentdomain.TaskQueue("db-1"), out.GetAgentTaskQueues()["db-1"])
	require.Equal(t, agentdomain.TaskQueue("runner-0"), out.GetAgentTaskQueues()["runner-0"])
	// original is not mutated (clone semantics)
	require.Empty(t, bootstrap.GetAgentTaskQueues())
}

// TestBootstrapWithAgentQueues_NilAndEmpty guards the no-op branches.
func TestBootstrapWithAgentQueues_NilAndEmpty(t *testing.T) {
	require.Nil(t, bootstrapWithAgentQueues(nil, map[string][]*deploymentpb.MachineState{"db": {{NodeId: "db-0"}}}))

	bootstrap := &workflowpb.AgentBootstrap{ServerAddr: "x"}
	require.Same(t, bootstrap, bootstrapWithAgentQueues(bootstrap, nil))
}
