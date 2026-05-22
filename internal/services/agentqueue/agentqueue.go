// Package agentqueue implements agent.CommandQueue: the command queue IS the set
// of ready agent-locus Dag nodes (no separate table, H19). The executor parks a
// ready agent node as RUNNING; this queue leases it to a polling agent and
// applies the agent's Report back onto the node. Lease state lives in node
// metadata (lease_expires_at + machine). Recovery of an expired lease = the node
// becomes leasable again (replaces the orphan-reaper, H20/H21).
package agentqueue

import (
	"context"
	"strings"
	"time"

	"github.com/gopherex/xlog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
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
	// the pool absent a Report. Report{RUNNING} pushes it forward. Override with
	// WithLeaseTTL.
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

// Option configures a Queue at construction.
type Option func(*Queue)

// WithLeaseTTL overrides the command lease TTL (default defaultLeaseTTL). A
// non-positive value is ignored (keeps the default).
func WithLeaseTTL(d time.Duration) Option {
	return func(q *Queue) {
		if d > 0 {
			q.ttl = d
		}
	}
}

// New builds a Queue.
func New(logger *xlog.Logger, store Store, opts ...Option) *Queue {
	q := &Queue{
		Entity: tracing.NewEntity(logger.AppendName("AgentQueue")),
		store:  store,
		ttl:    defaultLeaseTTL,
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

// Lease hands out the next ready command for the requesting agent's machine, or
// (nil, nil) when none. Routing: a node id is "<component>.<action>"; the dag's
// component->machine map (dag.Metadata[render.ComponentMachineMetaKey]) places
// the node's component on a machine. Only nodes mapped to target.MachineId are
// offered; nodes whose component can't be mapped are skipped (never leased to
// the wrong host).
func (q *Queue) Lease(ctx context.Context, tenantID string, target *rtagent.Target) (*agentpb.CommandLease, error) {
	dags, err := q.store.ListByTenant(ctx, tenantID, []primitive.Status{primitive.Status_STATUS_RUNNING})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list dags: %v", err)
	}
	now := time.Now()
	for _, dag := range dags {
		compMachine := render.DecodeComponentMachine(dag.GetMetadata()[render.ComponentMachineMetaKey])
		node := findLeasableAgentNode(dag, now, target.GetMachineId(), compMachine)
		if node == nil {
			continue
		}
		cmd := &rtagent.Command{}
		if input := node.GetTaskState().GetInput(); input != nil {
			if err := input.UnmarshalTo(cmd); err != nil {
				return nil, status.Errorf(codes.Internal, "decode command: %v", err)
			}
		}
		// Resolve render bindings (DB ip etc) from the run's terraform output
		// BEFORE leasing — anti-leak: never hand out an unresolved command.
		cmd, err = resolveCommand(dag, node, cmd)
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "resolve command bindings: %v", err)
		}
		expires := now.Add(q.ttl)
		setLease(node, target.GetMachineId(), expires)
		if err := q.store.SaveDag(ctx, dag); err != nil {
			return nil, status.Errorf(codes.Internal, "save lease: %v", err)
		}
		return &agentpb.CommandLease{
			Address:        &agentpb.NodeAddress{DagId: &models.DagId{Value: dag.GetId()}, NodeExecutionId: node.GetExecutionId()},
			Command:        cmd,
			LeaseExpiresAt: timestamppb.New(expires),
		}, nil
	}
	return nil, nil
}

// resolveCommand substitutes the node's render bindings in the command, reading
// runtime values from the dag's terraform output + component->machine map. A
// command with no bindings is returned unchanged.
func resolveCommand(dag *primitive.Dag, node *primitive.Dag_Node, cmd *rtagent.Command) (*rtagent.Command, error) {
	bindings, err := render.NodeBindings(node)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return cmd, nil
	}
	resolved := render.FromDeployment(findDeploymentOutput(dag), render.DecodeComponentMachine(dag.GetMetadata()[render.ComponentMachineMetaKey]))
	data, err := protojson.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	out, err := render.NewResolver(resolved).ResolveText(string(data), bindings)
	if err != nil {
		return nil, err
	}
	fresh := &rtagent.Command{}
	if err := protojson.Unmarshal([]byte(out), fresh); err != nil {
		return nil, err
	}
	return fresh, nil
}

// findDeploymentOutput returns the first node output that decodes to a PROVISIONED
// deployment.Deployment — one whose Output carries container/VM IPs (the deployDocker
// / deployYc node result), recursing into sub-dags. The run_stroppy command's
// binding holes resolve against it.
func findDeploymentOutput(dag *primitive.Dag) *deployment.Deployment {
	var found *deployment.Deployment
	walkNodes(dag, func(n *primitive.Dag_Node) {
		if found != nil {
			return
		}
		raw := n.GetTaskState().GetOutput()
		if raw == nil {
			return
		}
		var dep deployment.Deployment
		if err := raw.UnmarshalTo(&dep); err != nil {
			return
		}
		if len(dep.GetDocker().GetOutput().GetContainers()) > 0 || len(dep.GetYandex().GetOutput().GetVms()) > 0 {
			found = &dep
		}
	})
	return found
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
// lease (unleased or expired) that is placed on machineID, recursing into
// sub-dags. A node whose component can't be mapped to a machine is skipped.
func findLeasableAgentNode(dag *primitive.Dag, now time.Time, machineID string, compMachine map[string]string) *primitive.Dag_Node {
	var found *primitive.Dag_Node
	walkNodes(dag, func(n *primitive.Dag_Node) {
		if found != nil {
			return
		}
		if !isAgentCommand(n) || n.GetStatus() != primitive.Status_STATUS_RUNNING || leaseValid(n, now) {
			return
		}
		if nodeMachine(n, compMachine) != machineID {
			return
		}
		found = n
	})
	return found
}

// nodeMachine resolves the machine a node is placed on from its component
// (the node-id prefix before the first '.') via the component->machine map.
// Returns "" when the component can't be mapped (caller must not lease it).
// nodeMachine resolves the machine a leasable node belongs to from its id prefix.
// A node id is "<prefix>.<action>" where prefix is either a COMPONENT id (install
// nodes — mapped to its machine) or a MACHINE id directly (per-machine nodes like
// the monitoring chain).
func nodeMachine(n *primitive.Dag_Node, compMachine map[string]string) string {
	id := n.GetId()
	prefix := id
	if i := strings.IndexByte(id, '.'); i >= 0 {
		prefix = id[:i]
	}
	if m, ok := compMachine[prefix]; ok {
		return m // prefix is a component id
	}
	for _, m := range compMachine {
		if m == prefix {
			return prefix // prefix is a machine id (per-machine node)
		}
	}
	return ""
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
