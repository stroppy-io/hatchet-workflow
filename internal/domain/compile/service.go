package compile

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Service is the run.Compiler: it resolves the workload through the
// library, compiles the RunSpec and bakes it through spec.run@1.
type Service struct {
	renderer Renderer
	catalog  *catalog.Catalog
	library  *library.Service
	// Observability is where every run sends telemetry.
	Observability spec.Observability
	// StroppyImage overrides the catalog image of every run (staging).
	StroppyImage string
}

// NewService wires the compiler.
func NewService(r Renderer, cat *catalog.Catalog, lib *library.Service) *Service {
	return &Service{renderer: r, catalog: cat, library: lib}
}

// Compile implements run.Compiler.
func (s *Service) Compile(ctx context.Context, req run.CompileRequest) (run.Compiled, error) {
	_, baked, _, err := s.library.DeriveWorkload(ctx, req.Workload)
	if err != nil {
		return run.Compiled{}, err
	}
	prov, ok := s.catalog.Provider(string(req.Profile.Kind))
	if !ok {
		return run.Compiled{}, errs.Invalid(fmt.Sprintf("unknown provider %q", req.Profile.Kind))
	}
	obs := s.Observability
	obs.Labels = map[string]string{"stroppy_run_id": req.RunID.String(), "stroppy_tenant": req.Tenant}
	out, err := Compile(ctx, s.renderer, Input{
		RunID: req.RunID, Tenant: req.Tenant, Database: req.Database, Plan: req.Derived.Plan, EffectiveConfigs: req.Derived.EffectiveConfigs,
		Workload: req.Workload, WorkloadBaked: baked, Sizes: req.Sizes, Provider: prov, ProviderKind: string(req.Profile.Kind),
		ProviderSettings: req.Profile.Settings, CredentialsSecret: provider.CredentialsSecret(req.Profile.ID), ProviderConfigName: req.Namespace,
		StroppyImage: s.StroppyImage, Keep: req.Keep, Observability: obs, Catalog: s.catalog, Labels: req.Labels,
	})
	if err != nil {
		return run.Compiled{}, err
	}
	raw, err := json.Marshal(out.Spec)
	if err != nil {
		return run.Compiled{}, err
	}
	bakedSpec, err := s.renderer.Bake(ctx, "spec.run@1", raw)
	if err != nil {
		return run.Compiled{}, err
	}
	return run.Compiled{Spec: bakedSpec, Machines: out.Machines}, nil
}
