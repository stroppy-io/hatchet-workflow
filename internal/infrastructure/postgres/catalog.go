package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	catalogsvc "github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
)

/*
	===== CatalogEntryRepo =====

	Backs the SP-B catalog store: one catalog_entries row per (level,
	tenant_id, kind, slug, version) tuple. level/tenant_id/kind/slug/version/
	origin/source_entry_id are mirrored into indexed columns (alongside the
	canonical protojson in data) so the scoped lookups can run without
	decoding every row. tenant_id is '' (NOT NULL) for LEVEL_INSTANCE rows
	and the owning tenant for LEVEL_ORG rows; the unique index is a plain
	(level, tenant_id, kind, slug, version) column list — no NULL/coalesce
	involved, since NULL would break equality-based uniqueness anyway.
*/

// CatalogEntries returns the catalog.CatalogEntryRepo.
func (s *Store) CatalogEntries() *CatalogEntryRepo { return &CatalogEntryRepo{db: s.db} }

type CatalogEntryRepo struct{ db *DB }

var _ catalogsvc.CatalogEntryRepo = (*CatalogEntryRepo)(nil)

func levelStr(l catalogpb.Level) string {
	if l == catalogpb.Level_LEVEL_INSTANCE {
		return "instance"
	}
	return "org"
}

func kindStr(k catalogpb.Kind) string {
	if k == catalogpb.Kind_KIND_PROVIDER {
		return "provider"
	}
	return "workflow"
}

func originStr(o catalogpb.Origin) string {
	switch o {
	case catalogpb.Origin_ORIGIN_LINKED:
		return "linked"
	case catalogpb.Origin_ORIGIN_FORKED:
		return "forked"
	default:
		return "native"
	}
}

// nullableString maps "" to a nil *string (SQL NULL) and a non-empty string
// to a pointer to it, for optional envelope columns such as source_entry_id.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *CatalogEntryRepo) Create(ctx context.Context, e *catalogpb.CatalogEntry) error {
	data, err := marshal(e)
	if err != nil {
		return err
	}
	err = r.db.q().CreateCatalogEntry(ctx, dbgen.CreateCatalogEntryParams{
		ID:            e.GetEntity().GetId(),
		Level:         levelStr(e.GetLevel()),
		TenantID:      e.GetEntity().GetTenantId(),
		Kind:          kindStr(e.GetKind()),
		Slug:          e.GetSlug(),
		Version:       e.GetVersion(),
		Origin:        originStr(e.GetOrigin()),
		SourceEntryID: nullableString(e.GetSourceEntryId()),
		Data:          data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("catalog_entry", "entry already exists at this slug+version")
		}
		return err
	}
	return nil
}

func (r *CatalogEntryRepo) Get(ctx context.Context, level catalogpb.Level, tenantID, id string) (*catalogpb.CatalogEntry, error) {
	row, err := r.db.q().GetCatalogEntry(ctx, dbgen.GetCatalogEntryParams{
		Level:    levelStr(level),
		TenantID: tenantID,
		ID:       id,
	})
	if err != nil {
		return nil, translatePgErr("catalog_entry", err)
	}
	return decodeCatalogEntry(row.Data)
}

func (r *CatalogEntryRepo) List(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind) ([]*catalogpb.CatalogEntry, error) {
	rows, err := r.db.q().ListCatalogEntries(ctx, dbgen.ListCatalogEntriesParams{
		Level:    levelStr(level),
		TenantID: tenantID,
		Kind:     kindStr(kind),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*catalogpb.CatalogEntry, 0, len(rows))
	for _, row := range rows {
		e, err := decodeCatalogEntry(row.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *CatalogEntryRepo) GetLatestBySlug(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string) (*catalogpb.CatalogEntry, error) {
	row, err := r.db.q().GetLatestCatalogEntryBySlug(ctx, dbgen.GetLatestCatalogEntryBySlugParams{
		Level:    levelStr(level),
		TenantID: tenantID,
		Kind:     kindStr(kind),
		Slug:     slug,
	})
	if err != nil {
		return nil, translatePgErr("catalog_entry", err)
	}
	return decodeCatalogEntry(row.Data)
}

func (r *CatalogEntryRepo) GetBySlugVersion(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string, version uint32) (*catalogpb.CatalogEntry, error) {
	row, err := r.db.q().GetCatalogEntryBySlugVersion(ctx, dbgen.GetCatalogEntryBySlugVersionParams{
		Level:    levelStr(level),
		TenantID: tenantID,
		Kind:     kindStr(kind),
		Slug:     slug,
		Version:  int32(version), //nolint:gosec // version is a small monotonic counter, never approaches int32's range.
	})
	if err != nil {
		return nil, translatePgErr("catalog_entry", err)
	}
	return decodeCatalogEntry(row.Data)
}

func (r *CatalogEntryRepo) ListBySource(ctx context.Context, sourceEntryID string) ([]*catalogpb.CatalogEntry, error) {
	rows, err := r.db.q().ListCatalogEntriesBySource(ctx, &sourceEntryID)
	if err != nil {
		return nil, err
	}
	out := make([]*catalogpb.CatalogEntry, 0, len(rows))
	for _, row := range rows {
		e, err := decodeCatalogEntry(row.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *CatalogEntryRepo) Update(ctx context.Context, e *catalogpb.CatalogEntry) error {
	data, err := marshal(e)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateCatalogEntry(ctx, dbgen.UpdateCatalogEntryParams{
		Data:     data,
		Level:    levelStr(e.GetLevel()),
		TenantID: e.GetEntity().GetTenantId(),
		ID:       e.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("catalog_entry", "catalog entry not found")
	}
	return nil
}

func (r *CatalogEntryRepo) Delete(ctx context.Context, level catalogpb.Level, tenantID, id string) error {
	n, err := r.db.q().DeleteCatalogEntry(ctx, dbgen.DeleteCatalogEntryParams{
		Level:    levelStr(level),
		TenantID: tenantID,
		ID:       id,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("catalog_entry", "catalog entry not found")
	}
	return nil
}

func decodeCatalogEntry(data []byte) (*catalogpb.CatalogEntry, error) {
	e := &catalogpb.CatalogEntry{}
	if err := unmarshal(data, e); err != nil {
		return nil, err
	}
	return e, nil
}
