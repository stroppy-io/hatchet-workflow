package postgres

import (
	"context"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

const (
	tableTestRunRecords = "test_run_records"
)

// ListFacets returns distinct run-list facet values from tenant-scoped,
// entity-filtered rows without loading every record blob into memory.
func (r *TestRunRepo) ListFacets(ctx context.Context, req *api.ListTestRunFacetsRequest, callerAccountID string) (*api.ListTestRunFacetsResponse, error) {
	where, args := facetEntityWhere(req.GetTenantId(), req.GetFilter(), callerAccountID, commonpb.FavoriteKind_FAVORITE_KIND_TEST_RUN, tableTestRunRecords)
	authorIDs, err := distinctTextFacet(ctx, r.db, tableTestRunRecords, jAuthorID, where, args)
	if err != nil {
		return nil, err
	}
	stroppyVersions, err := distinctTextFacet(ctx, r.db, tableTestRunRecords, jStroppyVersion, where, args)
	if err != nil {
		return nil, err
	}
	dbPresetIDs, err := distinctTextFacet(ctx, r.db, tableTestRunRecords, jDBPresetID, where, args)
	if err != nil {
		return nil, err
	}
	workloadPresetIDs, err := distinctTextFacet(ctx, r.db, tableTestRunRecords, jWorkloadPreset, where, args)
	if err != nil {
		return nil, err
	}
	testPresetIDs, err := distinctTextFacet(ctx, r.db, tableTestRunRecords, jTestPresetID, where, args)
	if err != nil {
		return nil, err
	}
	return &api.ListTestRunFacetsResponse{
		AuthorIds:         authorIDs,
		StroppyVersions:   stroppyVersions,
		DbPresetIds:       dbPresetIDs,
		WorkloadPresetIds: workloadPresetIDs,
		TestPresetIds:     testPresetIDs,
	}, nil
}

func facetEntityWhere(tenantID string, filter *commonpb.EntityFilter, callerAccountID string, favoriteKind commonpb.FavoriteKind, table string) ([]string, []any) {
	b := &argBuilder{}
	where := []string{"tenant_id = " + b.place(tenantID)}

	if filter == nil || !filter.GetIncludeDeleted() {
		where = append(where, jDeletedAt+" IS NULL")
	}
	if filter != nil {
		if search := strings.TrimSpace(filter.GetSearch()); search != "" {
			where = append(where, testRunSearchClause(b, search))
		}
		if ids := filter.GetIds(); len(ids) > 0 {
			where = append(where, "id = ANY("+b.place(ids)+")")
		}
		if authorIDs := filter.GetAuthorIds(); len(authorIDs) > 0 {
			where = append(where, jAuthorID+" = ANY("+b.place(authorIDs)+")")
		}
		if t := filter.GetCreatedAfter(); t != nil {
			where = append(where, "created_at >= "+b.place(t.AsTime()))
		}
		if t := filter.GetCreatedBefore(); t != nil {
			where = append(where, "created_at <= "+b.place(t.AsTime()))
		}
		if t := filter.GetUpdatedAfter(); t != nil {
			where = append(where, "updated_at >= "+b.place(t.AsTime()))
		}
		if t := filter.GetUpdatedBefore(); t != nil {
			where = append(where, "updated_at <= "+b.place(t.AsTime()))
		}
		if filter.GetFavoritesOnly() {
			where = append(where, "EXISTS (SELECT 1 FROM favorite_records fr WHERE fr.tenant_id = "+
				b.place(tenantID)+" AND fr.account_id = "+
				b.place(callerAccountID)+" AND fr.kind = "+
				b.place(int32(favoriteKind))+" AND fr.target_id = "+table+".id)")
		}
	}
	return where, b.args
}

func distinctTextFacet(ctx context.Context, db *DB, table, expr string, where []string, args []any) ([]string, error) {
	sql := "SELECT DISTINCT value FROM (SELECT NULLIF(trim(" + expr + "), '') AS value FROM " +
		table + " WHERE " + strings.Join(where, " AND ") + ") facets WHERE value IS NOT NULL ORDER BY value"
	rows, err := db.TxDB.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
