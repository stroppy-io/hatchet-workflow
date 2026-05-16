package system

import (
	"context"
	"fmt"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// LaunchDag is a convenience wrapper that saves a Dag and immediately starts a
// DagRun for it. It satisfies the testing.SystemEnginePort interface.
func (s *Service) LaunchDag(ctx context.Context, dag *systempb.Dag, metadata map[string]string) (*systempb.DagRun, error) {
	saved, err := s.SaveDag(ctx, dag)
	if err != nil {
		return nil, err
	}
	return s.StartDagRun(ctx, StartDagRunInput{Dag: saved, Metadata: metadata})
}

// SubscribeProgress is not yet implemented. Watch streams are deferred to a
// later plan.
func (s *Service) SubscribeProgress(_ context.Context, _ *systempb.DagRunId, _ string) (<-chan *systempb.NodeRun, func(), error) {
	return nil, nil, fmt.Errorf("SubscribeProgress: not implemented")
}

// StreamLogs is not yet implemented. Watch streams are deferred to a later plan.
func (s *Service) StreamLogs(_ context.Context, _ *systempb.DagRunId, _ string, _ string) (<-chan *agentpb.LogLine, func(), error) {
	return nil, nil, fmt.Errorf("StreamLogs: not implemented")
}
