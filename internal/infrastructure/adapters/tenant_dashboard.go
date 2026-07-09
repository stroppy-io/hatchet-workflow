package adapters

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenant_dashboard"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ScheduledSuite is the minimal projection of a scheduled suite DEFINITION the
// "Upcoming suites" dashboard tile needs to compute the next planned auto-run.
// It is a plain (non-proto) struct rather than the models suite-record proto:
// suites and suite runs were removed with the old test-orchestration backend,
// so ListScheduledSuites has no dependency on that (now orphaned) proto model.
type ScheduledSuite struct {
	// ID is the suite definition's entity id.
	ID string
	// Name is the suite's display name.
	Name string
	// Cron is the schedule expression driving the auto-runs.
	Cron string
	// ScheduleEnabled mirrors the suite's spec.schedule.enabled.
	ScheduleEnabled bool
}

// DashboardRunsReader is the consumer interface the dashboard aggregates run data
// through. The gormstore TestRuns repo satisfies ListTenantRuns once injected:
//   - ListTenantRuns lists ALL of a tenant's test runs (status + started_at are
//     read for the headline tiles; ordering does not matter, the adapter sorts).
//   - ListScheduledSuites lists the tenant's suite DEFINITIONS that carry an
//     enabled schedule (so the adapter can compute the next planned auto-run).
type DashboardRunsReader interface {
	ListTenantRuns(ctx context.Context, tenantID string) ([]*models.Run, error)
	ListScheduledSuites(ctx context.Context, tenantID string) ([]ScheduledSuite, error)
}

// RunStatsReader implements tenant_dashboard.RunStatsReader: status breakdown and
// recent-window success rate, computed in memory from the tenant's runs.
type RunStatsReader struct{ runs DashboardRunsReader }

var _ tenant_dashboard.RunStatsReader = (*RunStatsReader)(nil)

// NewRunStatsReader builds the run-stats reader.
func NewRunStatsReader(runs DashboardRunsReader) *RunStatsReader { return &RunStatsReader{runs: runs} }

// StatusCounts tallies the tenant's runs by lifecycle status.
func (r *RunStatsReader) StatusCounts(ctx context.Context, tenantID string) (*api.StatusCounts, error) {
	all, err := r.runs.ListTenantRuns(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	counts := &api.StatusCounts{}
	for _, run := range all {
		counts.Total++
		switch run.GetStatus() {
		case common.Status_STATUS_PENDING, common.Status_STATUS_RETRY_WAIT,
			common.Status_STATUS_ALLOCATED, common.Status_STATUS_DEPLOYMENT,
			common.Status_STATUS_DEPLOYED:
			counts.Pending++
		case common.Status_STATUS_RUNNING:
			counts.Running++
		case common.Status_STATUS_COMPLETED:
			counts.Completed++
		case common.Status_STATUS_FAILED:
			counts.Failed++
		case common.Status_STATUS_CANCELLED, common.Status_STATUS_CANCELLING,
			common.Status_STATUS_SKIPPED:
			counts.Cancelled++
		}
	}
	return counts, nil
}

// SuccessRate is completed / (completed + failed) over runs that started within
// the window. Returns 0 when there are no finished runs in the window.
func (r *RunStatsReader) SuccessRate(ctx context.Context, tenantID string, window time.Duration) (float32, error) {
	all, err := r.runs.ListTenantRuns(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-window)
	var completed, finished uint32
	for _, run := range all {
		started := run.GetSummary().GetStartedAt()
		if window > 0 && started != nil && started.AsTime().Before(cutoff) {
			continue
		}
		switch run.GetStatus() {
		case common.Status_STATUS_COMPLETED:
			completed++
			finished++
		case common.Status_STATUS_FAILED:
			finished++
		}
	}
	if finished == 0 {
		return 0, nil
	}
	return float32(completed) / float32(finished), nil
}

// RecentRunsReader implements tenant_dashboard.RecentRunsReader: a newest-first,
// capped list of the tenant's test runs.
type RecentRunsReader struct{ runs DashboardRunsReader }

var _ tenant_dashboard.RecentRunsReader = (*RecentRunsReader)(nil)

// NewRecentRunsReader builds the recent-runs reader.
func NewRecentRunsReader(runs DashboardRunsReader) *RecentRunsReader {
	return &RecentRunsReader{runs: runs}
}

// RecentRuns returns the most recent test runs, newest first, capped at limit.
func (r *RecentRunsReader) RecentRuns(ctx context.Context, tenantID string, limit uint32) ([]*models.Run, error) {
	all, err := r.runs.ListTenantRuns(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sortByCreatedDesc(all, func(rec *models.Run) time.Time { return createdAt(rec.GetEntity()) })
	return capRuns(all, limit), nil
}

// ScheduleReader implements tenant_dashboard.ScheduleReader: the tenant's
// scheduled suites with their next planned auto-run, soonest first.
type ScheduleReader struct{ runs DashboardRunsReader }

var _ tenant_dashboard.ScheduleReader = (*ScheduleReader)(nil)

// NewScheduleReader builds the schedule reader.
func NewScheduleReader(runs DashboardRunsReader) *ScheduleReader { return &ScheduleReader{runs: runs} }

// UpcomingSuites returns scheduled suites ordered by the soonest next_run_at,
// capped at limit. The next run is computed from the cron expression; suites whose
// cron cannot be parsed or whose schedule is disabled are skipped.
func (r *ScheduleReader) UpcomingSuites(ctx context.Context, tenantID string, limit uint32) ([]*api.UpcomingSuite, error) {
	suites, err := r.runs.ListScheduledSuites(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	now := time.Now()

	var upcoming []*api.UpcomingSuite
	for _, suite := range suites {
		if !suite.ScheduleEnabled || suite.Cron == "" {
			continue
		}
		schedule, perr := parser.Parse(suite.Cron)
		if perr != nil {
			continue
		}
		next := schedule.Next(now)
		upcoming = append(upcoming, &api.UpcomingSuite{
			SuiteId:   suite.ID,
			Name:      suite.Name,
			Cron:      suite.Cron,
			NextRunAt: timestamppb.New(next),
		})
	}
	sortByTimeAsc(upcoming, func(u *api.UpcomingSuite) time.Time { return u.GetNextRunAt().AsTime() })
	if limit > 0 && uint32(len(upcoming)) > limit {
		upcoming = upcoming[:limit]
	}
	return upcoming, nil
}

// DashboardRatingReader implements tenant_dashboard.RatingReader by delegating to
// the tenant rating board (in_tenant_rating runs) and capping to the top N.
type DashboardRatingReader struct {
	board      *RatingBoard
	metricKeys []string
}

var _ tenant_dashboard.RatingReader = (*DashboardRatingReader)(nil)

// NewDashboardRatingReader builds the rating reader over the shared RatingBoard.
// metricKey is the headline metric the dashboard ranks the top benchmarks by.
func NewDashboardRatingReader(board *RatingBoard, metricKey string) *DashboardRatingReader {
	keys := []string{metricKey}
	for _, fallback := range []string{"db_tps", "db_qps", "stroppy_ops"} {
		if fallback == "" || containsString(keys, fallback) {
			continue
		}
		keys = append(keys, fallback)
	}
	return &DashboardRatingReader{board: board, metricKeys: keys}
}

// TopBenchmarks returns this tenant's top-N benchmarks ranked by the configured
// metric. A board with no rated runs (metric not found) yields an empty list
// rather than an error, so the dashboard tile is simply empty.
func (r *DashboardRatingReader) TopBenchmarks(ctx context.Context, tenantID string, limit uint32) ([]*api.RatingEntry, error) {
	for _, key := range r.metricKeys {
		if key == "" {
			continue
		}
		entries, _, err := r.board.Rank(ctx, ratingTenantQuery(tenantID, key, limit))
		if err != nil {
			// An empty board (metric absent) is not a dashboard error; try the next
			// common throughput metric before yielding an empty tile.
			continue
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}
	return nil, nil
}
