package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/token"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// TokenRepo stores API tokens.
type TokenRepo struct {
	q *db.Queries
}

var _ token.Repository = (*TokenRepo)(nil)

// NewTokenRepo builds the repo.
func NewTokenRepo(database tx.DB) *TokenRepo { return &TokenRepo{q: db.New(database)} }

func (r *TokenRepo) Insert(ctx context.Context, t token.Token, secretHash []byte) error {
	err := r.q.InsertToken(ctx, db.InsertTokenParams{
		ID: t.ID, Kind: string(t.Kind), Name: t.Name, Prefix: t.Prefix, SecretHash: secretHash,
		TenantID: t.TenantID, Role: t.Role, OwnerID: t.OwnerID, ExpiresAt: t.ExpiresAt,
	})
	if err != nil {
		return infraf("token: insert: %v", err)
	}
	return nil
}

func (r *TokenRepo) ByPrefix(ctx context.Context, prefix string) (token.Stored, error) {
	row, err := r.q.TokenByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return token.Stored{}, errs.NotFound("token")
		}
		return token.Stored{}, infraf("token: by prefix: %v", err)
	}
	return token.Stored{
		Token: tokenRow(db.TokensOfOwnerRow{
			ID: row.ID, Kind: row.Kind, Name: row.Name, Prefix: row.Prefix, TenantID: row.TenantID, Role: row.Role,
			OwnerID: row.OwnerID, ExpiresAt: row.ExpiresAt, LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt,
			TenantSlug: row.TenantSlug, TenantName: row.TenantName,
		}),
		SecretHash: row.SecretHash,
	}, nil
}

func (r *TokenRepo) OfOwner(ctx context.Context, ownerID uuid.UUID) ([]token.Token, error) {
	rows, err := r.q.TokensOfOwner(ctx, &ownerID)
	if err != nil {
		return nil, infraf("token: of owner: %v", err)
	}
	out := make([]token.Token, 0, len(rows))
	for _, row := range rows {
		out = append(out, tokenRow(row))
	}
	return out, nil
}

func (r *TokenRepo) ServiceOfTenant(ctx context.Context, tenantID uuid.UUID) ([]token.Token, error) {
	rows, err := r.q.ServiceTokensOfTenant(ctx, tenantID)
	if err != nil {
		return nil, infraf("token: of tenant: %v", err)
	}
	out := make([]token.Token, 0, len(rows))
	for _, row := range rows {
		out = append(out, tokenRow(db.TokensOfOwnerRow(row)))
	}
	return out, nil
}

func (r *TokenRepo) Revoke(ctx context.Context, id uuid.UUID, ownerID, tenantID *uuid.UUID) (bool, error) {
	n, err := r.q.RevokeToken(ctx, db.RevokeTokenParams{ID: id, OwnerID: ownerID, TenantID: tenantID})
	if err != nil {
		return false, infraf("token: revoke: %v", err)
	}
	return n > 0, nil
}

func (r *TokenRepo) RevokeOfMember(ctx context.Context, tenantID, ownerID uuid.UUID) error {
	if _, err := r.q.RevokeTokensOfMember(ctx, db.RevokeTokensOfMemberParams{TenantID: tenantID, OwnerID: &ownerID}); err != nil {
		return infraf("token: revoke of member: %v", err)
	}
	return nil
}

func (r *TokenRepo) Touch(ctx context.Context, id uuid.UUID) error {
	return r.q.TouchToken(ctx, id)
}

func tokenRow(row db.TokensOfOwnerRow) token.Token {
	return token.Token{
		ID: row.ID, Kind: token.Kind(row.Kind), Name: row.Name, Prefix: row.Prefix, TenantID: row.TenantID,
		TenantSlug: row.TenantSlug, TenantName: row.TenantName, Role: row.Role, OwnerID: row.OwnerID,
		ExpiresAt: row.ExpiresAt, LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt,
	}
}
