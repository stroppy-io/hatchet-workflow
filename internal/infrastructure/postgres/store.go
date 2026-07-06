package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
)

// Store is the Postgres-backed persistence facade. It exposes typed repos that
// satisfy the service port interfaces. All repos issue SQL through the
// sqld-generated query set bound to db.TxDB (the ctx-aware pgtx executor), so a
// repo call made inside the service's tx.Do* runs in that same transaction;
// outside one it runs on the pool (auto-commit).
type Store struct{ db *DB }

// New wraps a *DB. It does not touch the database.
func New(db *DB) *Store { return &Store{db: db} }

// Store returns the typed-repo facade over this connection bundle.
func (db *DB) Store() *Store { return &Store{db: db} }

// TestRuns returns the TestRunRepo (also the overview TestRunReader; the
// recipe/DSL path reads and writes runs through the same repo).
func (s *Store) TestRuns() *TestRunRepo { return &TestRunRepo{db: s.db} }

/*
	===== TestRunRepo =====

	Backs test_run_overview.TestRunReader (the old test_run.TestRunRepo port it
	also satisfied was deleted along with the test_run service).
*/

type TestRunRepo struct{ db *DB }

var _ test_run_overview.TestRunReader = (*TestRunRepo)(nil)

func (r *TestRunRepo) Create(ctx context.Context, run *models.TestRunRecord) error {
	data, err := marshal(run)
	if err != nil {
		return err
	}
	err = r.db.q().CreateTestRunRecord(ctx, dbgen.CreateTestRunRecordParams{
		ID:       run.GetEntity().GetId(),
		TenantID: run.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("test_run", "run already exists")
		}
		return err
	}
	return nil
}

func (r *TestRunRepo) Get(ctx context.Context, tenantID, id string) (*models.TestRunRecord, error) {
	row, err := r.db.q().GetTestRunRecord(ctx, dbgen.GetTestRunRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("test_run", err)
	}
	rec := &models.TestRunRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List is implemented in test_run_list.go: it honors the full
// ListTestRunsRequest (filters/facets/sort/pagination) against postgres.

func (r *TestRunRepo) Update(ctx context.Context, run *models.TestRunRecord) error {
	data, err := marshal(run)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateTestRunRecord(ctx, dbgen.UpdateTestRunRecordParams{
		Data:     data,
		TenantID: run.GetEntity().GetTenantId(),
		ID:       run.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("test_run", "run not found")
	}
	return nil
}

func (r *TestRunRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteTestRunRecord(ctx, dbgen.DeleteTestRunRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("test_run", "run not found")
	}
	return nil
}

// Exists satisfies test_run_overview.TestRunReader: a cross-tenant or unknown id
// is reported as not-found so observability data never leaks across tenants.
func (r *TestRunRepo) Exists(ctx context.Context, tenantID, runID string) error {
	_, err := r.db.q().ExistsTestRunRecord(ctx, dbgen.ExistsTestRunRecordParams{TenantID: tenantID, ID: runID})
	if err != nil {
		return translatePgErr("test_run", err)
	}
	return nil
}
