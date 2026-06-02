package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
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

// TestRuns returns the test_run.TestRunRepo (also the overview TestRunReader).
func (s *Store) TestRuns() *TestRunRepo { return &TestRunRepo{db: s.db} }

// Drafts returns the test_wizard.DraftRepo.
func (s *Store) Drafts() *DraftRepo { return &DraftRepo{db: s.db} }

// Presets returns the preset repo backing PresetRepo/PresetReader/PresetSaver.
func (s *Store) Presets() *PresetRepo { return &PresetRepo{db: s.db} }

/*
	===== TestRunRepo =====

	Backs test_run.TestRunRepo and test_run_overview.TestRunReader.
*/

type TestRunRepo struct{ db *DB }

var (
	_ test_run.TestRunRepo            = (*TestRunRepo)(nil)
	_ test_run_overview.TestRunReader = (*TestRunRepo)(nil)
)

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

// List returns every run for the request tenant, ignoring filters/sort/paging
// (demo). The next-page token is always empty.
func (r *TestRunRepo) List(ctx context.Context, query *api.ListTestRunsRequest) ([]*models.TestRunRecord, string, error) {
	rows, err := r.db.q().ListTestRunRecords(ctx, query.GetTenantId())
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.TestRunRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.TestRunRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	return out, "", nil
}

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

/*
	===== DraftRepo (test_wizard.DraftRepo) =====
*/

type DraftRepo struct{ db *DB }

var _ test_wizard.DraftRepo = (*DraftRepo)(nil)

func (r *DraftRepo) Create(ctx context.Context, draft *models.TestWizardDraftRecord) error {
	data, err := marshal(draft)
	if err != nil {
		return err
	}
	err = r.db.q().CreateTestWizardDraft(ctx, dbgen.CreateTestWizardDraftParams{
		ID:       draft.GetEntity().GetId(),
		TenantID: draft.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("test_wizard_draft", "draft already exists")
		}
		return err
	}
	return nil
}

func (r *DraftRepo) Get(ctx context.Context, tenantID, id string) (*models.TestWizardDraftRecord, error) {
	row, err := r.db.q().GetTestWizardDraft(ctx, dbgen.GetTestWizardDraftParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("test_wizard_draft", err)
	}
	rec := &models.TestWizardDraftRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns every draft for the tenant, ignoring filter/sort/page (demo).
func (r *DraftRepo) List(ctx context.Context, tenantID string, _ *commonpb.EntityFilter, _ *commonpb.EntitySort, _ *commonpb.Page) ([]*models.TestWizardDraftRecord, string, error) {
	rows, err := r.db.q().ListTestWizardDrafts(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.TestWizardDraftRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.TestWizardDraftRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	return out, "", nil
}

func (r *DraftRepo) Update(ctx context.Context, draft *models.TestWizardDraftRecord) error {
	data, err := marshal(draft)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateTestWizardDraft(ctx, dbgen.UpdateTestWizardDraftParams{
		Data:     data,
		TenantID: draft.GetEntity().GetTenantId(),
		ID:       draft.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("test_wizard_draft", "draft not found")
	}
	return nil
}

func (r *DraftRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteTestWizardDraft(ctx, dbgen.DeleteTestWizardDraftParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("test_wizard_draft", "draft not found")
	}
	return nil
}

/*
	===== PresetRepo =====

	One preset table backs three ports at once:
	  - test_run.PresetRepo      (CreateTestPreset)
	  - test_wizard.PresetReader (Get)
	  - test_wizard.PresetSaver  (Save)
*/

type PresetRepo struct{ db *DB }

var (
	_ test_run.PresetRepo      = (*PresetRepo)(nil)
	_ test_wizard.PresetReader = (*PresetRepo)(nil)
	_ test_wizard.PresetSaver  = (*PresetRepo)(nil)
)

func (r *PresetRepo) CreateTestPreset(ctx context.Context, preset *models.TestPresetRecord) error {
	return r.create(ctx, preset)
}

func (r *PresetRepo) create(ctx context.Context, preset *models.TestPresetRecord) error {
	data, err := marshal(preset)
	if err != nil {
		return err
	}
	err = r.db.q().CreateTestPresetRecord(ctx, dbgen.CreateTestPresetRecordParams{
		ID:       preset.GetEntity().GetId(),
		TenantID: preset.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("test_preset", "preset already exists")
		}
		return err
	}
	return nil
}

func (r *PresetRepo) Get(ctx context.Context, tenantID, presetID string) (*models.TestPresetRecord, error) {
	row, err := r.db.q().GetTestPresetRecord(ctx, dbgen.GetTestPresetRecordParams{TenantID: tenantID, ID: presetID})
	if err != nil {
		return nil, translatePgErr("test_preset", err)
	}
	rec := &models.TestPresetRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Save persists a baked run's database+workload as a reusable preset.
func (r *PresetRepo) Save(ctx context.Context, tenantID, authorID, name string, run *domain.TestRun) (*models.TestPresetRecord, error) {
	now := timestamppbNow()
	preset := &models.TestPresetRecord{
		Entity: &commonpb.Entity{
			Id:       newUUID(),
			TenantId: tenantID,
			Name:     name,
			AuthorId: authorID,
			Timings:  &commonpb.Timings{CreatedAt: now, UpdatedAt: now},
		},
		Test: &domain.Test{
			Database: run.GetDatabase(),
			Workload: run.GetWorkload(),
			Tags:     run.GetTags(),
		},
		IsSystem: false,
	}
	if err := r.create(ctx, preset); err != nil {
		return nil, err
	}
	return preset, nil
}
