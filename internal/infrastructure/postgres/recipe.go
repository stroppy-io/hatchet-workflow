package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

/*
	===== RecipeRepo =====

	Backs the DSL recipe store: one recipe_records row per (tenant_id, name,
	version) triple. name/version are mirrored into indexed columns (alongside
	the canonical protojson in data) so GetLatestByName can pick the newest
	version without decoding every row.

	source_ref is a plain column (NOT part of data): the git-backed
	BundleStore ref (see internal/services/catalog.EncodeGitCommitRef) this
	row's file bytes actually live at, one repo per (tenant, recipe name) —
	see internal/services/recipe's recipeBundleIdentity/healRecipeSourceRef.
	"" means a pre-migration legacy row whose bytes are still embedded in
	data.bundle.files; every method here returns source_ref alongside the
	decoded record so the service layer can heal it lazily on read, exactly
	like catalog.healSourceRef does for catalog_entries.

	The service-layer port (internal/services/recipe, Task 4) is the canonical
	interface this repo satisfies; it is not asserted here since that package
	does not exist yet.
*/

// Recipes returns the recipe.RecipeRepo.
func (s *Store) Recipes() *RecipeRepo { return &RecipeRepo{db: s.db} }

type RecipeRepo struct{ db *DB }

// Create persists rec's row envelope plus sourceRef, the ref its bundle
// bytes were just written under (recipe.Service always resolves this via
// BundleStore.Write before calling Create — see recipe.go's CreateRecipe).
func (r *RecipeRepo) Create(ctx context.Context, rec *models.RecipeRecord, sourceRef string) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	err = r.db.q().CreateRecipeRecord(ctx, dbgen.CreateRecipeRecordParams{
		ID:        rec.GetEntity().GetId(),
		TenantID:  rec.GetEntity().GetTenantId(),
		Name:      rec.GetEntity().GetName(),
		Version:   rec.GetVersion(),
		SourceRef: sourceRef,
		Data:      data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("recipe", "recipe already exists")
		}
		return err
	}
	return nil
}

// Get returns the tenant-scoped row and its source_ref ("" for a
// pre-migration legacy row — see the package doc above).
func (r *RecipeRepo) Get(ctx context.Context, tenantID, id string) (*models.RecipeRecord, string, error) {
	row, err := r.db.q().GetRecipeRecord(ctx, dbgen.GetRecipeRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, "", translatePgErr("recipe", err)
	}
	rec := &models.RecipeRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, "", err
	}
	return rec, row.SourceRef, nil
}

// List returns every recipe version for the tenant, ordered by name then
// version (per ListRecipeRecordsSQL), paired with each row's source_ref.
func (r *RecipeRepo) List(ctx context.Context, tenantID string) ([]*models.RecipeRecord, []string, error) {
	rows, err := r.db.q().ListRecipeRecords(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*models.RecipeRecord, 0, len(rows))
	refs := make([]string, 0, len(rows))
	for _, row := range rows {
		rec := &models.RecipeRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, nil, err
		}
		out = append(out, rec)
		refs = append(refs, row.SourceRef)
	}
	return out, refs, nil
}

// GetLatestByName returns the highest-version recipe row for name in tenant,
// and its source_ref.
func (r *RecipeRepo) GetLatestByName(ctx context.Context, tenantID, name string) (*models.RecipeRecord, string, error) {
	row, err := r.db.q().GetLatestRecipeRecordByName(ctx, dbgen.GetLatestRecipeRecordByNameParams{TenantID: tenantID, Name: name})
	if err != nil {
		return nil, "", translatePgErr("recipe", err)
	}
	rec := &models.RecipeRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, "", err
	}
	return rec, row.SourceRef, nil
}

func (r *RecipeRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteRecipeRecord(ctx, dbgen.DeleteRecipeRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("recipe", "recipe not found")
	}
	return nil
}

// UpdateSourceRef persists a healed/materialized sourceRef onto an existing
// row, together with rec's refreshed envelope (the caller passes rec with
// Bundle.Files already stripped — see recipe.go's healRecipeSourceRef) so
// the row's git ref and its protojson blob change atomically. Used ONLY by
// the lazy migration path; a normal Create/Get/List round-trip never calls
// this.
func (r *RecipeRepo) UpdateSourceRef(ctx context.Context, rec *models.RecipeRecord, sourceRef string) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateRecipeRecordSourceRef(ctx, dbgen.UpdateRecipeRecordSourceRefParams{
		SourceRef: sourceRef,
		Data:      data,
		TenantID:  rec.GetEntity().GetTenantId(),
		ID:        rec.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("recipe", "recipe not found")
	}
	return nil
}
