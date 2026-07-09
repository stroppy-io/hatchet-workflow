package provider

import (
	"context"
	"fmt"
	"io"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// RunContext carries the per-run identifiers NewProviderForRef needs beyond
// the provider ref itself: which tenant owns the run (F1: credential scope,
// F2: log labeling) and which run this is (F2: log labeling, F4: workdir
// isolation). RecipeActivities' ProvisionActivity/TeardownActivity builds one
// per activity invocation from its own Input's TenantID/RunID.
type RunContext struct {
	TenantID string
	RunID    string
}

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
	// EnvFn resolves the terraform provider's deploy credentials (e.g.
	// YC_TOKEN) for tenantID, called once per NewProviderForRef invocation —
	// i.e. once per ProvisionActivity/TeardownActivity call, inside the
	// activity (never inside the workflow, so no secret value is ever
	// serialized into Temporal's workflow history). Replaces the old static
	// Env map[string]string (was resolved once, process-wide, from
	// os.Getenv — every tenant shared one YC account). nil is tolerated for
	// docker-only deployments (the docker branch never calls it); a nil
	// EnvFn used on the terraform branch resolves to "no credentials" (empty
	// env), matching pre-F1 test/dev wiring that never set Env at all — see
	// envFor.
	EnvFn func(ctx context.Context, tenantID string) (map[string]string, error)
	// LogSinkFn resolves the per-run provisioning log sink (F2): stdout/stderr
	// writers labeled with runID that every terraform apply/destroy for this
	// run's provider is mirrored into (in addition to the terraform.Actor's
	// own process-wide default writer). nil is tolerated (no per-run sink;
	// pre-F2 behavior).
	LogSinkFn func(ctx context.Context, runID string) (stdout, stderr io.Writer)
	// ModuleDir resolves a provider NAME (ref.GetName(), e.g. "yandex"),
	// scoped by tenantID/runID, to its module directory id (also used as the
	// terraform.WdId — F4: per-run, not a shared constant) and embedded HCL
	// files. The builtin "docker" provider is handled before ModuleDir is
	// ever consulted, so ModuleDir only needs to know about terraform-module
	// providers. Returns ok=false for a name it does not recognize.
	//
	// v1 convention: the builtin "yandex" provider resolves via
	// YandexModuleDirResolver (deployments/terraform/yandex embedded files)
	// with a per-run dir id "yandex-<runID>". Recipe-supplied provider
	// modules (loaded from recipe storage, phase 1A) are a later wiring
	// concern — this factory only needs a ModuleDir resolver injected; how
	// that resolver is built (builtin-only vs. builtin+recipe-storage-backed)
	// is the caller's choice.
	ModuleDir func(tenantID, runID, providerName string) (dir string, tfFiles []terraform.TfFile, ok bool)
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
// a non-"docker" ref), if ModuleDir does not recognize the name, or if
// EnvFn fails to resolve credentials for run.TenantID.
//
// ctx/run are used only by the terraform branch (EnvFn/LogSinkFn/ModuleDir
// resolution); the docker branch ignores both — deps.EnvFn/deps.LogSinkFn are
// never called for "docker".
func NewProviderForRef(ctx context.Context, ref *dslpb.ProviderRef, run RunContext, deps Deps) (Provider, error) {
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

	dir, tfFiles, ok := deps.ModuleDir(run.TenantID, run.RunID, name)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}

	env, err := envFor(ctx, deps.EnvFn, run.TenantID)
	if err != nil {
		return nil, fmt.Errorf("provider %q: resolve deploy credentials for tenant %q: %w", name, run.TenantID, err)
	}

	stdout, stderr := logSinkFor(ctx, deps.LogSinkFn, run.RunID)

	return NewTerraform(dir, NewTerraformActorExec(deps.Actor, tfFiles, env, stdout, stderr)), nil
}

// envFor resolves deps.EnvFn against tenantID, tolerating a nil EnvFn as "no
// credentials configured" (empty env, not an error) — matches today's
// behavior for docker-only dev/test wiring that never sets EnvFn at all.
func envFor(ctx context.Context, fn func(context.Context, string) (map[string]string, error), tenantID string) (map[string]string, error) {
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, tenantID)
}

// logSinkFor resolves deps.LogSinkFn against runID, tolerating a nil
// LogSinkFn as "no per-run sink" (nil, nil) — every apply/destroy then falls
// back to the terraform.Actor's own default writer, matching pre-F2 behavior
// for callers (tests, docker-only dev wiring) that never set it.
func logSinkFor(ctx context.Context, fn func(context.Context, string) (io.Writer, io.Writer), runID string) (io.Writer, io.Writer) {
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, runID)
}
