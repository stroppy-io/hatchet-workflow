// Package agentqueue implements agent.CommandQueue: the command queue IS the set
// of ready agent-locus Dag nodes (no separate table, H19). The executor parks a
// ready agent node as RUNNING; this queue leases it to a polling agent and
// applies the agent's Report back onto the node. Lease state lives in node
// metadata (lease_expires_at + machine). Recovery of an expired lease = the node
// becomes leasable again (replaces the orphan-reaper, H20/H21).
package agentqueue

import (
	"context"
	"time"

	"github.com/gopherex/xlog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/services/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

const (
	handlerAgentCommand = "agent.command"
	leaseMetaExpiresAt  = "agent.lease_expires_at"
	leaseMetaMachine    = "agent.lease_machine"
	// defaultLeaseTTL is how long a leased command is held before it returns to
	// the pool absent a Report. Report{RUNNING} pushes it forward.
	//
	// TODO(agentqueue): make configurable.
	defaultLeaseTTL = 60 * time.Second
)

// Store is the dag persistence the queue needs (implemented by dagstore.Store).
type Store interface {
	ListByTenant(ctx context.Context, tenantID string, statuses []primitive.Status) ([]*primitive.Dag, error)
	GetDag(ctx context.Context, id string) (*primitive.Dag, error)
	SaveDag(ctx context.Context, dag *primitive.Dag) error
}

// Queue implements agent.CommandQueue over a dag Store.
type Queue struct {
	*tracing.Entity
	store Store
	ttl   time.Duration
}

var _ agent.CommandQueue = (*Queue)(nil)

// New builds a Queue.
func New(logger *xlog.Logger, store Store) *Queue {
	return &Queue{
		Entity: tracing.NewEntity(logger.AppendName("AgentQueue")),
		store:  store,
		ttl:    defaultLeaseTTL,
	}
}

// Lease hands out the next ready command for the machine, or (nil, nil) when none.
//
// TODO(agentqueue): no machine routing yet — any ready agent command in the
// tenant is offered to the polling agent. Match Command.Target to the agent's
// machine once topology placement wires node->machine. Reported.
func (q *Queue) Lease(ctx context.Context, tenantID string, target *rtagent.Target) (*agentpb.CommandLease, error) {
	dags, err := q.store.ListByTenant(ctx, tenantID, []primitive.Status{primitive.Status_STATUS_RUNNING})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list dags: %v", err)
	}
	now := time.Now()
	for _, dag := range dags {
		node := findLeasableAgentNode(dag, now)
		if node == nil {
			continue
		}
		expires := now.Add(q.ttl)
		setLease(node, target.GetMachineId(), expires)
		if err := q.store.SaveDag(ctx, dag); err != nil {
			return nil, status.Errorf(codes.Internal, "save lease: %v", err)
		}
		cmd := &rtagent.Command{}
		if input := node.GetTaskState().GetInput(); input != nil {
			if err := input.UnmarshalTo(cmd); err != nil {
				return nil, status.Errorf(codes.Internal, "decode command: %v", err)
			}
		}
		return &agentpb.CommandLease{
			Address:        &agentpb.NodeAddress{DagId: &models.DagId{Value: dag.GetId()}, NodeExecutionId: node.GetExecutionId()},
			Command:        cmd,
			LeaseExpiresAt: timestamppb.New(expires),
		}, nil
	}
	return nil, nil
}

// Report applies a command report to the addressed node and persists the dag.
func (q *Queue) Report(ctx context.Context, tenantID string, addr *agentpb.NodeAddress, report *rtagent.Report) error {
	dag, err := q.store.GetDag(ctx, addr.GetDagId().GetValue())
	if err != nil {
		return status.Errorf(codes.Internal, "load dag: %v", err)
	}
	if dag == nil {
		return status.Error(codes.NotFound, "dag not found")
	}
	node := findByExecutionID(dag, addr.GetNodeExecutionId())
	if node == nil {
		return status.Error(codes.NotFound, "node not found")
	}

	switch report.GetStatus() {
	case rtagent.CommandStatus_COMMAND_STATUS_RUNNING:
		// keepalive — extend the lease, node stays RUNNING.
		setLease(node, leaseMachine(node), time.Now().Add(q.ttl))
	case rtagent.CommandStatus_COMMAND_STATUS_COMPLETED:
		clearLease(node)
		if res := report.GetResult(); res != nil {
			if out, err := anypb.New(res); err == nil {
				node.GetTaskState().Output = out
			}
		}
		setNodeStatus(node, primitive.Status_STATUS_COMPLETED)
	case rtagent.CommandStatus_COMMAND_STATUS_FAILED:
		clearLease(node)
		setNodeStatus(node, primitive.Status_STATUS_FAILED)
	case rtagent.CommandStatus_COMMAND_STATUS_CANCELLED:
		clearLease(node)
		setNodeStatus(node, primitive.Status_STATUS_CANCELLED)
	default:
		return status.Errorf(codes.InvalidArgument, "unexpected command status %s", report.GetStatus())
	}
	return q.store.SaveDag(ctx, dag)
}

// InvalidateLeases drops the machine's in-flight leases (new boot_id) so its
// commands return to the pool immediately.
func (q *Queue) InvalidateLeases(ctx context.Context, tenantID, machineID string) error {
	dags, err := q.store.ListByTenant(ctx, tenantID, []primitive.Status{primitive.Status_STATUS_RUNNING})
	if err != nil {
		return status.Errorf(codes.Internal, "list dags: %v", err)
	}
	for _, dag := range dags {
		changed := false
		walkNodes(dag, func(n *primitive.Dag_Node) {
			if isAgentCommand(n) && leaseMachine(n) == machineID {
				clearLease(n)
				changed = true
			}
		})
		if changed {
			if err := q.store.SaveDag(ctx, dag); err != nil {
				return status.Errorf(codes.Internal, "save dag: %v", err)
			}
		}
	}
	return nil
}

// ── node traversal + lease helpers (operate on the primitive payload) ──────────

func isAgentCommand(n *primitive.Dag_Node) bool {
	ts := n.GetTaskState()
	return ts != nil &&
		ts.GetLocus() == primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT &&
		ts.GetHandlerName() == handlerAgentCommand
}

// walkNodes visits every node in the dag, recursing into embedded sub-dags.
func walkNodes(dag *primitive.Dag, fn func(*primitive.Dag_Node)) {
	for _, n := range dag.GetNodes() {
		fn(n)
		if sub := n.GetSubDag(); sub != nil {
			walkNodes(sub, fn)
		}
	}
}

// findLeasableAgentNode returns the first RUNNING agent command without a valid
// lease (unleased or expired), recursing into sub-dags.
func findLeasableAgentNode(dag *primitive.Dag, now time.Time) *primitive.Dag_Node {
	var found *primitive.Dag_Node
	walkNodes(dag, func(n *primitive.Dag_Node) {
		if found != nil {
			return
		}
		if isAgentCommand(n) && n.GetStatus() == primitive.Status_STATUS_RUNNING && !leaseValid(n, now) {
			found = n
		}
	})
	return found
}

func findByExecutionID(dag *primitive.Dag, execID string) *primitive.Dag_Node {
	var found *primitive.Dag_Node
	walkNodes(dag, func(n *primitive.Dag_Node) {
		if found == nil && n.GetExecutionId() == execID {
			found = n
		}
	})
	return found
}

func leaseValid(n *primitive.Dag_Node, now time.Time) bool {
	raw := n.GetMetadata()[leaseMetaExpiresAt]
	if raw == "" {
		return false
	}
	expires, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	return expires.After(now)
}

func leaseMachine(n *primitive.Dag_Node) string { return n.GetMetadata()[leaseMetaMachine] }

func setLease(n *primitive.Dag_Node, machineID string, expires time.Time) {
	if n.Metadata == nil {
		n.Metadata = map[string]string{}
	}
	n.Metadata[leaseMetaExpiresAt] = expires.UTC().Format(time.RFC3339)
	n.Metadata[leaseMetaMachine] = machineID
}

func clearLease(n *primitive.Dag_Node) {
	delete(n.GetMetadata(), leaseMetaExpiresAt)
	delete(n.GetMetadata(), leaseMetaMachine)
}

func setNodeStatus(n *primitive.Dag_Node, st primitive.Status) {
	n.Status = st
	if exec := n.GetExecution(); exec != nil {
		exec.Status = st
	}
}
