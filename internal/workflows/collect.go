// Package workflows drives a benchmark run on Temporal: RunWorkflow orchestrates
// deploy -> build recipe -> per-machine agent activities -> teardown, reusing the
// existing run recipe (internal/domain/run) to BUILD the per-machine command
// sequences (in "collect mode") instead of dispatching them over a transport.
package workflows

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// InstancePlan is the ordered command sequence for ONE machine, plus the routing
// info the workflow needs (the agent task queue is derived from TargetID) and the
// tier that orders machines against each other (consensus/DB before proxy before
// the workload runner).
type InstancePlan struct {
	TargetID string
	Role     string
	Tier     int
	Commands []agent.Command
}

// roleTier orders machines: lower runs first.
func roleTier(role string) int {
	switch role {
	case "etcd", "coordinator":
		return 0
	case "database", "replica", "ydb-storage", "ydb-database":
		return 1
	case "pgbouncer":
		return 2
	case "proxy":
		return 3
	case "stroppy", "workload":
		return 4
	case "monitor":
		return 5
	default:
		return 6
	}
}

// collectClient implements run.CommandSink by APPENDING every command to a
// per-target plan instead of dispatching it. Running the run recipe against it
// captures exactly the command sequence each machine needs.
type collectClient struct {
	mu    sync.Mutex
	plans map[string][]agent.Command
	order []string
}

func newCollectClient() *collectClient {
	return &collectClient{plans: map[string][]agent.Command{}}
}

func (c *collectClient) append(target agent.Target, cmd agent.Command) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.plans[target.ID]; !ok {
		c.order = append(c.order, target.ID)
	}
	c.plans[target.ID] = append(c.plans[target.ID], cmd)
}

func (c *collectClient) Send(_ *run.NodeContext, target agent.Target, cmd agent.Command) error {
	c.append(target, cmd)
	return nil
}

func (c *collectClient) SendAll(_ *run.NodeContext, targets []agent.Target, cmd agent.Command) error {
	for _, t := range targets {
		c.append(t, cmd)
	}
	return nil
}

// BuildPlans drives the run recipe in collect mode and returns the per-machine
// command plans, tier-ordered. dep carries the deployed targets + endpoint (the
// run.State the tasks read); cfg is the resolved run config.
func BuildPlans(ctx context.Context, logger *zap.Logger, cfg types.RunConfig, dep *workflowpb.Deployment, mon MonitoringRefs) ([]InstancePlan, error) {
	state := run.NewState()
	state.ImportRunState(deploymentToRunState(dep))

	cc := newCollectClient()
	deps := run.Deps{
		Client:          cc,
		State:           state,
		ServerAddr:      mon.ServerAddr,
		MonitoringURL:   mon.MetricsURL,
		MonitoringToken: mon.Token,
		AccountID:       mon.AccountID,
		TenantID:        mon.TenantID,
	}

	nc := run.NewNodeContext(ctx, logger)
	if err := run.BuildRecipe(nc, cfg, deps); err != nil {
		return nil, fmt.Errorf("workflows: build recipe: %w", err)
	}

	roleByID := targetRoles(dep)
	plans := make([]InstancePlan, 0, len(cc.order))
	for _, id := range cc.order {
		role := roleByID[id]
		plans = append(plans, InstancePlan{
			TargetID: id,
			Role:     role,
			Tier:     roleTier(role),
			Commands: cc.plans[id],
		})
	}
	sort.SliceStable(plans, func(i, j int) bool { return plans[i].Tier < plans[j].Tier })
	return plans, nil
}

// deploymentToRunState maps the proto Deployment into the run.RunState the
// run.State import expects (targets keyed by role, endpoint, infra handles).
func deploymentToRunState(dep *workflowpb.Deployment) *run.RunState {
	rs := &run.RunState{
		DBHost:        dep.GetDbHost(),
		DBPort:        int(dep.GetDbPort()),
		ContainerIDs:  dep.GetContainerIds(),
		NetworkID:     dep.GetNetworkId(),
		TerraformWdId: dep.GetTerraformWdId(),
	}
	for _, t := range dep.GetTargets() {
		rs.Targets = append(rs.Targets, run.TargetInfo{
			ID:           t.GetId(),
			Host:         t.GetHost(),
			InternalHost: t.GetInternalHost(),
			AgentPort:    int(t.GetAgentPort()),
			Zone:         t.GetZone(),
			Role:         t.GetRole(),
		})
	}
	return rs
}

func targetRoles(dep *workflowpb.Deployment) map[string]string {
	out := map[string]string{}
	for _, t := range dep.GetTargets() {
		out[t.GetId()] = t.GetRole()
	}
	return out
}

// MonitoringRefs carries the server-derived settings the monitor/stroppy recipe
// tasks need (metrics/logs endpoints, tenant).
type MonitoringRefs struct {
	ServerAddr string
	MetricsURL string
	Token      string
	AccountID  int32
	TenantID   string
}
