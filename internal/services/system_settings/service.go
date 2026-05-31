package system_settings

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	system_settings owns the GLOBAL control-plane settings singleton
	(api.PlatformSettings). Every side effect goes through one of these ports;
	implementations are wired in via SystemSettingsDeps. Persistence methods return
	derrors.ErrNotFound / derrors.ErrConflict so handlers can translate them into
	status codes via utils.MapErr.

	AUTHZ NOTE: both RPCs are admin_only and the RBAC interceptor enforces that
	upstream from the proto annotation — handlers do NOT re-check the permission.
*/

// PlatformSettingsRepo persists the single control-plane settings row. The store
// is a singleton: there is exactly one PlatformSettings. Get returns the current
// row; on a brand-new install where no row has been written yet it returns
// derrors.ErrNotFound, which the handler treats as "defaults". Set replaces the
// singleton wholesale (upsert), matching the idempotent UpdateSystemSettings
// contract.
type PlatformSettingsRepo interface {
	Get(ctx context.Context) (*api.PlatformSettings, error)
	Set(ctx context.Context, settings *api.PlatformSettings) error
}

// SystemSettingsDeps bundles every dependency for the constructor.
type SystemSettingsDeps struct {
	Authn    utils.Authn
	Settings PlatformSettingsRepo
	Tx       tx.Trm
}

type SystemSettingsService struct {
	*api.UnimplementedSystemSettingsServiceServer
	tx.Trm
	d SystemSettingsDeps
}

var _ api.SystemSettingsServiceServer = (*SystemSettingsService)(nil)

func NewSystemSettingsService(deps SystemSettingsDeps) *SystemSettingsService {
	return &SystemSettingsService{UnimplementedSystemSettingsServiceServer: &api.UnimplementedSystemSettingsServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *SystemSettingsService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *SystemSettingsService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *SystemSettingsService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *SystemSettingsService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}
