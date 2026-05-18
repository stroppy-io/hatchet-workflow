package iam

import (
	"context"
	"errors"
	"time"

	"github.com/avito-tech/go-transaction-manager/trm"
	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/dml/set"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Login validates credentials and returns a fresh TokenPair starting a new rotation family.
func (s *Service) Login(ctx context.Context, email, password string) (*iampb.TokenPair, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "Login",
		func(ctx context.Context, _ trace.Span) (*iampb.TokenPair, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.TokenPair, error) {
					// Query via scanner to access virtual PasswordHash field.
					userScanner, err := s.userRepo.Scanner().QueryRow(ctx,
						iampb.Users.SelectAll().Where(
							iampb.Users.Email.Eq(email),
							iampb.Users.DeletedAt.IsNull(),
						),
					)
					if err != nil {
						if errors.Is(err, pgx.ErrNoRows) {
							return nil, domainerr.Unauthenticated()
						}
						return nil, err
					}
					if !verifyPassword(userScanner.PasswordHash, password) {
						return nil, domainerr.Unauthenticated()
					}
					userID := &iampb.UserId{Value: userScanner.Id}
					pair, _, err := s.issuePair(ctx, userID, ids.New() /* new family */)
					return pair, err
				})
		})
}

// RefreshTokens rotates a refresh token; reuse of a revoked token revokes the whole family.
//
// Algorithm: atomically attempt to revoke the token within the same tx by issuing
// UPDATE ... WHERE token_hash = ? AND revoked_at IS NULL.
//   - 1 row affected → token was valid and is now revoked; issue new pair and
//     link via replaced_by + revocation_reason=ROTATED.
//   - 0 rows affected → token unknown or already revoked; detect reuse and
//     revoke the whole family with revocation_reason=REUSED.
func (s *Service) RefreshTokens(ctx context.Context, refreshTokenValue string) (*iampb.TokenPair, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "RefreshTokens",
		func(ctx context.Context, _ trace.Span) (*iampb.TokenPair, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.TokenPair, error) {
					tokHash := sha256Hex(refreshTokenValue)
					now := time.Now()

					// Atomically revoke: only matches if not yet revoked and within same tx.
					affected, err := s.refreshRepo.Execute(ctx,
						iampb.RefreshTokens.Update().
							Set(
								iampb.RefreshTokens.RevokedAt.Set(&now),
								set.NewSetter[iampb.RefreshTokenColumnAlias](
									iampb.RefreshTokenColumnRevocationReason,
									iampb.RevocationReason_REVOCATION_REASON_ROTATED.String(),
								),
							).
							Where(
								iampb.RefreshTokens.TokenHash.Eq(tokHash),
								iampb.RefreshTokens.RevokedAt.IsNull(),
							),
					)
					if err != nil {
						return nil, err
					}

					if affected == 0 {
						// Token not found or already revoked — reuse detection.
						// Look up by hash to get the family for revocation.
						tokScanner, err := s.refreshRepo.Scanner().QueryRow(ctx,
							iampb.RefreshTokens.SelectAll().Where(
								iampb.RefreshTokens.TokenHash.Eq(tokHash),
							),
						)
						if err != nil {
							// Token completely unknown — just reject (no family to kill).
							return nil, domainerr.Unauthenticated()
						}
						// Token was already revoked → reuse detected; kill family.
						// Use trm.Skippable so the tx COMMITS the family revocation
						// even though we return an error to the caller.
						_ = s.revokeFamily(ctx, tokScanner.FamilyId, iampb.RevocationReason_REVOCATION_REASON_REUSED)
						return nil, trm.Skippable(domainerr.Unauthenticated())
					}

					// Fetch the row we just revoked to get userId, familyId, expiresAt.
					oldRow, err := s.refreshRepo.Scanner().QueryRow(ctx,
						iampb.RefreshTokens.SelectAll().Where(
							iampb.RefreshTokens.TokenHash.Eq(tokHash),
						),
					)
					if err != nil {
						return nil, err
					}
					if oldRow.ExpiresAt.Before(now) {
						return nil, domainerr.Unauthenticated()
					}

					userID := &iampb.UserId{Value: oldRow.UserId}
					pair, newID, err := s.issuePair(ctx, userID, oldRow.FamilyId)
					if err != nil {
						return nil, err
					}

					// Link the rotation chain: old.replaced_by = new.id.
					if _, err := s.refreshRepo.Execute(ctx,
						iampb.RefreshTokens.Update().
							Set(iampb.RefreshTokens.ReplacedBy.Set(&newID)).
							Where(iampb.RefreshTokens.Id.Eq(oldRow.Id)),
					); err != nil {
						return nil, err
					}
					return pair, nil
				})
		})
}

// Logout revokes the refresh token with reason=LOGOUT.
func (s *Service) Logout(ctx context.Context, refreshTokenValue string) error {
	now := time.Now()
	_, err := s.refreshRepo.Execute(ctx,
		iampb.RefreshTokens.Update().
			Set(
				iampb.RefreshTokens.RevokedAt.Set(&now),
				set.NewSetter[iampb.RefreshTokenColumnAlias](
					iampb.RefreshTokenColumnRevocationReason,
					iampb.RevocationReason_REVOCATION_REASON_LOGOUT.String(),
				),
			).
			Where(
				iampb.RefreshTokens.TokenHash.Eq(sha256Hex(refreshTokenValue)),
				iampb.RefreshTokens.RevokedAt.IsNull(),
			),
	)
	return err
}

// issuePair issues access + refresh tokens; refresh is stored hashed.
// Returns the pair and the id of the persisted refresh-token row.
func (s *Service) issuePair(ctx context.Context, userID *iampb.UserId, familyID string) (*iampb.TokenPair, string, error) {
	now := time.Now()
	jti := ids.New()

	// Embed platform_role into the access token so the admin middleware can gate
	// admin-only endpoints without an extra DB round-trip.
	platformRole := ""
	if user, err := s.GetUserByID(ctx, userID); err == nil {
		platformRole = user.GetPlatformRole().String()
	}

	access, err := s.signAccessToken(userID.GetValue(), jti, platformRole)
	if err != nil {
		return nil, "", err
	}
	refreshValue := ids.New() + ids.New() // 52 chars opaque
	rowID := ids.New()
	row := &iampb.RefreshToken{
		Id:        &iampb.RefreshTokenId{Value: rowID},
		UserId:    userID,
		FamilyId:  familyID,
		Jti:       jti,
		TokenHash: sha256Hex(refreshValue),
		ExpiresAt: timestamppb.New(now.Add(s.cfg.RefreshTTL)),
		Timestamps: &commonpb.Timestamps{
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
	}
	scanner := row.IntoPlain()
	if _, err := s.refreshRepo.Execute(ctx,
		iampb.RefreshTokens.Insert().From(scanner.AllSetters()...),
	); err != nil {
		return nil, "", err
	}
	return &iampb.TokenPair{
		AccessToken:           access,
		RefreshToken:          refreshValue,
		AccessTokenExpiresIn:  durationpb.New(s.cfg.AccessTTL),
		RefreshTokenExpiresIn: durationpb.New(s.cfg.RefreshTTL),
	}, rowID, nil
}

// revokeFamily marks every active token in a family revoked with the given reason.
func (s *Service) revokeFamily(ctx context.Context, familyID string, reason iampb.RevocationReason) error {
	now := time.Now()
	_, err := s.refreshRepo.Execute(ctx,
		iampb.RefreshTokens.Update().
			Set(
				iampb.RefreshTokens.RevokedAt.Set(&now),
				set.NewSetter[iampb.RefreshTokenColumnAlias](
					iampb.RefreshTokenColumnRevocationReason,
					reason.String(),
				),
			).
			Where(
				iampb.RefreshTokens.FamilyId.Eq(familyID),
				iampb.RefreshTokens.RevokedAt.IsNull(),
			),
	)
	return err
}
