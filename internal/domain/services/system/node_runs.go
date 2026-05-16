package system

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// ListNodeRunsByDagRun returns all non-deleted NodeRuns for a given DagRun.
func (s *Service) ListNodeRunsByDagRun(ctx context.Context, id *systempb.DagRunId) ([]*systempb.NodeRun, error) {
	return s.nodeRunRepo.Query(ctx,
		systempb.NodeRuns.SelectAll().Where(
			systempb.NodeRuns.DagRunId.Eq(id.GetValue()),
			systempb.NodeRuns.DeletedAt.IsNull(),
		),
	)
}

// GetNodeRun fetches a single NodeRun by ID. Returns domainerr.NotFound when the
// row does not exist or has been soft-deleted.
func (s *Service) GetNodeRun(ctx context.Context, id *systempb.NodeRunId) (*systempb.NodeRun, error) {
	n, err := s.nodeRunRepo.QueryRow(ctx,
		systempb.NodeRuns.SelectAll().Where(
			systempb.NodeRuns.Id.Eq(id.GetValue()),
			systempb.NodeRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("node_run", id.GetValue()))
		}
		return nil, err
	}
	return n, nil
}

// MarkNodeRunSucceeded transitions a NodeRun to DONE, records finished_at and
// output, then unblocks downstream nodes and finalizes the DagRun if terminal.
func (s *Service) MarkNodeRunSucceeded(ctx context.Context, id *systempb.NodeRunId, output *anypb.Any) error {
	node, err := s.GetNodeRun(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now()
	node.Status = systempb.NodeRunStatus_NODE_RUN_STATUS_SUCCEEDED
	node.FinishedAt = timestamppb.New(now)
	node.Output = output
	touchTimestamps(node, now)

	scanner := node.IntoPlain()
	if _, err := s.nodeRunRepo.Execute(ctx,
		systempb.NodeRuns.Update().
			Set(
				scanner.GetSetter(systempb.NodeRunColumnStatus)(),
				scanner.GetSetter(systempb.NodeRunColumnFinishedAt)(),
				scanner.GetSetter(systempb.NodeRunColumnOutput)(),
				scanner.GetSetter(systempb.NodeRunColumnUpdatedAt)(),
			).
			Where(
				systempb.NodeRuns.Id.Eq(id.GetValue()),
			),
	); err != nil {
		return err
	}

	return s.recomputeAfterNode(ctx, node)
}

// MarkNodeRunFailed handles failure of a NodeRun. If the node has remaining
// attempts (per its Dag_Node.MaxAttempts) it is reset to READY for retry;
// otherwise it is set to FAILED. Downstream recompute runs only on terminal
// FAILED.
func (s *Service) MarkNodeRunFailed(ctx context.Context, id *systempb.NodeRunId, errMsg string) error {
	node, err := s.GetNodeRun(ctx, id)
	if err != nil {
		return err
	}

	// Resolve parent DagRun → Dag → Dag_Node to read MaxAttempts.
	dagRun, err := s.GetDagRun(ctx, node.GetDagRunId())
	if err != nil {
		return err
	}

	dag, err := s.GetDag(ctx, dagRun.GetDagId())
	if err != nil {
		return err
	}

	var maxAttempts uint32
	for _, n := range dag.GetGraph().GetNodes() {
		if n.GetId() == node.NodeId {
			maxAttempts = n.GetMaxAttempts()
			break
		}
	}

	newAttempt := node.Attempt + 1
	now := time.Now()

	if maxAttempts > 0 && newAttempt < maxAttempts {
		// Retry: reset to READY, bump attempt, clear started_at / error.
		node.Status = systempb.NodeRunStatus_NODE_RUN_STATUS_READY
		node.Attempt = newAttempt
		node.Error = ""
		node.StartedAt = nil
		touchTimestamps(node, now)

		scanner := node.IntoPlain()
		if _, err := s.nodeRunRepo.Execute(ctx,
			systempb.NodeRuns.Update().
				Set(
					scanner.GetSetter(systempb.NodeRunColumnStatus)(),
					scanner.GetSetter(systempb.NodeRunColumnAttempt)(),
					scanner.GetSetter(systempb.NodeRunColumnError)(),
					scanner.GetSetter(systempb.NodeRunColumnStartedAt)(),
					scanner.GetSetter(systempb.NodeRunColumnUpdatedAt)(),
				).
				Where(
					systempb.NodeRuns.Id.Eq(id.GetValue()),
				),
		); err != nil {
			return err
		}
		return nil
	}

	// Terminal failure.
	node.Status = systempb.NodeRunStatus_NODE_RUN_STATUS_FAILED
	node.Error = errMsg
	node.FinishedAt = timestamppb.New(now)
	touchTimestamps(node, now)

	scanner := node.IntoPlain()
	if _, err := s.nodeRunRepo.Execute(ctx,
		systempb.NodeRuns.Update().
			Set(
				scanner.GetSetter(systempb.NodeRunColumnStatus)(),
				scanner.GetSetter(systempb.NodeRunColumnError)(),
				scanner.GetSetter(systempb.NodeRunColumnFinishedAt)(),
				scanner.GetSetter(systempb.NodeRunColumnUpdatedAt)(),
			).
			Where(
				systempb.NodeRuns.Id.Eq(id.GetValue()),
			),
	); err != nil {
		return err
	}

	return s.recomputeAfterNode(ctx, node)
}

// recomputeAfterNode unblocks PENDING_DEPS siblings whose deps are now all
// DONE/SKIPPED, then finalizes the DagRun if all nodes are in terminal states.
func (s *Service) recomputeAfterNode(ctx context.Context, completed *systempb.NodeRun) error {
	siblings, err := s.ListNodeRunsByDagRun(ctx, completed.GetDagRunId())
	if err != nil {
		return err
	}

	dagRun, err := s.GetDagRun(ctx, completed.GetDagRunId())
	if err != nil {
		return err
	}

	dag, err := s.GetDag(ctx, dagRun.GetDagId())
	if err != nil {
		return err
	}

	// Build lookup maps.
	nodeById := make(map[string]*systempb.Dag_Node, len(dag.GetGraph().GetNodes()))
	for _, n := range dag.GetGraph().GetNodes() {
		nodeById[n.GetId()] = n
	}

	runByNodeId := make(map[string]*systempb.NodeRun, len(siblings))
	for _, r := range siblings {
		runByNodeId[r.NodeId] = r
	}

	now := time.Now()

	// Unblock PENDING_DEPS siblings whose all deps are satisfied.
	for _, sibling := range siblings {
		if sibling.Status != systempb.NodeRunStatus_NODE_RUN_STATUS_PENDING_DEPS {
			continue
		}
		dagNode, ok := nodeById[sibling.NodeId]
		if !ok {
			continue
		}
		allSatisfied := true
		for _, depId := range dagNode.GetDeps() {
			depRun, exists := runByNodeId[depId]
			if !exists {
				allSatisfied = false
				break
			}
			if depRun.Status != systempb.NodeRunStatus_NODE_RUN_STATUS_SUCCEEDED &&
				depRun.Status != systempb.NodeRunStatus_NODE_RUN_STATUS_SKIPPED {
				allSatisfied = false
				break
			}
		}
		if !allSatisfied {
			continue
		}
		sibling.Status = systempb.NodeRunStatus_NODE_RUN_STATUS_READY
		touchTimestamps(sibling, now)
		sc := sibling.IntoPlain()
		if _, err := s.nodeRunRepo.Execute(ctx,
			systempb.NodeRuns.Update().
				Set(
					sc.GetSetter(systempb.NodeRunColumnStatus)(),
					sc.GetSetter(systempb.NodeRunColumnUpdatedAt)(),
				).
				Where(
					systempb.NodeRuns.Id.Eq(sibling.GetId().GetValue()),
				),
		); err != nil {
			return err
		}
	}

	// Re-fetch siblings to reflect the updates we just made.
	siblings, err = s.ListNodeRunsByDagRun(ctx, completed.GetDagRunId())
	if err != nil {
		return err
	}

	allDone, anyFailed := terminalAggregate(siblings)
	if !allDone {
		return nil
	}

	var newStatus systempb.DagRunStatus
	if anyFailed {
		newStatus = systempb.DagRunStatus_DAG_RUN_STATUS_FAILED
	} else {
		newStatus = systempb.DagRunStatus_DAG_RUN_STATUS_SUCCEEDED
	}

	dagRun.Status = newStatus
	dagRun.FinishedAt = timestamppb.New(now)
	if dagRun.Timestamps == nil {
		dagRun.Timestamps = &commonpb.Timestamps{}
	}
	dagRun.Timestamps.UpdatedAt = timestamppb.New(now)
	dagRunScanner := dagRun.IntoPlain()
	if _, err := s.dagRunRepo.Execute(ctx,
		systempb.DagRuns.Update().
			Set(
				dagRunScanner.GetSetter(systempb.DagRunColumnStatus)(),
				dagRunScanner.GetSetter(systempb.DagRunColumnFinishedAt)(),
				dagRunScanner.GetSetter(systempb.DagRunColumnUpdatedAt)(),
			).
			Where(
				systempb.DagRuns.Id.Eq(dagRun.GetId().GetValue()),
			),
	); err != nil {
		return err
	}

	return nil
}

// touchTimestamps sets UpdatedAt on a NodeRun's Timestamps to t.
// It allocates the Timestamps struct if it is nil.
func touchTimestamps(node *systempb.NodeRun, t time.Time) {
	if node.Timestamps == nil {
		node.Timestamps = &commonpb.Timestamps{}
	}
	node.Timestamps.UpdatedAt = timestamppb.New(t)
}

// terminalAggregate reports whether all siblings are in a terminal state and
// whether any of them failed or were cancelled.
func terminalAggregate(siblings []*systempb.NodeRun) (allDone bool, anyFailed bool) {
	for _, s := range siblings {
		switch s.Status {
		case systempb.NodeRunStatus_NODE_RUN_STATUS_SUCCEEDED,
			systempb.NodeRunStatus_NODE_RUN_STATUS_SKIPPED:
			// terminal, non-failure
		case systempb.NodeRunStatus_NODE_RUN_STATUS_FAILED,
			systempb.NodeRunStatus_NODE_RUN_STATUS_CANCELLED:
			anyFailed = true
		default:
			// still running / pending / ready
			return false, false
		}
	}
	return true, anyFailed
}
