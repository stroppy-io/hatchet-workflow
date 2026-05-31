package run

import (
	"context"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
)

// NodeContext is the lightweight execution context passed to a task's Execute.
// It carries a parent context + a logger. It replaces the old dag.NodeContext
// now that the DAG engine is gone: tasks no longer run in a graph executor, they
// run sequentially in the Temporal BuildRecipe activity (recipe collection) or
// the Deploy/Teardown activities (infra side effects).
type NodeContext struct {
	context.Context
	log *zap.Logger
}

// NewNodeContext builds a NodeContext from a context + logger.
func NewNodeContext(ctx context.Context, logger *zap.Logger) *NodeContext {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &NodeContext{Context: ctx, log: logger}
}

// Log returns the node-scoped logger.
func (nc *NodeContext) Log() *zap.Logger { return nc.log }

// SaveSnapshot is a no-op now that runs are durable via Temporal history. Kept so
// the infra tasks (which used to persist the terraform workdir mid-apply) compile
// unchanged.
func (nc *NodeContext) SaveSnapshot() {}

// WithContext returns a shallow copy with a different context.Context.
func (nc *NodeContext) WithContext(ctx context.Context) *NodeContext {
	return &NodeContext{Context: ctx, log: nc.log}
}

// CommandSink receives the agent commands a recipe task produces. In production
// it is the plan collector (BuildRecipe); the commands are then executed on the
// agents as Temporal activities. This replaces the old agent.Client HTTP/poll
// transport.
type CommandSink interface {
	// Send records one command for a single target, in order.
	Send(nc *NodeContext, target agent.Target, cmd agent.Command) error
	// SendAll records the same command for every target.
	SendAll(nc *NodeContext, targets []agent.Target, cmd agent.Command) error
}

// TargetInfo is the serializable per-machine target descriptor carried in
// RunState (deploy result / teardown handles). Moved out of the deleted dag pkg.
type TargetInfo struct {
	ID           string
	Host         string
	InternalHost string
	AgentPort    int
	Zone         string
	Role         string
}

// RunState is the serializable snapshot of a run's provisioned infrastructure:
// the agent targets, the DB endpoint, and the provider teardown handles. Built by
// the Deploy activity, consumed by BuildRecipe + Teardown.
type RunState struct {
	Targets          []TargetInfo
	DBHost           string
	DBPort           int
	ContainerIDs     []string
	NetworkID        string
	TerraformWdId    string
	EffectiveConfigs map[string]map[string]string
}
