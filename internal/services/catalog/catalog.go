// Package catalog holds the SP-B catalog domain: provider and workflow
// objects promoted to first-class catalog entries at two levels
// (LEVEL_INSTANCE and LEVEL_ORG), each carrying an Origin (native, linked,
// or forked) for lineage.
package catalog

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
	// platform (e.g. "docker", "yandex"), seeded as LEVEL_INSTANCE
	// KIND_PROVIDER rows by ensureBuiltinInstanceEntries.
	BuiltinProviders map[string]map[string][]byte
	// BuiltinWorkflows is slug -> files for the example/starter workflow
	// bundles shipped with the platform (e.g. "postgres-ha", sourced from
	// examples/dsl/**), seeded as LEVEL_INSTANCE KIND_WORKFLOW rows by
	// ensureBuiltinInstanceEntries so a fresh install has at least one
	// launchable recipe in its catalog.
	BuiltinWorkflows map[string]map[string][]byte
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
	summary.ProviderSlug = providerSlug(doc.Provider.Use)
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
// providerSlug strips a catalog pin ("slug@version", SP-B §B5) down to its
// bare slug for display fields such as CatalogEntry_Summary.provider_slug,
// which is meant to read as the provider's identity, not a specific pinned
// version. Mirrors internal/services/dsl.parseProviderUse's permissive
// splitting (an unparseable/non-numeric "@..." suffix is treated as part of
// the slug itself) without importing that unexported helper across packages;
// a bare, unpinned slug (the legacy path, which never contains "@") passes
// through unchanged.
func providerSlug(use string) string {
	i := strings.LastIndex(use, "@")
	if i < 0 {
		return use
	}
	if _, err := strconv.ParseUint(use[i+1:], 10, 32); err != nil {
		return use
	}
	return use[:i]
}

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

// ForkEntry materializes an org-owned copy of a LINKED entry on first edit:
// bumps version, sets origin=FORKED, and calls Bundles.Fork against the
// bundle the LINKED row's files live in. A LINKED row seeded by
// SeedOrgCatalog carries no source_ref of its own (see seed.go's doc) — its
// files live at the *instance* row's source_ref, resolved here via
// source_entry_id; a LINKED row created by LinkInstanceEntry does carry a
// copy of that same ref directly. Either way, source_entry_id is preserved
// on the forked row for future diff/re-sync tooling. UpdateOrgProvider/
// UpdateOrgWorkflow do NOT call this internally (see service.go's
// updateOrgEntry doc) — it is exposed standalone so an explicit "customize
// before editing" UI action can fork a row without submitting a file diff
// yet. Calling ForkEntry on a NATIVE or already-FORKED row is a no-op: both
// are already org-owned, so the entry is returned unchanged.
func (s *Service) ForkEntry(ctx context.Context, tenantID, orgEntryID string) (*catalogpb.CatalogEntry, error) {
	entry, err := s.d.Entries.Get(ctx, LevelOrg, tenantID, orgEntryID)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if entry.GetOrigin() != OriginLinked {
		return entry, nil
	}
	sourceRef := entry.GetSourceRef()
	if sourceRef == "" {
		// LINKED row with no own source_ref yet — read the instance row's ref.
		src, err := s.d.Entries.Get(ctx, LevelInstance, "", entry.GetSourceEntryId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		sourceRef = src.GetSourceRef()
	}
	forkedRef, err := s.d.Bundles.Fork(ctx, sourceRef)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	forked, ok := proto.Clone(entry).(*catalogpb.CatalogEntry)
	if !ok {
		return nil, status.Error(codes.Internal, "fork: cloned entry has unexpected type")
	}
	// entry.GetVersion()+1 is not safe to stamp directly: a prior fork (or a
	// native Create) may already occupy that (level, tenant, kind, slug)
	// version, which would collide against uq_catalog_entries_scope_slug_version.
	// nextVersion computes the actual next-free version for this scope instead.
	nextVer, err := s.nextVersion(ctx, LevelOrg, tenantID, entry.GetKind(), entry.GetSlug())
	if err != nil {
		return nil, err
	}
	forked.Entity.Id = uuid.NewString()
	forked.Version = nextVer
	forked.Origin = OriginForked
	forked.SourceRef = forkedRef
	// SourceEntryId is preserved (cloned from entry) for lineage.
	if err := s.d.Entries.Create(ctx, forked); err != nil {
		return nil, utils.MapErr(err)
	}
	return forked, nil
}
