package execution

import (
	"context"
	"errors"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/provider"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	dslservice "github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

// QuotaManager is the subset of *quotas.Manager (internal/infrastructure/
// quotas/reserve.go) RecipeActivities' three quota activities call —
// declared as an interface here (mirroring RecipeActivityImpl's own
// precedent in internal/workflows/register.go) so this package need not
// import internal/infrastructure/quotas at every call site, and so
// recipe_activities_test.go can inject a fake instead of a real *quotas.
// Manager (which needs a live postgres.DB — see quotas.NewStore).
type QuotaManager interface {
	Reserve(ctx context.Context, tenantID, runID, workflowID, provider string, groups []*dslpb.MachineGroup) error
	Commit(ctx context.Context, tenantID, runID string) error
	Release(ctx context.Context, tenantID, runID string) error
}

// RecipeActivities implements the real CompileRecipeActivity/
// ProvisionActivity/TeardownActivity bodies RunRecipeWorkflow
// (internal/workflows/runrecipe.go, Task 3) calls by name. It satisfies
// workflows.RecipeActivityImpl (register.go) structurally — this package
// never imports internal/workflows for anything but the Input/Output types
// runrecipe.go declares, and internal/workflows never imports this package,
// so RegisterRecipeActivities takes the interface rather than *RecipeActivities
// directly to keep it that way.
type RecipeActivities struct {
	// deps carries the real provider executors (docker/terraform) NewProviderForRef
	// selects between. Phase 1D's app wiring (Task 6) constructs one with real
	// adapters; tests construct one with fakes (see recipe_activities_test.go).
	deps provider.Deps
	// quotas backs ReserveQuotasActivity/CommitQuotasActivity/
	// ReleaseQuotasActivity — nil is tolerated (Compile/Provision/Teardown
	// tests that never touch quotas construct RecipeActivities with a nil
	// QuotaManager), but calling any of the three quota activities with a
	// nil quotas is a configuration error, not a silent no-op: RunRecipeWorkflow
	// depends on ReserveQuotasActivity actually failing closed if quota
	// enforcement was never wired, rather than silently reserving nothing.
	quotas QuotaManager
}

// NewRecipeActivities constructs RecipeActivities against deps and quotas
// (the Manager backing the quota reserve/commit/release activities — see
// internal/app/run.go's quotaManager construction site, the only real
// caller).
func NewRecipeActivities(deps provider.Deps, quotas QuotaManager) *RecipeActivities {
	return &RecipeActivities{deps: deps, quotas: quotas}
}

// CompileRecipeActivity compiles in.Bundle via internal/services/dsl.
// CompileBundle — the same provider-resolution + dsl.Compile pipeline
// DslService.Check/CheckBundle run, reused here (rather than duplicated) so
// this activity and the connect/recipe check surfaces never drift on how a
// bundle's provider manifest is resolved. Diagnostics (errors AND warnings)
// are always returned; Plan is nil whenever Diagnostics.HasErrors() is true —
// RunRecipeWorkflow.compileRecipe gates on that, never on a nil Plan check
// alone, mirroring dsl.Compile's own contract (see CompileBundle's doc
// comment on why a non-nil plan can otherwise coexist with an error
// diagnostic).
func (a *RecipeActivities) CompileRecipeActivity(
	_ context.Context,
	in *workflows.CompileRecipeActivityInput,
) (*workflows.CompileRecipeActivityOutput, error) {
	if in == nil {
		return nil, errors.New("compile recipe activity: input is required")
	}

	plan, diags := dslservice.CompileBundle(in.Bundle)
	if diags.HasErrors() {
		plan = nil
	}

	return &workflows.CompileRecipeActivityOutput{Plan: plan, Diagnostics: diags}, nil
}

// ProvisionActivity builds the Provider for in.ProviderRef via
// provider.NewProviderForRef(ref, a.deps) and requests machines for every
// entry in in.Groups, marshaling the result into ProvisionActivityOutput.
func (a *RecipeActivities) ProvisionActivity(
	ctx context.Context,
	in *workflows.ProvisionActivityInput,
) (*workflows.ProvisionActivityOutput, error) {
	if in == nil {
		return nil, errors.New("provision activity: input is required")
	}

	p, err := provider.NewProviderForRef(in.ProviderRef, a.deps)
	if err != nil {
		return nil, err
	}

	machines, err := p.Provision(ctx, in.ProviderRef, in.Groups)
	if err != nil {
		return nil, err
	}

	return &workflows.ProvisionActivityOutput{Machines: machines}, nil
}

// TeardownActivity builds the Provider for in.ProviderRef and destroys every
// resource it previously provisioned.
func (a *RecipeActivities) TeardownActivity(ctx context.Context, in *workflows.TeardownActivityInput) error {
	if in == nil {
		return errors.New("teardown activity: input is required")
	}
	if in.ProviderRef == nil {
		// Mirrors runrecipe.go's own teardown no-op guard (compile never
		// resolved a provider, so nothing was ever provisioned) — kept here too
		// since TeardownActivity may also be invoked directly in a future
		// caller that does not already special-case a nil ref itself.
		return nil
	}

	p, err := provider.NewProviderForRef(in.ProviderRef, a.deps)
	if err != nil {
		return err
	}

	return p.Destroy(ctx, in.ProviderRef)
}

// ReserveQuotasActivity reserves in.Provider/in.Groups' computed quota
// demand for (in.TenantID, in.RunID) via a.quotas.Reserve — see
// internal/infrastructure/quotas/reserve.go's Manager.Reserve for the
// machine_groups -> per-kind amount computation and the over-limit check.
func (a *RecipeActivities) ReserveQuotasActivity(ctx context.Context, in *workflows.ReserveQuotasActivityInput) error {
	if in == nil {
		return errors.New("reserve quotas activity: input is required")
	}
	if a.quotas == nil {
		return errors.New("reserve quotas activity: quota manager is not configured")
	}
	return a.quotas.Reserve(ctx, in.TenantID, in.RunID, in.WorkflowID, in.Provider, in.Groups)
}

// CommitQuotasActivity promotes (in.TenantID, in.RunID)'s reservation to
// allocated via a.quotas.Commit.
func (a *RecipeActivities) CommitQuotasActivity(ctx context.Context, in *workflows.CommitQuotasActivityInput) error {
	if in == nil {
		return errors.New("commit quotas activity: input is required")
	}
	if a.quotas == nil {
		return errors.New("commit quotas activity: quota manager is not configured")
	}
	return a.quotas.Commit(ctx, in.TenantID, in.RunID)
}

// ReleaseQuotasActivity frees (in.TenantID, in.RunID)'s reservation via
// a.quotas.Release. Unlike Reserve/Commit above, a nil a.quotas here is a
// documented no-op rather than an error: releaseQuotas (runrecipe.go) calls
// this unconditionally from the teardown defer, including on paths where
// reserveQuotas itself was never reached (e.g. a compile-stage failure) —
// treating "quota manager not configured" as fatal here would turn every
// such run's teardown stage FAILED for a dependency it never actually
// needed.
func (a *RecipeActivities) ReleaseQuotasActivity(ctx context.Context, in *workflows.ReleaseQuotasActivityInput) error {
	if in == nil {
		return errors.New("release quotas activity: input is required")
	}
	if a.quotas == nil {
		return nil
	}
	return a.quotas.Release(ctx, in.TenantID, in.RunID)
}
