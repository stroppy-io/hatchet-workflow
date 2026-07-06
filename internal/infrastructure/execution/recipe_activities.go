package execution

import (
	"context"
	"errors"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/provider"
	dslservice "github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

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
}

// NewRecipeActivities constructs RecipeActivities against deps.
func NewRecipeActivities(deps provider.Deps) *RecipeActivities {
	return &RecipeActivities{deps: deps}
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
