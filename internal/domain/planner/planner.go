// Package planner compiles a domain TestPreset into an executable primitive.Dag
// (the Plan phase, C12). Cluster shapes (single/ha/replica/scale) are NOT an enum:
// they emerge structurally from the topology graph (component kinds + connections
// + Database.Options). One graph-driven template compiles any topology — a per-
// component install chain (recipe + rendered config writes + start), ordered by
// kind rank + REPLICATION connections, with run_stroppy bound to its FLOW target.
// Endpoints resolve via render.Binding at Execute. Canon: runtime/primitive/
// dag.proto, domain/topology.proto, features/orchestration/compile-to-dag.feature.
//
// TODO(planner): component.config is consumed as-is — the HA config renderers
// (patroni/haproxy/etcd/mysql-repl/picodata/ydb, in internal/domain/render) that
// populate empty component configs + their bindings are the next piece. ydb/
// cockroach binary recipes are minimal (single-node).
package planner

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

// Server-locus task handlers (run by the control-plane executor's TasksRegistry).
const (
	HandlerRenderConfig     = "render_config"
	HandlerTerraformApply   = "terraform_apply"
	HandlerTerraformDestroy = "terraform_destroy"
	HandlerCollectResults   = "collect_results"
	// HandlerAgentCommand marks an agent-locus node. The server never runs it; the
	// agent leases it via Poll and executes the embedded ops.Operation.
	HandlerAgentCommand = "agent.command"
)

// Compiler compiles a TestPreset's topology graph into a Dag. Cluster shapes
// (single / ha / replica / scale) are NOT enumerated — they emerge structurally
// from the graph (component kinds + connections + Database.Options), so one
// graph-driven template handles every shape (topology.proto C-note).
type Compiler struct{}

// New builds a Compiler.
func New() *Compiler { return &Compiler{} }

// Compile builds the Dag for a preset with resolved deployment params (nil = empty).
func (c *Compiler) Compile(preset *domain.TestPreset, params *DeploymentParams) (*primitive.Dag, error) {
	if params == nil {
		params = &DeploymentParams{}
	}
	return graphTemplate(preset, params)
}

// graphTemplate: render_config -> terraform_apply -> install_and_run(sub_dag) ->
// collect_results -> terraform_destroy(always_run). apply/destroy share a workdir.
func graphTemplate(preset *domain.TestPreset, params *DeploymentParams) (*primitive.Dag, error) {
	workdirID := ids.New()

	applyOp, err := buildTfOperation(ops.TfOperation_ACTION_APPLY, workdirID, preset, params)
	if err != nil {
		return nil, err
	}
	applyInput, err := anypb.New(applyOp)
	if err != nil {
		return nil, fmt.Errorf("planner: wrap apply op: %w", err)
	}
	destroyOp, err := buildTfOperation(ops.TfOperation_ACTION_DESTROY, workdirID, preset, params)
	if err != nil {
		return nil, err
	}
	destroyInput, err := anypb.New(destroyOp)
	if err != nil {
		return nil, fmt.Errorf("planner: wrap destroy op: %w", err)
	}

	view := viewTopology(preset.GetTopology())

	renderNode := serverTask("render_config", HandlerRenderConfig)
	apply := serverTaskInput("terraform_apply", HandlerTerraformApply, applyInput)
	install, err := componentSubDag(preset)
	if err != nil {
		return nil, err
	}
	collect := serverTask("collect_results", HandlerCollectResults)
	destroy := serverTaskInput("terraform_destroy", HandlerTerraformDestroy, destroyInput)
	destroy.Scheduling.AlwaysRun = true // teardown runs on failure/cancel too (H58)

	return &primitive.Dag{
		Id:     ids.New(),
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  []*primitive.Dag_Node{renderNode, apply, install, collect, destroy},
		Edges: []*primitive.Dag_Edge{
			edge(renderNode, apply),
			edge(apply, install),
			edge(install, collect),
			edge(collect, destroy), // success path; always_run covers failure
		},
		// component->machine lets the execute-seam resolver map binding component
		// ids to terraform vm outputs.
		Metadata: map[string]string{
			render.ComponentMachineMetaKey: render.EncodeComponentMachine(view.componentToMachine),
		},
	}, nil
}

// kindRank orders component install across the topology: coordinators (etcd) come
// up first, then databases, then proxies/monitors, then the workload.
func kindRank(k domain.Topology_Component_Kind) int {
	switch k {
	case domain.Topology_Component_KIND_COORDINATOR:
		return 0
	case domain.Topology_Component_KIND_DATABASE:
		return 1
	case domain.Topology_Component_KIND_PROXY, domain.Topology_Component_KIND_MONITOR:
		return 2
	case domain.Topology_Component_KIND_STROPPY:
		return 3
	default:
		return 1
	}
}

// componentGroup is one component's node chain plus its install rank.
type componentGroup struct {
	id    string
	rank  int
	nodes []*primitive.Dag_Node
}

// componentSubDag is the agent's on-host work for the whole topology: a per-
// component chain (install recipe + rendered config writes + start), ordered by
// kind rank and by REPLICATION connections (primary before replica). Each node is
// an agent.command carrying an ops.Operation (D17/H19).
func componentSubDag(preset *domain.TestPreset) (*primitive.Dag_Node, error) {
	topo := preset.GetTopology()
	db := preset.GetDatabase()

	var groups []componentGroup
	byID := map[string]*componentGroup{}
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			nodes, err := componentChain(c, db, topo, int(m.GetMemoryGb())*1024)
			if err != nil {
				return nil, err
			}
			if len(nodes) == 0 {
				continue // AGENT/ADDON, or a component with nothing to do
			}
			groups = append(groups, componentGroup{id: c.GetId(), rank: kindRank(c.GetKind()), nodes: nodes})
			byID[c.GetId()] = &groups[len(groups)-1]
		}
	}

	var allNodes []*primitive.Dag_Node
	var edges []*primitive.Dag_Edge
	byRank := map[int][]*componentGroup{}
	for i := range groups {
		g := &groups[i]
		allNodes = append(allNodes, g.nodes...)
		for j := 1; j < len(g.nodes); j++ {
			edges = append(edges, edge(g.nodes[j-1], g.nodes[j])) // intra-component chain
		}
		byRank[g.rank] = append(byRank[g.rank], g)
	}

	// Cross-rank ordering: every component of rank r must finish before any
	// component of the next present rank starts.
	ranks := presentRanks(byRank)
	for i := 1; i < len(ranks); i++ {
		for _, prev := range byRank[ranks[i-1]] {
			for _, cur := range byRank[ranks[i]] {
				edges = append(edges, edge(last(prev.nodes), cur.nodes[0]))
			}
		}
	}
	// REPLICATION connections refine intra-database order: source (primary) before target.
	for _, conn := range topo.GetConnections() {
		if conn.GetKind() != domain.Topology_Connection_KIND_REPLICATION {
			continue
		}
		src, okS := byID[conn.GetFrom()]
		tgt, okT := byID[conn.GetTo()]
		if okS && okT && src.rank == tgt.rank {
			edges = append(edges, edge(last(src.nodes), tgt.nodes[0]))
		}
	}

	sub := &primitive.Dag{
		Id:     ids.New(),
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  allNodes,
		Edges:  edges,
	}
	return subDagNode("install_and_run", sub), nil
}

// componentChain builds one component's ordered agent.command nodes. STROPPY is a
// single run node; AGENT/ADDON contribute nothing.
func componentChain(c *domain.Topology_Component, db *domain.Database, topo *domain.Topology, memoryMB int) ([]*primitive.Dag_Node, error) {
	switch c.GetKind() {
	case domain.Topology_Component_KIND_STROPPY:
		n, err := stroppyNode(c, db, topo)
		if err != nil {
			return nil, err
		}
		return []*primitive.Dag_Node{n}, nil
	case domain.Topology_Component_KIND_AGENT, domain.Topology_Component_KIND_ADDON:
		return nil, nil
	}

	config, err := componentConfig(c, db, topo, memoryMB)
	if err != nil {
		return nil, err
	}
	rec := recipeForComponent(c, db)
	prefix := c.GetId()
	var nodes []*primitive.Dag_Node
	add := func(suffix string, op *ops.Operation, bindings []*renderpb.Config_Binding) error {
		n, err := agentCommand(prefix+"."+suffix, op)
		if err != nil {
			return err
		}
		render.AttachBindings(n, bindings)
		nodes = append(nodes, n)
		return nil
	}

	for i, cmd := range rec.preInstall {
		if err := add(fmt.Sprintf("pre_install_%d", i), scriptOp(cmd), nil); err != nil {
			return nil, err
		}
	}
	if len(rec.aptPackages) > 0 {
		if err := add("apt_install",
			scriptOp("DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(rec.aptPackages, " ")), nil); err != nil {
			return nil, err
		}
	}
	// Config FILE items are written before the service starts.
	for _, item := range config.GetItems() {
		if item.GetFile() == nil {
			continue
		}
		if err := add("write_"+item.GetId(), writeFileOpFor(item.GetFile()), item.GetBindings()); err != nil {
			return nil, err
		}
	}
	switch {
	case rec.serviceName != "":
		if err := add("start_service", scriptOp("systemctl enable --now "+rec.serviceName), nil); err != nil {
			return nil, err
		}
	case rec.startScript != "":
		if err := add("start_service", scriptOp(rec.startScript), nil); err != nil {
			return nil, err
		}
	}
	// Config COMMAND items run after the service is up (e.g. CHANGE MASTER TO on a
	// replica, with a late binding to the primary's ip).
	for _, item := range config.GetItems() {
		if item.GetCommand() == nil {
			continue
		}
		op := &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: item.GetCommand()}}
		if err := add("cmd_"+item.GetId(), op, item.GetBindings()); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// stroppyNode is the workload command: stroppy run against its FLOW target (the
// proxy if present, else the primary database), via a late-binding host token.
func stroppyNode(c *domain.Topology_Component, db *domain.Database, topo *domain.Topology) (*primitive.Dag_Node, error) {
	target := flowTarget(c.GetId(), topo)
	n, err := agentCommand(c.GetId()+".run_stroppy", runStroppyOp(db))
	if err != nil {
		return nil, err
	}
	if target != "" {
		render.AttachBindings(n, []*renderpb.Config_Binding{{
			Token:        stroppyDBHostToken,
			ComponentIds: []string{target},
			Attr:         render.AttrPrivateIP,
		}})
	}
	return n, nil
}

// flowTarget is the component the STROPPY load points at: its FLOW connection
// target, else the first PROXY, else the first DATABASE.
func flowTarget(stroppyID string, topo *domain.Topology) string {
	for _, conn := range topo.GetConnections() {
		if conn.GetKind() == domain.Topology_Connection_KIND_FLOW && conn.GetFrom() == stroppyID {
			return conn.GetTo()
		}
	}
	var firstDB string
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			if c.GetKind() == domain.Topology_Component_KIND_PROXY {
				return c.GetId()
			}
			if firstDB == "" && c.GetKind() == domain.Topology_Component_KIND_DATABASE {
				firstDB = c.GetId()
			}
		}
	}
	return firstDB
}

func presentRanks(byRank map[int][]*componentGroup) []int {
	var ranks []int
	for r := 0; r <= 3; r++ {
		if len(byRank[r]) > 0 {
			ranks = append(ranks, r)
		}
	}
	return ranks
}

func last(nodes []*primitive.Dag_Node) *primitive.Dag_Node { return nodes[len(nodes)-1] }

// serverTaskInput is a server task node carrying a typed input payload.
func serverTaskInput(id, handler string, input *anypb.Any) *primitive.Dag_Node {
	node := serverTask(id, handler)
	node.GetTaskState().Input = input
	return node
}

func serverTask(id, handler string) *primitive.Dag_Node {
	return &primitive.Dag_Node{
		Id:          id,
		ExecutionId: id,
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant: &primitive.Dag_Node_TaskState_{TaskState: &primitive.Dag_Node_TaskState{
			HandlerName: handler,
			Locus:       primitive.Dag_Node_TaskState_EXECUTION_LOCUS_SERVER,
		}},
	}
}

// agentCommand builds an agent-locus node whose input is Any(agent.Command{op}).
func agentCommand(id string, op *ops.Operation) (*primitive.Dag_Node, error) {
	input, err := anypb.New(&rtagent.Command{Operation: op})
	if err != nil {
		return nil, fmt.Errorf("planner: wrap command %q: %w", id, err)
	}
	return &primitive.Dag_Node{
		Id:          id,
		ExecutionId: "install_and_run." + id, // unique across the whole aggregate
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant: &primitive.Dag_Node_TaskState_{TaskState: &primitive.Dag_Node_TaskState{
			HandlerName: HandlerAgentCommand,
			Locus:       primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT,
			Input:       input,
		}},
	}, nil
}

func subDagNode(id string, sub *primitive.Dag) *primitive.Dag_Node {
	return &primitive.Dag_Node{
		Id:          id,
		ExecutionId: id,
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant:     &primitive.Dag_Node_SubDag{SubDag: sub},
	}
}

func edge(source, target *primitive.Dag_Node) *primitive.Dag_Edge {
	return &primitive.Dag_Edge{
		Id:     source.GetId() + "->" + target.GetId(),
		Source: source.GetId(),
		Target: target.GetId(),
	}
}

// scriptOp wraps a shell script into a RUN_CMD operation (recipe steps use shell
// features — pipes, $(...) — so they run via the shell, not argv).
func scriptOp(text string) *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{
		Command: &system.Cmd_Spec_Script{Script: &system.Cmd_Script{Text: text, Shell: "/bin/bash"}},
	}}}
}

// writeFileOpFor wraps a rendered config file into a WRITE_FILE operation.
func writeFileOpFor(file *system.File) *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_WRITE_FILE, Operation: &ops.Operation_WriteFile{WriteFile: file}}
}

// runStroppyOp builds the workload command. The DB host is a late-binding token
// resolved to the target component's private ip at the plan->execute seam; the URL
// scheme + port follow the engine.
func runStroppyOp(db *domain.Database) *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{
		Command: &system.Cmd_Spec_Argv{Argv: &system.Cmd_Argv{
			Args: []string{"stroppy", "run", "--url", stroppyURL(db)},
		}},
	}}}
}

// stroppyURL is the engine-specific connection URL with the late-binding host token.
func stroppyURL(db *domain.Database) string {
	switch db.GetKind() {
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return "mysql://stroppy@" + stroppyDBHostToken + ":3306/stroppy"
	case domain.Database_KIND_COCKROACH:
		return "postgres://stroppy@" + stroppyDBHostToken + ":26257/stroppy"
	default:
		return "postgres://stroppy@" + stroppyDBHostToken + ":5432/stroppy"
	}
}
