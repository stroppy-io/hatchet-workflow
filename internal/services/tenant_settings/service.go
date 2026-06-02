package tenant_settings

import (
	"context"
	"errors"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deployment "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	TenantSettingsService owns the per-tenant settings row (models.TenantSettingsRecord,
	exactly one per tenant). It holds no storage or schema engine of its own: every
	side effect goes through one of these ports. Implementations live elsewhere and
	are wired in via TenantSettingsDeps. Persistence methods return derrors.ErrNotFound /
	derrors.ErrConflict so handlers can translate them via utils.MapErr.

	AUTHZ NOTE: the RESOURCE_SETTINGS permission is enforced by the auth interceptor
	from the proto annotations — handlers do NOT re-check it. Handlers DO resolve the
	caller (for entity authorship), validate the active tenant, confirm the tenant
	exists, and enforce the documented per-tenant / per-provider invariants.
*/

// TenantSettingsRepo persists the singleton settings row per tenant. Get returns
// derrors.ErrNotFound when the tenant has no row yet. Upsert creates-or-replaces
// the whole row (the contract is idempotent / wholesale-replace).
type TenantSettingsRepo interface {
	Get(ctx context.Context, tenantID string) (*models.TenantSettingsRecord, error)
	Upsert(ctx context.Context, settings *models.TenantSettingsRecord) error
}

// TenantSettingsDeps bundles every dependency for the constructor.
type TenantSettingsDeps struct {
	Authn    utils.Authn
	Tenants  utils.TenantReader
	Settings TenantSettingsRepo
	Tx       tx.Trm
}

type TenantSettingsService struct {
	*api.UnimplementedTenantSettingsServiceServer
	tx.Trm
	d TenantSettingsDeps
}

var _ api.TenantSettingsServiceServer = (*TenantSettingsService)(nil)

func NewTenantSettingsService(deps TenantSettingsDeps) *TenantSettingsService {
	return &TenantSettingsService{UnimplementedTenantSettingsServiceServer: &api.UnimplementedTenantSettingsServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *TenantSettingsService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *TenantSettingsService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat (DB-only side effects, never
// synchronous external IO).
func (s *TenantSettingsService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics.
func doTxRet[T any](ctx context.Context, s *TenantSettingsService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// requireTenant verifies the tenant exists, translating not-found into a clean
// gRPC code. It returns the resolved tenant for downstream use.
func (s *TenantSettingsService) requireTenant(ctx context.Context, tenantID string) (*iam.Tenant, error) {
	t, err := s.d.Tenants.Get(ctx, tenantID)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return t, nil
}

// loadOrInit returns the tenant's current settings row, or a freshly seeded one
// (empty entity stamped with the tenant id) when none exists yet.
func (s *TenantSettingsService) loadOrInit(ctx context.Context, tenantID, callerID string) (*models.TenantSettingsRecord, error) {
	rec, err := s.d.Settings.Get(ctx, tenantID)
	if err == nil {
		return rec, nil
	}
	if !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}
	return &models.TenantSettingsRecord{
		Entity: &commonpb.Entity{
			TenantId: tenantID,
			AuthorId: callerID,
			Timings:  &commonpb.Timings{CreatedAt: s.now(), UpdatedAt: s.now()},
		},
	}, nil
}

/*
	===== RPCs =====
*/

// GetTenantSettings returns the tenant's settings row, seeding an empty default
// row (never persisted here) when the tenant has none yet, so callers always get
// a well-formed record for a tenant that exists.
func (s *TenantSettingsService) GetTenantSettings(ctx context.Context, req *api.GetTenantSettingsRequest) (*api.GetTenantSettingsResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	rec, err := s.loadOrInit(ctx, req.GetTenantId(), c.GetAccountId())
	if err != nil {
		return nil, err
	}
	return &api.GetTenantSettingsResponse{Settings: rec}, nil
}

// UpdateTenantSettings replaces the tenant's settings wholesale (idempotent). The
// server owns the identity fields on the entity (tenant_id, author, timings); the
// client-supplied entity is ignored for those, and the entity's tenant_id is
// forced to match the request to keep the row keyed to the right tenant.
func (s *TenantSettingsService) UpdateTenantSettings(ctx context.Context, req *api.UpdateTenantSettingsRequest) (*api.UpdateTenantSettingsResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetSettings() == nil {
		return nil, status.Error(codes.InvalidArgument, "settings required")
	}
	if err := req.GetSettings().ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if _, err := s.requireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}

	saved, err := doTxRet(ctx, s, func(ctx context.Context) (*models.TenantSettingsRecord, error) {
		existing, err := s.loadOrInit(ctx, req.GetTenantId(), c.GetAccountId())
		if err != nil {
			return nil, err
		}
		next := req.GetSettings()
		// Wholesale replace of the editable payload, but server-owned identity is
		// preserved from the existing row (or the seeded default).
		next.Entity = s.mergeEntity(existing.GetEntity(), req.GetTenantId(), c.GetAccountId())
		if err := s.d.Settings.Upsert(ctx, next); err != nil {
			return nil, utils.MapErr(err)
		}
		return next, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.UpdateTenantSettingsResponse{Settings: saved}, nil
}

// SetTenantProviderSettings sets/replaces the config for ONE provider (idempotent,
// keyed by provider). The provider is identified by the oneof variant carried in
// ProviderSettings; the typed settings are validated and stored on the tenant's
// settings row (and the tenant's default_provider is pointed at it). Returns Empty.
func (s *TenantSettingsService) SetTenantProviderSettings(ctx context.Context, req *api.SetTenantProviderSettingsRequest) (*emptypb.Empty, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetSettings() == nil || req.GetSettings().GetSettings() == nil {
		return nil, status.Error(codes.InvalidArgument, "settings required")
	}
	if err := req.GetSettings().ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if _, err := s.requireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}

	if err := s.doTx(ctx, func(ctx context.Context) error {
		existing, err := s.loadOrInit(ctx, req.GetTenantId(), c.GetAccountId())
		if err != nil {
			return err
		}
		existing.Entity = s.mergeEntity(existing.GetEntity(), req.GetTenantId(), c.GetAccountId())
		applyProviderSettings(existing, req.GetSettings())
		if err := s.d.Settings.Upsert(ctx, existing); err != nil {
			return utils.MapErr(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// applyProviderSettings copies the typed provider config from a ProviderSettings
// oneof onto the tenant's settings row and points default_provider at it. At most
// one config per provider is stored (the row carries one typed field per provider).
func applyProviderSettings(rec *models.TenantSettingsRecord, cfg *deployment.ProviderSettings) {
	switch {
	case cfg.GetYandex() != nil:
		rec.YandexSettings = cfg.GetYandex()
		rec.DefaultProvider = deployment.Provider_PROVIDER_YANDEX
	case cfg.GetDocker() != nil:
		rec.DefaultProvider = deployment.Provider_PROVIDER_DOCKER
	}
}

// mergeEntity preserves the server-owned identity of an existing settings row
// (id, created-at, original author) while forcing the tenant id and bumping the
// updated-at stamp. A nil prior entity (first write) seeds a fresh one.
func (s *TenantSettingsService) mergeEntity(prior *commonpb.Entity, tenantID, callerID string) *commonpb.Entity {
	if prior == nil {
		return &commonpb.Entity{
			TenantId: tenantID,
			AuthorId: callerID,
			Timings:  &commonpb.Timings{CreatedAt: s.now(), UpdatedAt: s.now()},
		}
	}
	e := &commonpb.Entity{
		Id:          prior.GetId(),
		TenantId:    tenantID,
		Name:        prior.GetName(),
		Description: prior.GetDescription(),
		AuthorId:    prior.GetAuthorId(),
	}
	if e.AuthorId == "" {
		e.AuthorId = callerID
	}
	created := s.now()
	if prior.GetTimings().GetCreatedAt() != nil {
		created = prior.GetTimings().GetCreatedAt()
	}
	e.Timings = &commonpb.Timings{CreatedAt: created, UpdatedAt: s.now()}
	return e
}
