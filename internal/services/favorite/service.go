package favorite

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	favorite is the generic, per-user favorite service. One mechanic covers every
	favoritable kind (the 3 preset tables, test runs, suites, suite runs). The
	favoriting user is always the CALLER — never a request field — so favorites are
	personal and tenant-scoped.

	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage of its own: every side effect goes through one of
	these ports. Implementations live elsewhere and are wired in via FavoriteDeps.
	Persistence methods return derrors.ErrNotFound / derrors.ErrConflict so the
	handler can translate them through utils.MapErr.
*/

// FavoriteRepo persists the per-(account, kind, target) favorite join rows.
//
// Every method is scoped by accountID so one caller can never read or mutate
// another caller's favorites. Get/Delete return derrors.ErrNotFound when the row
// is absent; Create returns derrors.ErrConflict on a duplicate (the handler treats
// both as idempotent no-ops). List returns the rows for the account, optionally
// narrowed to a single kind (FAVORITE_KIND_UNSPECIFIED = all kinds), paginated by
// an opaque cursor token.
type FavoriteRepo interface {
	Create(ctx context.Context, accountID string, record *models.FavoriteRecord) error
	Get(ctx context.Context, accountID string, kind common.FavoriteKind, targetID string) (*models.FavoriteRecord, error)
	Delete(ctx context.Context, accountID string, kind common.FavoriteKind, targetID string) error
	List(ctx context.Context, accountID, tenantID string, kind common.FavoriteKind, pageSize uint32, pageToken string) (records []*models.FavoriteRecord, nextPageToken string, err error)
}

// TargetResolver verifies that a favoritable target exists and lives in the given
// tenant before a favorite is written. It abstracts the per-kind lookup across the
// preset/run/suite tables behind the FavoriteKind discriminator. Returns
// derrors.ErrNotFound when the target row does not exist in that tenant.
type TargetResolver interface {
	// Resolve returns the target's Entity (carrying its tenant_id) when the row
	// exists; derrors.ErrNotFound otherwise.
	Resolve(ctx context.Context, kind common.FavoriteKind, targetID string) (*common.Entity, error)
}

// FavoriteDeps bundles every dependency for the constructor.
type FavoriteDeps struct {
	Authn     utils.Authn
	Favorites FavoriteRepo
	Targets   TargetResolver
	Tx        tx.Trm
}

type FavoriteService struct {
	*api.UnimplementedFavoriteServiceServer
	tx.Trm
	d FavoriteDeps
}

var _ api.FavoriteServiceServer = (*FavoriteService)(nil)

func NewFavoriteService(deps FavoriteDeps) *FavoriteService {
	return &FavoriteService{d: deps}
}

/*
	===== helpers =====
*/

func (s *FavoriteService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *FavoriteService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from scratch
// on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *FavoriteService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics: fn
// must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *FavoriteService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// validKind rejects FAVORITE_KIND_UNSPECIFIED for the mutating RPCs (proto already
// constrains it, but the handler defends the same invariant).
func validKind(kind common.FavoriteKind) error {
	if kind == common.FavoriteKind_FAVORITE_KIND_UNSPECIFIED {
		return status.Error(codes.InvalidArgument, "kind is required")
	}
	return nil
}

/*
	===== Favorites =====
*/

// AddFavorite marks (kind, target_id) as a favorite of the caller. Idempotent:
// favoriting an already-favorited row is a no-op that returns the existing record.
// The server validates the target exists and belongs to the request's tenant, and
// scopes the favorite to the caller — never to a request-supplied account.
func (s *FavoriteService) AddFavorite(ctx context.Context, req *api.AddFavoriteRequest) (*api.AddFavoriteResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := validKind(req.GetKind()); err != nil {
		return nil, err
	}

	rec, err := doTxRet(ctx, s, func(ctx context.Context) (*models.FavoriteRecord, error) {
		// Idempotent: if already favorited, return the existing row untouched.
		if existing, gerr := s.d.Favorites.Get(ctx, c.GetAccountId(), req.GetKind(), req.GetTargetId()); gerr == nil {
			return existing, nil
		} else if !errors.Is(gerr, derrors.ErrNotFound) {
			return nil, utils.MapErr(gerr)
		}

		// Validate the target exists and lives in the caller's active tenant.
		entity, terr := s.d.Targets.Resolve(ctx, req.GetKind(), req.GetTargetId())
		if terr != nil {
			return nil, utils.MapErr(terr)
		}
		if entity.GetTenantId() != req.GetTenantId() {
			return nil, status.Error(codes.NotFound, "target not found in tenant")
		}

		record := &models.FavoriteRecord{
			Entity: &common.Entity{
				Id:       uuid.NewString(),
				TenantId: req.GetTenantId(),
				AuthorId: c.GetAccountId(),
				Timings: &common.Timings{
					CreatedAt: s.now(),
					UpdatedAt: s.now(),
				},
				IsFavorite: true,
			},
			Kind:     req.GetKind(),
			TargetId: req.GetTargetId(),
		}
		if cerr := s.d.Favorites.Create(ctx, c.GetAccountId(), record); cerr != nil {
			// Lost an idempotency race: another writer created it concurrently.
			if errors.Is(cerr, derrors.ErrConflict) {
				if existing, gerr := s.d.Favorites.Get(ctx, c.GetAccountId(), req.GetKind(), req.GetTargetId()); gerr == nil {
					return existing, nil
				}
			}
			return nil, utils.MapErr(cerr)
		}
		return record, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.AddFavoriteResponse{Favorite: rec}, nil
}

// RemoveFavorite unmarks (kind, target_id) for the caller. Idempotent: removing an
// absent favorite is a no-op.
func (s *FavoriteService) RemoveFavorite(ctx context.Context, req *api.RemoveFavoriteRequest) (*api.RemoveFavoriteResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := validKind(req.GetKind()); err != nil {
		return nil, err
	}
	if err := derrors.IgnoreNotFound(s.d.Favorites.Delete(ctx, c.GetAccountId(), req.GetKind(), req.GetTargetId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.RemoveFavoriteResponse{}, nil
}

// ListFavorites returns the caller's favorites in the request's tenant, optionally
// narrowed to one kind (FAVORITE_KIND_UNSPECIFIED = all kinds).
func (s *FavoriteService) ListFavorites(ctx context.Context, req *api.ListFavoritesRequest) (*api.ListFavoritesResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	records, next, err := s.d.Favorites.List(
		ctx,
		c.GetAccountId(),
		req.GetTenantId(),
		req.GetKind(),
		req.GetPage().GetSize(),
		req.GetPage().GetToken(),
	)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListFavoritesResponse{Favorites: records, NextPageToken: next}, nil
}
