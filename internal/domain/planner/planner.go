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

// Shape is a finite Dag form. The graph is fixed per shape; inputs vary.
type Shape string

const (
	ShapeSingle Shape = "single"
	// TODO(planner): ShapeHA / ShapeReplica / ShapeExternal — emergent from Options.
)

// Template builds the Dag for a shape from a preset + resolved deployment params.
type Template func(preset *domain.TestPreset, params *DeploymentParams) (*primitive.Dag, error)

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

// Compile detects the shape of the preset and instantiates its template with the
// resolved deployment params (nil is treated as empty).
func (c *Compiler) Compile(preset *domain.TestPreset, params *DeploymentParams) (*primitive.Dag, error) {
	if params == nil {
		params = &DeploymentParams{}
	}
	shape := shapeOf(preset)
	tmpl, ok := c.templates[shape]
	if !ok {
		return nil, fmt.Errorf("planner: no template for shape %q", shape)
	}
	return tmpl(preset, params)
}

// shapeOf picks the Dag form from the preset.
//
// TODO(planner): detect HA/replica/external from Database.Options + Topology; for
// now everything maps to the single linear shape.
func shapeOf(_ *domain.TestPreset) Shape {
	return ShapeSingle
}

// singleTemplate: render_config -> terraform_apply -> install_and_run(sub_dag) ->
// collect_results -> terraform_destroy(always_run). apply/destroy share a workdir.
func singleTemplate(preset *domain.TestPreset, params *DeploymentParams) (*primitive.Dag, error) {
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
	config := databaseConfig(preset.GetDatabase(), view.dbMemoryMB)

	renderNode := serverTask("render_config", HandlerRenderConfig)
	apply := serverTaskInput("terraform_apply", HandlerTerraformApply, applyInput)
	install, err := installAndRunSubDag(config, view.dbComponentID)
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

// installAndRunSubDag is the agent's on-host work — one node per command, each a
// task handler="agent.command" carrying an ops.Operation (D17, H19). The rendered
// database config files become WRITE_FILE commands; run_stroppy carries a late
// binding to the database's private ip (resolved at the plan->execute seam).
func installAndRunSubDag(config *renderpb.Config, dbComponentID string) (*primitive.Dag_Node, error) {
	var nodes []*primitive.Dag_Node
	add := func(id string, op *ops.Operation, bindings []*renderpb.Config_Binding) error {
		n, err := agentCommand(id, op)
		if err != nil {
			return err
		}
		render.AttachBindings(n, bindings)
		nodes = append(nodes, n)
		return nil
	}

	if err := add("write_sources_list", writeFileOp(), nil); err != nil {
		return nil, err
	}
	if err := add("apt_install", runCmdOp(), nil); err != nil {
		return nil, err
	}
	for _, item := range config.GetItems() {
		file := item.GetFile()
		if file == nil {
			continue
		}
		if err := add("write_"+item.GetId(), writeFileOpFor(file), item.GetBindings()); err != nil {
			return nil, err
		}
	}
	if err := add("start_service", runCmdOp(), nil); err != nil {
		return nil, err
	}
	var stroppyBindings []*renderpb.Config_Binding
	if dbComponentID != "" {
		stroppyBindings = []*renderpb.Config_Binding{{
			Token:        stroppyDBHostToken,
			ComponentIds: []string{dbComponentID},
			Attr:         render.AttrPrivateIP,
		}}
	}
	if err := add("run_stroppy", runStroppyOp(), stroppyBindings); err != nil {
		return nil, err
	}

	edges := make([]*primitive.Dag_Edge, 0, len(nodes))
	for i := 1; i < len(nodes); i++ {
		edges = append(edges, edge(nodes[i-1], nodes[i]))
	}
	sub := &primitive.Dag{
		Id:     ids.New(),
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  nodes,
		Edges:  edges,
	}
	return subDagNode("install_and_run", sub), nil
}

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

// TODO(planner): placeholder operations — the real per-engine recipe (file paths,
// package install, config, service start, stroppy invocation) is backend data.
func writeFileOp() *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_WRITE_FILE, Operation: &ops.Operation_WriteFile{WriteFile: &system.File{}}}
}

func runCmdOp() *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{}}}
}

// writeFileOpFor wraps a rendered config file into a WRITE_FILE operation.
func writeFileOpFor(file *system.File) *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_WRITE_FILE, Operation: &ops.Operation_WriteFile{WriteFile: file}}
}

// runStroppyOp builds the workload command. The DB host is a late-binding token
// resolved to the database component's private ip at the plan->execute seam.
func runStroppyOp() *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{
		Command: &system.Cmd_Spec_Argv{Argv: &system.Cmd_Argv{
			Args: []string{"stroppy", "run", "--url", "postgres://stroppy@" + stroppyDBHostToken + ":5432/stroppy"},
		}},
	}}}
}
