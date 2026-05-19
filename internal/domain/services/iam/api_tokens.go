package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// CreateApiToken creates a long-lived API token tied to (tenant, user).
// Returns the proto (without cleartext) and the cleartext token value.
func (s *Service) CreateApiToken(ctx context.Context, tenantID *iampb.TenantId, userID *iampb.UserId, name string) (*iampb.ApiToken, string, error) {
	now := time.Now()
	plain := "sct_" + ids.New() + ids.New()
	createdByStr := userID.GetValue()
	row := &iampb.ApiToken{
		Id:       &iampb.ApiTokenId{Value: ids.New()},
		TenantId: tenantID,
		Name:     name,
		Timestamps: &commonpb.Timestamps{
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
	}
	scanner := row.IntoPlain()
	// TokenHash is virtual — IntoPlain() leaves it empty; set it explicitly.
	scanner.TokenHash = sha256Hex(plain)
	// CreatedBy is *string in scanner (derived from *UserId in proto).
	scanner.CreatedBy = &createdByStr

	if _, err := s.tokenRepo.Execute(ctx,
		iampb.ApiTokens.Insert().From(scanner.AllSetters()...),
	); err != nil {
		return nil, "", err
	}
	return row, plain, nil
}

// VerifyApiToken hashes the plaintext value, looks up by token_hash,
// and rejects soft-deleted tokens.
func (s *Service) VerifyApiToken(ctx context.Context, plain string) (*iampb.ApiToken, error) {
	row, err := s.tokenRepo.QueryRow(ctx,
		iampb.ApiTokens.SelectAll().Where(
			iampb.ApiTokens.TokenHash.Eq(sha256Hex(plain)),
			iampb.ApiTokens.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.Unauthenticated()
		}
		return nil, err
	}
	return row, nil
}

// RevokeApiToken soft-deletes a token by ID and returns the pre-delete row.
func (s *Service) RevokeApiToken(ctx context.Context, id *iampb.ApiTokenId) (*iampb.ApiToken, error) {
	existing, err := s.GetApiToken(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if _, err := s.tokenRepo.Execute(ctx,
		iampb.ApiTokens.Update().
			Set(iampb.ApiTokens.DeletedAt.Set(&now)).
			Where(iampb.ApiTokens.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return existing, nil
}

// GetApiToken returns a non-deleted token by ID.
func (s *Service) GetApiToken(ctx context.Context, id *iampb.ApiTokenId) (*iampb.ApiToken, error) {
	row, err := s.tokenRepo.QueryRow(ctx,
		iampb.ApiTokens.SelectAll().Where(
			iampb.ApiTokens.Id.Eq(id.GetValue()),
			iampb.ApiTokens.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("api_token", id.GetValue()))
		}
		return nil, err
	}
	return row, nil
}

// ListApiTokens returns all non-deleted tokens for a tenant.
func (s *Service) ListApiTokens(ctx context.Context, tenantID *iampb.TenantId) ([]*iampb.ApiToken, error) {
	return s.tokenRepo.Query(ctx,
		iampb.ApiTokens.SelectAll().Where(
			iampb.ApiTokens.TenantId.Eq(tenantID.GetValue()),
			iampb.ApiTokens.DeletedAt.IsNull(),
		),
	)
}

// UpdateApiTokenExpiry sets expires_at on an existing token. expiresAt=nil
// clears the expiry (token becomes long-lived).
func (s *Service) UpdateApiTokenExpiry(ctx context.Context, id *iampb.ApiTokenId, expiresAt *time.Time) (*iampb.ApiToken, error) {
	now := time.Now()
	if _, err := s.tokenRepo.Execute(ctx,
		iampb.ApiTokens.Update().
			Set(
				iampb.ApiTokens.ExpiresAt.Set(expiresAt),
				iampb.ApiTokens.UpdatedAt.Set(now),
			).
			Where(iampb.ApiTokens.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return s.GetApiToken(ctx, id)
}
