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

// ListApiTokens returns the tenant's tokens (hash never exposed) with cursor
// pagination (newest-first by default).
func (s *ApiTokenService) ListApiTokens(ctx context.Context, req *uipb.ListApiTokensRequest) (*uipb.ListApiTokensResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListApiTokens",
		func(ctx context.Context, _ trace.Span) (*uipb.ListApiTokensResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.ApiTokens.SelectAll().Where(
				models.ApiTokens.TenantId.Eq(req.GetTenantId().GetValue()),
				models.ApiTokens.DeletedAt.IsNull(),
			)
			// search: match the token name (free text).
			if req.Search != nil && req.GetSearch() != "" {
				q = q.Where(models.ApiTokens.Name.ILike("%" + req.GetSearch() + "%"))
			}
			// role: Role is stored as the enum String() value (apitoken_plain converter).
			if req.Role != nil {
				q = q.Where(models.ApiTokens.Role.Eq(req.GetRole().String()))
			}
			// expired: tri-state (*bool) — true = past expiry (expires_at < now);
			// false = active or never-expires (expires_at is null OR >= now).
			if req.Expired != nil {
				now := time.Now()
				if req.GetExpired() {
					q = q.Where(models.ApiTokens.ExpiresAt.Lt(&now))
				} else {
					q = q.Where(models.ApiTokens.Or(
						models.ApiTokens.ExpiresAt.IsNull(),
						models.ApiTokens.ExpiresAt.Gte(&now),
					))
				}
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.ApiTokens.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.ApiTokens.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.ApiTokens.Id.Lt(tok))
				} else {
					q = q.Where(models.ApiTokens.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.ApiTokenColumnId)
			} else {
				q = q.OrderByASC(models.ApiTokenColumnId)
			}
			rows, err := s.tokens.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list api tokens: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(t *models.ApiToken) string {
				return t.GetEntity().GetId().GetValue()
			})
			return &uipb.ListApiTokensResponse{ApiTokens: items, PageInfo: pageInfo}, nil
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
