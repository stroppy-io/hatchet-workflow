// Package dag is the consolidated DAG-building domain: it builds the whole TestRun
// (and SuiteRun) as ONE static proto Dag. The runtime NEVER mutates structure;
// conditional EDGES (a predicate over an enum) select which path runs.
//
// BuildTestDag is the BLUEPRINT (canonical shape): a pipeline of typed proto->proto
// tasks streaming through DagContext.GetOutput, with the provider switch expressed
// as conditional EDGES (predicates over the proto deployment.Deployment.provider)
// and teardown as always-run branches. Every external effect is an INTERFACE (the
// Deps seams), so the whole dag is unit-testable with fakes; swapping the Deps impls
// for real ones (docker daemon, terraform, victoria) turns the blueprint into the
// product.
//
//	prepareDeployment              preset -> deployment.Deployment{provider,network,quota,oneof Input}
//	  ├─[provider==DOCKER]─► deployDocker   Docker.Input  -> Docker.Output   (DockerProvisioner)
//	  └─[provider==YANDEX]─► deployYc       Yandex.Input  -> Yandex.Output   (TerraformProvisioner)
//	        ▼ join ANY (the unselected branch is SKIPPED by the executor per edge rules)
//	  install_and_run (SUB-DAG)    per-component agent recipe chains (InstallBuilder)
//	        ▼
//	  ├─[provider==DOCKER]─► destroyDocker  always-run cleanup (DockerProvisioner.Destroy)
//	  └─[provider==YANDEX]─► destroyYc      always-run cleanup (TerraformProvisioner.Destroy)
//
// This package folds in the former internal/domain/{example,planner,compat}: the
// blueprint (here), the install sub-dag builder + recipe selection (install.go,
// recipe.go), the recipe DATA matrix (recipes_data.go), the legacy linear Compile +
// terraform tfvars (compile.go), and the SuiteRun orchestration dag (suite.go).
// proto is the single source of truth: model in proto + transform proto->proto +
// switch on enums.
package dag

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/protohelp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ── seams (every external effect is an interface -> testable with fakes) ─────────

// DockerProvisioner materializes (and tears down) a local Docker deployment:
// Apply turns Docker.Input into Docker.Output (created container endpoints).
type DockerProvisioner interface {
	Apply(ctx context.Context, d *deployment.Docker) (*deployment.Docker, error)
	Destroy(ctx context.Context, d *deployment.Docker) error
}

// TerraformProvisioner materializes (and tears down) a Yandex deployment via tf.
type TerraformProvisioner interface {
	Apply(ctx context.Context, y *deployment.Yandex) (*deployment.Yandex, error)
	Destroy(ctx context.Context, y *deployment.Yandex) error
}

// InstallBuilder builds the install_and_run sub-dag (per-component agent recipe
// chains, emergent from the topology graph). Pure: plan-time from the preset.
type InstallBuilder interface {
	Build(preset *domain.TestPreset, runID string) *primitive.Dag
}

// Deps bundles the seams. Swap the impls for real ones and the dag is the product.
// (There is no metrics seam: stroppy + node-exporter push metrics to the server ->
// VictoriaMetrics during the run; the metrics service queries VM by run_id later.)
type Deps struct {
	Docker  DockerProvisioner
	TF      TerraformProvisioner
	Install InstallBuilder
}

// ── node ids + edge predicates ───────────────────────────────────────────────

const (
	prepareDeployment = "prepareDeployment"
	deployDocker      = "deployDocker"
	deployYc          = "deployYc"
	installAndRun     = "installAndRun"
	destroyDocker     = "destroyDocker"
	destroyYc         = "destroyYc"

	// Edge predicates read the prepareDeployment OUTPUT (a deployment.Deployment)
	// and gate the branch edge on its provider enum. This IS the provider switch.
	predProviderDocker = "provider_docker"
	predProviderYandex = "provider_yandex"
)

// BuildDeployment is the pure topology->infra transform: preset -> deployment.Deployment
// with the provider oneof Input populated. The deploy branch only APPLIES it.
func BuildDeployment(preset *domain.TestPreset, net *system.Network, quota []*deployment.QuotaRequest) *deployment.Deployment {
	d := &deployment.Deployment{
		Provider:      preset.GetDeployment().GetProvider(),
		Deployed:      false,
		Network:       net,
		QuotaRequests: quota,
	}
	switch d.GetProvider() {
	case deployment.Provider_PROVIDER_DOCKER:
		containers := make(map[string]*deployment.Docker_Container, len(preset.GetTopology().GetMachines()))
		for _, m := range preset.GetTopology().GetMachines() {
			containers[m.GetId()] = &deployment.Docker_Container{
				Image:         dockerAgentImage,
				FallbackImage: systemdImage,
				Privileged:    true,   // systemd-in-docker
				CgroupnsMode:  "host", // systemd-in-docker
				// Env + /etc/stroppy-agent.env (SERVER_ADDR/MACHINE_ID/AGENT_TOKEN) injected by deploy.Resolve
			}
		}
		d.Deployment = &deployment.Deployment_Docker{Docker: &deployment.Docker{
			Input: &deployment.Docker_Input{Containers: containers /*, Network */},
		}}
	case deployment.Provider_PROVIDER_YANDEX:
		d.Deployment = &deployment.Deployment_Yandex{Yandex: &deployment.Yandex{ /* Input: instances per machine */ }}
	}
	return d
}

// BuildTestDag builds the static execution DAG STRUCTURE (named provision nodes +
// the install sub-dag + always-run teardown). It does NOT register handlers: the
// nodes reference handlers by name, registered once at startup via
// RegisterProvisionHandlers (so the shared DagProcessor registry holds them with the
// real Deps injected). deps is used only for deps.Install.Build (a pure plan-time
// step that embeds the per-component agent recipe sub-dag).
//
// dep is the run-scoped deployment.Deployment, resolved at submit time (provider
// settings + per-machine IPs + agent token/cloud-init) and baked into the
// prepareDeployment node — which the runtime just passes through, so the provider
// branches and the deploy/teardown handlers see the fully-resolved deployment.
func BuildTestDag(preset *domain.TestPreset, dep *deployment.Deployment, deps Deps) *primitive.Dag {
	id := ids.New()
	return &primitive.Dag{
		Id:     id,
		Status: primitive.Status_STATUS_PENDING, // so the DagProcessor picks it up
		Input:  _toAny(preset),
		// component->machine map: agentqueue resolves agent-command binding holes
		// (e.g. run_stroppy's DB host) against the deploy node's deployment Output.
		Metadata: map[string]string{
			render.ComponentMachineMetaKey: render.EncodeComponentMachine(componentMachine(preset.GetTopology())),
		},
		// install_and_run joins whichever deploy branch ran (JOIN ANY); the unselected
		// provider branch is SKIPPED by the executor. No collect node — stroppy +
		// node-exporter PUSH metrics to the server -> VM during the run.
		Nodes: []*primitive.Dag_Node{
			taskNode(prepareDeployment, dep),
			taskNode(deployDocker, &emptypb.Empty{}),
			taskNode(deployYc, &emptypb.Empty{}),
			subDagNode(installAndRun, deps.Install.Build(preset, id), joinAny()),
			taskNode(destroyDocker, &emptypb.Empty{}, alwaysRun()),
			taskNode(destroyYc, &emptypb.Empty{}, alwaysRun()),
		},
		Edges: []*primitive.Dag_Edge{
			condEdge(prepareDeployment, deployDocker, predProviderDocker),
			condEdge(prepareDeployment, deployYc, predProviderYandex),
			edge(deployDocker, installAndRun),
			edge(deployYc, installAndRun),
			condEdge(installAndRun, destroyDocker, predProviderDocker),
			condEdge(installAndRun, destroyYc, predProviderYandex),
		},
	}
}

// RegisterProvisionHandlers registers the SERVER-locus provision/teardown task
// handlers (the named nodes BuildTestDag emits) into reg, with the real Deps
// injected. Call once at startup (services/tasks) with production Deps, or in a test
// with fakes. The deploy branches read prepareDeployment's output and apply the
// active provider oneof; teardown is always-run and provider-guarded in-task.
func RegisterProvisionHandlers(reg runtime.TasksRegistry, deps Deps) {
	// prepareDeployment: the fan-out point. The deployment is resolved at submit time
	// and baked into this node's input; the handler passes it through so the provider
	// branches (and deploy/teardown) read the fully-resolved deployment.
	registerTask(reg, prepareDeployment, func(_ runtime.DagContext, dep *deployment.Deployment) (*deployment.Deployment, error) {
		return dep, nil
	})
	registerTask(reg, deployDocker, func(ctx runtime.DagContext, _ *emptypb.Empty) (*deployment.Deployment, error) {
		dep := mustDeployment(ctx)
		docker, err := deps.Docker.Apply(ctx, dep.GetDocker())
		if err != nil {
			return nil, err
		}
		dep.Deployment = &deployment.Deployment_Docker{Docker: docker}
		dep.Deployed = true
		return dep, nil
	})
	registerTask(reg, deployYc, func(ctx runtime.DagContext, _ *emptypb.Empty) (*deployment.Deployment, error) {
		dep := mustDeployment(ctx)
		yandex, err := deps.TF.Apply(ctx, dep.GetYandex())
		if err != nil {
			return nil, err
		}
		dep.Deployment = &deployment.Deployment_Yandex{Yandex: yandex}
		dep.Deployed = true
		return dep, nil
	})
	registerTask(reg, destroyDocker, func(ctx runtime.DagContext, _ *emptypb.Empty) (*emptypb.Empty, error) {
		if dep := provisionedDeployment(ctx, deployDocker); dep.GetProvider() == deployment.Provider_PROVIDER_DOCKER {
			return &emptypb.Empty{}, deps.Docker.Destroy(ctx, dep.GetDocker())
		}
		return &emptypb.Empty{}, nil
	})
	registerTask(reg, destroyYc, func(ctx runtime.DagContext, _ *emptypb.Empty) (*emptypb.Empty, error) {
		if dep := provisionedDeployment(ctx, deployYc); dep.GetProvider() == deployment.Provider_PROVIDER_YANDEX {
			return &emptypb.Empty{}, deps.TF.Destroy(ctx, dep.GetYandex())
		}
		return &emptypb.Empty{}, nil
	})
}

// componentMachine maps each component id to the machine hosting it (for binding
// resolution against the deployment Output).
func componentMachine(topo *domain.Topology) map[string]string {
	m := map[string]string{}
	for _, machine := range topo.GetMachines() {
		for _, c := range machine.GetComponents() {
			m[c.GetId()] = machine.GetId()
		}
	}
	return m
}

// mustDeployment reads the prepareDeployment node output (the run's deployment).
// mustDeployment returns the PLAN-TIME deployment (input spec) baked into the
// prepareDeployment node. Used by the deploy handlers, which Apply this spec.
func mustDeployment(ctx runtime.DagContext) *deployment.Deployment {
	out, err := ctx.GetOutput(prepareDeployment)
	if err != nil {
		return &deployment.Deployment{}
	}
	return _fromAny[*deployment.Deployment](out)
}

// provisionedDeployment returns the LIVE deployment produced by deployNode — the
// deploy handler records the provisioner Output (container ids, IPs, network) and
// sets Deployed=true. A teardown handler reads its paired deploy node's output so it
// has the ids to actually destroy; if the deploy never ran it falls back to the spec.
func provisionedDeployment(ctx runtime.DagContext, deployNode string) *deployment.Deployment {
	if out, err := ctx.GetOutput(deployNode); err == nil {
		if dep := _fromAny[*deployment.Deployment](out); dep.GetDeployed() {
			return dep
		}
	}
	return mustDeployment(ctx)
}

// ProviderPredicates gate the provider branches by reading prepareDeployment output.
func ProviderPredicates() runtime.PredicateRegistryMap {
	provider := func(ctx runtime.DagContext) deployment.Provider {
		out, err := ctx.GetOutput(prepareDeployment)
		if err != nil {
			return deployment.Provider_PROVIDER_UNSPECIFIED
		}
		return _fromAny[*deployment.Deployment](out).GetProvider()
	}
	return runtime.PredicateRegistryMap{
		predProviderDocker: func(ctx runtime.DagContext, _ *primitive.Dag_Edge) bool {
			return provider(ctx) == deployment.Provider_PROVIDER_DOCKER
		},
		predProviderYandex: func(ctx runtime.DagContext, _ *primitive.Dag_Edge) bool {
			return provider(ctx) == deployment.Provider_PROVIDER_YANDEX
		},
	}
}

// ── node/edge builders (blueprint) ─────────────────────────────────────────────

// taskNode builds a SERVER-locus task node referencing handler `name` (registered
// separately via RegisterProvisionHandlers). It does not register anything.
func taskNode(name string, input proto.Message, opts ...nodeOption) *primitive.Dag_Node {
	node := &primitive.Dag_Node{
		Id:          name,
		ExecutionId: name, // node ids are unique within the dag
		Variant: &primitive.Dag_Node_TaskState_{
			TaskState: &primitive.Dag_Node_TaskState{
				Input:       _toAny(input),
				HandlerName: name,
				Locus:       primitive.Dag_Node_TaskState_EXECUTION_LOCUS_SERVER,
			},
		},
		Scheduling: &primitive.Dag_Node_Scheduling{},
	}
	for _, o := range opts {
		o(node)
	}
	return node
}

// registerTask registers a typed proto->proto handler under name.
func registerTask[I, O proto.Message](reg runtime.TasksRegistry, name string, task runtime.TaskFn[I, O]) {
	reg.Register(runtime.NewTask[I, O](name, task))
}

func subDagNode(name string, sub *primitive.Dag, opts ...nodeOption) *primitive.Dag_Node {
	node := &primitive.Dag_Node{
		Id:          name,
		ExecutionId: name,
		Variant:     &primitive.Dag_Node_SubDag{SubDag: sub},
		Scheduling:  &primitive.Dag_Node_Scheduling{},
	}
	for _, o := range opts {
		o(node)
	}
	return node
}

// nodeOption sets execution locus / scheduling.
type nodeOption func(*primitive.Dag_Node)

func onAgent() nodeOption {
	return func(n *primitive.Dag_Node) {
		if ts := n.GetTaskState(); ts != nil {
			ts.Locus = primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT
		}
	}
}
func alwaysRun() nodeOption {
	return func(n *primitive.Dag_Node) { n.GetScheduling().AlwaysRun = true }
}
func joinAny() nodeOption {
	return func(n *primitive.Dag_Node) {
		n.GetScheduling().JoinPolicy = primitive.Dag_Node_Scheduling_JOIN_POLICY_ANY
	}
}

func edge(src, dst string) *primitive.Dag_Edge {
	return &primitive.Dag_Edge{Id: src + "->" + dst, Source: src, Target: dst}
}

// condEdge fires only when the named predicate returns true (the provider switch).
func condEdge(src, dst, predicate string) *primitive.Dag_Edge {
	e := edge(src, dst)
	e.Condition = &primitive.Dag_Edge_PredicateName{PredicateName: predicate}
	return e
}

var _ = onAgent // agent locus is set inside InstallBuilder impls' sub-dag nodes.

// DO NOT USE IN PROD WITHOUT ERR HANDLING
func _toAny[T proto.Message](msg T) *anypb.Any {
	an := &anypb.Any{}
	anypb.MarshalFrom(an, msg, proto.MarshalOptions{})
	return an
}

// DO NOT USE IN PROD WITHOUT ERR HANDLING
func _fromAny[T proto.Message](msg *anypb.Any) T {
	t := protohelp.ProtoNew[T]()
	msg.UnmarshalTo(t)
	return t
}
