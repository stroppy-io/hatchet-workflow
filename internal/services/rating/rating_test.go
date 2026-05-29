package rating

// rating_test.go: tests for GetSystemRating and GetTenantRating.

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// helpers

func newSvc(ctrl *gomock.Controller) (*RatingService, *utils.MockAuthn, *MockRatingBoard, *utils.MockTenantReader) {
	authn := utils.NewMockAuthn(ctrl)
	board := NewMockRatingBoard(ctrl)
	tenants := utils.NewMockTenantReader(ctrl)
	svc := NewRatingService(RatingDeps{
		Authn:   authn,
		Board:   board,
		Tenants: tenants,
		Tx:      &utils.MockTrm{},
	})
	return svc, authn, board, tenants
}

func validFilter() *api.RatingFilter {
	return &api.RatingFilter{MetricKey: "tps"}
}

// ===== GetSystemRating =====

func TestGetSystemRating(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, board, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return([]*api.RatingEntry{
			{Rank: 1, MetricValue: 9000.0},
		}, "next-token", nil)

		resp, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{
			Filter: validFilter(),
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(resp.Entries))
		}
		if resp.NextPageToken != "next-token" {
			t.Errorf("expected next-token, got %s", resp.NextPageToken)
		}
	})

	t.Run("Unauthenticated_CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{Filter: validFilter()})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("FilterNil_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)

		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{Filter: nil})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("FilterMetricKeyEmpty_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)

		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{
			Filter: &api.RatingFilter{MetricKey: ""},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("FilterTimeWindow_Inverted_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)

		after := time.Now()
		before := after.Add(-1 * time.Hour) // before is earlier than after
		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{
			Filter: &api.RatingFilter{
				MetricKey:     "tps",
				StartedAfter:  timestamppb.New(after),
				StartedBefore: timestamppb.New(before),
			},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for inverted time window, got %v", err)
		}
	})

	t.Run("FilterTimeWindow_Valid_Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, board, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return(nil, "", nil)

		before := time.Now()
		after := before.Add(-24 * time.Hour)
		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{
			Filter: &api.RatingFilter{
				MetricKey:     "tps",
				StartedAfter:  timestamppb.New(after),
				StartedBefore: timestamppb.New(before),
			},
		})
		if err != nil {
			t.Fatalf("unexpected error with valid time window: %v", err)
		}
	})

	t.Run("BoardRankError_NotFound_MapsToNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, board, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return(nil, "", derrors.NotFound("metric", "unknown metric"))

		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{Filter: validFilter()})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("BoardRankError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, board, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return(nil, "", errors.New("db error"))

		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{Filter: validFilter()})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Success_EmptyEntries_WithPagination", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, board, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return([]*api.RatingEntry{}, "", nil)

		resp, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{
			Filter:    validFilter(),
			PageToken: "cursor-abc",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(resp.Entries))
		}
	})

	t.Run("ScopeSystem_QueryPropagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, board, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		board.EXPECT().Rank(ctx, gomock.AssignableToTypeOf(RatingQuery{})).DoAndReturn(
			func(_ context.Context, q RatingQuery) ([]*api.RatingEntry, string, error) {
				if q.Scope != ScopeSystem {
					t.Errorf("expected ScopeSystem, got %v", q.Scope)
				}
				if q.TenantID != "" {
					t.Errorf("expected empty TenantID for system scope, got %s", q.TenantID)
				}
				if q.MetricKey != "tps" {
					t.Errorf("expected metric_key tps, got %s", q.MetricKey)
				}
				if q.Limit != 20 {
					t.Errorf("expected limit 20, got %d", q.Limit)
				}
				if q.PageToken != "page-1" {
					t.Errorf("expected page-1, got %s", q.PageToken)
				}
				return nil, "", nil
			},
		)

		_, err := svc.GetSystemRating(ctx, &api.GetSystemRatingRequest{
			Filter:    validFilter(),
			Limit:     20,
			PageToken: "page-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ===== GetTenantRating =====

func TestGetTenantRating(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, board, tenants := newSvc(ctrl)

		tenants.EXPECT().Get(ctx, "tenant-1").Return(&iampb.Tenant{Id: "tenant-1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return([]*api.RatingEntry{
			{Rank: 1}, {Rank: 2},
		}, "", nil)

		resp, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-1",
			Filter:   validFilter(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(resp.Entries))
		}
	})

	t.Run("TenantIdEmpty_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _ := newSvc(ctrl)

		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "",
			Filter:   validFilter(),
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("FilterNil_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _ := newSvc(ctrl)

		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-1",
			Filter:   nil,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("FilterMetricKeyEmpty_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _ := newSvc(ctrl)

		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-1",
			Filter:   &api.RatingFilter{MetricKey: ""},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("FilterTimeWindow_Inverted_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _ := newSvc(ctrl)

		after := time.Now()
		before := after.Add(-1 * time.Hour)
		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-1",
			Filter: &api.RatingFilter{
				MetricKey:     "tps",
				StartedAfter:  timestamppb.New(after),
				StartedBefore: timestamppb.New(before),
			},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for inverted window, got %v", err)
		}
	})

	t.Run("TenantNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, tenants := newSvc(ctrl)

		tenants.EXPECT().Get(ctx, "unknown").Return(nil, derrors.NotFound("tenant", "not found"))

		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "unknown",
			Filter:   validFilter(),
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("TenantGetError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, tenants := newSvc(ctrl)

		tenants.EXPECT().Get(ctx, "tenant-1").Return(nil, errors.New("db error"))

		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-1",
			Filter:   validFilter(),
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("BoardRankError_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, board, tenants := newSvc(ctrl)

		tenants.EXPECT().Get(ctx, "tenant-1").Return(&iampb.Tenant{Id: "tenant-1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return(nil, "", derrors.NotFound("metric", "unknown metric_key"))

		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-1",
			Filter:   validFilter(),
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("BoardRankError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, board, tenants := newSvc(ctrl)

		tenants.EXPECT().Get(ctx, "tenant-1").Return(&iampb.Tenant{Id: "tenant-1"}, nil)
		board.EXPECT().Rank(ctx, gomock.Any()).Return(nil, "", errors.New("rank error"))

		_, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-1",
			Filter:   validFilter(),
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("ScopeTenant_QueryPropagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, board, tenants := newSvc(ctrl)

		tenants.EXPECT().Get(ctx, "tenant-42").Return(&iampb.Tenant{Id: "tenant-42"}, nil)
		board.EXPECT().Rank(ctx, gomock.AssignableToTypeOf(RatingQuery{})).DoAndReturn(
			func(_ context.Context, q RatingQuery) ([]*api.RatingEntry, string, error) {
				if q.Scope != ScopeTenant {
					t.Errorf("expected ScopeTenant, got %v", q.Scope)
				}
				if q.TenantID != "tenant-42" {
					t.Errorf("expected tenant-42, got %s", q.TenantID)
				}
				return nil, "tok", nil
			},
		)

		resp, err := svc.GetTenantRating(ctx, &api.GetTenantRatingRequest{
			TenantId: "tenant-42",
			Filter:   validFilter(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NextPageToken != "tok" {
			t.Errorf("expected next page token 'tok', got %s", resp.NextPageToken)
		}
	})
}

// ===== validateFilter direct tests =====

func TestValidateFilter(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		err := validateFilter(nil)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for nil filter, got %v", err)
		}
	})

	t.Run("EmptyMetricKey", func(t *testing.T) {
		err := validateFilter(&api.RatingFilter{})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for empty metric_key, got %v", err)
		}
	})

	t.Run("ValidNoWindow", func(t *testing.T) {
		err := validateFilter(&api.RatingFilter{MetricKey: "latency"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ValidWithWindow", func(t *testing.T) {
		now := time.Now()
		err := validateFilter(&api.RatingFilter{
			MetricKey:     "latency",
			StartedAfter:  timestamppb.New(now.Add(-time.Hour)),
			StartedBefore: timestamppb.New(now),
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("InvertedWindow", func(t *testing.T) {
		now := time.Now()
		err := validateFilter(&api.RatingFilter{
			MetricKey:     "latency",
			StartedAfter:  timestamppb.New(now),
			StartedBefore: timestamppb.New(now.Add(-time.Hour)),
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for inverted window, got %v", err)
		}
	})
}

// ===== toQuery direct tests =====

func TestToQuery(t *testing.T) {
	t.Run("SystemScope", func(t *testing.T) {
		f := &api.RatingFilter{MetricKey: "tps"}
		q := toQuery(ScopeSystem, "", f, 50, "tok")
		if q.Scope != ScopeSystem {
			t.Errorf("expected ScopeSystem")
		}
		if q.TenantID != "" {
			t.Errorf("expected empty TenantID")
		}
		if q.MetricKey != "tps" {
			t.Errorf("expected tps")
		}
		if q.Limit != 50 {
			t.Errorf("expected limit 50, got %d", q.Limit)
		}
		if q.PageToken != "tok" {
			t.Errorf("expected page token tok")
		}
	})

	t.Run("TenantScope_WithTimeWindow", func(t *testing.T) {
		now := time.Now()
		after := now.Add(-time.Hour)
		before := now
		f := &api.RatingFilter{
			MetricKey:     "latency",
			StartedAfter:  timestamppb.New(after),
			StartedBefore: timestamppb.New(before),
		}
		q := toQuery(ScopeTenant, "t1", f, 10, "")
		if q.Scope != ScopeTenant {
			t.Errorf("expected ScopeTenant")
		}
		if q.TenantID != "t1" {
			t.Errorf("expected t1, got %s", q.TenantID)
		}
		if q.StartedAfter == nil {
			t.Error("expected StartedAfter to be set")
		}
		if q.StartedBefore == nil {
			t.Error("expected StartedBefore to be set")
		}
	})
}
