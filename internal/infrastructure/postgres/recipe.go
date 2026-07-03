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

	The service-layer port (internal/services/recipe, Task 4) is the canonical
	interface this repo satisfies; it is not asserted here since that package
	does not exist yet.
*/

// Recipes returns the recipe.RecipeRepo.
func (s *Store) Recipes() *RecipeRepo { return &RecipeRepo{db: s.db} }

type RecipeRepo struct{ db *DB }

func (r *RecipeRepo) Create(ctx context.Context, rec *models.RecipeRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	err = r.db.q().CreateRecipeRecord(ctx, dbgen.CreateRecipeRecordParams{
		ID:       rec.GetEntity().GetId(),
		TenantID: rec.GetEntity().GetTenantId(),
		Name:     rec.GetEntity().GetName(),
		Version:  rec.GetVersion(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("recipe", "recipe already exists")
		}
		return err
	}
	return nil
}

func (r *RecipeRepo) Get(ctx context.Context, tenantID, id string) (*models.RecipeRecord, error) {
	row, err := r.db.q().GetRecipeRecord(ctx, dbgen.GetRecipeRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("recipe", err)
	}
	rec := &models.RecipeRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns every recipe version for the tenant, ordered by name then
// version (per ListRecipeRecordsSQL).
func (r *RecipeRepo) List(ctx context.Context, tenantID string) ([]*models.RecipeRecord, error) {
	rows, err := r.db.q().ListRecipeRecords(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]*models.RecipeRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.RecipeRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// GetLatestByName returns the highest-version recipe row for name in tenant.
func (r *RecipeRepo) GetLatestByName(ctx context.Context, tenantID, name string) (*models.RecipeRecord, error) {
	row, err := r.db.q().GetLatestRecipeRecordByName(ctx, dbgen.GetLatestRecipeRecordByNameParams{TenantID: tenantID, Name: name})
	if err != nil {
		return nil, translatePgErr("recipe", err)
	}
	rec := &models.RecipeRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
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
