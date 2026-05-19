package system

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
)

// parseSince parses an RFC3339Nano cursor; returns the zero time on empty
// input or parse failure (callers fall back to "no filter").
func parseSince(cursor string) time.Time {
	if cursor == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, cursor)
	if err != nil {
		return time.Time{}
	}
	return t
}

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
// since is an RFC3339Nano cursor: when set, the channel is pre-filled with
// every NodeRun for this DagRun whose updated_at > since, so a reconnecting
// viewer doesn't miss completions that occurred while disconnected.
func (s *Service) SubscribeProgress(ctx context.Context, dagRunID *systempb.DagRunId, since string) (<-chan *systempb.NodeRun, func(), error) {
	ch := make(chan *systempb.NodeRun, 64)
	cursor := parseSince(since)
	if !cursor.IsZero() {
		// Replay terminal NodeRuns that completed after the cursor. The query
		// is bounded by dag_run_id so cost is O(nodes-in-this-DAG).
		rows, err := s.nodeRunRepo.Query(ctx,
			systempb.NodeRuns.SelectAll().Where(
				systempb.NodeRuns.DagRunId.Eq(dagRunID.GetValue()),
				systempb.NodeRuns.UpdatedAt.Gt(cursor),
			),
		)
		if err == nil {
			for _, nr := range rows {
				select {
				case ch <- nr:
				case <-ctx.Done():
					break
				}
			}
		}
	}

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
// for live lines. stepID, when non-empty, filters by node_run_id. since is
// an RFC3339Nano cursor; rows with ts <= since are skipped on backfill.
func (s *Service) StreamLogs(ctx context.Context, dagRunID *systempb.DagRunId, stepID string, since string) (<-chan *agentpb.LogLine, func(), error) {
	out := make(chan *agentpb.LogLine, 256)
	busCh, release := s.logBus.Subscribe(dagRunID.GetValue())
	cursor := parseSince(since)

	go func() {
		defer close(out)
		// Backfill from DB, respecting the since cursor when set.
		query := systempb.NodeRunLogs.SelectAll().Where(systempb.NodeRunLogs.DagRunId.Eq(dagRunID.GetValue()))
		if !cursor.IsZero() {
			query = systempb.NodeRunLogs.SelectAll().Where(
				systempb.NodeRunLogs.DagRunId.Eq(dagRunID.GetValue()),
				systempb.NodeRunLogs.Ts.Gt(cursor),
			)
		}
		rows, err := s.logRepo.Query(ctx, query)
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
