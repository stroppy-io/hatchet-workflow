// Package planner compiles a domain TestPreset into an executable primitive.Dag
// (the Plan phase, C12). The graph is STATIC per shape (engine x topology — a
// finite set of forms); only node inputs vary (render config baked at Plan,
// endpoints via render.Binding at Execute). So the planner is a template registry
// + parameterization, not a general graph compiler. Canon:
// runtime/primitive/dag.proto, features/orchestration/compile-to-dag.feature.
//
// TODO(planner): only the linear single shape exists. NOT implemented and
// reported: emergent HA (patroni/etcd/proxy) / replica / external shapes;
// real node-input parameterization from the preset (render configs, machine
// specs, render.Binding wiring); recipes as backend data (H29). The agent
// commands carry placeholder Operations — fill the real per-engine recipe.
package planner

import (
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
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

// Shape is a finite Dag form. The graph is fixed per shape; inputs vary.
type Shape string

const (
	ShapeSingle Shape = "single"
	// TODO(planner): ShapeHA / ShapeReplica / ShapeExternal — emergent from Options.
)

// Template builds the Dag for a shape from a preset.
type Template func(preset *domain.TestPreset) (*primitive.Dag, error)

// Compiler selects and instantiates Dag templates.
type Compiler struct {
	templates map[Shape]Template
}

// New builds a Compiler with the known templates.
func New() *Compiler {
	return &Compiler{templates: map[Shape]Template{
		ShapeSingle: singleTemplate,
	}}
}

// Compile detects the shape of the preset and instantiates its template.
func (c *Compiler) Compile(preset *domain.TestPreset) (*primitive.Dag, error) {
	shape := shapeOf(preset)
	tmpl, ok := c.templates[shape]
	if !ok {
		return nil, fmt.Errorf("planner: no template for shape %q", shape)
	}
	return tmpl(preset)
}

// shapeOf picks the Dag form from the preset.
//
// TODO(planner): detect HA/replica/external from Database.Options + Topology; for
// now everything maps to the single linear shape.
func shapeOf(_ *domain.TestPreset) Shape {
	return ShapeSingle
}

// singleTemplate: render_config -> terraform_apply -> install_and_run(sub_dag) ->
// collect_results -> terraform_destroy(always_run).
func singleTemplate(_ *domain.TestPreset) (*primitive.Dag, error) {
	render := serverTask("render_config", HandlerRenderConfig)
	apply := serverTask("terraform_apply", HandlerTerraformApply)
	install, err := installAndRunSubDag()
	if err != nil {
		return nil, err
	}
	collect := serverTask("collect_results", HandlerCollectResults)
	destroy := serverTask("terraform_destroy", HandlerTerraformDestroy)
	destroy.Scheduling.AlwaysRun = true // teardown runs on failure/cancel too (H58)

	return &primitive.Dag{
		Id:     ids.New(),
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  []*primitive.Dag_Node{render, apply, install, collect, destroy},
		Edges: []*primitive.Dag_Edge{
			edge(render, apply),
			edge(apply, install),
			edge(install, collect),
			edge(collect, destroy), // success path; always_run covers failure
		},
		Metadata: map[string]string{},
	}, nil
}

// installAndRunSubDag is the agent's on-host work — one node per command, each a
// task handler="agent.command" carrying an ops.Operation (D17, H19).
func installAndRunSubDag() (*primitive.Dag_Node, error) {
	steps := []struct {
		id string
		op *ops.Operation
	}{
		{"write_sources_list", writeFileOp()},
		{"apt_install", runCmdOp()},
		{"write_config", writeFileOp()},
		{"start_service", runCmdOp()},
		{"run_stroppy", runCmdOp()},
	}

	nodes := make([]*primitive.Dag_Node, 0, len(steps))
	edges := make([]*primitive.Dag_Edge, 0, len(steps))
	for i, st := range steps {
		node, err := agentCommand(st.id, st.op)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
		if i > 0 {
			edges = append(edges, edge(nodes[i-1], node))
		}
	}

	sub := &primitive.Dag{
		Id:     ids.New(),
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  nodes,
		Edges:  edges,
	}
	return subDagNode("install_and_run", sub), nil
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

// TODO(planner): placeholder operations — the real per-engine recipe (file paths,
// package install, config, service start, stroppy invocation) is backend data.
func writeFileOp() *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_WRITE_FILE, Operation: &ops.Operation_WriteFile{WriteFile: &system.File{}}}
}

func runCmdOp() *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{}}}
}
