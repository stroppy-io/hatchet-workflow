package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestApiTokenVerifierAcceptsPersonalToken(t *testing.T) {
	secret := "stp_prefix.secret"
	store := &fakeApiTokenStore{token: &iam.ApiToken{
		Id:        "token-1",
		AccountId: "account-1",
		Type:      iam.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
		Prefix:    "stp_prefix",
		CreatedAt: timestamppb.New(time.Unix(10, 0)),
	}}
	verifier := newApiTokenVerifierTest(secret, store, &iam.Account{Id: "account-1", IsAdmin: true})
	verifier.now = func() time.Time { return time.Unix(100, 0) }

	claims, permissions, restricted, err := verifier.VerifyWithRestrictions(context.Background(), secret)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.GetAccountId() != "account-1" || !claims.GetIsAdmin() {
		t.Fatalf("claims = (%q, %v), want account-1 admin", claims.GetAccountId(), claims.GetIsAdmin())
	}
	if restricted {
		t.Fatal("personal token was marked restricted")
	}
	if len(permissions) != 0 {
		t.Fatalf("permissions = %d, want 0", len(permissions))
	}
	if store.touchedID != "token-1" || !store.touchedAt.Equal(time.Unix(100, 0)) {
		t.Fatalf("touch = (%q, %s), want token-1 at now", store.touchedID, store.touchedAt)
	}
}

func TestApiTokenVerifierReturnsServiceTokenRestrictions(t *testing.T) {
	secret := "stp_prefix.secret"
	expected := []*iam.Permission{
		{Resource: iam.Resource_RESOURCE_TEST_RUN, Action: iam.Action_ACTION_READ},
	}
	verifier := newApiTokenVerifierTest(secret, &fakeApiTokenStore{token: &iam.ApiToken{
		Id:          "token-1",
		AccountId:   "account-1",
		Type:        iam.ApiTokenType_API_TOKEN_TYPE_SERVICE,
		Prefix:      "stp_prefix",
		Permissions: expected,
	}}, &iam.Account{Id: "account-1", IsAdmin: true})

	claims, permissions, restricted, err := verifier.VerifyWithRestrictions(context.Background(), secret)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.GetAccountId() != "account-1" {
		t.Fatalf("account_id = %q, want account-1", claims.GetAccountId())
	}
	if !restricted {
		t.Fatal("service token was not marked restricted")
	}
	if len(permissions) != 1 || permissions[0].GetAction() != iam.Action_ACTION_READ {
		t.Fatalf("permissions = %v, want service grant", permissions)
	}
}

func TestApiTokenVerifierRejectsWrongSecret(t *testing.T) {
	secret := "stp_prefix.secret"
	store := &fakeApiTokenStore{token: &iam.ApiToken{
		Id:        "token-1",
		AccountId: "account-1",
		Type:      iam.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
		Prefix:    "stp_prefix",
	}}
	verifier := newApiTokenVerifierTest(secret, store, &iam.Account{Id: "account-1"})

	if _, _, _, err := verifier.VerifyWithRestrictions(context.Background(), "stp_prefix.other"); err == nil {
		t.Fatal("wrong secret was accepted")
	}
	if store.touchedID != "" {
		t.Fatalf("wrong secret touched last_used_at for %q", store.touchedID)
	}
}

func TestApiTokenVerifierRejectsExpiredToken(t *testing.T) {
	secret := "stp_prefix.secret"
	store := &fakeApiTokenStore{token: &iam.ApiToken{
		Id:        "token-1",
		AccountId: "account-1",
		Type:      iam.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
		Prefix:    "stp_prefix",
		ExpiresAt: timestamppb.New(time.Unix(100, 0)),
	}}
	verifier := newApiTokenVerifierTest(secret, store, &iam.Account{Id: "account-1"})
	verifier.now = func() time.Time { return time.Unix(100, 0) }

	if _, _, _, err := verifier.VerifyWithRestrictions(context.Background(), secret); err == nil {
		t.Fatal("expired token was accepted")
	}
	if store.touchedID != "" {
		t.Fatalf("expired token touched last_used_at for %q", store.touchedID)
	}
}

func TestApiTokenVerifierThrottlesLastUsedTouch(t *testing.T) {
	secret := "stp_prefix.secret"
	store := &fakeApiTokenStore{token: &iam.ApiToken{
		Id:         "token-1",
		AccountId:  "account-1",
		Type:       iam.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
		Prefix:     "stp_prefix",
		LastUsedAt: timestamppb.New(time.Unix(90, 0)),
	}}
	verifier := newApiTokenVerifierTest(secret, store, &iam.Account{Id: "account-1"})
	verifier.now = func() time.Time { return time.Unix(100, 0) }

	if _, _, _, err := verifier.VerifyWithRestrictions(context.Background(), secret); err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if store.touchedID != "" {
		t.Fatalf("recent token touched last_used_at for %q", store.touchedID)
	}
}

func newApiTokenVerifierTest(secret string, store *fakeApiTokenStore, account *iam.Account) *ApiTokenVerifier {
	return NewApiTokenVerifier(
		store,
		fakeApiTokenAccountStore{account: account},
		fakeApiTokenHashStore{hash: hashApiToken(secret)},
	)
}

func hashApiToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

type fakeApiTokenStore struct {
	token     *iam.ApiToken
	touchedID string
	touchedAt time.Time
}

func (f *fakeApiTokenStore) GetByPrefix(_ context.Context, prefix string) (*iam.ApiToken, error) {
	if f.token != nil && f.token.GetPrefix() == prefix {
		return f.token, nil
	}
	return nil, errApiTokenNotFound{}
}

func (f *fakeApiTokenStore) TouchLastUsed(_ context.Context, tokenID string, at time.Time) error {
	f.touchedID = tokenID
	f.touchedAt = at
	return nil
}

type fakeApiTokenAccountStore struct {
	account *iam.Account
}

func (f fakeApiTokenAccountStore) Get(_ context.Context, id string) (*iam.Account, error) {
	if f.account != nil && f.account.GetId() == id {
		return f.account, nil
	}
	return nil, errApiTokenNotFound{}
}

type fakeApiTokenHashStore struct {
	hash string
}

func (f fakeApiTokenHashStore) GetHash(context.Context, string) (string, error) {
	return f.hash, nil
}

type errApiTokenNotFound struct{}

func (errApiTokenNotFound) Error() string { return "not found" }
