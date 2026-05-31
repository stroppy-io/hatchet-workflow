// Package workflows implements the TestWorkflow orchestration on Temporal for
// the docker demo: it deploys a container, drives its in-container agent via a
// Temporal worker SESSION to install postgres + run stroppy, exposes the live
// stage state through the GetRunState query, then tears the stand down.
//
// The package is split into two halves that run on different task queues:
//
//   - ServerActivities run on the SERVER worker (queue "stroppy-cloud") where the
//     docker SDK and process config (stroppy URL) are available. They do the
//     non-deterministic IO the workflow must not do itself: deploy/teardown the
//     containers and build the concrete deploy ops (substituting the runtime
//     database IP into the recipe placeholders).
//   - The agent activities (WriteFile/CallCmd/...) are generated and run on the
//     AGENT worker (queue "stroppy-agent") inside the deployed container. The
//     workflow pins them to one container by creating a Temporal worker SESSION
//     on the agent queue and running them with the session context.
package workflows

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/deploy/dockerprov"
	"github.com/stroppy-io/stroppy-cloud/internal/deploy/recipe"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"

	"github.com/stroppy-io/schemapb/schemapb"
)

// infra is the seam between ServerActivities and the concrete docker deployer.
// *dockerprov.Deployer satisfies it; tests pass a fake that returns canned IPs
// so the activities can run without a real docker daemon.
type infra interface {
	Deploy(ctx context.Context, topo *topology.Topology, runID, serverAddr string) (*dockerprov.Deployed, error)
	Teardown(ctx context.Context, runID string) error
}

// ensure the production deployer satisfies the seam.
var _ infra = (*dockerprov.Deployer)(nil)

// settingsReader reads the singleton control-plane settings (admin-set). Only
// ServerAddr is consumed here; nil disables the lookup (derive fallback only).
type settingsReader interface {
	Get(ctx context.Context) (*api.PlatformSettings, error)
}

// ServerActivities are the activities registered on the SERVER worker (task
// queue "stroppy-cloud"). They hold the docker deployer and the process-level
// stroppy artifact config; the workflow stays deterministic by delegating all
// of this IO to them.
type ServerActivities struct {
	Deployer infra
	// StroppyArtifactPath is the gateway path the agent fetches stroppy from
	// (e.g. "/artifacts/stroppy"); empty means no concrete stroppy artifact. The
	// full URL is built at deploy time from the resolved server address so it
	// always matches the address the agent VM was given.
	StroppyArtifactPath string
	StroppyChecksum     string
	// Settings reads the admin-set instance settings (ServerAddr). Optional.
	Settings settingsReader
	// DefaultServerAddr is the agent-facing address used when the admin has not
	// set one (derived docker-host address).
	DefaultServerAddr string
}

// resolveServerAddr returns the address agent VMs are told: the admin-set
// PlatformSettings.ServerAddr when present, else the derived default. Any lookup
// error (e.g. no settings row yet) falls back to the default.
func (sa *ServerActivities) resolveServerAddr(ctx context.Context) string {
	if sa.Settings != nil {
		if s, err := sa.Settings.Get(ctx); err == nil && s.GetServerAddr() != "" {
			return s.GetServerAddr()
		}
	}
	return sa.DefaultServerAddr
}

// DeployRequest asks the deployer to bring up the containers for a run.
type DeployRequest struct {
	Topology *topology.Topology
	RunID    string
}

// DeployResult carries the per-instance IPs of the deployed containers plus the
// resolved database instance IP the recipe substitution needs.
type DeployResult struct {
	// InstanceIPs maps a topology instance id to its container IP.
	InstanceIPs map[string]string
	// DatabaseInstanceID is the instance chosen as the database node.
	DatabaseInstanceID string
	// DatabaseIP is the IP of the database instance.
	DatabaseIP string
	// Network is the docker bridge network the instances share.
	Network string
}

// DeployActivity brings up the run's containers and returns their IPs.
func (sa *ServerActivities) DeployActivity(ctx context.Context, req *DeployRequest) (*DeployResult, error) {
	serverAddr := sa.resolveServerAddr(ctx)
	dep, err := sa.Deployer.Deploy(ctx, req.Topology, req.RunID, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("workflows: deploy: %w", err)
	}

	res := &DeployResult{
		InstanceIPs: make(map[string]string, len(dep.Instances)),
		Network:     dep.Network,
	}
	for id, inst := range dep.Instances {
		res.InstanceIPs[id] = inst.IP
	}

	// The database IP feeds the workload's DSN (and, later, the proxy backend).
	// Pick the instance whose ROLE is database; fall back to the lowest id when no
	// role is marked (single-node demo).
	for _, inst := range req.Topology.GetInstances() {
		if recipe.InstanceRole(inst) == expand.RoleDatabase {
			res.DatabaseInstanceID = inst.GetId()
			res.DatabaseIP = res.InstanceIPs[inst.GetId()]
			break
		}
	}
	if res.DatabaseInstanceID == "" {
		ids := make([]string, 0, len(res.InstanceIPs))
		for id := range res.InstanceIPs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			res.DatabaseInstanceID = ids[0]
			res.DatabaseIP = res.InstanceIPs[ids[0]]
		}
	}

	return res, nil
}

// BuildRequest carries the test run and the deployed per-instance IPs so the
// recipe can render concrete cross-node config (the Cluster view).
type BuildRequest struct {
	TestRun     *domain.TestRun
	InstanceIPs map[string]string
}

// InstancePlan is the ordered set of discrete deploy steps for ONE machine
// (topology instance), plus the routing info the workflow needs to place them on
// that machine's agent. Steps carry plain fields (no proto) so they round-trip
// cleanly across the activity boundary.
type InstancePlan struct {
	// InstanceID is the topology instance these steps run on.
	InstanceID string
	// Role is the machine's logical role (database, workload, coordinator, ...).
	Role string
	// Tier is the deploy ordering across machines (lower first).
	Tier int
	// Steps are the role's discrete deploy actions, in order.
	Steps []recipe.Step
}

// BuildResult is the per-machine deploy plan: one InstancePlan per topology
// instance that has recipe steps, sorted by Tier so dependencies come up first.
// The workflow runs each plan on its own machine's agent session.
type BuildResult struct {
	Plans []InstancePlan
}

// BuildRecipeActivity builds the concrete, per-machine deploy plan for the run.
// It walks the topology instances, derives each machine's role from its
// component, builds that role's recipe steps, substitutes the runtime
// placeholders, and orders the machines by tier. Instances with no recipe steps
// (a role without a builder yet) are skipped.
func (sa *ServerActivities) BuildRecipeActivity(ctx context.Context, req *BuildRequest) (*BuildResult, error) {
	dbVals := bakedMap(req.TestRun.GetDatabase().GetParams())
	wlVals := bakedMap(req.TestRun.GetWorkload().GetParams())

	// Build the stroppy fetch URL from the SAME server address the agent VM was
	// given, so it fetches stroppy through the one address it knows.
	stroppyURL := ""
	if sa.StroppyArtifactPath != "" {
		stroppyURL = strings.TrimRight(sa.resolveServerAddr(ctx), "/") + sa.StroppyArtifactPath
	}
	refs := recipe.Refs{
		StroppyBinaryURL: stroppyURL,
		StroppyChecksum:  sa.StroppyChecksum,
	}

	kind := dbKindToRecipe(req.TestRun.GetDatabase().GetKind())

	// Build the full Cluster view: every instance as a Node (id, runtime IP, role).
	var all []recipe.Node
	for _, inst := range req.TestRun.GetTopology().GetInstances() {
		role := recipe.InstanceRole(inst)
		if role == "" {
			continue
		}
		all = append(all, recipe.Node{ID: inst.GetId(), IP: req.InstanceIPs[inst.GetId()], Role: role})
	}

	var plans []InstancePlan
	for _, self := range all {
		cl := recipe.NewCluster(self, all)
		steps := recipe.Steps(kind, self, cl, dbVals, wlVals, refs)
		if len(steps) == 0 {
			continue
		}
		plans = append(plans, InstancePlan{
			InstanceID: self.ID,
			Role:       string(self.Role),
			Tier:       self.Role.Tier(),
			Steps:      steps,
		})
	}
	if len(plans) == 0 {
		return nil, fmt.Errorf("workflows: no recipe steps for any instance (kind %q)", kind)
	}

	// Stable sort by tier: lower tiers (consensus, DB) come up before higher
	// (proxy, workload).
	sort.SliceStable(plans, func(i, j int) bool { return plans[i].Tier < plans[j].Tier })

	return &BuildResult{Plans: plans}, nil
}

// dbKindToRecipe maps the domain database kind to the recipe registry key.
func dbKindToRecipe(kind domain.Database_Kind) string {
	switch kind {
	case domain.Database_KIND_POSTGRES:
		return "postgres"
	case domain.Database_KIND_MYSQL:
		return "mysql"
	case domain.Database_KIND_MARIADB:
		return "mariadb"
	case domain.Database_KIND_YDB:
		return "ydb"
	case domain.Database_KIND_COCKROACH:
		return "cockroach"
	case domain.Database_KIND_PICODATA:
		return "picodata"
	default:
		return "postgres"
	}
}

// bakedMap extracts a plain map[string]any from a baked schema value. Returns
// nil when the baked value or its struct is absent (the recipe defaults the
// rest from this).
func bakedMap(b *schemapb.Baked) map[string]any {
	vals := b.GetValues()
	if vals == nil {
		return nil
	}
	return vals.AsMap()
}

// TeardownRequest asks the deployer to remove a run's containers and network.
type TeardownRequest struct {
	RunID string
}

// TeardownResult is the (empty) result of a teardown.
type TeardownResult struct{}

// TeardownActivity removes the run's containers/network. Best-effort: it always
// runs (even after a failed run) via a disconnected workflow context.
func (sa *ServerActivities) TeardownActivity(ctx context.Context, req *TeardownRequest) (*TeardownResult, error) {
	if err := sa.Deployer.Teardown(ctx, req.RunID); err != nil {
		return nil, fmt.Errorf("workflows: teardown: %w", err)
	}
	return &TeardownResult{}, nil
}
