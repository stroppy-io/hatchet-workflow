package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// RefreshSession is one server-side refresh-token record. It is single-use: a
// successful Rotate revokes the consumed id and the caller mints a new one.
type RefreshSession struct {
	// ID is the session id (the JWT "jti" of the refresh token).
	ID string
	// AccountID is the account the refresh token belongs to.
	AccountID string
	// ExpiresAt is when the refresh token stops being valid.
	ExpiresAt time.Time
}

// RefreshSessionStore is the consumer interface the token service needs to keep
// refresh sessions server-side so they can be single-use and revocable. The
// integration layer supplies an implementation (a gorm-backed store satisfies
// it); its methods MUST participate in the ambient ctx transaction. Get returns
// derrors.ErrNotFound for an unknown/consumed/revoked session.
type RefreshSessionStore interface {
	Create(ctx context.Context, s RefreshSession) error
	Get(ctx context.Context, id string) (RefreshSession, error)
	Delete(ctx context.Context, id string) error
}

// accessJWTClaims is the access-token claim set. It carries is_admin so the
// gate can short-circuit platform super-users without a DB lookup.
type accessJWTClaims struct {
	IsAdmin bool `json:"is_admin"`
	jwt.RegisteredClaims
}

// JWTTokenService implements both iamsvc.TokenService (mint/rotate/revoke the
// bearer pair) and iamsvc.TokenVerifier (validate an access token -> claims).
type JWTTokenService struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	sessions   RefreshSessionStore
}

var (
	_ iamsvc.TokenService  = (*JWTTokenService)(nil)
	_ iamsvc.TokenVerifier = (*JWTTokenService)(nil)
)

// NewJWTTokenService builds the JWT token service. It returns a typed
// FailedPrecondition error when the signing secret is empty (never panics, never
// signs with an empty key).
func NewJWTTokenService(cfg Config, sessions RefreshSessionStore) (*JWTTokenService, error) {
	if cfg.SigningSecret == "" {
		return nil, derrors.FailedPrecondition("identity.signing_secret_missing", "jwt signing secret is not configured")
	}
	if sessions == nil {
		return nil, derrors.FailedPrecondition("identity.refresh_store_missing", "refresh session store is not configured")
	}
	return &JWTTokenService{
		secret:     []byte(cfg.SigningSecret),
		issuer:     cfg.issuer(),
		accessTTL:  cfg.accessTTL(),
		refreshTTL: cfg.refreshTTL(),
		sessions:   sessions,
	}, nil
}

// IssuePair mints a fresh access+refresh pair. The refresh token is persisted as
// a single-use server-side session inside the ambient ctx transaction.
func (s *JWTTokenService) IssuePair(ctx context.Context, accountID string, isAdmin bool) (*api.TokenPair, error) {
	now := time.Now()

	access, err := s.signAccess(accountID, isAdmin, now)
	if err != nil {
		return nil, err
	}

	sessionID := uuid.NewString()
	refreshExp := now.Add(s.refreshTTL)
	refresh, err := s.signRefresh(accountID, sessionID, now, refreshExp)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.Create(ctx, RefreshSession{
		ID:        sessionID,
		AccountID: accountID,
		ExpiresAt: refreshExp,
	}); err != nil {
		return nil, err
	}

	return &api.TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresIn:  durationpb.New(s.accessTTL),
		RefreshExpiresIn: durationpb.New(s.refreshTTL),
	}, nil
}

// Rotate consumes a refresh token (single-use): it validates the signature,
// confirms the server-side session still exists, deletes it, and returns the
// owning account so the caller can re-mint a pair.
func (s *JWTTokenService) Rotate(ctx context.Context, refreshToken string) (string, error) {
	claims, err := s.parseRefresh(refreshToken)
	if err != nil {
		return "", derrors.Unauthenticated("invalid refresh token")
	}
	sess, err := s.sessions.Get(ctx, claims.ID)
	if errors.Is(err, derrors.ErrNotFound) {
		return "", derrors.Unauthenticated("refresh token already used or revoked")
	}
	if err != nil {
		return "", err
	}
	if err := s.sessions.Delete(ctx, sess.ID); err != nil {
		return "", err
	}
	return sess.AccountID, nil
}

// Revoke deletes the refresh session so the token can no longer be rotated.
// Revoking an unknown/already-consumed token is a no-op (idempotent).
func (s *JWTTokenService) Revoke(ctx context.Context, refreshToken string) error {
	claims, err := s.parseRefresh(refreshToken)
	if err != nil {
		// A malformed token has no session to revoke; treat as done.
		return nil
	}
	if err := s.sessions.Delete(ctx, claims.ID); err != nil {
		return derrors.IgnoreNotFound(err)
	}
	return nil
}

// Verify validates a bearer access token and reconstructs the AccessClaims.
func (s *JWTTokenService) Verify(_ context.Context, token string) (*iam.AccessClaims, error) {
	claims := &accessJWTClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, s.keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid {
		return nil, derrors.Unauthenticated("invalid access token")
	}
	if claims.Subject == "" {
		return nil, derrors.Unauthenticated("access token has no subject")
	}
	out := &iam.AccessClaims{
		AccountId: claims.Subject,
		IsAdmin:   claims.IsAdmin,
	}
	if claims.IssuedAt != nil {
		out.IssuedAt = timestamppb.New(claims.IssuedAt.Time)
	}
	if claims.ExpiresAt != nil {
		out.ExpiresAt = timestamppb.New(claims.ExpiresAt.Time)
	}
	return out, nil
}

func (s *JWTTokenService) signAccess(accountID string, isAdmin bool, now time.Time) (string, error) {
	claims := accessJWTClaims{
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   accountID,
			Issuer:    s.issuer,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *JWTTokenService) signRefresh(accountID, sessionID string, now, exp time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   accountID,
		Issuer:    s.issuer,
		ID:        sessionID,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *JWTTokenService) parseRefresh(token string) (*jwt.RegisteredClaims, error) {
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, s.keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid refresh token")
	}
	if claims.ID == "" {
		return nil, errors.New("refresh token has no session id")
	}
	return claims, nil
}

func (s *JWTTokenService) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
	}
	return s.secret, nil
}

// HashedApiTokenMinter implements iamsvc.ApiTokenMinter. It mints an opaque
// random secret of the form "<prefix>.<random>", stores the SHA-256 hash for
// verification, and returns the non-secret prefix for display/lookup.
type HashedApiTokenMinter struct {
	prefixBytes int
	secretBytes int
}

var _ iamsvc.ApiTokenMinter = (*HashedApiTokenMinter)(nil)

// NewApiTokenMinter builds the minter with sensible default entropy.
func NewApiTokenMinter() *HashedApiTokenMinter {
	return &HashedApiTokenMinter{prefixBytes: 6, secretBytes: 32}
}

// Mint generates a fresh token. secret is the full plaintext handed to the
// caller once; prefix is the non-secret lookup/display fragment; hash is the
// SHA-256 (hex) of the full secret, persisted for verification.
func (m *HashedApiTokenMinter) Mint() (secret, prefix, hash string, err error) {
	prefixRaw := make([]byte, m.prefixBytes)
	if _, err = rand.Read(prefixRaw); err != nil {
		return "", "", "", err
	}
	secretRaw := make([]byte, m.secretBytes)
	if _, err = rand.Read(secretRaw); err != nil {
		return "", "", "", err
	}
	prefix = "stp_" + base64.RawURLEncoding.EncodeToString(prefixRaw)
	secret = prefix + "." + base64.RawURLEncoding.EncodeToString(secretRaw)
	sum := sha256.Sum256([]byte(secret))
	hash = hex.EncodeToString(sum[:])
	return secret, prefix, hash, nil
}
