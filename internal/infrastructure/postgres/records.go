package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/favorite"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
	publicshare "github.com/stroppy-io/stroppy-cloud/internal/services/public_share"
	"github.com/stroppy-io/stroppy-cloud/internal/services/share"
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

// Shares returns the share.ShareRepo (also the public_share.ShareRepo resolver).
func (s *Store) Shares() *ShareRepo { return &ShareRepo{db: s.db} }

// Favorites returns the favorite.FavoriteRepo.
func (s *Store) Favorites() *FavoriteRepo { return &FavoriteRepo{db: s.db} }

// Packages returns the packages.PackageRepo.
func (s *Store) Packages() *PackageRepo { return &PackageRepo{db: s.db} }

// TenantSettings returns the tenant_settings.TenantSettingsRepo.
func (s *Store) TenantSettings() *TenantSettingsRepo { return &TenantSettingsRepo{db: s.db} }

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
	out := make([]*models.PackageRecord, 0, len(rows)+len(q.Builtin))
	for _, row := range rows {
		rec := &models.PackageRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	out = append(out, q.Builtin...)
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
