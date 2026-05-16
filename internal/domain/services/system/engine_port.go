package system

import (
	"context"
	"fmt"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
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

// SubscribeProgress subscribes to NodeRun completion events for the given
// DagRun. It returns a buffered channel that delivers *systempb.NodeRun values
// as individual nodes finish, a release function that unsubscribes, and an
// error.
//
// The since cursor is not yet implemented and is silently ignored.
func (s *Service) SubscribeProgress(ctx context.Context, dagRunID *systempb.DagRunId, _ string) (<-chan *systempb.NodeRun, func(), error) {
	// TODO(future): implement since-cursor replay from DB.
	ch := make(chan *systempb.NodeRun, 64)

	sub := s.events.Subscribe(eventing.TopicNodeRunDone, func(eventCtx context.Context, e eventing.Event) {
		payload, ok := e.Payload.(eventing.NodeRunDone)
		if !ok {
			return
		}
		if payload.DagRunID != dagRunID.GetValue() {
			return
		}
		nr, err := s.GetNodeRun(eventCtx, &systempb.NodeRunId{Value: payload.NodeRunID})
		if err != nil {
			return
		}
		select {
		case ch <- nr:
		case <-ctx.Done():
		}
	})

	release := func() {
		s.events.Unsubscribe(sub)
	}
	return ch, release, nil
}

// StreamLogs is not yet implemented. Watch streams are deferred to a later plan.
func (s *Service) StreamLogs(_ context.Context, _ *systempb.DagRunId, _ string, _ string) (<-chan *agentpb.LogLine, func(), error) {
	return nil, nil, fmt.Errorf("StreamLogs: not implemented")
}
