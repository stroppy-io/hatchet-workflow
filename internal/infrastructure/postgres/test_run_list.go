package postgres

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

/*
	===== ListTestRuns dynamic query =====

	The full ListTestRunsRequest is honored here against postgres. Rows store the
	canonical TestRunRecord as protojson in the `data jsonb` column (default
	protojson.MarshalOptions: camelCase json names, enums as their STRING names,
	timestamps as RFC3339 strings, durations as "<seconds>s" strings). Indexed
	envelope columns (tenant_id, created_at, updated_at, id) are used where they
	exist; every other facet reads the jsonb blob with jsonb path operators.

	The WHERE/ORDER/LIMIT are built dynamically but every value is bound through a
	$n placeholder (NO string interpolation of request data). Pagination is
	offset-based: page.token is a base64("offset:<n>") cursor and next_page_token
	is emitted only when a full page was returned (a further page may exist).
*/

const (
	defaultTestRunPageSize = 50
	maxTestRunPageSize     = 500
)

// jsonb path expressions into the TestRunRecord blob (protojson camelCase).
const (
	jNameLower      = "lower(coalesce(data->'entity'->>'name',''))"
	jSearchText     = "lower(coalesce(data->'entity'->>'name','') || E'\\n' || coalesce(data->'entity'->>'description',''))"
	jAuthorID       = "data->'entity'->>'authorId'"
	jDeletedAt      = "data->'entity'->'timings'->>'deletedAt'"
	jUpdatedAtTs    = "(data->'entity'->'timings'->>'updatedAt')::timestamptz"
	jStatus         = "data->>'status'"
	jTrigger        = "data->>'trigger'"
	jSuiteRunID     = "coalesce(data->>'suiteRunId','')"
	jSuiteCellID    = "coalesce(data->>'suiteCellId','')"
	jDBKind         = "data->'summary'->>'dbKind'"
	jProvider       = "data->'summary'->>'provider'"
	jProtocol       = "data->'summary'->>'workloadProtocol'"
	jStroppyVersion = "data->'summary'->>'stroppyVersion'"
	jWorkloadName   = "coalesce(data->'summary'->>'workloadName','')"
	jDBPresetID     = "data->'summary'->>'dbPresetId'"
	jWorkloadPreset = "data->'summary'->>'workloadPresetId'"
	jTestPresetID   = "data->'summary'->>'testPresetId'"
	jProgressPct    = "coalesce((data->'summary'->>'progressPct')::numeric,0)"
	jNodeCount      = "coalesce((data->'summary'->>'nodeCount')::numeric,0)"
	jStartedAtTs    = "(data->'summary'->>'startedAt')::timestamptz"
	jFinishedAtTs   = "(data->'summary'->>'finishedAt')::timestamptz"
	// duration is protojson "<float>s"; strip trailing 's' and cast to numeric seconds.
	jDurationSecs = "coalesce(nullif(rtrim(data->'summary'->>'duration','s'),'')::numeric,0)"
)

// argBuilder accumulates parameterized $n placeholders for a dynamic query.
type argBuilder struct {
	args []any
}

// place appends v and returns its $n placeholder.
func (b *argBuilder) place(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

// List honors the full ListTestRunsRequest against postgres: tenant scope plus
// every filter/facet, the requested sort (entity or kind, asc/desc) and
// offset-based pagination. Returns the matching page and the next-page token.
func (r *TestRunRepo) List(ctx context.Context, query *api.ListTestRunsRequest, callerAccountID string) ([]*models.TestRunRecord, string, error) {
	b := &argBuilder{}
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
		// favorites_only is per caller, matching common.Entity.is_favorite.
		if f.GetFavoritesOnly() {
			where = append(where, "EXISTS (SELECT 1 FROM favorite_records fr WHERE fr.tenant_id = "+
				b.place(query.GetTenantId())+" AND fr.account_id = "+
				b.place(callerAccountID)+" AND fr.kind = "+
				b.place(int32(commonpb.FavoriteKind_FAVORITE_KIND_TEST_RUN))+
				" AND fr.target_id = test_run_records.id)")
		}
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

	// suite membership.
	if id := query.GetSuiteRunId(); id != "" {
		where = append(where, jSuiteRunID+" = "+b.place(id))
	}
	if v := query.GetSuiteCellIds(); len(v) > 0 {
		where = append(where, jSuiteCellID+" = ANY("+b.place(v)+")")
	}
	if query.Standalone != nil {
		if query.GetStandalone() {
			where = append(where, jSuiteRunID+" = ''")
		} else {
			where = append(where, jSuiteRunID+" <> ''")
		}
	}

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
	sb.WriteString("SELECT data FROM test_run_records")
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

	out := make([]*models.TestRunRecord, 0, size)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, "", err
		}
		rec := &models.TestRunRecord{}
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

// statusNames maps a status enum slice to their protojson string names.
func statusNames(ss []commonpb.Status) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.String())
	}
	return out
}

func testRunSearchClause(b *argBuilder, search string) string {
	return jSearchText + " LIKE " + b.place("%"+strings.ToLower(search)+"%")
}

// buildOrderBy returns the ORDER BY clause for the requested sort. Unset sort or
// an unknown kind falls back to a stable created_at DESC. A secondary id ASC key
// makes every ordering deterministic for stable offset paging.
func buildOrderBy(sort *api.ListTestRunsRequest_Sort) string {
	col := "created_at"
	dir := "DESC"
	if sort != nil {
		dir = "ASC"
		if sort.GetDesc() {
			dir = "DESC"
		}
		switch by := sort.GetBy().(type) {
		case *api.ListTestRunsRequest_Sort_Entity:
			switch by.Entity {
			case commonpb.EntitySortField_ENTITY_SORT_FIELD_NAME:
				col = jNameLower
			case commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT:
				col = "created_at"
			case commonpb.EntitySortField_ENTITY_SORT_FIELD_UPDATED_AT:
				col = "updated_at"
			case commonpb.EntitySortField_ENTITY_SORT_FIELD_AUTHOR_ID:
				col = jAuthorID
			default:
				// FAVORITE / UNSPECIFIED: no per-row column without the caller —
				// keep the stable created_at default.
				col, dir = "created_at", "DESC"
			}
		case *api.ListTestRunsRequest_Sort_Kind_:
			switch by.Kind {
			case api.ListTestRunsRequest_Sort_KIND_STATUS:
				col = jStatus
			case api.ListTestRunsRequest_Sort_KIND_DB_KIND:
				col = jDBKind
			case api.ListTestRunsRequest_Sort_KIND_WORKLOAD:
				col = jWorkloadName
			case api.ListTestRunsRequest_Sort_KIND_PROVIDER:
				col = jProvider
			case api.ListTestRunsRequest_Sort_KIND_PROGRESS:
				col = jProgressPct
			case api.ListTestRunsRequest_Sort_KIND_DURATION:
				col = jDurationSecs
			case api.ListTestRunsRequest_Sort_KIND_STARTED_AT:
				col = jStartedAtTs
			case api.ListTestRunsRequest_Sort_KIND_FINISHED_AT:
				col = jFinishedAtTs
			case api.ListTestRunsRequest_Sort_KIND_NODE_COUNT:
				col = jNodeCount
			case api.ListTestRunsRequest_Sort_KIND_PROTOCOL:
				col = jProtocol
			case api.ListTestRunsRequest_Sort_KIND_TEST_PRESET:
				col = jTestPresetID
			case api.ListTestRunsRequest_Sort_KIND_TRIGGER:
				col = jTrigger
			default:
				col, dir = "created_at", "DESC"
			}
		}
	}
	// NULLS LAST keeps unset timestamps/values at the end for either direction.
	return fmt.Sprintf("%s %s NULLS LAST, id ASC", col, dir)
}

// encodeOffsetToken builds the opaque next-page cursor for offset n.
func encodeOffsetToken(n int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("offset:" + strconv.Itoa(n)))
}

// decodeOffsetToken parses an opaque cursor back to an offset; ok=false for an
// empty/invalid token (treated as first page).
func decodeOffsetToken(tok string) (int, bool) {
	if tok == "" {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return 0, false
	}
	s := string(raw)
	if !strings.HasPrefix(s, "offset:") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "offset:"))
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
