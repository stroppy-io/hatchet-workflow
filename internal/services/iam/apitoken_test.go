package iam

// apitoken_test.go: tests for API token management in apitoken.go:
// CreateApiToken, ListApiTokens, RevokeApiToken.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// -------- CreateApiToken --------

func TestCreateApiToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := NewMockAuthn(ctrl)
	tokens := NewMockApiTokenRepo(ctrl)
	secrets := NewMockApiTokenSecrets(ctrl)
	minter := NewMockApiTokenMinter(ctrl)
	accounts := NewMockAccountRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx: &MockTrm{}, Clock: FakeClock{},
		Authn: authn, ApiTokens: tokens, ApiTokenSecrets: secrets, ApiTokenMinter: minter, Accounts: accounts,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		req := &api.CreateApiTokenRequest{AccountId: "a1", Name: "my token"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		minter.EXPECT().Mint().Return("secret", "prefix1", "hash", nil)
		tokens.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, token *iampb.ApiToken) error {
			if token.AccountId != "a1" {
				t.Errorf("expected account id a1, got %s", token.AccountId)
			}
			return nil
		})
		secrets.EXPECT().SetHash(ctx, gomock.Any(), "hash").Return(nil)

		resp, err := svc.CreateApiToken(ctx, req)
		if err != nil {
			t.Fatalf("CreateApiToken: %v", err)
		}
		if resp.Secret != "secret" {
			t.Errorf("expected secret, got %s", resp.Secret)
		}
	})

	t.Run("NotSelfOrAdmin_Denied", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a2"}, nil)
		_, err := svc.CreateApiToken(ctx, &api.CreateApiTokenRequest{AccountId: "a1"})
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("PersonalToken_WithPermissions_Invalid", func(t *testing.T) {
		req := &api.CreateApiTokenRequest{
			AccountId:   "a1",
			Type:        iampb.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
			Permissions: []*iampb.Permission{{Resource: iampb.Resource_RESOURCE_ACCOUNT, Action: iampb.Action_ACTION_READ}},
			Ttl:         durationpb.New(3600000000000), // 1 hour
		}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		_, err := svc.CreateApiToken(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("PersonalToken_NoTTL_Invalid", func(t *testing.T) {
		req := &api.CreateApiTokenRequest{
			AccountId: "a1",
			Type:      iampb.ApiTokenType_API_TOKEN_TYPE_PERSONAL,
			Ttl:       &durationpb.Duration{Seconds: 0},
		}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		_, err := svc.CreateApiToken(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("AccountGetError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.CreateApiToken(ctx, &api.CreateApiTokenRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("MintError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		minter.EXPECT().Mint().Return("", "", "", errors.New("mint err"))
		_, err := svc.CreateApiToken(ctx, &api.CreateApiTokenRequest{AccountId: "a1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("CreateTokenError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		minter.EXPECT().Mint().Return("s", "p", "h", nil)
		tokens.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.CreateApiToken(ctx, &api.CreateApiTokenRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("SetHashError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		minter.EXPECT().Mint().Return("s", "p", "h", nil)
		tokens.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		secrets.EXPECT().SetHash(ctx, gomock.Any(), "h").Return(errors.New("vault err"))
		_, err := svc.CreateApiToken(ctx, &api.CreateApiTokenRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("WithTTL", func(t *testing.T) {
		req := &api.CreateApiTokenRequest{
			AccountId: "a1",
			Name:      "expiring token",
			Ttl:       durationpb.New(3600000000000), // 1 hour
		}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		minter.EXPECT().Mint().Return("secret", "p", "h", nil)
		tokens.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, token *iampb.ApiToken) error {
			if token.ExpiresAt == nil {
				t.Error("expected ExpiresAt to be set")
			}
			return nil
		})
		secrets.EXPECT().SetHash(ctx, gomock.Any(), "h").Return(nil)
		_, err := svc.CreateApiToken(ctx, req)
		if err != nil {
			t.Fatalf("CreateApiToken with TTL: %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.CreateApiToken(ctx, &api.CreateApiTokenRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ListApiTokens --------

func TestListApiTokens(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := NewMockAuthn(ctrl)
	tokens := NewMockApiTokenRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &MockTrm{}, Clock: FakeClock{}, Authn: authn, ApiTokens: tokens})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tokens.EXPECT().ListByAccount(ctx, "a1").Return([]*iampb.ApiToken{
			{Id: "t1"}, {Id: "t2"},
		}, nil)
		resp, err := svc.ListApiTokens(ctx, &api.ListApiTokensRequest{AccountId: "a1"})
		if err != nil {
			t.Fatalf("ListApiTokens: %v", err)
		}
		if len(resp.Tokens) != 2 {
			t.Errorf("expected 2 tokens, got %d", len(resp.Tokens))
		}
	})

	t.Run("NotSelfOrAdmin_Denied", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a2"}, nil)
		_, err := svc.ListApiTokens(ctx, &api.ListApiTokensRequest{AccountId: "a1"})
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("ListError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tokens.EXPECT().ListByAccount(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.ListApiTokens(ctx, &api.ListApiTokensRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.ListApiTokens(ctx, &api.ListApiTokensRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- RevokeApiToken --------

func TestRevokeApiToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := NewMockAuthn(ctrl)
	tokens := NewMockApiTokenRepo(ctrl)
	secrets := NewMockApiTokenSecrets(ctrl)
	svc := NewIamService(IamDeps{
		Tx: &MockTrm{}, Clock: FakeClock{},
		Authn: authn, ApiTokens: tokens, ApiTokenSecrets: secrets,
	})
	ctx := context.Background()
	req := &api.RevokeApiTokenRequest{Id: "t1"}

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tokens.EXPECT().Get(ctx, "t1").Return(&iampb.ApiToken{Id: "t1", AccountId: "a1"}, nil)
		secrets.EXPECT().Delete(ctx, "t1").Return(nil)
		tokens.EXPECT().Delete(ctx, "t1").Return(nil)
		_, err := svc.RevokeApiToken(ctx, req)
		if err != nil {
			t.Fatalf("RevokeApiToken: %v", err)
		}
	})

	t.Run("TokenNotFound_Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tokens.EXPECT().Get(ctx, "t1").Return(nil, derrors.ErrNotFound)
		_, err := svc.RevokeApiToken(ctx, req)
		if err != nil {
			t.Fatalf("RevokeApiToken not found: %v", err)
		}
	})

	t.Run("GetTokenError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tokens.EXPECT().Get(ctx, "t1").Return(nil, errors.New("db err"))
		_, err := svc.RevokeApiToken(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("NotOwner_PermissionDenied", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a2"}, nil)
		tokens.EXPECT().Get(ctx, "t1").Return(&iampb.ApiToken{Id: "t1", AccountId: "a1"}, nil)
		_, err := svc.RevokeApiToken(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.RevokeApiToken(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("Admin_CanRevoke_Others", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "admin", IsAdmin: true}, nil)
		tokens.EXPECT().Get(ctx, "t1").Return(&iampb.ApiToken{Id: "t1", AccountId: "a1"}, nil)
		secrets.EXPECT().Delete(ctx, "t1").Return(nil)
		tokens.EXPECT().Delete(ctx, "t1").Return(nil)
		_, err := svc.RevokeApiToken(ctx, req)
		if err != nil {
			t.Fatalf("RevokeApiToken admin: %v", err)
		}
	})
}
