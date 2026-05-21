package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// Login authenticates by email or nickname and issues an access+refresh pair.
func (s *AuthService) Login(ctx context.Context, req *uipb.LoginRequest) (*uipb.LoginResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Login",
		func(ctx context.Context, _ trace.Span) (*uipb.LoginResponse, error) {
			acc, err := s.getAccountByLogin(ctx, req.GetEmail())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "lookup account: %v", err)
			}
			// Constant-time check: run bcrypt even when the account is absent so
			// the response time does not reveal account existence.
			var pwHash *string
			if acc != nil {
				h, err := s.getPasswordHash(ctx, acc.GetEntity().GetId().GetValue())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "lookup credentials: %v", err)
				}
				pwHash = &h
			}
			if !domainauth.VerifyPasswordConstantTime(pwHash, req.GetPassword()) {
				return nil, status.Error(codes.Unauthenticated, "invalid credentials")
			}
			pair, err := s.issueTokenPair(ctx, acc)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "issue tokens: %v", err)
			}
			return &uipb.LoginResponse{Tokens: pair}, nil
		})
}

// RefreshTokens rotates a valid refresh session into a new pair; a reused/rotated
// token is rejected (its session is already gone from Valkey).
func (s *AuthService) RefreshTokens(ctx context.Context, req *uipb.RefreshTokenRequest) (*uipb.RefreshTokenResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RefreshTokens",
		func(ctx context.Context, _ trace.Span) (*uipb.RefreshTokenResponse, error) {
			raw := req.GetRefreshToken()
			if raw == "" {
				return nil, status.Error(codes.Unauthenticated, "missing refresh token")
			}
			hash := domainauth.HashToken(raw)
			sess, err := s.getRefreshSession(ctx, hash)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "load session: %v", err)
			}
			if sess == nil {
				return nil, status.Error(codes.Unauthenticated, "invalid or rotated refresh token")
			}
			acc, err := s.getAccountByID(ctx, sess.AccountID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "load account: %v", err)
			}
			if acc == nil {
				return nil, status.Error(codes.Unauthenticated, "account no longer exists")
			}
			// Rotate: drop the consumed session before issuing the next pair.
			if err := s.deleteRefreshSession(ctx, hash); err != nil {
				return nil, status.Errorf(codes.Internal, "rotate session: %v", err)
			}
			pair, err := s.issueTokenPair(ctx, acc)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "issue tokens: %v", err)
			}
			return &uipb.RefreshTokenResponse{Tokens: pair}, nil
		})
}

// Logout revokes the refresh session so a later refresh with that token fails.
func (s *AuthService) Logout(ctx context.Context, req *uipb.LogoutRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Logout",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if raw := req.GetRefreshToken(); raw != "" {
				if err := s.deleteRefreshSession(ctx, domainauth.HashToken(raw)); err != nil {
					return nil, status.Errorf(codes.Internal, "revoke session: %v", err)
				}
			}
			return &emptypb.Empty{}, nil
		})
}

// Me returns the account behind the access token in context.
func (s *AuthService) Me(ctx context.Context, _ *emptypb.Empty) (*models.Account, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Me",
		func(ctx context.Context, _ trace.Span) (*models.Account, error) {
			c, ok := caller.FromContext(ctx)
			if !ok || c.Kind != caller.PrincipalAccount {
				return nil, status.Error(codes.Unauthenticated, "no account principal")
			}
			acc, err := s.getAccountByID(ctx, c.AccountID.GetValue())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "load account: %v", err)
			}
			if acc == nil {
				return nil, status.Error(codes.NotFound, "account not found")
			}
			return acc, nil
		})
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (s *AuthService) issueTokenPair(ctx context.Context, acc *models.Account) (*uipb.TokenPair, error) {
	accessTTL := s.cfg.AccessTokenTTL()
	refreshTTL := s.cfg.RefreshTokenTTL()
	accountID := acc.GetEntity().GetId().GetValue()

	accessToken, err := s.signer.SignAccount(accountID, acc.GetIsAdmin(), accessTTL)
	if err != nil {
		return nil, err
	}
	refreshToken, err := domainauth.GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}
	if err := s.saveRefreshSession(ctx, domainauth.HashToken(refreshToken),
		refreshSession{AccountID: accountID, IssuedAt: time.Now().Unix()}, refreshTTL); err != nil {
		return nil, err
	}
	return &uipb.TokenPair{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresIn:  durationpb.New(accessTTL),
		RefreshTokenExpiresIn: durationpb.New(refreshTTL),
	}, nil
}

// getAccountByLogin resolves an account by email, then by nickname (the request
// field accepts either). A missing account returns (nil, nil).
func (s *AuthService) getAccountByLogin(ctx context.Context, login string) (*models.Account, error) {
	acc, err := s.accountRepo.QueryRow(ctx,
		models.Accounts.SelectAll().Where(
			models.Accounts.Email.Eq(login),
			models.Accounts.DeletedAt.IsNull(),
		))
	if err == nil {
		return acc, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	acc, err = s.accountRepo.QueryRow(ctx,
		models.Accounts.SelectAll().Where(
			models.Accounts.Nickname.Eq(login),
			models.Accounts.DeletedAt.IsNull(),
		))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return acc, nil
}

func (s *AuthService) getAccountByID(ctx context.Context, id string) (*models.Account, error) {
	acc, err := s.accountRepo.QueryRow(ctx,
		models.Accounts.SelectAll().Where(
			models.Accounts.Id.Eq(id),
			models.Accounts.DeletedAt.IsNull(),
		))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return acc, nil
}

// getPasswordHash reads the write-only virtual password_hash column via the
// scanner repo (the Account proto does not carry it).
func (s *AuthService) getPasswordHash(ctx context.Context, accountID string) (string, error) {
	scanner, err := s.accountRepo.Scanner().QueryRow(ctx,
		models.Accounts.Select(models.AccountColumnPasswordHash).Where(
			models.Accounts.Id.Eq(accountID),
		))
	if err != nil {
		return "", err
	}
	return scanner.PasswordHash, nil
}
