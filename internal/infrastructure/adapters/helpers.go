package adapters

import (
	"sort"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// nowTimestamp returns the current wall-clock time as a protobuf Timestamp. It is
// the single time source the adapters use so behavior is consistent across them.
func nowTimestamp() *timestamppb.Timestamp { return timestamppb.New(time.Now()) }

// createdAt extracts an entity's created-at instant (zero time when unset).
func createdAt(e *common.Entity) time.Time {
	if e.GetTimings().GetCreatedAt() == nil {
		return time.Time{}
	}
	return e.GetTimings().GetCreatedAt().AsTime()
}

// sortByCreatedDesc sorts a slice newest-first by the time projected from each
// element.
func sortByCreatedDesc[T any](items []T, at func(T) time.Time) {
	sort.SliceStable(items, func(i, j int) bool { return at(items[i]).After(at(items[j])) })
}

// sortByTimeAsc sorts a slice soonest-first by the time projected from each
// element.
func sortByTimeAsc[T any](items []T, at func(T) time.Time) {
	sort.SliceStable(items, func(i, j int) bool { return at(items[i]).Before(at(items[j])) })
}

// capRuns truncates a slice to the first limit elements (limit 0 = no cap).
func capRuns[T any](items []T, limit uint32) []T {
	if limit > 0 && uint32(len(items)) > limit {
		return items[:limit]
	}
	return items
}
