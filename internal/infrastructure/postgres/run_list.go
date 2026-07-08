package postgres

import (
	"context"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

/*
	===== RunRepo.List dynamic query =====

	Near-duplicate of test_run_list.go's TestRunRepo.List: same WHERE/ORDER-BY/
	paging builder, targeting run_records instead of test_run_records and
	unmarshaling into models.Run. This duplication is DELIBERATE (SP-E plan
	Task 7 deletes test_run_list.go once the write path fully cuts over to
	run_records, leaving this file as the sole copy) — do not try to unify the
	two List methods.

	Differences from TestRunRepo.List:
	  - table: run_records (FROM / favorite-EXISTS target / FavoriteKind target).
	  - strip-columns: Run's heavy blobs are runtimeState and compiledPlan (not
	    TestRunRecord's deploymentPlan/infrastructureState, which Run doesn't
	    have).
	  - no suite_run_id/suite_cell_id: Run has no suite membership (spec §5/§6.A:
	    dead field, dropped). query.GetSuiteRunId()/GetSuiteCellIds()/
	    GetStandalone() are silently ignored — ListTestRunsRequest keeps those
	    fields for TestRunRepo.List's sake until Task 7.
	  - favorites: reuses commonpb.FavoriteKind_FAVORITE_KIND_TEST_RUN (favorites
	    are keyed by (kind, target_id); a Run.entity.id and a TestRunRecord's are
	    drawn from the same id space and never coexist per row post-cutover, so
	    sharing the kind avoids a proto enum change).

	The j* jsonb-path consts (jStatus, jDBKind, ...) are shared, unchanged, from
	test_run_list.go — both tables store the same protojson shape under those
	paths, so redeclaring them here would just duplicate identical strings.
	buildOrderBy/testRunSearchClause/statusNames/encodeOffsetToken/
	decodeOffsetToken are likewise shared unchanged.
*/

// List honors the full ListTestRunsRequest against postgres for run_records:
// tenant scope plus every filter/facet (except suite membership, which Run has
// no field for), the requested sort and offset-based pagination. Returns the
// matching page and the next-page token.
func (r *RunRepo) List(ctx context.Context, query *api.ListTestRunsRequest, callerAccountID string) ([]*models.Run, string, error) {
	b := &argBuilder{}
	where := runListWhereClauses(b, query)
	if query.GetFilter().GetFavoritesOnly() {
		where = append(where, "EXISTS (SELECT 1 FROM favorite_records fr WHERE fr.tenant_id = "+
			b.place(query.GetTenantId())+" AND fr.account_id = "+
			b.place(callerAccountID)+" AND fr.kind = "+
			b.place(int32(commonpb.FavoriteKind_FAVORITE_KIND_TEST_RUN))+
			" AND fr.target_id = run_records.id)")
	}

	orderBy := buildOrderBy(query.GetSort())

	// pagination: offset cursor + size cap.
	size := defaultTestRunPageSize
	if p := query.GetPage(); p != nil && p.GetSize() > 0 {
		size = int(p.GetSize())
	}
	if size > maxTestRunPageSize {
		size = maxTestRunPageSize
	}
	offset := 0
	if p := query.GetPage(); p != nil {
		if o, ok := decodeOffsetToken(p.GetToken()); ok {
			offset = o
		}
	}

	sb := strings.Builder{}
	sb.WriteString(runListBaseSelect())
	if len(where) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(where, " AND "))
	}
	sb.WriteString(" ORDER BY ")
	sb.WriteString(orderBy)
	// LIMIT size+1 to detect whether a further page exists.
	sb.WriteString(" LIMIT " + b.place(size+1))
	sb.WriteString(" OFFSET " + b.place(offset))

	rows, err := r.db.TxDB.Query(ctx, sb.String(), b.args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([]*models.Run, 0, size)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, "", err
		}
		rec := &models.Run{}
		if err := unmarshal(data, rec); err != nil {
			return nil, "", err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	favorites, err := favoriteIDs(ctx, r.db, query.GetTenantId(), callerAccountID, commonpb.FavoriteKind_FAVORITE_KIND_TEST_RUN)
	if err != nil {
		return nil, "", err
	}
	for _, rec := range out {
		if rec.Entity != nil {
			rec.Entity.IsFavorite = favorites != nil && favorites[rec.GetEntity().GetId()]
		}
	}

	next := ""
	if len(out) > size {
		out = out[:size]
		next = encodeOffsetToken(offset + size)
	}
	return out, next, nil
}

// runListBaseSelect is the literal SELECT projection for RunRepo.List: it
// targets run_records and strips Run's heavy blobs (runtimeState,
// compiledPlan) — the list view only renders summary/entity/status fields.
// RunDetail loads the full record separately via Get. (jsonb `-` removes a
// top-level key.)
func runListBaseSelect() string {
	return "SELECT data - 'runtimeState' - 'compiledPlan' FROM run_records"
}

// runListWhereClauses builds the parameterized WHERE-clause list for
// RunRepo.List: tenant scope, soft-delete, and every filter/facet honored for
// run_records except favorites-only (List appends that separately, since it
// needs the caller's account id — see List's favorites-only EXISTS clause,
// which targets run_records.id). Unlike TestRunRepo.List's builder, it never
// emits a suite_run_id/suite_cell_id clause (Run has no suite membership).
func runListWhereClauses(b *argBuilder, query *api.ListTestRunsRequest) []string {
	var where []string

	// tenant scope (indexed column) — always applied.
	where = append(where, "tenant_id = "+b.place(query.GetTenantId()))

	f := query.GetFilter()

	// soft-delete: exclude rows with timings.deletedAt unless include_deleted.
	if f == nil || !f.GetIncludeDeleted() {
		where = append(where, jDeletedAt+" IS NULL")
	}

	if f != nil {
		// search: case-insensitive substring over name + description.
		if s := strings.TrimSpace(f.GetSearch()); s != "" {
			where = append(where, testRunSearchClause(b, s))
		}
		// ids: exact id set (indexed primary key).
		if ids := f.GetIds(); len(ids) > 0 {
			where = append(where, "id = ANY("+b.place(ids)+")")
		}
		// author_ids.
		if as := f.GetAuthorIds(); len(as) > 0 {
			where = append(where, jAuthorID+" = ANY("+b.place(as)+")")
		}
		// created_at window (indexed envelope column).
		if t := f.GetCreatedAfter(); t != nil {
			where = append(where, "created_at >= "+b.place(t.AsTime()))
		}
		if t := f.GetCreatedBefore(); t != nil {
			where = append(where, "created_at <= "+b.place(t.AsTime()))
		}
		// updated_at window (envelope column).
		if t := f.GetUpdatedAfter(); t != nil {
			where = append(where, "updated_at >= "+b.place(t.AsTime()))
		}
		if t := f.GetUpdatedBefore(); t != nil {
			where = append(where, "updated_at <= "+b.place(t.AsTime()))
		}
		// favorites_only is appended by List (needs the caller's account id).
	}

	// status facet (enum -> protojson string name).
	if len(query.GetStatuses()) > 0 {
		where = append(where, jStatus+" = ANY("+b.place(statusNames(query.GetStatuses()))+")")
	}
	if len(query.GetDbKinds()) > 0 {
		names := make([]string, 0, len(query.GetDbKinds()))
		for _, k := range query.GetDbKinds() {
			names = append(names, k.String())
		}
		where = append(where, jDBKind+" = ANY("+b.place(names)+")")
	}
	if len(query.GetProviders()) > 0 {
		names := make([]string, 0, len(query.GetProviders()))
		for _, p := range query.GetProviders() {
			names = append(names, p.String())
		}
		where = append(where, jProvider+" = ANY("+b.place(names)+")")
	}
	if len(query.GetProtocols()) > 0 {
		names := make([]string, 0, len(query.GetProtocols()))
		for _, p := range query.GetProtocols() {
			names = append(names, p.String())
		}
		where = append(where, jProtocol+" = ANY("+b.place(names)+")")
	}
	if len(query.GetTriggers()) > 0 {
		names := make([]string, 0, len(query.GetTriggers()))
		for _, t := range query.GetTriggers() {
			names = append(names, t.String())
		}
		where = append(where, jTrigger+" = ANY("+b.place(names)+")")
	}
	if v := query.GetStroppyVersions(); len(v) > 0 {
		where = append(where, jStroppyVersion+" = ANY("+b.place(v)+")")
	}
	if v := query.GetDbPresetIds(); len(v) > 0 {
		where = append(where, jDBPresetID+" = ANY("+b.place(v)+")")
	}
	if v := query.GetWorkloadPresetIds(); len(v) > 0 {
		where = append(where, jWorkloadPreset+" = ANY("+b.place(v)+")")
	}
	if v := query.GetTestPresetIds(); len(v) > 0 {
		where = append(where, jTestPresetID+" = ANY("+b.place(v)+")")
	}

	// suite membership: intentionally DROPPED — Run has no suite_run_id/
	// suite_cell_id (spec §5/§6.A). query.GetSuiteRunId()/GetSuiteCellIds()/
	// GetStandalone() are silently ignored here.

	// progress window (0..100).
	if query.ProgressMin != nil {
		where = append(where, jProgressPct+" >= "+b.place(int64(query.GetProgressMin())))
	}
	if query.ProgressMax != nil {
		where = append(where, jProgressPct+" <= "+b.place(int64(query.GetProgressMax())))
	}
	// duration window (seconds).
	if d := query.GetDurationMin(); d != nil {
		where = append(where, jDurationSecs+" >= "+b.place(d.AsDuration().Seconds()))
	}
	if d := query.GetDurationMax(); d != nil {
		where = append(where, jDurationSecs+" <= "+b.place(d.AsDuration().Seconds()))
	}
	// started/finished windows.
	if t := query.GetStartedAfter(); t != nil {
		where = append(where, jStartedAtTs+" >= "+b.place(t.AsTime()))
	}
	if t := query.GetStartedBefore(); t != nil {
		where = append(where, jStartedAtTs+" <= "+b.place(t.AsTime()))
	}
	if t := query.GetFinishedAfter(); t != nil {
		where = append(where, jFinishedAtTs+" >= "+b.place(t.AsTime()))
	}
	if t := query.GetFinishedBefore(); t != nil {
		where = append(where, jFinishedAtTs+" <= "+b.place(t.AsTime()))
	}

	return where
}
