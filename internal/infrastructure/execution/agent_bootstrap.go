package execution

import (
	"fmt"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/proto"
)

type AgentTokenIssuer interface {
	IssueAgentToken(tenantID, runID, machineID, taskQueue string) (string, error)
}

type AgentTokenVerifier interface {
	VerifyAgentToken(token string) (*agentdomain.TokenClaims, error)
}

func attachAgentTokens(bootstrap *workflowpb.AgentBootstrap, issuer AgentTokenIssuer, tenantID, runID string, plan *deploymentpb.InfrastructurePlan) (*workflowpb.AgentBootstrap, error) {
	if bootstrap == nil || issuer == nil || plan == nil {
		return bootstrap, nil
	}
	out := proto.Clone(bootstrap).(*workflowpb.AgentBootstrap)
	if out.AgentTokens == nil {
		out.AgentTokens = make(map[string]string, len(plan.GetMachines()))
	}
	if out.AgentTaskQueues == nil {
		out.AgentTaskQueues = make(map[string]string, len(plan.GetMachines()))
	}
	for _, machine := range plan.GetMachines() {
		nodeID := machine.GetNodeId()
		if nodeID == "" {
			return nil, fmt.Errorf("agent token: run %q has machine without node_id", runID)
		}
		taskQueue := agentdomain.NewTaskQueue(nodeID)
		token, err := issuer.IssueAgentToken(tenantID, runID, nodeID, taskQueue)
		if err != nil {
			return nil, fmt.Errorf("issue agent token for run %q node %q: %w", runID, nodeID, err)
		}
		out.AgentTokens[nodeID] = token
		out.AgentTaskQueues[nodeID] = taskQueue
	}
	return out, nil
}
