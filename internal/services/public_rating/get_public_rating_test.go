package public_rating

// get_public_rating_test.go: tests for GetPublicRating handler.
// Covers: success, nil filter, empty metric_key, limit > maxLimit,
//         limit==0 (default), page token forwarded, Board.Public error (not-found mapped to NotFound code,
//         internal error mapped to Internal code).

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

func newSvc(t *testing.T) (*PublicRatingService, *MockPublicRatingBoard) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	board := NewMockPublicRatingBoard(ctrl)
	svc := NewPublicRatingService(PublicRatingDeps{
		Board: board,
		Tx:    &utils.MockTrm{},
	})
	return svc, board
}

func TestGetPublicRating(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "tps"}
		entries := []*api.PublicRatingEntry{
			{Rank: 1, MetricValue: 1000.0},
			{Rank: 2, MetricValue: 900.0},
		}
		board.EXPECT().
			Public(ctx, filter, uint32(10), "").
			Return(entries, "next-token", nil)

		resp, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{
			Filter: filter,
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(resp.Entries))
		}
		if resp.NextPageToken != "next-token" {
			t.Errorf("expected next-token, got %q", resp.NextPageToken)
		}
	})

	t.Run("NilFilter_InvalidArgument", func(t *testing.T) {
		svc, _ := newSvc(t)

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("EmptyMetricKey_InvalidArgument", func(t *testing.T) {
		svc, _ := newSvc(t)

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{
			Filter: &api.RatingFilter{MetricKey: ""},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("LimitExceedsMax_InvalidArgument", func(t *testing.T) {
		svc, _ := newSvc(t)

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{
			Filter: &api.RatingFilter{MetricKey: "tps"},
			Limit:  501,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("LimitAtMax_Success", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "tps"}
		board.EXPECT().
			Public(ctx, filter, uint32(500), "").
			Return(nil, "", nil)

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{
			Filter: filter,
			Limit:  500,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("LimitZero_UsesDefault", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "tps"}
		// limit==0 should be clamped to defaultLimit (50)
		board.EXPECT().
			Public(ctx, filter, defaultLimit, "").
			Return(nil, "", nil)

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{
			Filter: filter,
			Limit:  0,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("LimitOverMax_ClampedToMax", func(t *testing.T) {
		// limit > maxLimit (501+) is rejected before clampLimit is reached.
		// clampLimit itself handles the case >maxLimit, but the guard returns
		// InvalidArgument first for requests exceeding the proto cap. We test
		// clampLimit directly here.
		got := clampLimit(maxLimit + 1)
		if got != maxLimit {
			t.Errorf("clampLimit(%d) = %d; want %d", maxLimit+1, got, maxLimit)
		}
	})

	t.Run("PageTokenForwarded", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "latency"}
		board.EXPECT().
			Public(ctx, filter, uint32(20), "page2").
			Return([]*api.PublicRatingEntry{{Rank: 21}}, "", nil)

		resp, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{
			Filter:    filter,
			Limit:     20,
			PageToken: "page2",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(resp.Entries))
		}
		if resp.NextPageToken != "" {
			t.Errorf("expected empty next token, got %q", resp.NextPageToken)
		}
	})

	t.Run("BoardError_NotFound_MapsToNotFound", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "tps"}
		board.EXPECT().
			Public(ctx, filter, defaultLimit, "").
			Return(nil, "", derrors.NotFound("board", "no data"))

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{Filter: filter})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("BoardError_Internal_MapsToInternal", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "tps"}
		board.EXPECT().
			Public(ctx, filter, defaultLimit, "").
			Return(nil, "", errors.New("storage failure"))

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{Filter: filter})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("BoardError_DomainInternal_MapsToInternal", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "tps"}
		board.EXPECT().
			Public(ctx, filter, defaultLimit, "").
			Return(nil, "", derrors.Internal("db exploded"))

		_, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{Filter: filter})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("EmptyResultPage", func(t *testing.T) {
		svc, board := newSvc(t)

		filter := &api.RatingFilter{MetricKey: "tps"}
		board.EXPECT().
			Public(ctx, filter, defaultLimit, "").
			Return([]*api.PublicRatingEntry{}, "", nil)

		resp, err := svc.GetPublicRating(ctx, &api.GetPublicRatingRequest{Filter: filter})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Entries) != 0 {
			t.Errorf("expected empty entries, got %d", len(resp.Entries))
		}
	})
}

// TestClampLimit directly exercises clampLimit edge cases.
func TestClampLimit(t *testing.T) {
	cases := []struct {
		input uint32
		want  uint32
	}{
		{0, defaultLimit},
		{1, 1},
		{50, 50},
		{500, 500},
		{501, maxLimit},
		{1000, maxLimit},
	}
	for _, c := range cases {
		got := clampLimit(c.input)
		if got != c.want {
			t.Errorf("clampLimit(%d) = %d; want %d", c.input, got, c.want)
		}
	}
}
