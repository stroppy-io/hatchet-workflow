package public_share

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	PublicShareService is the PUBLIC, UNAUTHENTICATED resolve side of run shares.
	There is no caller identity, no tenant, no account — just the unguessable token
	from the share URL. The service owns no storage of its own: the lookup goes
	through ShareRepo, and the only other side effect is reading the wall clock
	to decide whether a share has expired. Persistence methods return
	derrors.ErrNotFound so the handler can translate via utils.MapErr; an unknown,
	revoked, or expired token all collapse to a single not-found ("gone") response so
	the endpoint never distinguishes a never-existed token from a withdrawn one.
*/

// ShareRepo resolves a public share by its unguessable token. It returns the
// stored ShareRecord (including the background-refreshed snapshot) and
// derrors.ErrNotFound when no live record carries that token. Implementations
// MUST scope the lookup to the token alone — there is no tenant/account context
// on this surface.
type ShareRepo interface {
	GetByToken(ctx context.Context, token string) (*models.ShareRecord, error)
}

// PublicShareDeps bundles every dependency for the constructor.
type PublicShareDeps struct {
	Shares ShareRepo
	Tx     tx.Trm
}

type PublicShareService struct {
	*api.UnimplementedPublicShareServiceServer
	tx.Trm
	d PublicShareDeps
}

var _ api.PublicShareServiceServer = (*PublicShareService)(nil)

func NewPublicShareService(deps PublicShareDeps) *PublicShareService {
	return &PublicShareService{d: deps}
}

/*
	===== helpers =====
*/

func (s *PublicShareService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat.
func (s *PublicShareService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *PublicShareService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// gone is the single response for every "this share does not serve content"
// case (unknown token, revoked, expired). NotFound is the closest gRPC code to
// the proto's "gone" semantics, and using one code keeps the three cases
// indistinguishable to an anonymous caller.
func gone() error {
	return status.Error(codes.NotFound, "share not found, revoked, or expired")
}

/*
	===== PublicShare (public) =====
*/

// GetSharedRun resolves a share token to its limited public snapshot. PUBLIC: no
// bearer token, no tenant. It serves ONLY the stored, background-refreshed
// snapshot — never the live system, spec, secrets, logs, or shell. A token that
// is unknown, revoked, or past its expiry all yield the same "gone" not-found.
func (s *PublicShareService) GetSharedRun(ctx context.Context, req *api.GetSharedRunRequest) (*api.GetSharedRunResponse, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token required")
	}
	// A single point read; wrapping it in the ambient serializable transaction
	// keeps the lookup consistent with any concurrent background refresh/revoke.
	rec, err := doTxRet(ctx, s, func(ctx context.Context) (*models.ShareRecord, error) {
		r, err := s.d.Shares.GetByToken(ctx, req.GetToken())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		return r, nil
	})
	if err != nil {
		// Never leak "exists but no read access" vs "missing": any failure to
		// produce a live record is a uniform gone for this public surface.
		if status.Code(err) == codes.NotFound {
			return nil, gone()
		}
		return nil, err
	}
	if rec == nil || !s.isLive(rec) {
		return nil, gone()
	}
	snap := rec.GetSnapshot()
	if snap == nil {
		// Share is live but the background job has not captured a snapshot yet:
		// there is genuinely nothing to serve to the public.
		return nil, gone()
	}
	return &api.GetSharedRunResponse{Snapshot: snap}, nil
}

// ensure doTx is part of the package surface (used by background-write helpers /
// future flows) so the serializable-write seam mirrors the reference exactly.
var _ = (*PublicShareService).doTx

// isLive reports whether the share is currently servable: not manually revoked
// and not past its expiry (an unset expiry never expires).
func (s *PublicShareService) isLive(rec *models.ShareRecord) bool {
	if rec.GetRevoked() {
		return false
	}
	if exp := rec.GetExpiresAt(); exp != nil {
		if !s.now().AsTime().Before(exp.AsTime()) {
			return false
		}
	}
	return true
}
