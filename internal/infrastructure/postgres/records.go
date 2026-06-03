package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/favorite"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/services/preset"
	publicshare "github.com/stroppy-io/stroppy-cloud/internal/services/public_share"
	"github.com/stroppy-io/stroppy-cloud/internal/services/share"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite"
	suiterun "github.com/stroppy-io/stroppy-cloud/internal/services/suite_run"
	suitewizard "github.com/stroppy-io/stroppy-cloud/internal/services/suite_wizard"
	tenantsettings "github.com/stroppy-io/stroppy-cloud/internal/services/tenant_settings"
)

/*
	===== Store accessors =====

	Same JSONB-blob pattern as store.go: one table per record type, the
	common.Entity envelope (id, tenant_id) mirrored into indexed columns, the
	canonical protojson in the data column. Lookup-by-secondary-key (share token,
	favorite target) uses its own indexed column. Repo calls go through the
	sqld-generated query set bound to db.TxDB so they pick up the ambient
	transaction; List loads tenant-scoped rows and applies the API's filter,
	sort and pagination contract before returning them.
*/

// Suites returns the suite.SuiteRepo.
func (s *Store) Suites() *SuiteRepo { return &SuiteRepo{db: s.db} }

// SuiteRuns returns the suite_run.SuiteRunRepo.
func (s *Store) SuiteRuns() *SuiteRunRepo { return &SuiteRunRepo{db: s.db} }

// SuiteDrafts returns the suite_wizard.DraftRepo.
func (s *Store) SuiteDrafts() *SuiteDraftRepo { return &SuiteDraftRepo{db: s.db} }

// Shares returns the share.ShareRepo (also the public_share.ShareRepo resolver).
func (s *Store) Shares() *ShareRepo { return &ShareRepo{db: s.db} }

// Favorites returns the favorite.FavoriteRepo.
func (s *Store) Favorites() *FavoriteRepo { return &FavoriteRepo{db: s.db} }

// Packages returns the packages.PackageRepo.
func (s *Store) Packages() *PackageRepo { return &PackageRepo{db: s.db} }

// DatabasePresets returns the preset.DatabasePresetRepo.
func (s *Store) DatabasePresets() *DatabasePresetRepo { return &DatabasePresetRepo{db: s.db} }

// WorkloadPresets returns the preset.WorkloadPresetRepo.
func (s *Store) WorkloadPresets() *WorkloadPresetRepo { return &WorkloadPresetRepo{db: s.db} }

// TestPresets returns the preset.TestPresetRepo (the test_preset_records table,
// shared with the wizard PresetRepo).
func (s *Store) TestPresets() *TestPresetRepo { return &TestPresetRepo{db: s.db} }

// TenantSettings returns the tenant_settings.TenantSettingsRepo.
func (s *Store) TenantSettings() *TenantSettingsRepo { return &TenantSettingsRepo{db: s.db} }

/*
	===== SuiteRepo =====
*/

type SuiteRepo struct{ db *DB }

var _ suite.SuiteRepo = (*SuiteRepo)(nil)

func (r *SuiteRepo) Create(ctx context.Context, rec *models.SuiteRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	err = r.db.q().CreateSuiteRecord(ctx, dbgen.CreateSuiteRecordParams{
		ID:       rec.GetEntity().GetId(),
		TenantID: rec.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("suite", "suite already exists")
		}
		return err
	}
	return nil
}

func (r *SuiteRepo) Get(ctx context.Context, tenantID, id string) (*models.SuiteRecord, error) {
	row, err := r.db.q().GetSuiteRecord(ctx, dbgen.GetSuiteRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("suite", err)
	}
	rec := &models.SuiteRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns the matching suite page for the query tenant.
func (r *SuiteRepo) List(ctx context.Context, query suite.SuiteListQuery) ([]*models.SuiteRecord, string, error) {
	rows, err := r.db.q().ListSuiteRecords(ctx, query.TenantID)
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.SuiteRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.SuiteRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	return filterSuiteRecords(ctx, r.db, out, query)
}

func (r *SuiteRepo) Update(ctx context.Context, rec *models.SuiteRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateSuiteRecord(ctx, dbgen.UpdateSuiteRecordParams{
		Data:     data,
		TenantID: rec.GetEntity().GetTenantId(),
		ID:       rec.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("suite", "suite not found")
	}
	return nil
}

func (r *SuiteRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteSuiteRecord(ctx, dbgen.DeleteSuiteRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("suite", "suite not found")
	}
	return nil
}

/*
	===== SuiteRunRepo =====

	Get/Delete address rows by id alone (tenant ownership is verified by the
	handler), matching the service port signature.
*/

type SuiteRunRepo struct{ db *DB }

var _ suiterun.SuiteRunRepo = (*SuiteRunRepo)(nil)

func (r *SuiteRunRepo) Get(ctx context.Context, id string) (*models.SuiteRunRecord, error) {
	row, err := r.db.q().GetSuiteRunRecord(ctx, id)
	if err != nil {
		return nil, translatePgErr("suite_run", err)
	}
	rec := &models.SuiteRunRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns the matching suite-run page for the request tenant.
func (r *SuiteRunRepo) List(ctx context.Context, req *api.ListSuiteRunsRequest, callerID string) ([]*models.SuiteRunRecord, string, error) {
	rows, err := r.db.q().ListSuiteRunRecords(ctx, req.GetTenantId())
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.SuiteRunRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.SuiteRunRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	return filterSuiteRunRecords(ctx, r.db, out, req, callerID)
}

// Update doubles as upsert on first persist of an expanded suite run.
func (r *SuiteRunRepo) Update(ctx context.Context, rec *models.SuiteRunRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateSuiteRunRecord(ctx, dbgen.UpdateSuiteRunRecordParams{
		TenantID: rec.GetEntity().GetTenantId(),
		Data:     data,
		ID:       rec.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return r.create(ctx, rec)
	}
	return nil
}

func (r *SuiteRunRepo) create(ctx context.Context, rec *models.SuiteRunRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	err = r.db.q().CreateSuiteRunRecord(ctx, dbgen.CreateSuiteRunRecordParams{
		ID:       rec.GetEntity().GetId(),
		TenantID: rec.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("suite_run", "suite run already exists")
		}
		return err
	}
	return nil
}

func (r *SuiteRunRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteSuiteRunRecord(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("suite_run", "suite run not found")
	}
	return nil
}

/*
	===== SuiteDraftRepo (suite_wizard.DraftRepo) =====
*/

type SuiteDraftRepo struct{ db *DB }

var _ suitewizard.DraftRepo = (*SuiteDraftRepo)(nil)

func (r *SuiteDraftRepo) Create(ctx context.Context, draft *models.SuiteWizardDraftRecord) error {
	data, err := marshal(draft)
	if err != nil {
		return err
	}
	err = r.db.q().CreateSuiteWizardDraft(ctx, dbgen.CreateSuiteWizardDraftParams{
		ID:       draft.GetEntity().GetId(),
		TenantID: draft.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("suite_wizard_draft", "draft already exists")
		}
		return err
	}
	return nil
}

func (r *SuiteDraftRepo) Get(ctx context.Context, tenantID, id string) (*models.SuiteWizardDraftRecord, error) {
	row, err := r.db.q().GetSuiteWizardDraft(ctx, dbgen.GetSuiteWizardDraftParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("suite_wizard_draft", err)
	}
	rec := &models.SuiteWizardDraftRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *SuiteDraftRepo) List(ctx context.Context, tenantID string, filter *commonpb.EntityFilter, sort *commonpb.EntitySort, page *commonpb.Page) ([]*models.SuiteWizardDraftRecord, string, error) {
	rows, err := r.db.q().ListSuiteWizardDrafts(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.SuiteWizardDraftRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.SuiteWizardDraftRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	pageRecords, next := filterDraftRecords(out, filter, sort, page)
	return pageRecords, next, nil
}

func (r *SuiteDraftRepo) Update(ctx context.Context, draft *models.SuiteWizardDraftRecord) error {
	data, err := marshal(draft)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateSuiteWizardDraft(ctx, dbgen.UpdateSuiteWizardDraftParams{
		Data:     data,
		TenantID: draft.GetEntity().GetTenantId(),
		ID:       draft.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("suite_wizard_draft", "draft not found")
	}
	return nil
}

func (r *SuiteDraftRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteSuiteWizardDraft(ctx, dbgen.DeleteSuiteWizardDraftParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("suite_wizard_draft", "draft not found")
	}
	return nil
}

/*
	===== ShareRepo =====

	One share_records table backs two ports:
	  - share.ShareRepo        (tenant-scoped CRUD)
	  - public_share.ShareRepo (token-only GetByToken resolve)
*/

type ShareRepo struct{ db *DB }

var (
	_ share.ShareRepo       = (*ShareRepo)(nil)
	_ publicshare.ShareRepo = (*ShareRepo)(nil)
)

func (r *ShareRepo) Create(ctx context.Context, rec *models.ShareRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	err = r.db.q().CreateShareRecord(ctx, dbgen.CreateShareRecordParams{
		ID:       rec.GetEntity().GetId(),
		TenantID: rec.GetEntity().GetTenantId(),
		Token:    rec.GetToken(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("share", "share already exists")
		}
		return err
	}
	return nil
}

func (r *ShareRepo) Get(ctx context.Context, tenantID, id string) (*models.ShareRecord, error) {
	row, err := r.db.q().GetShareRecord(ctx, dbgen.GetShareRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("share", err)
	}
	return decodeShare(row.Data)
}

func (r *ShareRepo) GetByToken(ctx context.Context, token string) (*models.ShareRecord, error) {
	row, err := r.db.q().GetShareRecordByToken(ctx, token)
	if err != nil {
		return nil, translatePgErr("share", err)
	}
	return decodeShare(row.Data)
}

// List returns the tenant's shares, optionally narrowed to a single target run.
func (r *ShareRepo) List(ctx context.Context, tenantID, targetID string, filter *commonpb.EntityFilter, sort *commonpb.EntitySort, page *commonpb.Page) ([]*models.ShareRecord, string, error) {
	rows, err := r.db.q().ListShareRecords(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.ShareRecord, 0, len(rows))
	for _, row := range rows {
		rec, err := decodeShare(row.Data)
		if err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	pageRecords, next := filterShareRecords(out, targetID, filter, sort, page)
	return pageRecords, next, nil
}

func (r *ShareRepo) Update(ctx context.Context, rec *models.ShareRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateShareRecord(ctx, dbgen.UpdateShareRecordParams{
		Token:    rec.GetToken(),
		Data:     data,
		TenantID: rec.GetEntity().GetTenantId(),
		ID:       rec.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("share", "share not found")
	}
	return nil
}

func (r *ShareRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteShareRecord(ctx, dbgen.DeleteShareRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("share", "share not found")
	}
	return nil
}

func decodeShare(data []byte) (*models.ShareRecord, error) {
	rec := &models.ShareRecord{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

/*
	===== FavoriteRepo =====

	Keyed by (account_id, kind, target_id) — a per-caller join row.
*/

type FavoriteRepo struct{ db *DB }

var _ favorite.FavoriteRepo = (*FavoriteRepo)(nil)

func (r *FavoriteRepo) Create(ctx context.Context, accountID string, rec *models.FavoriteRecord) error {
	data, err := marshal(rec)
	if err != nil {
		return err
	}
	err = r.db.q().CreateFavoriteRecord(ctx, dbgen.CreateFavoriteRecordParams{
		ID:        rec.GetEntity().GetId(),
		AccountID: accountID,
		TenantID:  rec.GetEntity().GetTenantId(),
		Kind:      int32(rec.GetKind()),
		TargetID:  rec.GetTargetId(),
		Data:      data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("favorite", "favorite already exists")
		}
		return err
	}
	return nil
}

func (r *FavoriteRepo) Get(ctx context.Context, accountID string, kind commonpb.FavoriteKind, targetID string) (*models.FavoriteRecord, error) {
	row, err := r.db.q().GetFavoriteRecord(ctx, dbgen.GetFavoriteRecordParams{
		AccountID: accountID,
		Kind:      int32(kind),
		TargetID:  targetID,
	})
	if err != nil {
		return nil, translatePgErr("favorite", err)
	}
	rec := &models.FavoriteRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *FavoriteRepo) Delete(ctx context.Context, accountID string, kind commonpb.FavoriteKind, targetID string) error {
	n, err := r.db.q().DeleteFavoriteRecord(ctx, dbgen.DeleteFavoriteRecordParams{
		AccountID: accountID,
		Kind:      int32(kind),
		TargetID:  targetID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("favorite", "favorite not found")
	}
	return nil
}

// List returns the account's favorites in the tenant, optionally narrowed to a
// single kind (FAVORITE_KIND_UNSPECIFIED = all).
func (r *FavoriteRepo) List(ctx context.Context, accountID, tenantID string, kind commonpb.FavoriteKind, pageSize uint32, pageToken string) ([]*models.FavoriteRecord, string, error) {
	rows, err := r.db.q().ListFavoriteRecords(ctx, dbgen.ListFavoriteRecordsParams{
		AccountID: accountID,
		TenantID:  tenantID,
		Kind:      int32(kind),
	})
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.FavoriteRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.FavoriteRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	page, next := pageRecords(out, pageSize, pageToken)
	return page, next, nil
}

/*
	===== PackageRepo =====
*/

type PackageRepo struct{ db *DB }

var _ packages.PackageRepo = (*PackageRepo)(nil)

func (r *PackageRepo) Create(ctx context.Context, pkg *models.PackageRecord) error {
	data, err := marshal(pkg)
	if err != nil {
		return err
	}
	err = r.db.q().CreatePackageRecord(ctx, dbgen.CreatePackageRecordParams{
		ID:       pkg.GetEntity().GetId(),
		TenantID: pkg.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("package", "package already exists")
		}
		return err
	}
	return nil
}

func (r *PackageRepo) Get(ctx context.Context, tenantID, id string) (*models.PackageRecord, error) {
	row, err := r.db.q().GetPackageRecord(ctx, dbgen.GetPackageRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("package", err)
	}
	rec := &models.PackageRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns the matching package page for the query tenant.
func (r *PackageRepo) List(ctx context.Context, q packages.PackageQuery) ([]*models.PackageRecord, string, error) {
	rows, err := r.db.q().ListPackageRecords(ctx, q.TenantID)
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.PackageRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.PackageRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	page, next := filterPackageRecords(out, q)
	return page, next, nil
}

func (r *PackageRepo) Update(ctx context.Context, pkg *models.PackageRecord) error {
	data, err := marshal(pkg)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdatePackageRecord(ctx, dbgen.UpdatePackageRecordParams{
		Data:     data,
		TenantID: pkg.GetEntity().GetTenantId(),
		ID:       pkg.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("package", "package not found")
	}
	return nil
}

func (r *PackageRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeletePackageRecord(ctx, dbgen.DeletePackageRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("package", "package not found")
	}
	return nil
}

/*
	===== DatabasePresetRepo =====
*/

type DatabasePresetRepo struct{ db *DB }

var _ preset.DatabasePresetRepo = (*DatabasePresetRepo)(nil)

func (r *DatabasePresetRepo) Create(ctx context.Context, p *models.DatabasePresetRecord) error {
	data, err := marshal(p)
	if err != nil {
		return err
	}
	err = r.db.q().CreateDatabasePresetRecord(ctx, dbgen.CreateDatabasePresetRecordParams{
		ID:       p.GetEntity().GetId(),
		TenantID: p.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("database_preset", "preset already exists")
		}
		return err
	}
	return nil
}

func (r *DatabasePresetRepo) Get(ctx context.Context, tenantID, id, callerID string) (*models.DatabasePresetRecord, error) {
	row, err := r.db.q().GetDatabasePresetRecord(ctx, dbgen.GetDatabasePresetRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("database_preset", err)
	}
	rec := &models.DatabasePresetRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	ensureDatabasePresetSummary(rec)
	favorites, err := favoriteIDs(ctx, r.db, tenantID, callerID, commonpb.FavoriteKind_FAVORITE_KIND_DATABASE_PRESET)
	if err != nil {
		return nil, err
	}
	if rec.Entity != nil {
		rec.Entity.IsFavorite = favorites != nil && favorites[rec.GetEntity().GetId()]
	}
	return rec, nil
}

// List returns the matching database preset page for the request tenant.
func (r *DatabasePresetRepo) List(ctx context.Context, req *api.ListDatabasePresetsRequest, callerID string) ([]*models.DatabasePresetRecord, string, error) {
	rows, err := r.db.q().ListDatabasePresetRecords(ctx, req.GetTenantId())
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.DatabasePresetRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.DatabasePresetRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	return filterDatabasePresetRecords(ctx, r.db, out, req, callerID)
}

func (r *DatabasePresetRepo) Update(ctx context.Context, p *models.DatabasePresetRecord) error {
	data, err := marshal(p)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateDatabasePresetRecord(ctx, dbgen.UpdateDatabasePresetRecordParams{
		Data:     data,
		TenantID: p.GetEntity().GetTenantId(),
		ID:       p.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("database_preset", "preset not found")
	}
	return nil
}

func (r *DatabasePresetRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteDatabasePresetRecord(ctx, dbgen.DeleteDatabasePresetRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("database_preset", "preset not found")
	}
	return nil
}

/*
	===== WorkloadPresetRepo =====
*/

type WorkloadPresetRepo struct{ db *DB }

var _ preset.WorkloadPresetRepo = (*WorkloadPresetRepo)(nil)

func (r *WorkloadPresetRepo) Create(ctx context.Context, p *models.WorkloadPresetRecord) error {
	data, err := marshal(p)
	if err != nil {
		return err
	}
	err = r.db.q().CreateWorkloadPresetRecord(ctx, dbgen.CreateWorkloadPresetRecordParams{
		ID:       p.GetEntity().GetId(),
		TenantID: p.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("workload_preset", "preset already exists")
		}
		return err
	}
	return nil
}

func (r *WorkloadPresetRepo) Get(ctx context.Context, tenantID, id, callerID string) (*models.WorkloadPresetRecord, error) {
	row, err := r.db.q().GetWorkloadPresetRecord(ctx, dbgen.GetWorkloadPresetRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("workload_preset", err)
	}
	rec := &models.WorkloadPresetRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	ensureWorkloadPresetSummary(rec)
	favorites, err := favoriteIDs(ctx, r.db, tenantID, callerID, commonpb.FavoriteKind_FAVORITE_KIND_WORKLOAD_PRESET)
	if err != nil {
		return nil, err
	}
	if rec.Entity != nil {
		rec.Entity.IsFavorite = favorites != nil && favorites[rec.GetEntity().GetId()]
	}
	return rec, nil
}

// List returns the matching workload preset page for the request tenant.
func (r *WorkloadPresetRepo) List(ctx context.Context, req *api.ListWorkloadPresetsRequest, callerID string) ([]*models.WorkloadPresetRecord, string, error) {
	rows, err := r.db.q().ListWorkloadPresetRecords(ctx, req.GetTenantId())
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.WorkloadPresetRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.WorkloadPresetRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	return filterWorkloadPresetRecords(ctx, r.db, out, req, callerID)
}

func (r *WorkloadPresetRepo) Update(ctx context.Context, p *models.WorkloadPresetRecord) error {
	data, err := marshal(p)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateWorkloadPresetRecord(ctx, dbgen.UpdateWorkloadPresetRecordParams{
		Data:     data,
		TenantID: p.GetEntity().GetTenantId(),
		ID:       p.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("workload_preset", "preset not found")
	}
	return nil
}

func (r *WorkloadPresetRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteWorkloadPresetRecord(ctx, dbgen.DeleteWorkloadPresetRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("workload_preset", "preset not found")
	}
	return nil
}

/*
	===== TestPresetRepo =====

	Backs preset.TestPresetRepo. It reuses the same test_preset_records table as
	the seed PresetRepo (test_run/test_wizard ports), so the wizard and the preset
	CRUD service see the same rows.
*/

type TestPresetRepo struct{ db *DB }

var _ preset.TestPresetRepo = (*TestPresetRepo)(nil)

func (r *TestPresetRepo) Create(ctx context.Context, p *models.TestPresetRecord) error {
	data, err := marshal(p)
	if err != nil {
		return err
	}
	err = r.db.q().CreateTestPresetRecord(ctx, dbgen.CreateTestPresetRecordParams{
		ID:       p.GetEntity().GetId(),
		TenantID: p.GetEntity().GetTenantId(),
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

func (r *TestPresetRepo) Get(ctx context.Context, tenantID, id, callerID string) (*models.TestPresetRecord, error) {
	row, err := r.db.q().GetTestPresetRecord(ctx, dbgen.GetTestPresetRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("test_preset", err)
	}
	rec := &models.TestPresetRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	ensureTestPresetSummary(rec)
	favorites, err := favoriteIDs(ctx, r.db, tenantID, callerID, commonpb.FavoriteKind_FAVORITE_KIND_TEST_PRESET)
	if err != nil {
		return nil, err
	}
	if rec.Entity != nil {
		rec.Entity.IsFavorite = favorites != nil && favorites[rec.GetEntity().GetId()]
	}
	return rec, nil
}

// List returns the matching test preset page for the request tenant.
func (r *TestPresetRepo) List(ctx context.Context, req *api.ListTestPresetsRequest, callerID string) ([]*models.TestPresetRecord, string, error) {
	rows, err := r.db.q().ListTestPresetRecords(ctx, req.GetTenantId())
	if err != nil {
		return nil, "", err
	}
	out := make([]*models.TestPresetRecord, 0, len(rows))
	for _, row := range rows {
		rec := &models.TestPresetRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	return filterTestPresetRecords(ctx, r.db, out, req, callerID)
}

func (r *TestPresetRepo) Update(ctx context.Context, p *models.TestPresetRecord) error {
	data, err := marshal(p)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateTestPresetRecord(ctx, dbgen.UpdateTestPresetRecordParams{
		Data:     data,
		TenantID: p.GetEntity().GetTenantId(),
		ID:       p.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("test_preset", "preset not found")
	}
	return nil
}

func (r *TestPresetRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteTestPresetRecord(ctx, dbgen.DeleteTestPresetRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("test_preset", "preset not found")
	}
	return nil
}

/*
	===== TenantSettingsRepo =====

	One row per tenant (primary key = tenant_id). Upsert is wholesale-replace.
*/

type TenantSettingsRepo struct{ db *DB }

var _ tenantsettings.TenantSettingsRepo = (*TenantSettingsRepo)(nil)

func (r *TenantSettingsRepo) Get(ctx context.Context, tenantID string) (*models.TenantSettingsRecord, error) {
	row, err := r.db.q().GetTenantSettingsRecord(ctx, tenantID)
	if err != nil {
		return nil, translatePgErr("tenant_settings", err)
	}
	rec := &models.TenantSettingsRecord{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *TenantSettingsRepo) Upsert(ctx context.Context, settings *models.TenantSettingsRecord) error {
	data, err := marshal(settings)
	if err != nil {
		return err
	}
	return r.db.q().UpsertTenantSettingsRecord(ctx, dbgen.UpsertTenantSettingsRecordParams{
		TenantID: settings.GetEntity().GetTenantId(),
		Data:     data,
	})
}
