package postgres

// Package doc: RunRepo backs models.Run — the run-model rewrite's storage
// (SP-E). It is added ADDITIVELY alongside TestRunRepo/test_run_records: the
// write path does not switch to run_records until SP-E Task 3, so this repo
// is unused (and run_records stays empty) until then. List is implemented in
// run_list.go (a near-duplicate of test_run_list.go, see that file's package
// doc and the SP-E plan for why the duplication is deliberate).

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// Runs returns the RunRepo (the runs-table repo + overview run reader for
// models.Run — replaces TestRuns()/TestRunRepo for the live RunRecipeWorkflow
// path; see run_store.go's package doc).
func (s *Store) Runs() *RunRepo { return &RunRepo{db: s.db} }

type RunRepo struct{ db *DB }

func (r *RunRepo) Create(ctx context.Context, run *models.Run) error {
	data, err := marshal(run)
	if err != nil {
		return err
	}
	err = r.db.q().CreateRunRecord(ctx, dbgen.CreateRunRecordParams{
		ID:       run.GetEntity().GetId(),
		TenantID: run.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("run", "run already exists")
		}
		return err
	}
	return nil
}

func (r *RunRepo) Get(ctx context.Context, tenantID, id string) (*models.Run, error) {
	row, err := r.db.q().GetRunRecord(ctx, dbgen.GetRunRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("run", err)
	}
	rec := &models.Run{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List is implemented in run_list.go: it honors the full ListTestRunsRequest
// (filters/facets/sort/pagination) against postgres.

func (r *RunRepo) Update(ctx context.Context, run *models.Run) error {
	data, err := marshal(run)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateRunRecord(ctx, dbgen.UpdateRunRecordParams{
		Data:     data,
		TenantID: run.GetEntity().GetTenantId(),
		ID:       run.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("run", "run not found")
	}
	return nil
}

func (r *RunRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteRunRecord(ctx, dbgen.DeleteRunRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("run", "run not found")
	}
	return nil
}

// Exists reports whether tenantID/runID resolves to a run: a cross-tenant or
// unknown id is reported as not-found so observability data never leaks
// across tenants (mirrors TestRunRepo.Exists).
func (r *RunRepo) Exists(ctx context.Context, tenantID, runID string) error {
	_, err := r.db.q().ExistsRunRecord(ctx, dbgen.ExistsRunRecordParams{TenantID: tenantID, ID: runID})
	if err != nil {
		return translatePgErr("run", err)
	}
	return nil
}
