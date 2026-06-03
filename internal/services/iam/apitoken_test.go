package iam

import (
	"context"
	"testing"
	"time"

	trm "github.com/avito-tech/go-transaction-manager/trm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestCreateServiceApiTokenRejectsPermissionsOwnerDoesNotHold(t *testing.T) {
	tokens := &fakeApiTokenRepo{}
	secrets := &fakeApiTokenSecrets{}
	svc := newApiTokenTestService(tokens, secrets, []*iampb.Permission{
		{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_READ},
	})

	_, err := svc.CreateApiToken(context.Background(), &api.CreateApiTokenRequest{
		AccountId: "account-1",
		Name:      "ci",
		Type:      iampb.ApiTokenType_API_TOKEN_TYPE_SERVICE,
		Permissions: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_DELETE},
		},
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.PermissionDenied, err)
	}
	if tokens.created != nil {
		t.Fatalf("token was created despite permission rejection")
	}
	if secrets.set {
		t.Fatalf("token secret was stored despite permission rejection")
	}
}

func TestCreateServiceApiTokenAllowsHeldPermission(t *testing.T) {
	tokens := &fakeApiTokenRepo{}
	secrets := &fakeApiTokenSecrets{}
	svc := newApiTokenTestService(tokens, secrets, []*iampb.Permission{
		{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_MANAGE},
	})

	resp, err := svc.CreateApiToken(context.Background(), &api.CreateApiTokenRequest{
		AccountId: "account-1",
		Name:      "ci",
		Type:      iampb.ApiTokenType_API_TOKEN_TYPE_SERVICE,
		Permissions: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_DELETE},
		},
	})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if resp.GetSecret() != "secret" {
		t.Fatalf("secret = %q, want secret", resp.GetSecret())
	}
	if tokens.created == nil || tokens.created.GetPrefix() != "prefix" {
		t.Fatalf("created token prefix = %q, want prefix", tokens.created.GetPrefix())
	}
	if !secrets.set || secrets.tokenID != tokens.created.GetId() || secrets.hash != "hash" {
		t.Fatalf("stored secret = (%v, %q, %q), want created token hash", secrets.set, secrets.tokenID, secrets.hash)
	}
}

func TestCreateServiceApiTokenRejectsUndefinedPermission(t *testing.T) {
	svc := newApiTokenTestService(&fakeApiTokenRepo{}, &fakeApiTokenSecrets{}, nil)

	_, err := svc.CreateApiToken(context.Background(), &api.CreateApiTokenRequest{
		AccountId: "account-1",
		Name:      "ci",
		Type:      iampb.ApiTokenType_API_TOKEN_TYPE_SERVICE,
		Permissions: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_UNSPECIFIED},
		},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestCreatePersonalApiTokenStillRequiresTTL(t *testing.T) {
	svc := newApiTokenTestService(&fakeApiTokenRepo{}, &fakeApiTokenSecrets{}, nil)

	_, err := svc.CreateApiToken(context.Background(), &api.CreateApiTokenRequest{
		AccountId: "account-1",
		Name:      "human",
		Type:      iampb.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.InvalidArgument, err)
	}

	_, err = svc.CreateApiToken(context.Background(), &api.CreateApiTokenRequest{
		AccountId: "account-1",
		Name:      "human",
		Type:      iampb.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
		Ttl:       durationpb.New(time.Hour),
	})
	if err != nil {
		t.Fatalf("create personal token with ttl: %v", err)
	}
}

func newApiTokenTestService(tokens *fakeApiTokenRepo, secrets *fakeApiTokenSecrets, granted []*iampb.Permission) *IamService {
	return NewIamService(IamDeps{
		Authn: fakeIamAuthn{claims: &iampb.AccessClaims{AccountId: "account-1"}},
		Accounts: fakeAccountRepo{account: &iampb.Account{
			Id: "account-1",
		}},
		Tenants: fakeTenantRepo{tenants: []*iampb.Tenant{
			tenant("tenant-1", "acme"),
		}},
		Authz:           fakeApiTokenAuthz{granted: granted},
		ApiTokens:       tokens,
		ApiTokenSecrets: secrets,
		ApiTokenMinter:  fakeApiTokenMinter{},
		Tx:              noopTrm{},
	})
}

type fakeApiTokenAuthz struct {
	granted []*iampb.Permission
}

func (f fakeApiTokenAuthz) EffectivePermissions(context.Context, string, string) ([]*iampb.Permission, error) {
	return f.granted, nil
}

type fakeApiTokenRepo struct {
	created *iampb.ApiToken
}

func (f *fakeApiTokenRepo) Create(_ context.Context, token *iampb.ApiToken) error {
	f.created = token
	return nil
}
func (f *fakeApiTokenRepo) Get(context.Context, string) (*iampb.ApiToken, error) { return nil, nil }
func (f *fakeApiTokenRepo) GetByPrefix(context.Context, string) (*iampb.ApiToken, error) {
	return nil, nil
}
func (f *fakeApiTokenRepo) ListByAccount(context.Context, string) ([]*iampb.ApiToken, error) {
	return nil, nil
}
func (f *fakeApiTokenRepo) Delete(context.Context, string) error { return nil }

type fakeApiTokenSecrets struct {
	set     bool
	tokenID string
	hash    string
}

func (f *fakeApiTokenSecrets) SetHash(_ context.Context, tokenID, hash string) error {
	f.set = true
	f.tokenID = tokenID
	f.hash = hash
	return nil
}
func (f *fakeApiTokenSecrets) GetHash(context.Context, string) (string, error) { return "", nil }
func (f *fakeApiTokenSecrets) Delete(context.Context, string) error            { return nil }

type fakeApiTokenMinter struct{}

func (fakeApiTokenMinter) Mint() (string, string, string, error) {
	return "secret", "prefix", "hash", nil
}

type noopTrm struct{}

func (noopTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (noopTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
