package public_rating

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// defaultLimit caps an unbounded request: limit==0 means "use the default"
// rather than "return nothing". maxLimit mirrors the proto rule (uint32.lte =
// 500); a request over the cap is clamped rather than rejected so the public
// surface stays forgiving.
const (
	defaultLimit uint32 = 50
	maxLimit     uint32 = 500
)

/*
	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage of its own. The public leaderboard is NOT a
	stored rating record: it is computed/precomputed by a background job (same
	idea as the share-snapshot refresh), so the only port is a read-only board
	query. Persistence methods return derrors.* so the handler can translate them
	into status codes via utils.MapErr.
*/

// PublicRatingBoard resolves the public, sensitive-fields-stripped leaderboard
// over in_global_rating runs. The implementation reads the precomputed board
// (background-job output), ranks by the filter's metric_key honoring that
// metric's higher_is_better direction, applies the narrowing facets (all AND),
// and paginates. limit is the already-validated page size (>0, <=maxLimit) and
// pageToken is opaque ("" for the first page); it returns the next page token
// ("" when exhausted).
type PublicRatingBoard interface {
	Public(ctx context.Context, filter *api.RatingFilter, limit uint32, pageToken string) (entries []*api.PublicRatingEntry, nextPageToken string, err error)
}

// PublicRatingDeps bundles every dependency for the constructor.
type PublicRatingDeps struct {
	Board PublicRatingBoard
	Tx    tx.Trm
}

type PublicRatingService struct {
	*api.UnimplementedPublicRatingServiceServer
	tx.Trm
	d PublicRatingDeps
}

var _ api.PublicRatingServiceServer = (*PublicRatingService)(nil)

func NewPublicRatingService(deps PublicRatingDeps) *PublicRatingService {
	return &PublicRatingService{UnimplementedPublicRatingServiceServer: &api.UnimplementedPublicRatingServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *PublicRatingService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat.
func (s *PublicRatingService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *PublicRatingService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// clampLimit normalizes the request page size: 0 -> default, anything over the
// proto cap -> cap.
func clampLimit(limit uint32) uint32 {
	switch {
	case limit == 0:
		return defaultLimit
	case limit > maxLimit:
		return maxLimit
	default:
		return limit
	}
}

/*
	===== Public leaderboard =====
*/

// GetPublicRating resolves the public leaderboard. PUBLIC: no bearer token, so
// no caller identity is resolved. The board is precomputed (no stored rating
// record), so this is a pure read — but it still runs through doTx so the
// snapshot the implementation reads is consistent across the (potentially
// multi-table) board + facet lookups.
func (s *PublicRatingService) GetPublicRating(ctx context.Context, req *api.GetPublicRatingRequest) (*api.GetPublicRatingResponse, error) {
	filter := req.GetFilter()
	if filter == nil {
		return nil, status.Error(codes.InvalidArgument, "filter is required")
	}
	if filter.GetMetricKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "filter.metric_key is required")
	}
	if req.GetLimit() > maxLimit {
		return nil, status.Error(codes.InvalidArgument, "limit must be <= 500")
	}

	limit := clampLimit(req.GetLimit())
	resp, err := doTxRet(ctx, s, func(ctx context.Context) (*api.GetPublicRatingResponse, error) {
		entries, next, err := s.d.Board.Public(ctx, filter, limit, req.GetPageToken())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		return &api.GetPublicRatingResponse{Entries: entries, NextPageToken: next}, nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
