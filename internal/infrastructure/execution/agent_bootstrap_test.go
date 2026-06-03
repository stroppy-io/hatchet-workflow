package execution

import (
	"testing"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func TestAttachAgentTokensIssuesPerNodeTokensAndSecretQueues(t *testing.T) {
	issuer, err := agentdomain.NewTokenService("secret")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}
	boot, err := attachAgentTokens(&workflowpb.AgentBootstrap{ServerAddr: "http://server"}, issuer, "tenant-1", "run-1", &deploymentpb.InfrastructurePlan{
		Machines: []*deploymentpb.MachinePlan{
			{NodeId: "node-1"},
			{NodeId: "node-2"},
		},
	})
	if err != nil {
		t.Fatalf("attach tokens: %v", err)
	}

	for _, nodeID := range []string{"node-1", "node-2"} {
		token := boot.GetAgentTokens()[nodeID]
		queue := boot.GetAgentTaskQueues()[nodeID]
		if token == "" || queue == "" {
			t.Fatalf("node %s token/queue missing: %#v %#v", nodeID, boot.GetAgentTokens(), boot.GetAgentTaskQueues())
		}
		if queue == agentdomain.TaskQueue(nodeID) {
			t.Fatalf("node %s got guessable task queue %q", nodeID, queue)
		}
		claims, err := issuer.VerifyAgentToken(token)
		if err != nil {
			t.Fatalf("verify node %s token: %v", nodeID, err)
		}
		if claims.RunID != "run-1" || claims.MachineID != nodeID || claims.TaskQueue != queue {
			t.Fatalf("node %s claims = %#v, queue %q", nodeID, claims, queue)
		}
	}
}
