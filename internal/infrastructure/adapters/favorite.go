package adapters

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/favorite"
)

// FavoriteTargetRepos bundles the per-kind read ports the favorite target
// resolver dispatches to. Each Getter returns the target's storage envelope
// (common.Entity) carrying its tenant_id, or derrors.ErrNotFound when the row is
// absent. The gormstore repos satisfy these once injected:
//
//	DatabasePresets -> *DatabasePresetRepo.Get(ctx, tenantID, id)   (entity)
//	WorkloadPresets -> *WorkloadPresetRepo.Get(ctx, tenantID, id)   (entity)
//	TestPresets     -> *TestPresetRepo.Get(ctx, tenantID, id)       (entity)
//	TestRuns        -> *TestRunRepo.Get(ctx, id)                    (entity, by id)
//	Suites          -> *SuiteRepo.Get(ctx, tenantID, id)           (entity)
//	SuiteRuns       -> *SuiteRunRepo.Get(ctx, id)                   (entity, by id)
//	Recipes         -> *RecipeRepo.Get(ctx, tenantID, id)           (entity)
//
// A thin per-kind adapter (closure) maps each repo's record to its
// GetEntity(); the wiring layer supplies these so the resolver stays
// record-shape-agnostic.
type FavoriteTargetRepos struct {
	DatabasePresets EntityGetter
	WorkloadPresets EntityGetter
	TestPresets     EntityGetter
	TestRuns        EntityGetter
	Suites          EntityGetter
	SuiteRuns       EntityGetter
	Recipes         EntityGetter
}

// EntityGetter resolves a target's common.Entity by (tenant, id), returning
// derrors.ErrNotFound when absent. tenantID is honored by tenant-partitioned
// tables (presets, suites) and ignored by by-id tables (test_run, suite_run).
type EntityGetter interface {
	GetEntity(ctx context.Context, tenantID, id string) (*common.Entity, error)
}

// EntityGetterFunc adapts a function to EntityGetter so the wiring layer can pass
// a closure over a typed gormstore repo (e.g. mapping TestRunRecord ->
// GetEntity()).
type EntityGetterFunc func(ctx context.Context, tenantID, id string) (*common.Entity, error)

// GetEntity calls the underlying function.
func (f EntityGetterFunc) GetEntity(ctx context.Context, tenantID, id string) (*common.Entity, error) {
	return f(ctx, tenantID, id)
}

// FavoriteTargetResolver implements favorite.TargetResolver: it resolves a
// favoritable target's entity across the per-kind tables behind the FavoriteKind
// discriminator.
type FavoriteTargetResolver struct{ repos FavoriteTargetRepos }

var _ favorite.TargetResolver = (*FavoriteTargetResolver)(nil)

// NewFavoriteTargetResolver builds the resolver over the per-kind repos.
func NewFavoriteTargetResolver(repos FavoriteTargetRepos) *FavoriteTargetResolver {
	return &FavoriteTargetResolver{repos: repos}
}

// Resolve returns the target's Entity when it exists; derrors.ErrNotFound for an
// unknown kind or absent row.
func (r *FavoriteTargetResolver) Resolve(ctx context.Context, kind common.FavoriteKind, tenantID, targetID string) (*common.Entity, error) {
	var getter EntityGetter
	switch kind {
	case common.FavoriteKind_FAVORITE_KIND_DATABASE_PRESET:
		getter = r.repos.DatabasePresets
	case common.FavoriteKind_FAVORITE_KIND_WORKLOAD_PRESET:
		getter = r.repos.WorkloadPresets
	case common.FavoriteKind_FAVORITE_KIND_TEST_PRESET:
		getter = r.repos.TestPresets
	case common.FavoriteKind_FAVORITE_KIND_TEST_RUN:
		getter = r.repos.TestRuns
	case common.FavoriteKind_FAVORITE_KIND_SUITE:
		getter = r.repos.Suites
	case common.FavoriteKind_FAVORITE_KIND_SUITE_RUN:
		getter = r.repos.SuiteRuns
	case common.FavoriteKind_FAVORITE_KIND_RECIPE:
		getter = r.repos.Recipes
	default:
		return nil, derrors.Invalid("kind", "unsupported favorite kind")
	}
	if getter == nil {
		return nil, derrors.NotFound("favorite_target", "target kind is not resolvable")
	}
	entity, err := getter.GetEntity(ctx, tenantID, targetID)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, derrors.NotFound("favorite_target", "target not found")
	}
	return entity, nil
}

// EntityGetterFromRecord is a wiring helper: it maps any record getter whose
// record exposes GetEntity() to an EntityGetter, so the integration layer can
// plug a typed gormstore repo into FavoriteTargetRepos without writing the
// closure by hand (e.g.
// EntityGetterFromRecord(func(ctx, id) (*models.TestRunRecord, error){...})).
func EntityGetterFromRecord[T interface{ GetEntity() *common.Entity }](get func(ctx context.Context, id string) (T, error)) EntityGetterFunc {
	return func(ctx context.Context, _ string, id string) (*common.Entity, error) {
		rec, err := get(ctx, id)
		if err != nil {
			return nil, err
		}
		return rec.GetEntity(), nil
	}
}

var _ = EntityGetterFromRecord[*models.TestRunRecord]
