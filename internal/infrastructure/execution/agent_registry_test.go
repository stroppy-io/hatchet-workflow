package execution

import (
	"context"
	"testing"
	"time"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

func TestAgentRegistryRegistersPresenceAndClassifiesStale(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	registry := NewAgentRegistryService(nil)
	registry.now = func() time.Time { return now }
	registry.heartbeatInterval = 15 * time.Second

	ctx := context.WithValue(context.Background(), agentClaimsKey{}, &agentdomain.TokenClaims{
		RunID:     "run-1",
		MachineID: "node-1",
		TaskQueue: "queue-1",
	})
	resp, err := registry.Register(ctx, &agentpb.RegisterRequest{Info: &agentpb.AgentInfo{
		MachineId:    "node-1",
		Host:         "agent-host",
		AgentVersion: "test",
	}})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	if got, want := resp.GetHeartbeatIntervalSeconds(), uint32(15); got != want {
		t.Fatalf("heartbeat interval = %d, want %d", got, want)
	}

	samples, err := registry.AgentPresence(context.Background(), "run-1", []string{"node-1"})
	if err != nil {
		t.Fatalf("agent presence: %v", err)
	}
	if got := samples["node-1"].GetPresence(); got != monitor.WorkerPresence_WORKER_PRESENCE_ONLINE {
		t.Fatalf("presence = %s, want online", got)
	}
	if !samples["node-1"].GetOnline() {
		t.Fatal("online = false, want true")
	}
	if got, want := samples["node-1"].GetHost(), "agent-host"; got != want {
		t.Fatalf("host = %q, want %q", got, want)
	}

	now = now.Add(45 * time.Second)
	samples, err = registry.AgentPresence(context.Background(), "run-1", []string{"node-1"})
	if err != nil {
		t.Fatalf("agent presence after stale window: %v", err)
	}
	if got := samples["node-1"].GetPresence(); got != monitor.WorkerPresence_WORKER_PRESENCE_STALE {
		t.Fatalf("presence = %s, want stale", got)
	}
	if samples["node-1"].GetOnline() {
		t.Fatal("online = true, want false for stale sample")
	}
}
