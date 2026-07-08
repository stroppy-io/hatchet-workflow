// Package catalog holds the SP-B catalog domain: provider and workflow
// objects promoted to first-class catalog entries at two levels
// (LEVEL_INSTANCE and LEVEL_ORG), each carrying an Origin (native, linked,
// or forked) for lineage.
package catalog

import (
	"context"

	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// Level, Kind, and Origin are re-exported from catalogpb so callers outside
// this package can spell them as catalog.Level / catalog.Kind /
// catalog.Origin instead of reaching into the generated proto package.
type (
	Level  = catalogpb.Level
	Kind   = catalogpb.Kind
	Origin = catalogpb.Origin
)

const (
	LevelInstance = catalogpb.Level_LEVEL_INSTANCE
	LevelOrg      = catalogpb.Level_LEVEL_ORG

	KindProvider = catalogpb.Kind_KIND_PROVIDER
	KindWorkflow = catalogpb.Kind_KIND_WORKFLOW

	OriginNative = catalogpb.Origin_ORIGIN_NATIVE
	OriginLinked = catalogpb.Origin_ORIGIN_LINKED
	OriginForked = catalogpb.Origin_ORIGIN_FORKED
)

// clusterFile is the bundle-relative path of the DSL cluster document,
// matching internal/services/recipe's own constant (unexported there, so
// re-declared here rather than imported).
const clusterFile = "cluster.yaml"

// CatalogEntryRepo persists catalogpb.CatalogEntry rows. Every method is
// scoped by (level, tenantID) — tenantID MUST be empty for LEVEL_INSTANCE and
// non-empty for LEVEL_ORG (service layer validates via requireLevel, mirrors
// recipe.requireTenant).
type CatalogEntryRepo interface {
	Create(ctx context.Context, e *catalogpb.CatalogEntry) error
	Get(ctx context.Context, level catalogpb.Level, tenantID, id string) (*catalogpb.CatalogEntry, error)
	List(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind) ([]*catalogpb.CatalogEntry, error)
	GetLatestBySlug(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string) (*catalogpb.CatalogEntry, error)
	GetBySlugVersion(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string, version uint32) (*catalogpb.CatalogEntry, error)
	ListBySource(ctx context.Context, sourceEntryID string) ([]*catalogpb.CatalogEntry, error)
	Update(ctx context.Context, e *catalogpb.CatalogEntry) error
	Delete(ctx context.Context, level catalogpb.Level, tenantID, id string) error
}

// DeriveWorkflowSummary computes a KIND_WORKFLOW CatalogEntry's denormalized
// Summary: compiles reflects whether the Checker run found any diagnostic,
// and provider_slug/machine_group_count/service_count come from a light,
// error-tolerant parse of the bundle's cluster.yaml. A missing or
// unparseable cluster.yaml leaves the counts at their zero value rather than
// failing. Mirrors internal/services/recipe/recipe.go's deriveSummary.
func DeriveWorkflowSummary(files map[string][]byte, diags []*dslpb.Diagnostic) *catalogpb.CatalogEntry_Summary {
	summary := &catalogpb.CatalogEntry_Summary{Compiles: len(diags) == 0}
	src, ok := files[clusterFile]
	if !ok {
		return summary
	}
	providerUse := peekProviderUse(src)
	doc, _ := ast.DecodeCluster(clusterFile, src, providerUse)
	if doc == nil {
		return summary
	}
	summary.ProviderSlug = doc.Provider.Use
	summary.MachineGroupCount = uint32(len(doc.Machines)) //nolint:gosec // bundle sizes are bounded; a machine-group count never approaches uint32's range.
	summary.ServiceCount = uint32(len(doc.Services))      //nolint:gosec // same bound as above.
	return summary
}

// DeriveProviderSummary computes a KIND_PROVIDER CatalogEntry's denormalized
// Summary from its decoded manifest.yaml: provides mirrors the manifest's
// declared capabilities. A nil manifest yields a zero-value Summary rather
// than failing.
func DeriveProviderSummary(manifest *ast.ProviderManifest) *catalogpb.CatalogEntry_Summary {
	if manifest == nil {
		return &catalogpb.CatalogEntry_Summary{}
	}
	return &catalogpb.CatalogEntry_Summary{Provides: append([]string(nil), manifest.Provides...)}
}

// peekProviderUse best-effort reads cluster.yaml's provider.use field
// without going through ast.DecodeCluster's strict decode (which itself
// needs the provider name up front — see its providerKey parameter). A
// missing/unparseable cluster.yaml, or one with no provider.use, yields ""
// and DeriveWorkflowSummary simply gets a doc with an empty Provider.Use; it
// never treats this as a Go error. This is a catalog-owned copy of the same
// helper recipe.go and internal/services/dsl each carry their own unexported
// copy of (recipe.go's doc comment explains why it is not shared).
func peekProviderUse(clusterSrc []byte) string {
	if len(clusterSrc) == 0 {
		return ""
	}
	var doc struct {
		Provider struct {
			Use string `yaml:"use"`
		} `yaml:"provider"`
	}
	if err := yaml.Unmarshal(clusterSrc, &doc); err != nil {
		return ""
	}
	return doc.Provider.Use
}
