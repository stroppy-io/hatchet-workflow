package nodeworker

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

type stateProxy struct {
	svc      *system.Service
	dagRunID *systempb.DagRunId
}

func (p *stateProxy) Put(ctx context.Context, key string, value *anypb.Any) (*systempb.DagRunStateEntry, error) {
	return p.svc.PutState(ctx, p.dagRunID, key, value)
}
func (p *stateProxy) Get(ctx context.Context, key string) (*anypb.Any, bool, error) {
	return p.svc.GetState(ctx, p.dagRunID, key)
}

func execute(ctx context.Context, w *Worker, n *claimedNode) {
	// Fetch the full NodeRun.
	nodeRun, err := w.system.GetNodeRun(ctx, &systempb.NodeRunId{Value: n.ID})
	if err != nil {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "get node_run: "+err.Error())
		return
	}
	// Fetch parent DagRun → Dag, find Dag_Node by NodeId.
	dagRun, err := w.system.GetDagRun(ctx, &systempb.DagRunId{Value: n.DagRunID})
	if err != nil {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "get dag_run: "+err.Error())
		return
	}
	// If the parent DagRun has been cancelled, fail this node immediately.
	if dagRun.GetCancelRequested() {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "cancelled")
		return
	}
	dag, err := w.system.GetDag(ctx, dagRun.GetDagId())
	if err != nil {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "get dag: "+err.Error())
		return
	}
	var dagNode *systempb.Dag_Node
	for _, nd := range dag.GetGraph().GetNodes() {
		if nd.GetId() == n.NodeID {
			dagNode = nd
			break
		}
	}
	if dagNode == nil {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "no graph node for node_id "+n.NodeID)
		return
	}
	spec := dagNode.GetSpec()
	typeURL := ""
	if spec != nil {
		typeURL = spec.GetTypeUrl()
	}
	handler := w.registry.Resolve(typeURL)
	if handler == nil {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "no handler registered for spec "+typeURL)
		return
	}

	timeoutCtx := ctx
	if dagNode.GetTimeoutSeconds() > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, time.Duration(dagNode.GetTimeoutSeconds())*time.Second)
		defer cancel()
	}

	out, err := handler.Execute(timeoutCtx, nodeRun, spec, &stateProxy{svc: w.system, dagRunID: &systempb.DagRunId{Value: n.DagRunID}})
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "timeout exceeded"
		}
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, msg)
		return
	}
	_ = w.system.MarkNodeRunSucceeded(ctx, &systempb.NodeRunId{Value: n.ID}, out)
}
