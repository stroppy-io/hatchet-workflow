package iam

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// ApiTokenRepo persists ApiToken metadata (never the secret).
type ApiTokenRepo interface {
	Create(ctx context.Context, token *iam.ApiToken) error
	Get(ctx context.Context, id string) (*iam.ApiToken, error)
	GetByPrefix(ctx context.Context, prefix string) (*iam.ApiToken, error)
	ListByAccount(ctx context.Context, accountID string) ([]*iam.ApiToken, error)
	Delete(ctx context.Context, id string) error
}

// ApiTokenSecrets holds the hashed token secret keyed by token id, separate from
// the metadata. Verification (lookup by prefix) is the auth layer's concern.
type ApiTokenSecrets interface {
	SetHash(ctx context.Context, tokenID, hash string) error
	GetHash(ctx context.Context, tokenID string) (string, error)
	Delete(ctx context.Context, tokenID string) error
}

// ApiTokenMinter generates a fresh token: the plaintext secret handed to the
// caller once, the non-secret prefix stored for display/lookup, and the hash
// persisted for verification.
type ApiTokenMinter interface {
	Mint() (secret, prefix, hash string, err error)
}

func (s *IamService) CreateApiToken(ctx context.Context, req *api.CreateApiTokenRequest) (*api.CreateApiTokenResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if !selfOrAdmin(c, req.GetAccountId()) {
		return nil, status.Error(codes.PermissionDenied, "not allowed to create tokens for this account")
	}
	if req.GetType() == iam.ApiTokenType_API_TOKEN_TYPE_PERSONAL {
		if len(req.GetPermissions()) > 0 {
			return nil, status.Error(codes.InvalidArgument, "personal tokens cannot carry an explicit permission set")
		}
		// A personal token inherits the owner's full authority (incl. is_admin), so
		// an eternal one is an unbounded standing credential — require an expiry.
		if req.GetTtl().AsDuration() <= 0 {
			return nil, status.Error(codes.InvalidArgument, "personal tokens require an expiry (ttl)")
		}
	} else if req.GetType() != iam.ApiTokenType_API_TOKEN_TYPE_SERVICE {
		return nil, status.Error(codes.InvalidArgument, "api token type is required")
	}
	owner, err := s.d.Accounts.Get(ctx, req.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if req.GetType() == iam.ApiTokenType_API_TOKEN_TYPE_SERVICE {
		if err := s.validateServiceTokenPermissions(ctx, owner, req.GetPermissions()); err != nil {
			return nil, err
		}
	}

	secret, prefix, hash, err := s.d.ApiTokenMinter.Mint()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var expiresAt *timestamppb.Timestamp
	if d := req.GetTtl().AsDuration(); d > 0 {
		expiresAt = timestamppb.New(time.Now().Add(d))
	}
	tok := &iam.ApiToken{
		Id:          uuid.NewString(),
		AccountId:   req.GetAccountId(),
		Name:        req.GetName(),
		Type:        req.GetType(),
		Prefix:      prefix,
		Permissions: req.GetPermissions(),
		ExpiresAt:   expiresAt,
		CreatedAt:   s.now(),
		UpdatedAt:   s.now(),
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		if err := s.d.ApiTokens.Create(ctx, tok); err != nil {
			return utils.MapErr(err)
		}
		return utils.MapErr(s.d.ApiTokenSecrets.SetHash(ctx, tok.Id, hash))
	}); err != nil {
		return nil, err
	}
	return &api.CreateApiTokenResponse{Token: tok, Secret: secret}, nil
}

func (s *IamService) validateServiceTokenPermissions(ctx context.Context, owner *iam.Account, requested []*iam.Permission) error {
	for _, p := range requested {
		if p == nil ||
			p.GetResource() == iam.Resource_RESOURCE_UNSPECIFIED ||
			p.GetAction() == iam.Action_ACTION_UNSPECIFIED ||
			iam.Resource_name[int32(p.GetResource())] == "" ||
			iam.Action_name[int32(p.GetAction())] == "" {
			return status.Error(codes.InvalidArgument, "service token permissions must use defined resource and action values")
		}
	}
	if len(requested) == 0 || owner.GetIsAdmin() {
		return nil
	}
	if s.d.Tenants == nil || s.d.Authz == nil {
		return status.Error(codes.Internal, "service token permission resolver is not configured")
	}
	tenants, err := s.d.Tenants.ListByMember(ctx, owner.GetId())
	if err != nil {
		return utils.MapErr(err)
	}
	var granted []*iam.Permission
	for _, tenant := range tenants {
		perms, err := s.d.Authz.EffectivePermissions(ctx, owner.GetId(), tenant.GetId())
		if err != nil {
			return utils.MapErr(err)
		}
		granted = append(granted, perms...)
	}
	if !hasAll(granted, requested) {
		return status.Error(codes.PermissionDenied, "service token permissions exceed the owner's permissions")
	}
	return nil
}

func (s *IamService) ListApiTokens(ctx context.Context, req *api.ListApiTokensRequest) (*api.ListApiTokensResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if !selfOrAdmin(c, req.GetAccountId()) {
		return nil, status.Error(codes.PermissionDenied, "not allowed to view these tokens")
	}
	tokens, err := s.d.ApiTokens.ListByAccount(ctx, req.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListApiTokensResponse{Tokens: tokens}, nil
}

func (s *IamService) RevokeApiToken(ctx context.Context, req *api.RevokeApiTokenRequest) (*api.RevokeApiTokenResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	tok, err := s.d.ApiTokens.Get(ctx, req.GetId())
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.RevokeApiTokenResponse{}, nil
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if !selfOrAdmin(c, tok.AccountId) {
		return nil, status.Error(codes.PermissionDenied, "not allowed to revoke this token")
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		if err := derrors.IgnoreNotFound(s.d.ApiTokenSecrets.Delete(ctx, tok.Id)); err != nil {
			return utils.MapErr(err)
		}
		return utils.MapErr(derrors.IgnoreNotFound(s.d.ApiTokens.Delete(ctx, tok.Id)))
	}); err != nil {
		return nil, err
	}
	return &api.RevokeApiTokenResponse{}, nil
}
