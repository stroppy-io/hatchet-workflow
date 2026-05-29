package share

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// defaultShareTTL is the lifetime applied when CreateShare/SetShareExpiry are
// called with an unset (nil) ttl (1 week). An explicit ttl of 0 means "never"
// (no expiry) and is preserved as-is.
const defaultShareTTL = 7 * 24 * time.Hour

/*
	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage, crypto or run-introspection of its own: every
	side effect goes through one of these. Implementations live elsewhere and are
	wired in via ShareDeps. Persistence methods return derrors.ErrNotFound /
	derrors.ErrConflict so the handler can translate them via utils.MapErr.
*/

// ShareRepo persists ShareRecord rows. All lookups are tenant-scoped: Get takes
// the tenant_id so a share can never be read across tenants, and List filters to
// one tenant (optionally narrowed to a single target run). The token is minted
// by the TokenMinter and stored on the record; uniqueness is the store's
// concern (Create returns derrors.ErrConflict on a token collision).
type ShareRepo interface {
	Create(ctx context.Context, share *models.ShareRecord) error
	// Get returns the share by id, scoped to tenantID. derrors.ErrNotFound when
	// the row is absent OR belongs to another tenant.
	Get(ctx context.Context, tenantID, id string) (*models.ShareRecord, error)
	// List returns the tenant's shares, optionally narrowed to targetID (empty =
	// any target), honoring the common filter/sort/page.
	List(
		ctx context.Context,
		tenantID, targetID string,
		filter *common.EntityFilter,
		sort *common.EntitySort,
		page *common.Page,
	) (shares []*models.ShareRecord, nextPageToken string, err error)
	Update(ctx context.Context, share *models.ShareRecord) error
	// Delete removes the share by id within tenantID. derrors.ErrNotFound when
	// absent so the handler can treat deletes idempotently.
	Delete(ctx context.Context, tenantID, id string) error
}

// TokenMinter generates the public, unguessable (>=128-bit) share token carried
// in the share URL. A fresh token is minted on every CreateShare.
type TokenMinter interface {
	Mint() (token string, err error)
}

// SnapshotBuilder captures the FROZEN, limited public view of a run at share
// creation time (the background refresh job re-runs this later). It verifies the
// target run exists and belongs to tenantID, then projects it to the limited
// Snapshot — never the baked spec, params, secrets, raw logs, shell or tenant
// data. derrors.ErrNotFound when the target run does not exist in the tenant.
type SnapshotBuilder interface {
	Build(ctx context.Context, tenantID string, target *models.ShareRecord_Target) (*models.ShareRecord_Snapshot, error)
}

// ShareDeps bundles every dependency for the constructor.
type ShareDeps struct {
	Authn     utils.Authn
	Shares    ShareRepo
	Minter    TokenMinter
	Snapshots SnapshotBuilder
	Tx        tx.Trm
}

type ShareService struct {
	*api.UnimplementedShareServiceServer
	tx.Trm
	d ShareDeps
}

var _ api.ShareServiceServer = (*ShareService)(nil)

func NewShareService(deps ShareDeps) *ShareService {
	return &ShareService{d: deps}
}

/*
	===== helpers =====
*/

func (s *ShareService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *ShareService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *ShareService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *ShareService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// expiryFromTTL maps the create/set-expiry ttl semantics to an expires_at.
// The proto distinguishes "unset" (nil message) from "explicit zero" (set but
// zero), which carry different lifetime semantics:
//   - nil ttl   -> server default (defaultShareTTL from now)
//   - ttl == 0  -> never expires (nil expires_at)
//   - ttl > 0   -> now + ttl
func (s *ShareService) expiryFromTTL(ttl *durationpb.Duration) *timestamppb.Timestamp {
	if ttl == nil {
		return timestamppb.New(time.Now().Add(defaultShareTTL))
	}
	if d := ttl.AsDuration(); d > 0 {
		return timestamppb.New(time.Now().Add(d))
	}
	return nil
}

/*
	===== Shares =====
*/

func (s *ShareService) CreateShare(ctx context.Context, req *api.CreateShareRequest) (*api.CreateShareResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	target := req.GetTarget()
	if target == nil {
		return nil, status.Error(codes.InvalidArgument, "target is required")
	}
	if target.GetKind() == models.ShareRecord_Target_KIND_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "target.kind is required")
	}
	if target.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "target.id is required")
	}

	token, err := s.d.Minter.Mint()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	expiresAt := s.expiryFromTTL(req.GetTtl())

	share, err := doTxRet(ctx, s, func(ctx context.Context) (*models.ShareRecord, error) {
		// Capture the first snapshot; this also verifies the target run exists
		// and belongs to the tenant.
		snapshot, err := s.d.Snapshots.Build(ctx, req.GetTenantId(), target)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		now := s.now()
		rec := &models.ShareRecord{
			Entity: &common.Entity{
				Id:       uuid.NewString(),
				TenantId: req.GetTenantId(),
				AuthorId: c.GetAccountId(),
				Timings: &common.Timings{
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
			Target:    target,
			Token:     token,
			ExpiresAt: expiresAt,
			Revoked:   false,
			Snapshot:  snapshot,
		}
		if err := s.d.Shares.Create(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.CreateShareResponse{Share: share}, nil
}

func (s *ShareService) GetShare(ctx context.Context, req *api.GetShareRequest) (*api.GetShareResponse, error) {
	share, err := s.d.Shares.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetShareResponse{Share: share}, nil
}

func (s *ShareService) ListShares(ctx context.Context, req *api.ListSharesRequest) (*api.ListSharesResponse, error) {
	shares, next, err := s.d.Shares.List(
		ctx,
		req.GetTenantId(),
		req.GetTargetId(),
		req.GetFilter(),
		req.GetSort(),
		req.GetPage(),
	)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListSharesResponse{Shares: shares, NextPageToken: next}, nil
}

func (s *ShareService) RevokeShare(ctx context.Context, req *api.RevokeShareRequest) (*api.RevokeShareResponse, error) {
	share, err := doTxRet(ctx, s, func(ctx context.Context) (*models.ShareRecord, error) {
		rec, err := s.d.Shares.Get(ctx, req.GetTenantId(), req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		// Idempotent: revoking an already-revoked share is a no-op.
		if rec.GetRevoked() {
			return rec, nil
		}
		rec.Revoked = true
		touch(rec, s.now())
		if err := s.d.Shares.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.RevokeShareResponse{Share: share}, nil
}

func (s *ShareService) SetShareExpiry(ctx context.Context, req *api.SetShareExpiryRequest) (*api.SetShareExpiryResponse, error) {
	expiresAt := s.expiryFromTTL(req.GetTtl())
	share, err := doTxRet(ctx, s, func(ctx context.Context) (*models.ShareRecord, error) {
		rec, err := s.d.Shares.Get(ctx, req.GetTenantId(), req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		// Idempotent: setting the same lifetime converges.
		if sameExpiry(rec.GetExpiresAt(), expiresAt) {
			return rec, nil
		}
		rec.ExpiresAt = expiresAt
		touch(rec, s.now())
		if err := s.d.Shares.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.SetShareExpiryResponse{Share: share}, nil
}

func (s *ShareService) DeleteShare(ctx context.Context, req *api.DeleteShareRequest) (*api.DeleteShareResponse, error) {
	// Idempotent: deleting an absent share is a no-op.
	if err := derrors.IgnoreNotFound(s.d.Shares.Delete(ctx, req.GetTenantId(), req.GetId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteShareResponse{}, nil
}

// touch stamps the row's updated_at (creating the timings/entity envelope if a
// store handed back a partial row).
func touch(rec *models.ShareRecord, now *timestamppb.Timestamp) {
	if rec.Entity == nil {
		rec.Entity = &common.Entity{}
	}
	if rec.Entity.Timings == nil {
		rec.Entity.Timings = &common.Timings{}
	}
	rec.Entity.Timings.UpdatedAt = now
}

// sameExpiry reports whether two expires_at values denote the same instant
// (both nil = never; nil vs set differ).
func sameExpiry(a, b *timestamppb.Timestamp) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.AsTime().Equal(b.AsTime())
	}
}
