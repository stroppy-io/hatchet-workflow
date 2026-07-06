package tenant_dashboard

import (
	"context"
	"errors"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	tenant_dashboard implements the tenant landing page: a single aggregated,
	read-only view assembled on demand from several tenant-scoped data sources.
	The dashboard is never persisted as a DB record (the proto doc spells this
	out) — it is computed/precomputed by background jobs and projected here, the
	same pattern as the rating board and the share-snapshot refresh.

	RBAC (RESOURCE_TEST_RUN/LIST in the active tenant) is enforced upstream by the
	auth interceptor reading the proto auth annotation, so this handler does NOT
	re-check that permission. It DOES validate the requested tenant exists (so the
	caller gets a clean NotFound rather than a misleading empty dashboard) and then
	fans out to the projection ports.

	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage of its own: every read goes through one of these
	ports. Persistence ports return derrors.ErrNotFound / derrors.ErrConflict so
	the handler can translate them via utils.MapErr.
*/

// Window bounds the "recent" horizon used by the headline tiles (status counts
// and success rate). It is configurable rather than hard-coded so the landing
// view's notion of "recent" can be tuned without touching the RPC contract.
const (
	defaultRecentRuns       = 10
	defaultUpcomingSuites   = 10
	defaultTopBenchmarks    = 10
	defaultSuccessRateScope = 30 * 24 * time.Hour
)

// RunStatsReader yields the precomputed run breakdown by lifecycle status and
// the recent-window success rate (completed / finished), both scoped to one
// tenant. window bounds the success-rate horizon.
type RunStatsReader interface {
	StatusCounts(ctx context.Context, tenantID string) (*api.StatusCounts, error)
	SuccessRate(ctx context.Context, tenantID string, window time.Duration) (float32, error)
}

// RecentRunsReader yields the most recent test runs of a tenant, newest first,
// capped at limit.
type RecentRunsReader interface {
	RecentRuns(ctx context.Context, tenantID string, limit uint32) ([]*models.TestRunRecord, error)
}

// ScheduleReader yields the tenant's scheduled suites together with their next
// planned auto-run, ordered by the soonest next_run_at, capped at limit.
type ScheduleReader interface {
	UpcomingSuites(ctx context.Context, tenantID string, limit uint32) ([]*api.UpcomingSuite, error)
}

// RatingReader yields this tenant's top-N benchmarks — the same projection the
// tenant rating board exposes (in_tenant_rating runs), ranked and capped at
// limit.
type RatingReader interface {
	TopBenchmarks(ctx context.Context, tenantID string, limit uint32) ([]*api.RatingEntry, error)
}

// TenantDashboardDeps bundles every dependency for the constructor.
type TenantDashboardDeps struct {
	Authn    utils.Authn
	Tenants  utils.TenantReader
	RunStats RunStatsReader
	Recent   RecentRunsReader
	Schedule ScheduleReader
	Rating   RatingReader
	Tx       tx.Trm
}

type TenantDashboardService struct {
	*api.UnimplementedTenantDashboardServiceServer
	tx.Trm
	d TenantDashboardDeps
}

var _ api.TenantDashboardServiceServer = (*TenantDashboardService)(nil)

func NewTenantDashboardService(deps TenantDashboardDeps) *TenantDashboardService {
	return &TenantDashboardService{UnimplementedTenantDashboardServiceServer: &api.UnimplementedTenantDashboardServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *TenantDashboardService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *TenantDashboardService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *TenantDashboardService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *TenantDashboardService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

/*
	===== GetTenantDashboard =====
*/

// GetTenantDashboard assembles the tenant landing payload. It is a pure read
// (NO_SIDE_EFFECTS): it resolves the caller, validates the requested tenant
// exists, then projects the aggregated view from the read ports inside a single
// consistent snapshot transaction so the tiles, recent lists, schedule and
// rating all reflect the same point in time.
func (s *TenantDashboardService) GetTenantDashboard(ctx context.Context, req *api.GetTenantDashboardRequest) (*api.GetTenantDashboardResponse, error) {
	if _, err := s.caller(ctx); err != nil {
		return nil, err
	}
	tenantID := req.GetTenantId()
	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	dashboard, err := doTxRet(ctx, s, func(ctx context.Context) (*api.TenantDashboard, error) {
		// The dashboard is tenant-scoped; an unknown tenant is a clean NotFound
		// rather than a silently-empty payload.
		if _, err := s.d.Tenants.Get(ctx, tenantID); err != nil {
			if errors.Is(err, derrors.ErrNotFound) {
				return nil, status.Error(codes.NotFound, "tenant not found")
			}
			return nil, utils.MapErr(err)
		}

		counts, err := s.d.RunStats.StatusCounts(ctx, tenantID)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if counts == nil {
			counts = &api.StatusCounts{}
		}

		successRate, err := s.d.RunStats.SuccessRate(ctx, tenantID, defaultSuccessRateScope)
		if err != nil {
			return nil, utils.MapErr(err)
		}

		recentRuns, err := s.d.Recent.RecentRuns(ctx, tenantID, defaultRecentRuns)
		if err != nil {
			return nil, utils.MapErr(err)
		}

		upcoming, err := s.d.Schedule.UpcomingSuites(ctx, tenantID, defaultUpcomingSuites)
		if err != nil {
			return nil, utils.MapErr(err)
		}

		topBenchmarks, err := s.d.Rating.TopBenchmarks(ctx, tenantID, defaultTopBenchmarks)
		if err != nil {
			return nil, utils.MapErr(err)
		}

		return &api.TenantDashboard{
			RunCounts:     counts,
			SuccessRate:   successRate,
			RecentRuns:    recentRuns,
			Upcoming:      upcoming,
			TopBenchmarks: topBenchmarks,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.GetTenantDashboardResponse{Dashboard: dashboard}, nil
}
