package tenant_dashboard

// get_tenant_dashboard_test.go: tests for GetTenantDashboard in service.go.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

const tenantID = "tenant-abc"

// newTestService builds a fully-wired service from individual mocks.
func newTestService(
	ctrl *gomock.Controller,
) (
	svc *TenantDashboardService,
	authn *utils.MockAuthn,
	tenants *utils.MockTenantReader,
	runStats *MockRunStatsReader,
	recent *MockRecentRunsReader,
	schedule *MockScheduleReader,
	rating *MockRatingReader,
) {
	authn = utils.NewMockAuthn(ctrl)
	tenants = utils.NewMockTenantReader(ctrl)
	runStats = NewMockRunStatsReader(ctrl)
	recent = NewMockRecentRunsReader(ctrl)
	schedule = NewMockScheduleReader(ctrl)
	rating = NewMockRatingReader(ctrl)

	svc = NewTenantDashboardService(TenantDashboardDeps{
		Authn:    authn,
		Tenants:  tenants,
		RunStats: runStats,
		Recent:   recent,
		Schedule: schedule,
		Rating:   rating,
		Tx:       &utils.MockTrm{},
	})
	return
}

// happyPathSetup sets up mocks for a fully successful GetTenantDashboard call.
func happyPathSetup(
	ctx context.Context,
	authn *utils.MockAuthn,
	tenants *utils.MockTenantReader,
	runStats *MockRunStatsReader,
	recent *MockRecentRunsReader,
	schedule *MockScheduleReader,
	rating *MockRatingReader,
) {
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
	tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
	runStats.EXPECT().StatusCounts(ctx, tenantID).Return(&api.StatusCounts{Total: 5}, nil)
	runStats.EXPECT().SuccessRate(ctx, tenantID, defaultSuccessRateScope).Return(float32(0.9), nil)
	recent.EXPECT().RecentRuns(ctx, tenantID, uint32(defaultRecentRuns)).Return([]*models.TestRunRecord{}, nil)
	recent.EXPECT().RecentSuiteRuns(ctx, tenantID, uint32(defaultRecentSuiteRuns)).Return([]*models.SuiteRunRecord{}, nil)
	schedule.EXPECT().UpcomingSuites(ctx, tenantID, uint32(defaultUpcomingSuites)).Return([]*api.UpcomingSuite{}, nil)
	rating.EXPECT().TopBenchmarks(ctx, tenantID, uint32(defaultTopBenchmarks)).Return([]*api.RatingEntry{}, nil)
}

func TestGetTenantDashboard(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, recent, schedule, rating := newTestService(ctrl)
		happyPathSetup(ctx, authn, tenants, runStats, recent, schedule, rating)

		resp, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Dashboard == nil {
			t.Fatal("expected non-nil dashboard")
		}
		if resp.Dashboard.SuccessRate != 0.9 {
			t.Errorf("expected success_rate=0.9, got %v", resp.Dashboard.SuccessRate)
		}
		if resp.Dashboard.RunCounts == nil || resp.Dashboard.RunCounts.Total != 5 {
			t.Errorf("expected run_counts.total=5, got %v", resp.Dashboard.RunCounts)
		}
	})

	t.Run("Success_NilStatusCounts_ReplacedWithEmpty", func(t *testing.T) {
		// Service replaces nil StatusCounts with an empty struct to avoid nil
		// fields in the response.
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, recent, schedule, rating := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		// RunStats returns nil counts — service must substitute empty struct.
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(nil, nil)
		runStats.EXPECT().SuccessRate(ctx, tenantID, defaultSuccessRateScope).Return(float32(0), nil)
		recent.EXPECT().RecentRuns(ctx, tenantID, uint32(defaultRecentRuns)).Return(nil, nil)
		recent.EXPECT().RecentSuiteRuns(ctx, tenantID, uint32(defaultRecentSuiteRuns)).Return(nil, nil)
		schedule.EXPECT().UpcomingSuites(ctx, tenantID, uint32(defaultUpcomingSuites)).Return(nil, nil)
		rating.EXPECT().TopBenchmarks(ctx, tenantID, uint32(defaultTopBenchmarks)).Return(nil, nil)

		resp, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Dashboard.RunCounts == nil {
			t.Error("expected non-nil RunCounts after nil substitution")
		}
	})

	t.Run("Unauthenticated_CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("InvalidArgument_EmptyTenantID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("NotFound_TenantDoesNotExist", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, _, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("Internal_TenantGetUnexpectedError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, _, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(nil, errors.New("connection reset"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Internal_StatusCountsError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(nil, errors.New("db err"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Internal_SuccessRateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(&api.StatusCounts{}, nil)
		runStats.EXPECT().SuccessRate(ctx, tenantID, defaultSuccessRateScope).Return(float32(0), errors.New("db err"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Internal_RecentRunsError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, recent, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(&api.StatusCounts{}, nil)
		runStats.EXPECT().SuccessRate(ctx, tenantID, defaultSuccessRateScope).Return(float32(1.0), nil)
		recent.EXPECT().RecentRuns(ctx, tenantID, uint32(defaultRecentRuns)).Return(nil, errors.New("db err"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Internal_RecentSuiteRunsError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, recent, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(&api.StatusCounts{}, nil)
		runStats.EXPECT().SuccessRate(ctx, tenantID, defaultSuccessRateScope).Return(float32(1.0), nil)
		recent.EXPECT().RecentRuns(ctx, tenantID, uint32(defaultRecentRuns)).Return([]*models.TestRunRecord{}, nil)
		recent.EXPECT().RecentSuiteRuns(ctx, tenantID, uint32(defaultRecentSuiteRuns)).Return(nil, errors.New("db err"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Internal_UpcomingSuitesError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, recent, schedule, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(&api.StatusCounts{}, nil)
		runStats.EXPECT().SuccessRate(ctx, tenantID, defaultSuccessRateScope).Return(float32(1.0), nil)
		recent.EXPECT().RecentRuns(ctx, tenantID, uint32(defaultRecentRuns)).Return([]*models.TestRunRecord{}, nil)
		recent.EXPECT().RecentSuiteRuns(ctx, tenantID, uint32(defaultRecentSuiteRuns)).Return([]*models.SuiteRunRecord{}, nil)
		schedule.EXPECT().UpcomingSuites(ctx, tenantID, uint32(defaultUpcomingSuites)).Return(nil, errors.New("db err"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Internal_TopBenchmarksError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, recent, schedule, rating := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(&api.StatusCounts{}, nil)
		runStats.EXPECT().SuccessRate(ctx, tenantID, defaultSuccessRateScope).Return(float32(1.0), nil)
		recent.EXPECT().RecentRuns(ctx, tenantID, uint32(defaultRecentRuns)).Return([]*models.TestRunRecord{}, nil)
		recent.EXPECT().RecentSuiteRuns(ctx, tenantID, uint32(defaultRecentSuiteRuns)).Return([]*models.SuiteRunRecord{}, nil)
		schedule.EXPECT().UpcomingSuites(ctx, tenantID, uint32(defaultUpcomingSuites)).Return([]*api.UpcomingSuite{}, nil)
		rating.EXPECT().TopBenchmarks(ctx, tenantID, uint32(defaultTopBenchmarks)).Return(nil, errors.New("db err"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	// Verify domain errors are properly translated (not just raw errors).
	t.Run("DomainError_NotFound_ThroughMapErr", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		// StatusCounts returns a domain NotFound (not the tenant, something else).
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(nil, derrors.NotFound("run_stats", "no stats"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound from MapErr, got %v", err)
		}
	})

	t.Run("DomainError_Conflict_ThroughMapErr", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tenants, runStats, _, _, _ := newTestService(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		runStats.EXPECT().StatusCounts(ctx, tenantID).Return(nil, derrors.Conflict("run_stats", "conflict"))

		_, err := svc.GetTenantDashboard(ctx, &api.GetTenantDashboardRequest{TenantId: tenantID})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists from MapErr, got %v", err)
		}
	})
}
