// Package provider implements the pluggable provider-runner: a uniform
// Provider interface that replaces the old enum-switch dispatch
// (PROVIDER_DOCKER / PROVIDER_YANDEX) between infrastructure backends. Any
// provider — the docker builtin or a terraform module living in a git
// organization's `providers/<name>/module` — satisfies the same interface,
// so adding a provider no longer requires touching the runtime (spec §3 of
// docs/superpowers/specs/2026-07-03-yaml-dsl-pivot-design.md).
package provider

import (
	"context"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// Provider provisions and tears down the machines requested by a compiled
// DSL bundle for a single provider reference (`provider.use` + its params).
type Provider interface {
	// Provision requests machines for every group and returns the resulting
	// MachineState list keyed by group name. Node ids are generated as
	// "<group>-<idx>" for idx in [0, group.Count).
	Provision(ctx context.Context, ref *dslpb.ProviderRef, groups []*dslpb.MachineGroup) (map[string][]*deploymentpb.MachineState, error)
	// Destroy tears down every resource previously provisioned for ref.
	Destroy(ctx context.Context, ref *dslpb.ProviderRef) error
}

// terraformExec is the minimal terraform surface NewTerraform needs: write a
// tfvars file, apply the module, read back its outputs, and destroy it.
//
// Adaptation gap (see task-17-report.md for detail): the real runner is
// internal/infrastructure/terraform.Actor, whose ApplyTerraform/
// DestroyExisting take a *WorkdirWithParams built from a chain of Option
// funcs (var file bytes, workdir root, env, parallelism, ...) rather than a
// plain (dir, varsJSON) pair, and ApplyTerraform already returns the
// module's outputs (TfOutput, i.e. map[string][]byte) instead of exposing a
// separate Output(name) call. terraformExec below is intentionally shaped
// after that reality — Apply returns the outputs map directly — so a later
// wiring task only needs a thin adapter:
//
//	func (a *terraform.Actor) Apply(ctx, dir, varsJSON) (map[string][]byte, error) {
//	    w := terraform.NewWorkdirWithParams(terraform.NewWdId(dir), terraform.WithVarFile(varsJSON), ...)
//	    out, err := a.ApplyTerraform(ctx, w)
//	    return map[string][]byte(out), err
//	}
type terraformExec interface {
	// Apply runs `terraform apply` using dir as the module/workdir and
	// varsJSON as the tfvars file content, returning the module's raw
	// (undecoded) outputs keyed by output name.
	Apply(ctx context.Context, dir string, varsJSON []byte) (map[string][]byte, error)
	// Destroy runs `terraform destroy` for dir with varsJSON as the tfvars
	// file content (varsJSON carries provider params only — see terraform.go
	// Destroy).
	Destroy(ctx context.Context, dir string, varsJSON []byte) error
}

// dockerExec is the minimal docker surface NewDocker needs: create/replace a
// single container and remove every container belonging to a run.
//
// Adaptation gap (see task-17-report.md): the real
// internal/infrastructure/docker.Executor operates on a whole
// *deployment.Docker_Input (a named map of containers plus one shared
// network) per call — Up creates every container in the map, Down removes
// every named container then the network. dockerExec below decomposes that
// into a per-machine EnsureContainer (one call per requested node, matching
// how this package builds one ContainerSpec per node) and a per-run
// RemoveContainers so Destroy — which per the Provider interface receives
// only the ProviderRef, not the machine groups — can still tear down
// everything for that run by network name. A later wiring task adapts
// docker.Executor.Up/Down to this shape.
type dockerExec interface {
	// EnsureContainer creates (replacing any existing container of the same
	// name) and starts a single container, returning its runtime state.
	EnsureContainer(ctx context.Context, spec ContainerSpec) (ContainerState, error)
	// RemoveContainers removes every container attached to networkName and
	// then the network itself. Mirrors docker.Executor.Down's loop-then-
	// remove-network behavior for one specific run's containers.
	RemoveContainers(ctx context.Context, networkName string) error
}

// ContainerSpec is the thin per-machine container request NewDocker builds
// for each requested node.
type ContainerSpec struct {
	Name    string
	Image   string
	Network string
	Env     map[string]string
	Labels  map[string]string
}

// ContainerState is the runtime result of ensuring a container.
type ContainerState struct {
	ID         string
	InternalIP string
}
