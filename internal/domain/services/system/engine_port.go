package system

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

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

// StreamLogs first back-fills from node_run_logs then subscribes the LogBus
// for live lines. stepID, when non-empty, filters by node_run_id. The since
// cursor is not yet implemented.
func (s *Service) StreamLogs(ctx context.Context, dagRunID *systempb.DagRunId, stepID string, _ string) (<-chan *agentpb.LogLine, func(), error) {
	out := make(chan *agentpb.LogLine, 256)
	busCh, release := s.logBus.Subscribe(dagRunID.GetValue())

	go func() {
		defer close(out)
		// Backfill from DB.
		rows, err := s.logRepo.Query(ctx,
			systempb.NodeRunLogs.SelectAll().Where(
				systempb.NodeRunLogs.DagRunId.Eq(dagRunID.GetValue()),
			),
		)
		if err == nil {
			for _, r := range rows {
				if stepID != "" && r.GetNodeRunId() != stepID {
					continue
				}
				line := &agentpb.LogLine{
					CommandId: r.GetCommandId(),
					Ts:        r.GetTs(),
					Line:      r.GetLine(),
				}
				select {
				case out <- line:
				case <-ctx.Done():
					return
				}
			}
		}
		// Live.
		for {
			select {
			case <-ctx.Done():
				return
			case l, ok := <-busCh:
				if !ok {
					return
				}
				if stepID != "" && l.NodeRunID != stepID {
					continue
				}
				select {
				case out <- &agentpb.LogLine{
					CommandId: l.CommandID,
					Ts:        timestamppb.New(l.Timestamp),
					Line:      l.Line,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, release, nil
}
