package adapters

import (
	"context"
	"sort"
	"strconv"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/public_rating"
	"github.com/stroppy-io/stroppy-cloud/internal/services/rating"
)

// defaultRatingPageSize caps an unbounded board request.
const defaultRatingPageSize uint32 = 50

// RatingRunsScope selects which pool of runs a board ranks.
type RatingRunsScope int

const (
	// RatingScopeGlobal ranks every run flagged in_global_rating across tenants.
	RatingScopeGlobal RatingRunsScope = iota
	// RatingScopeTenant ranks one tenant's runs flagged in_tenant_rating.
	RatingScopeTenant
)

// RatingRunsLister is the consumer interface the rating boards read candidate
// runs through. The implementation (gormstore TestRuns repo) lists every
// rating-flagged run for the scope (global -> in_global_rating across tenants;
// tenant -> in_tenant_rating for tenantID), already filtered to terminal/
// completed runs so a ranked entry always has metrics. The boards apply the
// remaining facets, ranking and pagination in memory.
type RatingRunsLister interface {
	ListRatingRuns(ctx context.Context, scope RatingRunsScope, tenantID string) ([]*models.TestRunRecord, error)
}

// RatingNameResolver optionally resolves display names for the entries
// (author/tenant). A nil resolver leaves those labels empty.
type RatingNameResolver interface {
	AccountName(ctx context.Context, accountID string) string
	TenantName(ctx context.Context, tenantID string) string
}

// RatingBoard implements rating.RatingBoard (system + tenant private boards) by
// reading rating-flagged runs, ranking them by the requested metric honoring its
// higher_is_better direction, applying the AND-ed facets and paginating with an
// offset cursor.
type RatingBoard struct {
	runs    RatingRunsLister
	metrics RunMetricsGetter
	names   RatingNameResolver
}

var _ rating.RatingBoard = (*RatingBoard)(nil)

// NewRatingBoard builds the rating board. names may be nil.
func NewRatingBoard(runs RatingRunsLister, metrics RunMetricsGetter, names RatingNameResolver) *RatingBoard {
	return &RatingBoard{runs: runs, metrics: metrics, names: names}
}

// Rank ranks the board for the query. derrors.ErrNotFound when the metric_key is
// not present on any candidate run.
func (b *RatingBoard) Rank(ctx context.Context, q rating.RatingQuery) ([]*api.RatingEntry, string, error) {
	scope := RatingScopeGlobal
	if q.Scope == rating.ScopeTenant {
		scope = RatingScopeTenant
	}
	runs, err := b.runs.ListRatingRuns(ctx, scope, q.TenantID)
	if err != nil {
		return nil, "", err
	}

	ranked, found, err := rankRuns(ctx, b.metrics, runs, ratingFacets{
		metricKey:       q.MetricKey,
		dbKinds:         q.DBKinds,
		stroppyVersions: q.StroppyVersions,
		providers:       q.Providers,
	})
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", derrors.NotFound("metric", "metric_key not found on any rated run")
	}

	limit := q.Limit
	if limit == 0 {
		limit = defaultRatingPageSize
	}
	page, next := paginate(ranked, q.PageToken, limit)

	entries := make([]*api.RatingEntry, 0, len(page))
	for _, r := range page {
		e := &api.RatingEntry{
			Rank:           r.rank,
			MetricValue:    r.value,
			MetricUnit:     r.unit,
			DbKind:         r.run.GetSummary().GetDbKind(),
			WorkloadName:   r.run.GetSummary().GetWorkloadName(),
			StroppyVersion: r.run.GetSummary().GetStroppyVersion(),
			Provider:       r.run.GetSummary().GetProvider(),
			TopologyLabel:  r.run.GetSummary().GetTopologyLabel(),
			NodeCount:      r.run.GetSummary().GetNodeCount(),
			RunAt:          r.run.GetSummary().GetStartedAt(),
			RunId:          r.run.GetEntity().GetId(),
		}
		if b.names != nil {
			e.AuthorName = b.names.AccountName(ctx, r.run.GetEntity().GetAuthorId())
			if scope == RatingScopeGlobal {
				e.TenantName = b.names.TenantName(ctx, r.run.GetEntity().GetTenantId())
			}
		}
		entries = append(entries, e)
	}
	return entries, next, nil
}

// ratingTenantQuery builds a tenant-scoped RatingQuery for the dashboard's
// top-benchmarks tile (no facets beyond the metric, capped at limit).
func ratingTenantQuery(tenantID, metricKey string, limit uint32) rating.RatingQuery {
	return rating.RatingQuery{
		Scope:     rating.ScopeTenant,
		TenantID:  tenantID,
		MetricKey: metricKey,
		Limit:     limit,
	}
}

// PublicRatingBoard implements public_rating.PublicRatingBoard: the public,
// sensitive-fields-stripped leaderboard over in_global_rating runs.
type PublicRatingBoard struct {
	runs    RatingRunsLister
	metrics RunMetricsGetter
}

var _ public_rating.PublicRatingBoard = (*PublicRatingBoard)(nil)

// NewPublicRatingBoard builds the public rating board.
func NewPublicRatingBoard(runs RatingRunsLister, metrics RunMetricsGetter) *PublicRatingBoard {
	return &PublicRatingBoard{runs: runs, metrics: metrics}
}

// Public resolves the public leaderboard for the filter. derrors.ErrNotFound when
// the metric_key is not present on any candidate run.
func (b *PublicRatingBoard) Public(ctx context.Context, filter *api.RatingFilter, limit uint32, pageToken string) ([]*api.PublicRatingEntry, string, error) {
	runs, err := b.runs.ListRatingRuns(ctx, RatingScopeGlobal, "")
	if err != nil {
		return nil, "", err
	}
	ranked, found, err := rankRuns(ctx, b.metrics, runs, ratingFacets{
		metricKey:       filter.GetMetricKey(),
		dbKinds:         filter.GetDbKinds(),
		stroppyVersions: filter.GetStroppyVersions(),
		providers:       filter.GetProviders(),
		startedAfter:    filter.GetStartedAfter().AsTime().UnixNano(),
		startedBefore:   filter.GetStartedBefore().AsTime().UnixNano(),
		hasAfter:        filter.GetStartedAfter() != nil,
		hasBefore:       filter.GetStartedBefore() != nil,
	})
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", derrors.NotFound("metric", "metric_key not found on any rated run")
	}
	if limit == 0 {
		limit = defaultRatingPageSize
	}
	page, next := paginate(ranked, pageToken, limit)

	entries := make([]*api.PublicRatingEntry, 0, len(page))
	for _, r := range page {
		entries = append(entries, &api.PublicRatingEntry{
			Rank:           r.rank,
			MetricValue:    r.value,
			MetricUnit:     r.unit,
			DbKind:         r.run.GetSummary().GetDbKind(),
			WorkloadName:   r.run.GetSummary().GetWorkloadName(),
			StroppyVersion: r.run.GetSummary().GetStroppyVersion(),
			Provider:       r.run.GetSummary().GetProvider(),
			TopologyLabel:  r.run.GetSummary().GetTopologyLabel(),
			NodeCount:      r.run.GetSummary().GetNodeCount(),
		})
	}
	return entries, next, nil
}

// ratingFacets are the resolved AND-ed narrowing facets shared by the boards.
type ratingFacets struct {
	metricKey       string
	dbKinds         []domain.Database_Kind
	stroppyVersions []string
	providers       []deployment.Provider
	startedAfter    int64
	startedBefore   int64
	hasAfter        bool
	hasBefore       bool
}

// rankedRun is one ranked board row.
type rankedRun struct {
	run   *models.TestRunRecord
	value float64
	unit  string
	rank  uint32
}

// rankRuns filters the candidate runs by the facets, reads each survivor's metric
// value for metricKey, sorts by that value honoring higher_is_better, assigns
// absolute ranks and reports whether the metric key was present on any run.
func rankRuns(ctx context.Context, metrics RunMetricsGetter, runs []*models.TestRunRecord, f ratingFacets) ([]rankedRun, bool, error) {
	var ranked []rankedRun
	metricSeen := false
	higherIsBetter := true

	for _, run := range runs {
		if run.GetEntity().GetTimings().GetDeletedAt() != nil {
			continue
		}
		if !matchFacets(run, f) {
			continue
		}
		rm, err := metrics.Get(ctx, run.GetEntity().GetId())
		if err != nil {
			if derrors.IgnoreNotFound(err) == nil {
				continue
			}
			return nil, false, err
		}
		var m *monitor.MetricSummary
		for _, candidate := range rm.GetMetrics() {
			if candidate.GetKey() == f.metricKey {
				m = candidate
				break
			}
		}
		if m == nil {
			continue
		}
		metricSeen = true
		higherIsBetter = m.GetHigherIsBetter()
		ranked = append(ranked, rankedRun{run: run, value: m.GetAvg(), unit: m.GetUnit()})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if higherIsBetter {
			return ranked[i].value > ranked[j].value
		}
		return ranked[i].value < ranked[j].value
	})
	for i := range ranked {
		ranked[i].rank = uint32(i + 1) //nolint:gosec // index is bounded by len.
	}
	return ranked, metricSeen, nil
}

// matchFacets reports whether a run passes every AND-ed facet filter.
func matchFacets(run *models.TestRunRecord, f ratingFacets) bool {
	sum := run.GetSummary()
	if len(f.dbKinds) > 0 && !containsKind(f.dbKinds, sum.GetDbKind()) {
		return false
	}
	if len(f.stroppyVersions) > 0 && !containsString(f.stroppyVersions, sum.GetStroppyVersion()) {
		return false
	}
	if len(f.providers) > 0 && !containsProvider(f.providers, sum.GetProvider()) {
		return false
	}
	if started := sum.GetStartedAt(); started != nil {
		ts := started.AsTime().UnixNano()
		if f.hasAfter && ts < f.startedAfter {
			return false
		}
		if f.hasBefore && ts > f.startedBefore {
			return false
		}
	}
	return true
}

func containsKind(list []domain.Database_Kind, v domain.Database_Kind) bool {
	for _, e := range list {
		if e == v {
			return true
		}
	}
	return false
}

func containsProvider(list []deployment.Provider, v deployment.Provider) bool {
	for _, e := range list {
		if e == v {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, e := range list {
		if e == v {
			return true
		}
	}
	return false
}

// paginate slices ranked by a simple decimal-offset cursor and returns the page
// plus the next-page token ("" when exhausted or the token is malformed).
func paginate(ranked []rankedRun, pageToken string, limit uint32) ([]rankedRun, string) {
	offset := 0
	if pageToken != "" {
		if v, err := strconv.Atoi(pageToken); err == nil && v > 0 {
			offset = v
		}
	}
	if offset >= len(ranked) {
		return nil, ""
	}
	end := offset + int(limit)
	if end >= len(ranked) {
		return ranked[offset:], ""
	}
	return ranked[offset:end], strconv.Itoa(end)
}
