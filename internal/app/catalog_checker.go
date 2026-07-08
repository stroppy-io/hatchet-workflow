package app

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	dslsvc "github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// newCatalogChecker adapts dslService into catalog.Checker (SP-B Task 9): a
// KIND_WORKFLOW bundle (cluster.yaml + optional providers/) checks through
// the exact same DslService.CheckBundle check-mode compile pipeline the live
// editor and stored-recipe Create/CheckRecipe already use. A KIND_PROVIDER
// bundle has no cluster.yaml to compile — the catalog's flat provider layout
// is just manifest.yaml (+ module/*.tf, see catalog.Deps.BuiltinProviders) —
// so it is checked by decoding manifest.yaml alone (ast.DecodeProviderManifest),
// the same structural validation catalog.Service.summaryFor already relies
// on to derive a provider entry's Summary. This intentionally does not
// re-derive the module's Terraform params/ext schema (that requires
// materializing the module to a temp dir under a providers/<slug>/ prefix,
// dsl-package-internal machinery not exported for reuse here); a malformed
// module surfaces later, when a recipe actually resolves the provider via
// CatalogProviderResolver and compiles against it.
func newCatalogChecker(dslService *dslsvc.DslService) func(ctx context.Context, kind catalogpb.Kind, files map[string][]byte) ([]*dslpb.Diagnostic, error) {
	return func(ctx context.Context, kind catalogpb.Kind, files map[string][]byte) ([]*dslpb.Diagnostic, error) {
		if kind == catalogpb.Kind_KIND_PROVIDER {
			_, diags := ast.DecodeProviderManifest("manifest.yaml", files["manifest.yaml"])
			return toProtoDiagnostics(diags), nil
		}
		return dslService.CheckBundle(ctx, files)
	}
}

// toProtoDiagnostics maps a diag.List to the wire Diagnostic shape. A
// package-local mirror of internal/services/dsl's own unexported
// toProtoDiagnostics — that package is kept a pure compiler with no catalog
// dependency (see its ProviderResolver doc comment), so this small mapping
// is duplicated here rather than exported across the package boundary.
func toProtoDiagnostics(diags diag.List) []*dslpb.Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	out := make([]*dslpb.Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, &dslpb.Diagnostic{
			Severity: toProtoSeverity(d.Severity),
			Path:     d.Path,
			Line:     clampUint32(d.Pos.Line),
			Col:      clampUint32(d.Pos.Col),
			Message:  d.Message,
			Module:   d.Module,
		})
	}
	return out
}

func toProtoSeverity(s diag.Severity) dslpb.Severity {
	switch s {
	case diag.Error:
		return dslpb.Severity_SEVERITY_ERROR
	case diag.Warning:
		return dslpb.Severity_SEVERITY_WARNING
	default:
		return dslpb.Severity_SEVERITY_UNSPECIFIED
	}
}

func clampUint32(v int) uint32 {
	if v < 0 {
		return 0
	}
	return uint32(v) //nolint:gosec // guarded non-negative above; a diag.Pos line/col never approaches uint32's range.
}
