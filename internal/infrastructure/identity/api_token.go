package identity

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// ApiTokenStore locates API-token metadata by its non-secret prefix.
type ApiTokenStore interface {
	GetByPrefix(ctx context.Context, prefix string) (*iam.ApiToken, error)
}

// ApiTokenUsageStore records successful token use. It is best-effort metadata,
// not part of the authentication decision.
type ApiTokenUsageStore interface {
	TouchLastUsed(ctx context.Context, tokenID string, at time.Time) error
}

// ApiTokenAccountStore loads the token owner so personal tokens inherit the
// account's current admin flag and service tokens still identify a live account.
type ApiTokenAccountStore interface {
	Get(ctx context.Context, id string) (*iam.Account, error)
}

type apiTokenHashStore interface {
	GetHash(ctx context.Context, tokenID string) (string, error)
}

// ApiTokenVerifier validates opaque API tokens minted by HashedApiTokenMinter.
// It returns a restricted permission set for SERVICE tokens; the IAM auth gate
// applies that cap in addition to the owner's live RBAC permissions.
type ApiTokenVerifier struct {
	tokens   ApiTokenStore
	accounts ApiTokenAccountStore
	secrets  apiTokenHashStore
	now      func() time.Time
}

var (
	_ iamsvc.TokenVerifier           = (*ApiTokenVerifier)(nil)
	_ iamsvc.RestrictedTokenVerifier = (*ApiTokenVerifier)(nil)
)

func NewApiTokenVerifier(tokens ApiTokenStore, accounts ApiTokenAccountStore, secrets apiTokenHashStore) *ApiTokenVerifier {
	return &ApiTokenVerifier{tokens: tokens, accounts: accounts, secrets: secrets, now: time.Now}
}

const apiTokenLastUsedTouchInterval = time.Minute

func (v *ApiTokenVerifier) Verify(ctx context.Context, token string) (*iam.AccessClaims, error) {
	claims, _, _, err := v.VerifyWithRestrictions(ctx, token)
	return claims, err
}

func (v *ApiTokenVerifier) VerifyWithRestrictions(ctx context.Context, token string) (*iam.AccessClaims, []*iam.Permission, bool, error) {
	prefix, _, ok := strings.Cut(token, ".")
	if !ok || prefix == "" {
		return nil, nil, false, derrors.Unauthenticated("invalid api token")
	}
	now := v.now()
	meta, err := v.tokens.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, nil, false, derrors.Unauthenticated("invalid api token").Wrap(err)
	}
	if meta.GetExpiresAt() != nil && !now.Before(meta.GetExpiresAt().AsTime()) {
		return nil, nil, false, derrors.Unauthenticated("api token expired")
	}
	storedHash, err := v.secrets.GetHash(ctx, meta.GetId())
	if err != nil {
		return nil, nil, false, derrors.Unauthenticated("invalid api token").Wrap(err)
	}
	if !apiTokenHashMatches(token, storedHash) {
		return nil, nil, false, derrors.Unauthenticated("invalid api token")
	}
	account, err := v.accounts.Get(ctx, meta.GetAccountId())
	if err != nil {
		return nil, nil, false, derrors.Unauthenticated("api token owner not found").Wrap(err)
	}
	v.touchLastUsed(ctx, meta, now)
	claims := &iam.AccessClaims{
		AccountId: account.GetId(),
		IsAdmin:   account.GetIsAdmin(),
		IssuedAt:  meta.GetCreatedAt(),
	}
	if meta.GetExpiresAt() != nil {
		claims.ExpiresAt = timestamppb.New(meta.GetExpiresAt().AsTime())
	}
	switch meta.GetType() {
	case iam.ApiTokenType_API_TOKEN_TYPE_PERSONAL:
		return claims, nil, false, nil
	case iam.ApiTokenType_API_TOKEN_TYPE_SERVICE:
		return claims, meta.GetPermissions(), true, nil
	default:
		return nil, nil, false, derrors.Unauthenticated("invalid api token type")
	}
}

func (v *ApiTokenVerifier) touchLastUsed(ctx context.Context, meta *iam.ApiToken, now time.Time) {
	store, ok := v.tokens.(ApiTokenUsageStore)
	if !ok {
		return
	}
	lastUsed := meta.GetLastUsedAt()
	if lastUsed != nil && now.Sub(lastUsed.AsTime()) < apiTokenLastUsedTouchInterval {
		return
	}
	_ = store.TouchLastUsed(ctx, meta.GetId(), now)
}

func apiTokenHashMatches(token, storedHash string) bool {
	sum := sha256.Sum256([]byte(token))
	actual := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(storedHash)) == 1
}

// CompositeTokenVerifier tries multiple bearer credential formats in order. A
// JWT access token remains the fast/default path; opaque API tokens are the
// fallback for programmatic callers.
type CompositeTokenVerifier struct {
	verifiers []iamsvc.TokenVerifier
}

var (
	_ iamsvc.TokenVerifier           = (*CompositeTokenVerifier)(nil)
	_ iamsvc.RestrictedTokenVerifier = (*CompositeTokenVerifier)(nil)
)

func NewCompositeTokenVerifier(verifiers ...iamsvc.TokenVerifier) *CompositeTokenVerifier {
	return &CompositeTokenVerifier{verifiers: verifiers}
}

func (v *CompositeTokenVerifier) Verify(ctx context.Context, token string) (*iam.AccessClaims, error) {
	claims, _, _, err := v.VerifyWithRestrictions(ctx, token)
	return claims, err
}

func (v *CompositeTokenVerifier) VerifyWithRestrictions(ctx context.Context, token string) (*iam.AccessClaims, []*iam.Permission, bool, error) {
	var last error
	for _, verifier := range v.verifiers {
		if verifier == nil {
			continue
		}
		if restricted, ok := verifier.(iamsvc.RestrictedTokenVerifier); ok {
			claims, permissions, isRestricted, err := restricted.VerifyWithRestrictions(ctx, token)
			if err == nil {
				return claims, permissions, isRestricted, nil
			}
			last = err
			continue
		}
		claims, err := verifier.Verify(ctx, token)
		if err == nil {
			return claims, nil, false, nil
		}
		last = err
	}
	if last != nil {
		return nil, nil, false, last
	}
	return nil, nil, false, derrors.Unauthenticated("invalid bearer token")
}
