// Package gormstore is the GORM-backed persistence for the demo: it replaces the
// in-memory repositories with a real postgres store, persisting each record as a
// JSONB blob row keyed by (tenant_id, id). One Store value exposes typed repos
// that satisfy the service-package port interfaces (test_run.TestRunRepo,
// test_wizard.DraftRepo, the preset ports, the overview TestRunReader, ...).
//
// Each record is marshaled to canonical protojson on write and unmarshaled on
// read, so the schema stays flexible (no per-field columns) while the queryable
// envelope keys — id, tenant_id — are mirrored into indexed columns. List
// returns all rows for the tenant (filters/sort/paging ignored, empty next
// token) which matches the demo semantics of the in-memory repos it replaces.
package gormstore

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
)

// marshalOpts/unmarshalOpts are the canonical protojson codecs used for every
// blob: deterministic output, tolerant input.
var (
	marshalOpts   = protojson.MarshalOptions{}
	unmarshalOpts = protojson.UnmarshalOptions{DiscardUnknown: true}
)

/*
	===== row models =====

	One table per record type. Data holds the protojson blob; ID/TenantID mirror
	the record's common.Entity envelope for keying + the tenant index.
*/

// testRunRow is a row in test_run_records.
type testRunRow struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Data      datatypes.JSON
}

func (testRunRow) TableName() string { return "test_run_records" }

// testWizardDraftRow is a row in test_wizard_drafts.
type testWizardDraftRow struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Data      datatypes.JSON
}

func (testWizardDraftRow) TableName() string { return "test_wizard_drafts" }

// testPresetRow is a row in test_preset_records.
type testPresetRow struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Data      datatypes.JSON
}

func (testPresetRow) TableName() string { return "test_preset_records" }

// Migrate auto-migrates all row models. It needs a live postgres connection.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&testRunRow{}, &testWizardDraftRow{}, &testPresetRow{}, &settingsRow{})
}

/*
	===== Store =====
*/

// Store is the GORM-backed persistence facade. It exposes typed repos that
// satisfy the service port interfaces.
type Store struct{ db *gorm.DB }

// New wraps a *gorm.DB. It does not touch the database.
func New(db *gorm.DB) *Store { return &Store{db: db} }

// TestRuns returns the test_run.TestRunRepo (also the overview TestRunReader).
func (s *Store) TestRuns() *TestRunRepo { return &TestRunRepo{db: s.db} }

// Drafts returns the test_wizard.DraftRepo.
func (s *Store) Drafts() *DraftRepo { return &DraftRepo{db: s.db} }

// Presets returns the preset repo backing PresetRepo/PresetReader/PresetSaver.
func (s *Store) Presets() *PresetRepo { return &PresetRepo{db: s.db} }

/*
	===== shared helpers =====
*/

// marshal serializes a record to a JSONB blob.
func marshal(m proto.Message) (datatypes.JSON, error) {
	b, err := marshalOpts.Marshal(m)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(b), nil
}

// translateGormErr maps gorm.ErrRecordNotFound onto the domain not-found so
// services translate it to gRPC NotFound; everything else passes through.
func translateGormErr(resource string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return derrors.NotFound(resource, "not found")
	}
	return err
}

/*
	===== TestRunRepo =====

	Backs test_run.TestRunRepo and test_run_overview.TestRunReader.
*/

type TestRunRepo struct{ db *gorm.DB }

var (
	_ test_run.TestRunRepo            = (*TestRunRepo)(nil)
	_ test_run_overview.TestRunReader = (*TestRunRepo)(nil)
)

func (r *TestRunRepo) Create(ctx context.Context, run *models.TestRunRecord) error {
	data, err := marshal(run)
	if err != nil {
		return err
	}
	now := time.Now()
	row := &testRunRow{
		ID:        run.GetEntity().GetId(),
		TenantID:  run.GetEntity().GetTenantId(),
		CreatedAt: now,
		UpdatedAt: now,
		Data:      data,
	}
	res := r.db.WithContext(ctx).Create(row)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return derrors.Conflict("test_run", "run already exists")
		}
		return res.Error
	}
	return nil
}

func (r *TestRunRepo) Get(ctx context.Context, tenantID, id string) (*models.TestRunRecord, error) {
	var row testRunRow
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&row).Error; err != nil {
		return nil, translateGormErr("test_run", err)
	}
	rec := &models.TestRunRecord{}
	if err := unmarshalOpts.Unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns every run for the request tenant, ignoring filters/sort/paging
// (demo). The next-page token is always empty.
func (r *TestRunRepo) List(ctx context.Context, query *api.ListTestRunsRequest) ([]*models.TestRunRecord, string, error) {
	var rows []testRunRow
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", query.GetTenantId()).
		Find(&rows).Error; err != nil {
		return nil, "", err
	}
	out := make([]*models.TestRunRecord, 0, len(rows))
	for i := range rows {
		rec := &models.TestRunRecord{}
		if err := unmarshalOpts.Unmarshal(rows[i].Data, rec); err != nil {
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
	tenantID := run.GetEntity().GetTenantId()
	id := run.GetEntity().GetId()
	res := r.db.WithContext(ctx).Model(&testRunRow{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]any{"data": data, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return derrors.NotFound("test_run", "run not found")
	}
	return nil
}

func (r *TestRunRepo) Delete(ctx context.Context, tenantID, id string) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&testRunRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return derrors.NotFound("test_run", "run not found")
	}
	return nil
}

// Exists satisfies test_run_overview.TestRunReader: a cross-tenant or unknown id
// is reported as not-found so observability data never leaks across tenants.
func (r *TestRunRepo) Exists(ctx context.Context, tenantID, runID string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&testRunRow{}).
		Where("tenant_id = ? AND id = ?", tenantID, runID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return derrors.NotFound("test_run", "run not found")
	}
	return nil
}

/*
	===== DraftRepo (test_wizard.DraftRepo) =====
*/

type DraftRepo struct{ db *gorm.DB }

var _ test_wizard.DraftRepo = (*DraftRepo)(nil)

func (r *DraftRepo) Create(ctx context.Context, draft *models.TestWizardDraftRecord) error {
	data, err := marshal(draft)
	if err != nil {
		return err
	}
	now := time.Now()
	row := &testWizardDraftRow{
		ID:        draft.GetEntity().GetId(),
		TenantID:  draft.GetEntity().GetTenantId(),
		CreatedAt: now,
		UpdatedAt: now,
		Data:      data,
	}
	res := r.db.WithContext(ctx).Create(row)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return derrors.Conflict("test_wizard_draft", "draft already exists")
		}
		return res.Error
	}
	return nil
}

func (r *DraftRepo) Get(ctx context.Context, tenantID, id string) (*models.TestWizardDraftRecord, error) {
	var row testWizardDraftRow
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&row).Error; err != nil {
		return nil, translateGormErr("test_wizard_draft", err)
	}
	rec := &models.TestWizardDraftRecord{}
	if err := unmarshalOpts.Unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns every draft for the tenant, ignoring filter/sort/page (demo).
func (r *DraftRepo) List(ctx context.Context, tenantID string, _ *commonpb.EntityFilter, _ *commonpb.EntitySort, _ *commonpb.Page) ([]*models.TestWizardDraftRecord, string, error) {
	var rows []testWizardDraftRow
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Find(&rows).Error; err != nil {
		return nil, "", err
	}
	out := make([]*models.TestWizardDraftRecord, 0, len(rows))
	for i := range rows {
		rec := &models.TestWizardDraftRecord{}
		if err := unmarshalOpts.Unmarshal(rows[i].Data, rec); err != nil {
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
	tenantID := draft.GetEntity().GetTenantId()
	id := draft.GetEntity().GetId()
	res := r.db.WithContext(ctx).Model(&testWizardDraftRow{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]any{"data": data, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return derrors.NotFound("test_wizard_draft", "draft not found")
	}
	return nil
}

func (r *DraftRepo) Delete(ctx context.Context, tenantID, id string) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&testWizardDraftRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
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

type PresetRepo struct{ db *gorm.DB }

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
	now := time.Now()
	row := &testPresetRow{
		ID:        preset.GetEntity().GetId(),
		TenantID:  preset.GetEntity().GetTenantId(),
		CreatedAt: now,
		UpdatedAt: now,
		Data:      data,
	}
	res := r.db.WithContext(ctx).Create(row)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return derrors.Conflict("test_preset", "preset already exists")
		}
		return res.Error
	}
	return nil
}

func (r *PresetRepo) Get(ctx context.Context, tenantID, presetID string) (*models.TestPresetRecord, error) {
	var row testPresetRow
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, presetID).
		First(&row).Error; err != nil {
		return nil, translateGormErr("test_preset", err)
	}
	rec := &models.TestPresetRecord{}
	if err := unmarshalOpts.Unmarshal(row.Data, rec); err != nil {
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
