// Package apitoken implements the tenant-scoped ApiTokenService (3rd auth
// principal). RBAC: OWNER-only (mint/list/revoke). Only the sha256 hash is
// stored; the plaintext secret is returned once on create.
package apitoken

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// ApiTokenService implements ui.ApiTokenActions.
type ApiTokenService struct {
	*tracing.Entity
	tokens *repository.ProtoRepository[
		models.ApiTokenAlias,
		models.ApiTokenColumnAlias,
		*models.ApiTokenScanner,
		*models.ApiToken,
	]
	authz *authz.Authz
	txm   tx.Trm
}

var _ uiapi.ApiTokenActions = (*ApiTokenService)(nil)

// NewApiTokenService builds the service.
func NewApiTokenService(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz) *ApiTokenService {
	return &ApiTokenService{
		Entity: tracing.NewEntity(logger.AppendName("ApiTokenService")),
		tokens: repository.NewProtoRepository(
			repository.NewScannerRepository(models.ApiTokens.Table, executor),
			models.ApiTokenConverter,
		),
		authz: az,
		txm:   txm,
	}
}

// CreateApiToken mints a token, stores only its hash, and returns the plaintext once.
func (s *ApiTokenService) CreateApiToken(ctx context.Context, req *uipb.CreateApiTokenRequest) (*uipb.CreateApiTokenResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateApiToken",
		func(ctx context.Context, _ trace.Span) (*uipb.CreateApiTokenResponse, error) {
			c := svcutil.CallerOf(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			secret, err := domainauth.GenerateOpaqueToken()
			if err != nil {
				return nil, status.Errorf(codes.Internal, "generate token: %v", err)
			}
			token := &models.ApiToken{
				Entity:    ids.NewEntity(),
				Owned:     &models.Own{OwnerAccountId: c.AccountID, TenantId: req.GetTenantId()},
				Name:      req.GetName(),
				Role:      req.GetRole(),
				ExpiresAt: req.GetExpiresAt(),
			}
			scanner := token.IntoPlain()
			scanner.TokenHash = domainauth.HashToken(secret)
			if _, err := tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (struct{}, error) {
				_, err := s.tokens.Scanner().Execute(ctx,
					models.ApiTokens.Insert().From(scanner.AllSetters()...))
				return struct{}{}, err
			}); err != nil {
				return nil, status.Errorf(codes.Internal, "insert api token: %v", err)
			}
			return &uipb.CreateApiTokenResponse{Token: token, Secret: secret}, nil
		})
}

// ListApiTokens returns the tenant's tokens (hash never exposed).
func (s *ApiTokenService) ListApiTokens(ctx context.Context, req *uipb.ListApiTokensRequest) (*models.ApiToken_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListApiTokens",
		func(ctx context.Context, _ trace.Span) (*models.ApiToken_List, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			tokens, err := s.tokens.Query(ctx,
				models.ApiTokens.SelectAll().Where(
					models.ApiTokens.TenantId.Eq(req.GetTenantId().GetValue()),
					models.ApiTokens.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list api tokens: %v", err)
			}
			return &models.ApiToken_List{ApiTokens: tokens}, nil
		})
}

// RevokeApiToken soft-deletes a token.
func (s *ApiTokenService) RevokeApiToken(ctx context.Context, req *uipb.RevokeApiTokenRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RevokeApiToken",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			now := time.Now()
			if _, err := s.tokens.Execute(ctx,
				models.ApiTokens.Update().Set(
					models.ApiTokens.DeletedAt.Set(&now),
					models.ApiTokens.UpdatedAt.Set(now),
				).Where(
					models.ApiTokens.Id.Eq(req.GetId().GetValue()),
					models.ApiTokens.TenantId.Eq(req.GetTenantId().GetValue()),
				)); err != nil {
				return nil, status.Errorf(codes.Internal, "revoke api token: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}
