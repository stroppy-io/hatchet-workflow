package postgres

import (
	"context"
	"sort"
	"strings"
	"time"

	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultRecordPageSize = 50
	maxRecordPageSize     = 500
)

type entityRecord interface {
	GetEntity() *commonpb.Entity
}

func favoriteIDs(ctx context.Context, db *DB, tenantID, callerID string, kind commonpb.FavoriteKind) (map[string]bool, error) {
	if callerID == "" || kind == commonpb.FavoriteKind_FAVORITE_KIND_UNSPECIFIED {
		return nil, nil
	}
	rows, err := db.q().ListFavoriteRecords(ctx, dbgen.ListFavoriteRecordsParams{
		AccountID: callerID,
		TenantID:  tenantID,
		Kind:      int32(kind),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		rec := &models.FavoriteRecord{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, err
		}
		out[rec.GetTargetId()] = true
	}
	return out, nil
}

func applyEntityFilter[T entityRecord](records []T, filter *commonpb.EntityFilter, favorites map[string]bool) []T {
	out := make([]T, 0, len(records))
	for _, rec := range records {
		ent := rec.GetEntity()
		if ent != nil {
			ent.IsFavorite = favorites != nil && favorites[ent.GetId()]
		}
		if matchesEntityFilter(ent, filter, favorites) {
			out = append(out, rec)
		}
	}
	return out
}

func matchesEntityFilter(ent *commonpb.Entity, filter *commonpb.EntityFilter, favorites map[string]bool) bool {
	if ent == nil {
		return false
	}
	if filter == nil {
		return ent.GetTimings().GetDeletedAt() == nil
	}
	if !filter.GetIncludeDeleted() && ent.GetTimings().GetDeletedAt() != nil {
		return false
	}
	if filter.GetFavoritesOnly() && (favorites == nil || !favorites[ent.GetId()]) {
		return false
	}
	if ids := stringSet(filter.GetIds()); len(ids) > 0 && !ids[ent.GetId()] {
		return false
	}
	if authors := stringSet(filter.GetAuthorIds()); len(authors) > 0 && !authors[ent.GetAuthorId()] {
		return false
	}
	if search := strings.TrimSpace(filter.GetSearch()); search != "" {
		haystack := strings.ToLower(ent.GetName() + "\n" + ent.GetDescription())
		if !strings.Contains(haystack, strings.ToLower(search)) {
			return false
		}
	}
	if !timestampInWindow(ent.GetTimings().GetCreatedAt(), filter.GetCreatedAfter(), filter.GetCreatedBefore()) {
		return false
	}
	if !timestampInWindow(ent.GetTimings().GetUpdatedAt(), filter.GetUpdatedAfter(), filter.GetUpdatedBefore()) {
		return false
	}
	return true
}

func stringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

func enumSet[T ~int32](values []T) map[T]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[T]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

func timestampInWindow(value *timestamppb.Timestamp, after, before *timestamppb.Timestamp) bool {
	if after == nil && before == nil {
		return true
	}
	if value == nil {
		return false
	}
	at := value.AsTime()
	if after != nil && at.Before(after.AsTime()) {
		return false
	}
	if before != nil && at.After(before.AsTime()) {
		return false
	}
	return true
}

func pageRecords[T any](records []T, size uint32, token string) ([]T, string) {
	limit := int(size)
	if limit <= 0 {
		limit = defaultRecordPageSize
	}
	if limit > maxRecordPageSize {
		limit = maxRecordPageSize
	}
	offset := 0
	if n, ok := decodeOffsetToken(token); ok {
		offset = n
	}
	if offset >= len(records) {
		return nil, ""
	}
	end := offset + limit
	next := ""
	if end < len(records) {
		next = encodeOffsetToken(end)
	} else {
		end = len(records)
	}
	return records[offset:end], next
}

func sortByEntity[T entityRecord](records []T, sortSpec *commonpb.EntitySort) {
	desc := false
	field := commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT
	if sortSpec != nil {
		desc = sortSpec.GetDesc()
		if sortSpec.GetField() != commonpb.EntitySortField_ENTITY_SORT_FIELD_UNSPECIFIED {
			field = sortSpec.GetField()
		} else {
			desc = true
		}
	} else {
		desc = true
	}
	sortBy(records, desc, func(a, b T) int {
		return compareEntityField(a.GetEntity(), b.GetEntity(), field)
	})
}

func sortBy[T entityRecord](records []T, desc bool, compare func(a, b T) int) {
	sort.SliceStable(records, func(i, j int) bool {
		c := compare(records[i], records[j])
		if c == 0 {
			c = strings.Compare(records[i].GetEntity().GetId(), records[j].GetEntity().GetId())
		}
		if desc {
			return c > 0
		}
		return c < 0
	})
}

func compareEntityField(a, b *commonpb.Entity, field commonpb.EntitySortField) int {
	switch field {
	case commonpb.EntitySortField_ENTITY_SORT_FIELD_NAME:
		return strings.Compare(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
	case commonpb.EntitySortField_ENTITY_SORT_FIELD_UPDATED_AT:
		return compareTimestamp(a.GetTimings().GetUpdatedAt(), b.GetTimings().GetUpdatedAt())
	case commonpb.EntitySortField_ENTITY_SORT_FIELD_AUTHOR_ID:
		return strings.Compare(a.GetAuthorId(), b.GetAuthorId())
	case commonpb.EntitySortField_ENTITY_SORT_FIELD_FAVORITE:
		return compareBool(a.GetIsFavorite(), b.GetIsFavorite())
	default:
		return compareTimestamp(a.GetTimings().GetCreatedAt(), b.GetTimings().GetCreatedAt())
	}
}

func compareString(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

func compareUint32(a, b uint32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareInt32(a, b int32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareDuration(a, b *durationpb.Duration) int {
	av, bv := durationValue(a), durationValue(b)
	switch {
	case av < bv:
		return -1
	case av > bv:
		return 1
	default:
		return 0
	}
}

func durationValue(d *durationpb.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return d.AsDuration()
}

func compareTimestamp(a, b *timestamppb.Timestamp) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}
	at, bt := a.AsTime(), b.AsTime()
	switch {
	case at.Before(bt):
		return -1
	case at.After(bt):
		return 1
	default:
		return 0
	}
}

func filterSuiteRecords(ctx context.Context, db *DB, records []*models.SuiteRecord, query suite.SuiteListQuery) ([]*models.SuiteRecord, string, error) {
	favorites, err := favoriteIDs(ctx, db, query.TenantID, query.CallerID, commonpb.FavoriteKind_FAVORITE_KIND_SUITE)
	if err != nil {
		return nil, "", err
	}
	out := applyEntityFilter(records, query.Filter, favorites)
	if providers := enumSet(query.Providers); len(providers) > 0 {
		out = filter(out, func(rec *models.SuiteRecord) bool {
			return providers[rec.GetSpec().GetProvider()]
		})
	}
	if query.ScheduleEnabled != nil {
		want := *query.ScheduleEnabled
		out = filter(out, func(rec *models.SuiteRecord) bool {
			return suiteScheduleEnabled(rec) == want
		})
	}
	sortSuites(out, query.Sort)
	page, next := pageRecords(out, query.PageSize, query.PageToken)
	return page, next, nil
}

func suiteScheduleEnabled(rec *models.SuiteRecord) bool {
	if rec.GetSummary() != nil {
		return rec.GetSummary().GetScheduleEnabled()
	}
	return rec.GetSpec().GetSchedule().GetEnabled()
}

func sortSuites(records []*models.SuiteRecord, sortSpec *api.ListSuitesRequest_Sort) {
	desc := true
	compare := func(a, b *models.SuiteRecord) int {
		return compareEntityField(a.GetEntity(), b.GetEntity(), commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT)
	}
	if sortSpec != nil {
		desc = sortSpec.GetDesc()
		switch by := sortSpec.GetBy().(type) {
		case *api.ListSuitesRequest_Sort_Entity:
			field := by.Entity
			if field == commonpb.EntitySortField_ENTITY_SORT_FIELD_UNSPECIFIED {
				desc = true
				field = commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT
			}
			compare = func(a, b *models.SuiteRecord) int {
				return compareEntityField(a.GetEntity(), b.GetEntity(), field)
			}
		case *api.ListSuitesRequest_Sort_Kind_:
			switch by.Kind {
			case api.ListSuitesRequest_Sort_KIND_PROVIDER:
				compare = func(a, b *models.SuiteRecord) int {
					return compareInt32(int32(a.GetSpec().GetProvider()), int32(b.GetSpec().GetProvider()))
				}
			case api.ListSuitesRequest_Sort_KIND_SCHEDULE_ENABLED:
				compare = func(a, b *models.SuiteRecord) int {
					return compareBool(suiteScheduleEnabled(a), suiteScheduleEnabled(b))
				}
			case api.ListSuitesRequest_Sort_KIND_NEXT_RUN_AT:
				compare = func(a, b *models.SuiteRecord) int {
					return compareTimestamp(a.GetSummary().GetNextRunAt(), b.GetSummary().GetNextRunAt())
				}
			case api.ListSuitesRequest_Sort_KIND_LAST_RUN_AT:
				compare = func(a, b *models.SuiteRecord) int {
					return compareTimestamp(a.GetSummary().GetLastRunAt(), b.GetSummary().GetLastRunAt())
				}
			case api.ListSuitesRequest_Sort_KIND_RUN_COUNT:
				compare = func(a, b *models.SuiteRecord) int {
					return compareUint32(a.GetSummary().GetRunCount(), b.GetSummary().GetRunCount())
				}
			}
		}
	}
	sortBy(records, desc, compare)
}

func filterSuiteRunRecords(ctx context.Context, db *DB, records []*models.SuiteRunRecord, req *api.ListSuiteRunsRequest, callerID string) ([]*models.SuiteRunRecord, string, error) {
	favorites, err := favoriteIDs(ctx, db, req.GetTenantId(), callerID, commonpb.FavoriteKind_FAVORITE_KIND_SUITE_RUN)
	if err != nil {
		return nil, "", err
	}
	out := applyEntityFilter(records, req.GetFilter(), favorites)
	if statuses := enumSet(req.GetStatuses()); len(statuses) > 0 {
		out = filter(out, func(rec *models.SuiteRunRecord) bool { return statuses[rec.GetStatus()] })
	}
	if providers := enumSet(req.GetProviders()); len(providers) > 0 {
		out = filter(out, func(rec *models.SuiteRunRecord) bool { return providers[rec.GetSummary().GetProvider()] })
	}
	if kinds := enumSet(req.GetDbKinds()); len(kinds) > 0 {
		out = filter(out, func(rec *models.SuiteRunRecord) bool {
			for _, k := range rec.GetSummary().GetDbKinds() {
				if kinds[k] {
					return true
				}
			}
			return false
		})
	}
	if suiteID := req.GetSuiteId(); suiteID != "" {
		out = filter(out, func(rec *models.SuiteRunRecord) bool { return rec.GetSuiteId() == suiteID })
	}
	if triggers := enumSet(req.GetTriggers()); len(triggers) > 0 {
		out = filter(out, func(rec *models.SuiteRunRecord) bool { return triggers[rec.GetTrigger()] })
	}
	if req.ProgressMin != nil {
		out = filter(out, func(rec *models.SuiteRunRecord) bool {
			return rec.GetSummary().GetProgressPct() >= req.GetProgressMin()
		})
	}
	if req.ProgressMax != nil {
		out = filter(out, func(rec *models.SuiteRunRecord) bool {
			return rec.GetSummary().GetProgressPct() <= req.GetProgressMax()
		})
	}
	if d := req.GetDurationMin(); d != nil {
		min := d.AsDuration()
		out = filter(out, func(rec *models.SuiteRunRecord) bool { return durationValue(rec.GetSummary().GetDuration()) >= min })
	}
	if d := req.GetDurationMax(); d != nil {
		max := d.AsDuration()
		out = filter(out, func(rec *models.SuiteRunRecord) bool { return durationValue(rec.GetSummary().GetDuration()) <= max })
	}
	out = filterTimeWindow(out, req.GetStartedAfter(), req.GetStartedBefore(), func(rec *models.SuiteRunRecord) *timestamppb.Timestamp {
		return rec.GetSummary().GetStartedAt()
	})
	out = filterTimeWindow(out, req.GetFinishedAfter(), req.GetFinishedBefore(), func(rec *models.SuiteRunRecord) *timestamppb.Timestamp {
		return rec.GetSummary().GetFinishedAt()
	})
	sortSuiteRuns(out, req.GetSort())
	page, next := pageRecords(out, req.GetPage().GetSize(), req.GetPage().GetToken())
	return page, next, nil
}

func sortSuiteRuns(records []*models.SuiteRunRecord, sortSpec *api.ListSuiteRunsRequest_Sort) {
	desc := true
	compare := func(a, b *models.SuiteRunRecord) int {
		return compareEntityField(a.GetEntity(), b.GetEntity(), commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT)
	}
	if sortSpec != nil {
		desc = sortSpec.GetDesc()
		switch by := sortSpec.GetBy().(type) {
		case *api.ListSuiteRunsRequest_Sort_Entity:
			field := by.Entity
			if field == commonpb.EntitySortField_ENTITY_SORT_FIELD_UNSPECIFIED {
				desc = true
				field = commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT
			}
			compare = func(a, b *models.SuiteRunRecord) int {
				return compareEntityField(a.GetEntity(), b.GetEntity(), field)
			}
		case *api.ListSuiteRunsRequest_Sort_Kind_:
			switch by.Kind {
			case api.ListSuiteRunsRequest_Sort_KIND_STATUS:
				compare = func(a, b *models.SuiteRunRecord) int { return compareInt32(int32(a.GetStatus()), int32(b.GetStatus())) }
			case api.ListSuiteRunsRequest_Sort_KIND_PROVIDER:
				compare = func(a, b *models.SuiteRunRecord) int {
					return compareInt32(int32(a.GetSummary().GetProvider()), int32(b.GetSummary().GetProvider()))
				}
			case api.ListSuiteRunsRequest_Sort_KIND_PROGRESS:
				compare = func(a, b *models.SuiteRunRecord) int {
					return compareUint32(a.GetSummary().GetProgressPct(), b.GetSummary().GetProgressPct())
				}
			case api.ListSuiteRunsRequest_Sort_KIND_DURATION:
				compare = func(a, b *models.SuiteRunRecord) int {
					return compareDuration(a.GetSummary().GetDuration(), b.GetSummary().GetDuration())
				}
			case api.ListSuiteRunsRequest_Sort_KIND_STARTED_AT:
				compare = func(a, b *models.SuiteRunRecord) int {
					return compareTimestamp(a.GetSummary().GetStartedAt(), b.GetSummary().GetStartedAt())
				}
			case api.ListSuiteRunsRequest_Sort_KIND_FINISHED_AT:
				compare = func(a, b *models.SuiteRunRecord) int {
					return compareTimestamp(a.GetSummary().GetFinishedAt(), b.GetSummary().GetFinishedAt())
				}
			case api.ListSuiteRunsRequest_Sort_KIND_TOTAL:
				compare = func(a, b *models.SuiteRunRecord) int {
					return compareUint32(a.GetSummary().GetTotal(), b.GetSummary().GetTotal())
				}
			}
		}
	}
	sortBy(records, desc, compare)
}

func filterPackageRecords(records []*models.PackageRecord, q packages.PackageQuery) ([]*models.PackageRecord, string) {
	out := applyEntityFilter(records, q.Filter, nil)
	if formats := enumSet(q.Formats); len(formats) > 0 {
		out = filter(out, func(rec *models.PackageRecord) bool { return formats[rec.GetFormat()] })
	}
	if kinds := enumSet(q.DbKinds); len(kinds) > 0 {
		out = filter(out, func(rec *models.PackageRecord) bool { return kinds[rec.GetTargetDbKind()] })
	}
	sortByEntity(out, q.Sort)
	return pageRecords(out, q.PageSize, q.PageToken)
}

func filterShareRecords(records []*models.ShareRecord, targetID string, filterSpec *commonpb.EntityFilter, sortSpec *commonpb.EntitySort, page *commonpb.Page) ([]*models.ShareRecord, string) {
	out := applyEntityFilter(records, filterSpec, nil)
	if targetID != "" {
		out = filter(out, func(rec *models.ShareRecord) bool { return rec.GetTarget().GetId() == targetID })
	}
	sortByEntity(out, sortSpec)
	return pageRecords(out, page.GetSize(), page.GetToken())
}

func filterDraftRecords[T entityRecord](records []T, filterSpec *commonpb.EntityFilter, sortSpec *commonpb.EntitySort, page *commonpb.Page) ([]T, string) {
	out := applyEntityFilter(records, filterSpec, nil)
	sortByEntity(out, sortSpec)
	return pageRecords(out, page.GetSize(), page.GetToken())
}

func filterDatabasePresetRecords(ctx context.Context, db *DB, records []*models.DatabasePresetRecord, req *api.ListDatabasePresetsRequest, callerID string) ([]*models.DatabasePresetRecord, string, error) {
	favorites, err := favoriteIDs(ctx, db, req.GetTenantId(), callerID, commonpb.FavoriteKind_FAVORITE_KIND_DATABASE_PRESET)
	if err != nil {
		return nil, "", err
	}
	for _, rec := range records {
		ensureDatabasePresetSummary(rec)
	}
	out := applyEntityFilter(records, req.GetFilter(), favorites)
	if tags := req.GetTags(); len(tags) > 0 {
		out = filter(out, func(rec *models.DatabasePresetRecord) bool { return tagsMatch(rec.GetDatabase().GetTags(), tags) })
	}
	if kinds := enumSet(req.GetDbKinds()); len(kinds) > 0 {
		out = filter(out, func(rec *models.DatabasePresetRecord) bool { return kinds[rec.GetSummary().GetDbKind()] })
	}
	if sources := enumSet(req.GetSources()); len(sources) > 0 {
		out = filter(out, func(rec *models.DatabasePresetRecord) bool { return sources[databasePresetSource(rec.GetDatabase())] })
	}
	if req.IsSystem != nil {
		want := req.GetIsSystem()
		out = filter(out, func(rec *models.DatabasePresetRecord) bool { return rec.GetIsSystem() == want })
	}
	sortDatabasePresets(out, req.GetSort())
	page, next := pageRecords(out, req.GetPage().GetSize(), req.GetPage().GetToken())
	return page, next, nil
}

func sortDatabasePresets(records []*models.DatabasePresetRecord, sortSpec *api.ListDatabasePresetsRequest_Sort) {
	desc := true
	compare := func(a, b *models.DatabasePresetRecord) int {
		return compareEntityField(a.GetEntity(), b.GetEntity(), commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT)
	}
	if sortSpec != nil {
		desc = sortSpec.GetDesc()
		switch by := sortSpec.GetBy().(type) {
		case *api.ListDatabasePresetsRequest_Sort_Entity:
			field := by.Entity
			if field == commonpb.EntitySortField_ENTITY_SORT_FIELD_UNSPECIFIED {
				desc = true
				field = commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT
			}
			compare = func(a, b *models.DatabasePresetRecord) int {
				return compareEntityField(a.GetEntity(), b.GetEntity(), field)
			}
		case *api.ListDatabasePresetsRequest_Sort_Kind_:
			switch by.Kind {
			case api.ListDatabasePresetsRequest_Sort_KIND_DB_KIND:
				compare = func(a, b *models.DatabasePresetRecord) int {
					return compareInt32(int32(a.GetSummary().GetDbKind()), int32(b.GetSummary().GetDbKind()))
				}
			case api.ListDatabasePresetsRequest_Sort_KIND_IS_SYSTEM:
				compare = func(a, b *models.DatabasePresetRecord) int { return compareBool(a.GetIsSystem(), b.GetIsSystem()) }
			}
		}
	}
	sortBy(records, desc, compare)
}

func filterWorkloadPresetRecords(ctx context.Context, db *DB, records []*models.WorkloadPresetRecord, req *api.ListWorkloadPresetsRequest, callerID string) ([]*models.WorkloadPresetRecord, string, error) {
	favorites, err := favoriteIDs(ctx, db, req.GetTenantId(), callerID, commonpb.FavoriteKind_FAVORITE_KIND_WORKLOAD_PRESET)
	if err != nil {
		return nil, "", err
	}
	for _, rec := range records {
		ensureWorkloadPresetSummary(rec)
	}
	out := applyEntityFilter(records, req.GetFilter(), favorites)
	if tags := req.GetTags(); len(tags) > 0 {
		out = filter(out, func(rec *models.WorkloadPresetRecord) bool { return tagsMatch(rec.GetWorkload().GetTags(), tags) })
	}
	if versions := stringSet(req.GetStroppyVersions()); len(versions) > 0 {
		out = filter(out, func(rec *models.WorkloadPresetRecord) bool { return versions[rec.GetSummary().GetStroppyVersion()] })
	}
	if req.IsSystem != nil {
		want := req.GetIsSystem()
		out = filter(out, func(rec *models.WorkloadPresetRecord) bool { return rec.GetIsSystem() == want })
	}
	if protocols := enumSet(req.GetProtocols()); len(protocols) > 0 {
		out = filter(out, func(rec *models.WorkloadPresetRecord) bool { return protocols[rec.GetSummary().GetProtocol()] })
	}
	if scripts := stringSet(req.GetScripts()); len(scripts) > 0 {
		out = filter(out, func(rec *models.WorkloadPresetRecord) bool { return scripts[rec.GetSummary().GetScript()] })
	}
	sortWorkloadPresets(out, req.GetSort())
	page, next := pageRecords(out, req.GetPage().GetSize(), req.GetPage().GetToken())
	return page, next, nil
}

func sortWorkloadPresets(records []*models.WorkloadPresetRecord, sortSpec *api.ListWorkloadPresetsRequest_Sort) {
	desc := true
	compare := func(a, b *models.WorkloadPresetRecord) int {
		return compareEntityField(a.GetEntity(), b.GetEntity(), commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT)
	}
	if sortSpec != nil {
		desc = sortSpec.GetDesc()
		switch by := sortSpec.GetBy().(type) {
		case *api.ListWorkloadPresetsRequest_Sort_Entity:
			field := by.Entity
			if field == commonpb.EntitySortField_ENTITY_SORT_FIELD_UNSPECIFIED {
				desc = true
				field = commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT
			}
			compare = func(a, b *models.WorkloadPresetRecord) int {
				return compareEntityField(a.GetEntity(), b.GetEntity(), field)
			}
		case *api.ListWorkloadPresetsRequest_Sort_Kind_:
			switch by.Kind {
			case api.ListWorkloadPresetsRequest_Sort_KIND_STROPPY_VERSION:
				compare = func(a, b *models.WorkloadPresetRecord) int {
					return compareString(a.GetSummary().GetStroppyVersion(), b.GetSummary().GetStroppyVersion())
				}
			case api.ListWorkloadPresetsRequest_Sort_KIND_IS_SYSTEM:
				compare = func(a, b *models.WorkloadPresetRecord) int { return compareBool(a.GetIsSystem(), b.GetIsSystem()) }
			case api.ListWorkloadPresetsRequest_Sort_KIND_PROTOCOL:
				compare = func(a, b *models.WorkloadPresetRecord) int {
					return compareInt32(int32(a.GetSummary().GetProtocol()), int32(b.GetSummary().GetProtocol()))
				}
			case api.ListWorkloadPresetsRequest_Sort_KIND_SCRIPT:
				compare = func(a, b *models.WorkloadPresetRecord) int {
					return compareString(a.GetSummary().GetScript(), b.GetSummary().GetScript())
				}
			}
		}
	}
	sortBy(records, desc, compare)
}

func filterTestPresetRecords(ctx context.Context, db *DB, records []*models.TestPresetRecord, req *api.ListTestPresetsRequest, callerID string) ([]*models.TestPresetRecord, string, error) {
	favorites, err := favoriteIDs(ctx, db, req.GetTenantId(), callerID, commonpb.FavoriteKind_FAVORITE_KIND_TEST_PRESET)
	if err != nil {
		return nil, "", err
	}
	for _, rec := range records {
		ensureTestPresetSummary(rec)
	}
	out := applyEntityFilter(records, req.GetFilter(), favorites)
	if tags := req.GetTags(); len(tags) > 0 {
		out = filter(out, func(rec *models.TestPresetRecord) bool { return tagsMatch(rec.GetTest().GetTags(), tags) })
	}
	if kinds := enumSet(req.GetDbKinds()); len(kinds) > 0 {
		out = filter(out, func(rec *models.TestPresetRecord) bool { return kinds[rec.GetSummary().GetDbKind()] })
	}
	if versions := stringSet(req.GetStroppyVersions()); len(versions) > 0 {
		out = filter(out, func(rec *models.TestPresetRecord) bool { return versions[rec.GetSummary().GetStroppyVersion()] })
	}
	if req.IsSystem != nil {
		want := req.GetIsSystem()
		out = filter(out, func(rec *models.TestPresetRecord) bool { return rec.GetIsSystem() == want })
	}
	if protocols := enumSet(req.GetProtocols()); len(protocols) > 0 {
		out = filter(out, func(rec *models.TestPresetRecord) bool { return protocols[rec.GetSummary().GetProtocol()] })
	}
	sortTestPresets(out, req.GetSort())
	page, next := pageRecords(out, req.GetPage().GetSize(), req.GetPage().GetToken())
	return page, next, nil
}

func sortTestPresets(records []*models.TestPresetRecord, sortSpec *api.ListTestPresetsRequest_Sort) {
	desc := true
	compare := func(a, b *models.TestPresetRecord) int {
		return compareEntityField(a.GetEntity(), b.GetEntity(), commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT)
	}
	if sortSpec != nil {
		desc = sortSpec.GetDesc()
		switch by := sortSpec.GetBy().(type) {
		case *api.ListTestPresetsRequest_Sort_Entity:
			field := by.Entity
			if field == commonpb.EntitySortField_ENTITY_SORT_FIELD_UNSPECIFIED {
				desc = true
				field = commonpb.EntitySortField_ENTITY_SORT_FIELD_CREATED_AT
			}
			compare = func(a, b *models.TestPresetRecord) int {
				return compareEntityField(a.GetEntity(), b.GetEntity(), field)
			}
		case *api.ListTestPresetsRequest_Sort_Kind_:
			switch by.Kind {
			case api.ListTestPresetsRequest_Sort_KIND_DB_KIND:
				compare = func(a, b *models.TestPresetRecord) int {
					return compareInt32(int32(a.GetSummary().GetDbKind()), int32(b.GetSummary().GetDbKind()))
				}
			case api.ListTestPresetsRequest_Sort_KIND_STROPPY_VERSION:
				compare = func(a, b *models.TestPresetRecord) int {
					return compareString(a.GetSummary().GetStroppyVersion(), b.GetSummary().GetStroppyVersion())
				}
			case api.ListTestPresetsRequest_Sort_KIND_IS_SYSTEM:
				compare = func(a, b *models.TestPresetRecord) int { return compareBool(a.GetIsSystem(), b.GetIsSystem()) }
			case api.ListTestPresetsRequest_Sort_KIND_PROTOCOL:
				compare = func(a, b *models.TestPresetRecord) int {
					return compareInt32(int32(a.GetSummary().GetProtocol()), int32(b.GetSummary().GetProtocol()))
				}
			}
		}
	}
	sortBy(records, desc, compare)
}

func filter[T any](records []T, keep func(T) bool) []T {
	out := records[:0]
	for _, rec := range records {
		if keep(rec) {
			out = append(out, rec)
		}
	}
	return out
}

func filterTimeWindow[T any](records []T, after, before *timestamppb.Timestamp, value func(T) *timestamppb.Timestamp) []T {
	if after == nil && before == nil {
		return records
	}
	return filter(records, func(rec T) bool {
		return timestampInWindow(value(rec), after, before)
	})
}

func tagsMatch(tags *commonpb.Tags, want map[string]string) bool {
	labels := tags.GetLabels()
	for k, v := range want {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func databasePresetSource(db *domain.Database) api.ListDatabasePresetsRequest_SourceKind {
	switch db.GetSource().(type) {
	case *domain.Database_Params:
		return api.ListDatabasePresetsRequest_SOURCE_KIND_PARAMS
	case *domain.Database_External_:
		return api.ListDatabasePresetsRequest_SOURCE_KIND_EXTERNAL
	case *domain.Database_DatabasePresetId:
		return api.ListDatabasePresetsRequest_SOURCE_KIND_PRESET_REF
	default:
		return api.ListDatabasePresetsRequest_SOURCE_KIND_UNSPECIFIED
	}
}

func ensureDatabasePresetSummary(rec *models.DatabasePresetRecord) {
	if rec == nil || rec.Summary != nil {
		return
	}
	rec.Summary = &models.DatabasePresetRecord_Summary{
		DbKind:   rec.GetDatabase().GetKind(),
		Version:  rec.GetDatabase().GetParams().GetVersion(),
		External: rec.GetDatabase().GetExternal() != nil,
	}
}

func ensureWorkloadPresetSummary(rec *models.WorkloadPresetRecord) {
	if rec == nil || rec.Summary != nil {
		return
	}
	rec.Summary = &models.WorkloadPresetRecord_Summary{
		Protocol:       rec.GetWorkload().GetProtocol(),
		StroppyVersion: rec.GetWorkload().GetStroppyVersion(),
		Script:         rec.GetWorkload().GetScript(),
	}
}

func ensureTestPresetSummary(rec *models.TestPresetRecord) {
	if rec == nil || rec.Summary != nil {
		return
	}
	rec.Summary = &models.TestPresetRecord_Summary{
		DbKind:         rec.GetTest().GetDatabase().GetKind(),
		Protocol:       rec.GetTest().GetWorkload().GetProtocol(),
		StroppyVersion: rec.GetTest().GetWorkload().GetStroppyVersion(),
	}
}
