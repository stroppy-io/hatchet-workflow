// Package catalog holds the SP-B catalog domain: provider and workflow
// objects promoted to first-class catalog entries at two levels
// (LEVEL_INSTANCE and LEVEL_ORG), each carrying an Origin (native, linked,
// or forked) for lineage.
package catalog

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
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

// Checker runs the DSL check-mode compile pipeline over a bundle's files and
// returns wire-shaped diagnostics, exactly like recipe.Service's own Checker
// dependency — except it also takes kind, since a catalog bundle can be
// either a KIND_PROVIDER manifest+module or a KIND_WORKFLOW cluster/workflow
// bundle, and the two compile through different paths. It never returns a Go
// error for a problem in the bundle itself — a non-nil error means a genuine
// transport/compute failure and is mapped via utils.MapErr by the caller.
type Checker func(ctx context.Context, kind catalogpb.Kind, files map[string][]byte) ([]*dslpb.Diagnostic, error)

// Deps bundles every dependency CatalogService's constructor needs.
type Deps struct {
	// Entries persists catalogpb.CatalogEntry rows.
	Entries CatalogEntryRepo
	// Bundles stores/reads/forks a catalog entry's raw files (SP-C seam — see
	// bundlestore.go's BundleStore doc).
	Bundles BundleStore
	// Check runs the DSL check-mode compile pipeline over a bundle's files.
	Check Checker
	// Authn resolves the caller's verified access claims from the request
	// context.
	Authn utils.Authn
	// BuiltinProviders is slug -> files for the providers shipped with the
	// platform, seeded as LEVEL_INSTANCE rows on first SeedOrgCatalog call
	// (a later task — this task only declares the field so Deps' shape is
	// stable for callers that construct it).
	BuiltinProviders map[string]map[string][]byte
}

// Service implements catalogpb.CatalogServiceServer: the connect-RPC surface
// over the catalog storage model (instance CRUD, org CRUD split per kind,
// link, check — see protocols/cloud/v1/catalog/service.proto's file doc for
// why the org-level RPC surface is split per kind while the Go
// implementation behind each pair is not).
type Service struct {
	*catalogpb.UnimplementedCatalogServiceServer
	d Deps
}

var _ catalogpb.CatalogServiceServer = (*Service)(nil)

// NewService constructs the CatalogService connect handler.
func NewService(d Deps) *Service {
	return &Service{UnimplementedCatalogServiceServer: &catalogpb.UnimplementedCatalogServiceServer{}, d: d}
}

// requireLevel validates tenantID against level's scoping contract:
// LEVEL_ORG rows must be tenant-scoped, LEVEL_INSTANCE rows must not be. RBAC
// is enforced upstream by the auth interceptor, but every handler still
// needs this to scope its repo call correctly — mirrors recipe.
// requireTenant's role, generalized over both levels.
func requireLevel(level catalogpb.Level, tenantID string) error {
	if level == catalogpb.Level_LEVEL_ORG && tenantID == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if level == catalogpb.Level_LEVEL_INSTANCE && tenantID != "" {
		return status.Error(codes.InvalidArgument, "tenant_id must be empty for instance-level entries")
	}
	return nil
}

// nextVersion computes the version a Create/Link call stamps on a new entry:
// one past the (level, tenantID, kind, slug) scope's latest existing
// version, or 1 when the scope has no entry of that slug yet
// (GetLatestBySlug returns NotFound). Mirrors recipe.Service.nextVersion.
func (s *Service) nextVersion(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string) (uint32, error) {
	latest, err := s.d.Entries.GetLatestBySlug(ctx, level, tenantID, kind, slug)
	if err != nil {
		if derrors.IgnoreNotFound(err) == nil {
			return 1, nil
		}
		return 0, utils.MapErr(err)
	}
	return latest.GetVersion() + 1, nil
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
