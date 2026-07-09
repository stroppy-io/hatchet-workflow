package postgres

import (
	"context"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
)

const (
	defaultRecordPageSize = 50
	maxRecordPageSize     = 500
)

// encodeOffsetToken builds the opaque next-page cursor for offset n. Shared by
// every offset-paginated List (pageRecords here, RunRepo.List in run_list.go).
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

func filter[T any](records []T, keep func(T) bool) []T {
	out := records[:0]
	for _, rec := range records {
		if keep(rec) {
			out = append(out, rec)
		}
	}
	return out
}
