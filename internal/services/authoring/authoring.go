// Package authoring implements the Wizard's stateless assembly surface (ui
// AuthoringService). It NEVER persists: each RPC takes presets/drafts by VALUE,
// delegates to the pure domain transforms in internal/domain/dag (assemble,
// merge topology, materialize deployment, render/probe previews, compile the DAG
// blueprint) and assembles the proto response. Persistence is the catalog
// PresetService; execution is the RunService.
//
// RBAC: tenant-scoped reads. RPCs that carry a tenant_id require at least VIEWER
// on that tenant (the same idempotency_level=NO_SIDE_EFFECTS, read-only contract
// as catalog list/read). The topology/deployment-only RPCs (MergeTopology,
// ValidateTopology, ValidateDeploymentIntent, MaterializeDeploymentIntent) carry
// no tenant_id and are pure transforms — no authz gate.
package authoring

import (
	"context"
	"fmt"
	"strings"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"

	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// VersionResolver resolves the set of published stroppy versions (release tags)
// from an external source (GitHub releases). It is the seam through which the
// authoring backend augments its STATIC compatibility matrix with the LIVE list
// of versions that actually exist upstream. Implemented by
// internal/infrastructure/stroppyrelease.Client.
//
// Both methods MUST degrade gracefully: on a network failure they return
// whatever is cached (possibly empty) plus the error. The service treats any
// error as "resolver unavailable" and falls back to the pure matrix/min check,
// so the wizard never fails because GitHub is unreachable.
type VersionResolver interface {
	Versions(ctx context.Context) ([]string, error)
	Latest(ctx context.Context) (string, error)
}

// AuthoringService implements ui.AuthoringActions. It is thin and stateless — no
// repository, no transaction manager — holding only the authz gate and the
// (optional) published-version resolver.
type AuthoringService struct {
	*tracing.Entity
	authz    *authz.Authz
	versions VersionResolver // nil => version-published augmentation is skipped
}

var _ uiapi.AuthoringActions = (*AuthoringService)(nil)

// New builds the stateless authoring service. versions may be nil (the
// published-release augmentation is then skipped and only the pure
// domain/dag matrix + minimum-version check applies).
func New(logger *xlog.Logger, az *authz.Authz, versions VersionResolver) *AuthoringService {
	return &AuthoringService{
		Entity:   tracing.NewEntity(logger.AppendName("AuthoringService")),
		authz:    az,
		versions: versions,
	}
}

// requireViewer gates a tenant-scoped read on >= VIEWER for the tenant.
func (s *AuthoringService) requireViewer(ctx context.Context, tenantID *models.TenantId) error {
	return s.authz.Require(ctx, svcutil.CallerOf(ctx), tenantID, models.TenantMember_ROLE_VIEWER)
}

// augmentVersionDiagnostics adds a SEVERITY_WARNING when the workload's declared
// stroppy version is not among the live published GitHub releases. This AUGMENTS
// (does not replace) the pure domain matrix/min check that already produced
// diags. It is deliberately non-fatal:
//   - no resolver wired (nil)            → return diags unchanged
//   - empty / commit:<sha> dev version   → skip (matches the domain semver-floor
//     bypass; dev builds aren't published releases)
//   - resolver error (GitHub unreachable) → log + return diags unchanged; the
//     domain min-version check stands as the fallback so the wizard never fails
//     because GitHub is down
//   - version not in the published list   → append a WARNING diagnostic
//
// It reuses DIAGNOSTIC_CODE_STROPPY_VERSION_BELOW_MIN (the closest existing code;
// no proto change). The domain emits that code at ERROR for the semver floor;
// this adds it at WARNING for "not published", so the two are distinguishable by
// severity + message.
func (s *AuthoringService) augmentVersionDiagnostics(ctx context.Context, version string, diags []*uipb.Diagnostic) []*uipb.Diagnostic {
	if s.versions == nil {
		return diags
	}
	v := strings.TrimSpace(version)
	if v == "" || strings.HasPrefix(v, "commit:") {
		return diags
	}
	published, err := s.versions.Versions(ctx)
	if err != nil {
		// Degrade to the matrix/min check already present in diags.
		s.Logger().Warn("stroppy version resolver unavailable; using static check only", xlog.Error("error", err))
		return diags
	}
	if len(published) == 0 {
		// Nothing resolved (cold cache + soft failure) — don't warn on an empty set.
		return diags
	}
	if versionPublished(v, published) {
		return diags
	}
	return append(diags, &uipb.Diagnostic{
		Severity:  uipb.Severity_SEVERITY_WARNING,
		Code:      uipb.DiagnosticCode_DIAGNOSTIC_CODE_STROPPY_VERSION_BELOW_MIN,
		FieldPath: "workload.stroppy_version",
		Message:   fmt.Sprintf("stroppy version %s not found in published releases", v),
	})
}

// versionPublished reports whether want matches any published tag, comparing
// with a leading "v" tolerated on either side ("v4.2.1" == "4.2.1").
func versionPublished(want string, published []string) bool {
	norm := func(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "v") }
	w := norm(want)
	for _, p := range published {
		if norm(p) == w {
			return true
		}
	}
	return false
}

// AssembleFromPresets (step 1) merges a DatabasePreset + WorkloadPreset by value
// into an immutable TestPreset and returns it with diagnostics + provenance.
func (s *AuthoringService) AssembleFromPresets(ctx context.Context, req *uipb.AssembleFromPresetsRequest) (*uipb.TestPresetAssembly, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "AssembleFromPresets",
		func(ctx context.Context, _ trace.Span) (*uipb.TestPresetAssembly, error) {
			if err := s.requireViewer(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			preset, diags, prov, err := dagdomain.AssembleFromPresets(
				req.GetDatabasePreset(), req.GetWorkloadPreset(), req.GetProvider())
			if err != nil {
				return nil, err
			}
			diags = s.augmentVersionDiagnostics(ctx, preset.GetWorkload().GetStroppyVersion(), diags)
			return &uipb.TestPresetAssembly{TestPreset: preset, Diagnostics: diags, Provenance: prov}, nil
		})
}

// AssembleTestPreset (steps 2-3) re-assembles a draft after the user's edits.
func (s *AuthoringService) AssembleTestPreset(ctx context.Context, req *uipb.AssembleTestPresetRequest) (*uipb.TestPresetAssembly, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "AssembleTestPreset",
		func(ctx context.Context, _ trace.Span) (*uipb.TestPresetAssembly, error) {
			if err := s.requireViewer(ctx, req.GetDraft().GetTenantId()); err != nil {
				return nil, err
			}
			preset, diags, prov, err := dagdomain.AssembleTestPreset(req.GetDraft().GetTestPreset())
			if err != nil {
				return nil, err
			}
			diags = s.augmentVersionDiagnostics(ctx, preset.GetWorkload().GetStroppyVersion(), diags)
			return &uipb.TestPresetAssembly{TestPreset: preset, Diagnostics: diags, Provenance: prov}, nil
		})
}

// ValidateTestPreset returns the semantic diagnostics for a draft.
func (s *AuthoringService) ValidateTestPreset(ctx context.Context, req *uipb.TestPresetRef) (*uipb.DiagnosticsReport, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ValidateTestPreset",
		func(ctx context.Context, _ trace.Span) (*uipb.DiagnosticsReport, error) {
			if err := s.requireViewer(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			diags := dagdomain.ValidateTestPreset(req.GetTestPreset())
			diags = s.augmentVersionDiagnostics(ctx, req.GetTestPreset().GetWorkload().GetStroppyVersion(), diags)
			return &uipb.DiagnosticsReport{Diagnostics: diags}, nil
		})
}

// PreviewDatabaseRender renders the on-host config files for the preset.
func (s *AuthoringService) PreviewDatabaseRender(ctx context.Context, req *uipb.TestPresetRef) (*uipb.RenderPreview, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "PreviewDatabaseRender",
		func(ctx context.Context, _ trace.Span) (*uipb.RenderPreview, error) {
			if err := s.requireViewer(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			configs, err := dagdomain.PreviewDatabaseRender(req.GetTestPreset())
			if err != nil {
				return nil, err
			}
			return &uipb.RenderPreview{Configs: configs}, nil
		})
}

// ProbeWorkload statically checks the workload against the compatibility matrix,
// then AUGMENTS the result with a live GitHub-releases lookup: if the declared
// stroppy version is not among the published releases, a SEVERITY_WARNING is
// added. The augmentation is non-blocking — when the resolver is unavailable
// (GitHub unreachable) the probe degrades to the pure matrix/min check.
func (s *AuthoringService) ProbeWorkload(ctx context.Context, req *uipb.TestPresetRef) (*uipb.WorkloadProbe, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ProbeWorkload",
		func(ctx context.Context, _ trace.Span) (*uipb.WorkloadProbe, error) {
			if err := s.requireViewer(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			version, ok, diags := dagdomain.ProbeWorkload(req.GetTestPreset())
			diags = s.augmentVersionDiagnostics(ctx, version, diags)
			// ok stays a function of ERROR-severity diagnostics; the published-
			// release check only adds WARNINGs, so it never flips ok to false.
			return &uipb.WorkloadProbe{StroppyVersion: version, Ok: ok, Diagnostics: diags}, nil
		})
}

// PreviewWorkloadConfig previews the stroppy run config (JSON).
func (s *AuthoringService) PreviewWorkloadConfig(ctx context.Context, req *uipb.TestPresetRef) (*uipb.WorkloadConfigPreview, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "PreviewWorkloadConfig",
		func(ctx context.Context, _ trace.Span) (*uipb.WorkloadConfigPreview, error) {
			if err := s.requireViewer(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			cfg, err := dagdomain.PreviewWorkloadConfig(req.GetTestPreset())
			if err != nil {
				return nil, err
			}
			return &uipb.WorkloadConfigPreview{StroppyConfig: cfg}, nil
		})
}

// MergeTopology (step 4) merges the db + workload topologies with a user patch.
// Pure transform — the request carries no tenant_id, so no authz gate.
func (s *AuthoringService) MergeTopology(ctx context.Context, req *uipb.MergeTopologyRequest) (*uipb.TopologyMerge, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "MergeTopology",
		func(ctx context.Context, _ trace.Span) (*uipb.TopologyMerge, error) {
			topo, prov, diags := dagdomain.MergeTopology(
				req.GetDatabaseTopology(), req.GetWorkloadTopology(), req.GetUserPatch())
			return &uipb.TopologyMerge{Topology: topo, Provenance: prov, Diagnostics: diags}, nil
		})
}

// ValidateTopology returns the semantic diagnostics for a topology. Pure.
func (s *AuthoringService) ValidateTopology(ctx context.Context, req *domain.Topology) (*uipb.DiagnosticsReport, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ValidateTopology",
		func(ctx context.Context, _ trace.Span) (*uipb.DiagnosticsReport, error) {
			return &uipb.DiagnosticsReport{Diagnostics: dagdomain.ValidateTopology(req)}, nil
		})
}

// MaterializeDeploymentIntent (step 5) derives the provider deployment intent
// from the preset's topology. Pure transform — no tenant_id, no authz gate.
func (s *AuthoringService) MaterializeDeploymentIntent(ctx context.Context, req *uipb.MaterializeDeploymentIntentRequest) (*deployment.DeploymentIntent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "MaterializeDeploymentIntent",
		func(ctx context.Context, _ trace.Span) (*deployment.DeploymentIntent, error) {
			return dagdomain.MaterializeDeploymentIntent(req.GetTestPreset(), req.GetProvider())
		})
}

// ValidateDeploymentIntent returns the semantic diagnostics for an intent. Pure.
func (s *AuthoringService) ValidateDeploymentIntent(ctx context.Context, req *deployment.DeploymentIntent) (*uipb.DiagnosticsReport, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ValidateDeploymentIntent",
		func(ctx context.Context, _ trace.Span) (*uipb.DiagnosticsReport, error) {
			return &uipb.DiagnosticsReport{Diagnostics: dagdomain.ValidateDeploymentIntent(req)}, nil
		})
}

// CheckDeployment derives quota requests + best-effort feasibility for a draft.
func (s *AuthoringService) CheckDeployment(ctx context.Context, req *uipb.TestPresetRef) (*uipb.DeploymentCheck, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CheckDeployment",
		func(ctx context.Context, _ trace.Span) (*uipb.DeploymentCheck, error) {
			if err := s.requireViewer(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			quota, feasible, diags := dagdomain.CheckDeployment(req.GetTestPreset())
			return &uipb.DeploymentCheck{QuotaRequests: quota, Feasible: feasible, Diagnostics: diags}, nil
		})
}

// CompileTestPresetPreview (step 6) compiles the static execution DAG blueprint.
func (s *AuthoringService) CompileTestPresetPreview(ctx context.Context, req *uipb.TestPresetRef) (*primitive.Dag, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CompileTestPresetPreview",
		func(ctx context.Context, _ trace.Span) (*primitive.Dag, error) {
			if err := s.requireViewer(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			return dagdomain.CompileTestPresetPreview(req.GetTestPreset()), nil
		})
}
