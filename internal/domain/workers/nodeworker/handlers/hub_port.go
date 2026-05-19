package handlers

import (
	"context"
	"time"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

// HubPort is implemented by agent.Hub.
type HubPort interface {
	Dispatch(ctx context.Context, agentID string, machineID string, action *agentpb.Action, timeout time.Duration) (*agentpb.Report, error)
	ResolveByMachine(dagRunID, machineID string) (string, bool)
}
