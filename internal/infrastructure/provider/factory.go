package provider

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// Deps carries the real executors + module source NewProviderForRef needs to
// build a Provider from a ProviderRef. Callers (phase 1D's RunRecipeWorkflow
// wiring) construct one Deps per run with the real adapters (Tasks 2/3);
// tests construct one with fakes.
//
// Actor is typed as tfActor (this package's unexported minimal surface of
// *terraform.Actor, see terraform_adapter.go) rather than the exported
// *terraform.Actor concrete type. This keeps the factory's dependency on
// terraform.Actor narrow and, because tfActor is already the type
// NewTerraformActorExec accepts, avoids introducing a second interface or an
// exported alias just for this struct field: external packages can still set
// Deps.Actor to a *terraform.Actor value (or any other value satisfying the
// three methods) without needing to name the unexported interface type
// themselves, and in-package tests (this package's factory_test.go) can
// implement it directly for a fake.
type Deps struct {
	// DockerExec is the real docker adapter (Task 3's NewDockerExecutorExec)
	// or a fake in tests. Required when ref.GetName() == "docker".
	DockerExec dockerExec
	// Actor is the terraform actor (Task 2's tfActor surface; *terraform.Actor
	// satisfies it) used to build a terraformExec for any non-"docker"
	// provider name.
	Actor tfActor
	// Env carries provider credentials (e.g. YC_TOKEN) passed to every
	// terraform apply/destroy for non-"docker" providers.
	Env map[string]string
	// ModuleDir resolves a provider NAME (ref.GetName(), e.g. "yandex") to its
	// module directory id and embedded HCL files. The builtin "docker"
	// provider is handled before ModuleDir is ever consulted, so ModuleDir
	// only needs to know about terraform-module providers. Returns ok=false
	// for a name it does not recognize.
	//
	// v1 convention: the builtin "yandex" provider resolves to
	// yandextf.EmbeddedTfFiles() (deployments/terraform/yandex) with the
	// conventional dir id "yandex". Recipe-supplied provider modules (loaded
	// from recipe storage, phase 1A) are a later wiring concern — this
	// factory only needs a ModuleDir resolver injected; how that resolver is
	// built (builtin-only vs. builtin+recipe-storage-backed) is the caller's
	// choice.
	ModuleDir func(providerName string) (dir string, tfFiles []terraform.TfFile, ok bool)
	// AgentTokens issues the per-node agent bearer token the docker builtin
	// bakes into each agent container's STROPPY_AGENT_TOKEN env (the agent
	// refuses to start without one — see cmd/cli/agent_cmd.go — and the
	// gateway's Temporal proxy rejects an unauthenticated worker). Only the
	// docker branch uses it; terraform-module providers issue their tokens
	// through the classic attachAgentTokens path instead. Nil is tolerated
	// (tests whose fake DockerExec never runs a real agent), in which case the
	// docker provider renders no token.
	AgentTokens AgentTokenIssuer
}

// AgentTokenIssuer mints an agent bearer token scoped to a single
// (tenant, run, machine, task-queue). It is the minimal surface of
// internal/domain/agent.TokenService.IssueAgentToken the docker provider needs.
type AgentTokenIssuer interface {
	IssueAgentToken(tenantID, runID, machineID, taskQueue string) (string, error)
}

// NewProviderForRef returns the Provider that provisions/destroys machines
// for ref: the "docker" builtin backed by deps.DockerExec, or a terraform
// module resolved through deps.ModuleDir and run through deps.Actor for any
// other provider name. Returns an error if the required dependency for the
// selected branch is missing (nil DockerExec for "docker"; nil ModuleDir for
// a non-"docker" ref) or if ModuleDir does not recognize the name.
func NewProviderForRef(ref *dslpb.ProviderRef, deps Deps) (Provider, error) {
	name := ref.GetName()

	if name == "docker" {
		if deps.DockerExec == nil {
			return nil, fmt.Errorf("provider %q: no DockerExec configured", name)
		}
		return NewDocker(deps.DockerExec, deps.AgentTokens), nil
	}

	if deps.ModuleDir == nil {
		return nil, fmt.Errorf("provider %q: no ModuleDir resolver configured", name)
	}

	dir, tfFiles, ok := deps.ModuleDir(name)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}

	return NewTerraform(dir, NewTerraformActorExec(deps.Actor, tfFiles, deps.Env)), nil
}
