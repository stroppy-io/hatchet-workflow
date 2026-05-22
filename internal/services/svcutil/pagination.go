package svcutil

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"

const (
	defaultPageSize = 50
	maxPageSize     = 1000
)

// PageSize returns the effective page size from a request Page: the requested size
// clamped to (0, maxPageSize], defaulting to defaultPageSize when unset. List queries
// should fetch PageSize()+1 rows to detect whether a next page exists.
func PageSize(p *models.Page) int {
	n := int(p.GetSize())
	if n <= 0 {
		return defaultPageSize
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return n
}

// CursorDesc reports the sort direction for cursor pagination — newest-first (DESC)
// by default, ASC only when explicitly requested.
func CursorDesc(order models.SortOrder) bool {
	return order != models.SortOrder_SORT_ORDER_ASC
}

// Paginate finalizes a cursor page: given the over-fetched rows (PageSize()+1) it
// trims them to size and builds the PageInfo whose next_token is the last row's
// opaque id cursor (ULIDs are monotonic, so id order == creation order). When fewer
// than size+1 rows came back there is no next page.
func Paginate[T any](rows []T, size int, idOf func(T) string) ([]T, *models.PageInfo) {
	if len(rows) > size {
		rows = rows[:size]
		return rows, &models.PageInfo{NextToken: idOf(rows[len(rows)-1]), HasMore: true}
	}
	return rows, &models.PageInfo{HasMore: false}
}
