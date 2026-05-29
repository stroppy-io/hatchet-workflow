package iam

// account_test.go: tests for account management operations in service.go:
// CreateAccount, GetAccount, GetMyAccount, ListAccounts, UpdateAccount, DeleteAccount.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

func strPtr(s string) *string { return &s }

// -------- CreateAccount --------

func TestCreateAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	credentials := NewMockCredentialStore(ctrl)
	identities := NewMockExternalIdentityRepo(ctrl)
	hasher := NewMockPasswordHasher(ctrl)

	svc := NewIamService(IamDeps{
		Tx:       &utils.MockTrm{},
		Accounts: accounts, Credentials: credentials, ExternalIdentities: identities, Hasher: hasher,
	})
	ctx := context.Background()

	t.Run("Success_WithPassword", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "new@example.com", Nickname: "newuser", Password: strPtr("pw"),
		}
		accounts.EXPECT().GetByEmail(ctx, "new@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, "newuser").Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash("pw").Return("hash", nil)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		credentials.EXPECT().SetPassword(ctx, gomock.Any(), "hash").Return(nil)

		resp, err := svc.CreateAccount(ctx, req)
		if err != nil {
			t.Fatalf("CreateAccount failed: %v", err)
		}
		if resp.Account.Email != "new@example.com" {
			t.Errorf("expected new@example.com, got %s", resp.Account.Email)
		}
	})

	t.Run("Success_WithLink", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "linked@example.com", Nickname: "linkeduser",
			Link: &api.ExternalIdentityLink{ProviderId: "p1", Subject: "s1"},
		}
		accounts.EXPECT().GetByEmail(ctx, "linked@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, "linkeduser").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "s1").Return(nil, derrors.ErrNotFound)
		identities.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CreateAccount(ctx, req)
		if err != nil {
			t.Fatalf("CreateAccount with link failed: %v", err)
		}
		if resp.Account.Email != "linked@example.com" {
			t.Errorf("expected linked@example.com, got %s", resp.Account.Email)
		}
	})

	t.Run("NoPasswordNoLink", func(t *testing.T) {
		req := &api.CreateAccountRequest{Email: "x@example.com", Nickname: "x"}
		_, err := svc.CreateAccount(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("EmailAlreadyExists", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "exists@example.com", Nickname: "user", Password: strPtr("pw"),
		}
		accounts.EXPECT().GetByEmail(ctx, "exists@example.com").Return(&iampb.Account{}, nil)
		_, err := svc.CreateAccount(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("EmailCheckError", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "fail@example.com", Nickname: "user", Password: strPtr("pw"),
		}
		accounts.EXPECT().GetByEmail(ctx, "fail@example.com").Return(nil, errors.New("db err"))
		_, err := svc.CreateAccount(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("NicknameAlreadyExists", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "new2@example.com", Nickname: "taken", Password: strPtr("pw"),
		}
		accounts.EXPECT().GetByEmail(ctx, "new2@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, "taken").Return(&iampb.Account{}, nil)
		_, err := svc.CreateAccount(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("NicknameCheckError", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "new3@example.com", Nickname: "erruser", Password: strPtr("pw"),
		}
		accounts.EXPECT().GetByEmail(ctx, "new3@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, "erruser").Return(nil, errors.New("db err"))
		_, err := svc.CreateAccount(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("HashError", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "new4@example.com", Nickname: "new4", Password: strPtr("pw"),
		}
		accounts.EXPECT().GetByEmail(ctx, "new4@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, "new4").Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash("pw").Return("", errors.New("hash err"))
		_, err := svc.CreateAccount(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("LinkIdentity_AlreadyLinkedToOther", func(t *testing.T) {
		req := &api.CreateAccountRequest{
			Email: "new5@example.com", Nickname: "new5",
			Link: &api.ExternalIdentityLink{ProviderId: "p1", Subject: "s5"},
		}
		accounts.EXPECT().GetByEmail(ctx, "new5@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, "new5").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		// linkIdentity: already linked to a different account
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "s5").Return(&iampb.ExternalIdentity{AccountId: "other"}, nil)
		_, err := svc.CreateAccount(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})
}

// -------- GetAccount --------

func TestGetAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Accounts: accounts})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Email: "user@example.com"}, nil)
		resp, err := svc.GetAccount(ctx, &api.GetAccountRequest{Id: "a1"})
		if err != nil {
			t.Fatalf("GetAccount: %v", err)
		}
		if resp.Account.Id != "a1" {
			t.Errorf("expected a1, got %s", resp.Account.Id)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "a2").Return(nil, derrors.ErrNotFound)
		_, err := svc.GetAccount(ctx, &api.GetAccountRequest{Id: "a2"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("Error", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "err").Return(nil, errors.New("db err"))
		_, err := svc.GetAccount(ctx, &api.GetAccountRequest{Id: "err"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- GetMyAccount --------

func TestGetMyAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	accounts := NewMockAccountRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Authn: authn, Accounts: accounts})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Email: "user@example.com"}, nil)
		resp, err := svc.GetMyAccount(ctx, &api.GetMyAccountRequest{})
		if err != nil {
			t.Fatalf("GetMyAccount: %v", err)
		}
		if resp.Account.Id != "a1" {
			t.Errorf("expected a1, got %s", resp.Account.Id)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.GetMyAccount(ctx, &api.GetMyAccountRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("AccountError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.GetMyAccount(ctx, &api.GetMyAccountRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ListAccounts --------

func TestListAccounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Accounts: accounts})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		accounts.EXPECT().List(ctx, uint32(10), "").Return([]*iampb.Account{
			{Id: "a1"}, {Id: "a2"},
		}, "next-token", nil)
		resp, err := svc.ListAccounts(ctx, &api.ListAccountsRequest{PageSize: 10, PageToken: ""})
		if err != nil {
			t.Fatalf("ListAccounts: %v", err)
		}
		if len(resp.Accounts) != 2 {
			t.Errorf("expected 2 accounts, got %d", len(resp.Accounts))
		}
		if resp.NextPageToken != "next-token" {
			t.Errorf("expected next-token, got %s", resp.NextPageToken)
		}
	})

	t.Run("Error", func(t *testing.T) {
		accounts.EXPECT().List(ctx, gomock.Any(), gomock.Any()).Return(nil, "", errors.New("db err"))
		_, err := svc.ListAccounts(ctx, &api.ListAccountsRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- UpdateAccount --------

func TestUpdateAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Accounts: accounts})
	ctx := context.Background()

	t.Run("Success_UpdateEmail", func(t *testing.T) {
		email := "updated@example.com"
		req := &api.UpdateAccountRequest{Id: "a1", Email: &email}
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Email: "old@example.com"}, nil)
		accounts.EXPECT().GetByEmail(ctx, "updated@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateAccount(ctx, req)
		if err != nil {
			t.Fatalf("UpdateAccount: %v", err)
		}
		if resp.Account.Email != "updated@example.com" {
			t.Errorf("expected updated@example.com, got %s", resp.Account.Email)
		}
	})

	t.Run("Success_UpdateNickname", func(t *testing.T) {
		nick := "newnick"
		req := &api.UpdateAccountRequest{Id: "a1", Nickname: &nick}
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Nickname: "old"}, nil)
		accounts.EXPECT().GetByNickname(ctx, "newnick").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateAccount(ctx, req)
		if err != nil {
			t.Fatalf("UpdateAccount: %v", err)
		}
		if resp.Account.Nickname != "newnick" {
			t.Errorf("expected newnick, got %s", resp.Account.Nickname)
		}
	})

	t.Run("EmailAlreadyUsedByOther", func(t *testing.T) {
		email := "other@example.com"
		req := &api.UpdateAccountRequest{Id: "a1", Email: &email}
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Email: "old@example.com"}, nil)
		accounts.EXPECT().GetByEmail(ctx, "other@example.com").Return(&iampb.Account{Id: "a2"}, nil)
		_, err := svc.UpdateAccount(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("NicknameAlreadyUsedByOther", func(t *testing.T) {
		nick := "othernick"
		req := &api.UpdateAccountRequest{Id: "a1", Nickname: &nick}
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Nickname: "old"}, nil)
		accounts.EXPECT().GetByNickname(ctx, "othernick").Return(&iampb.Account{Id: "a2"}, nil)
		_, err := svc.UpdateAccount(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("GetError", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "err").Return(nil, errors.New("db err"))
		_, err := svc.UpdateAccount(ctx, &api.UpdateAccountRequest{Id: "err"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		name := "x"
		req := &api.UpdateAccountRequest{Id: "a1", Nickname: &name}
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Nickname: "old"}, nil)
		accounts.EXPECT().GetByNickname(ctx, "x").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.UpdateAccount(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("EmailSameValue_NoConflictCheck", func(t *testing.T) {
		// Same email as current — no GetByEmail call needed
		sameEmail := "same@example.com"
		req := &api.UpdateAccountRequest{Id: "a1", Email: &sameEmail}
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Email: "same@example.com"}, nil)
		accounts.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		_, err := svc.UpdateAccount(ctx, req)
		if err != nil {
			t.Fatalf("UpdateAccount same email: %v", err)
		}
	})
}

// -------- DeleteAccount --------

func TestDeleteAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	tenants := NewMockTenantRepo(ctrl)
	memberships := NewMockMembershipRepo(ctrl)
	extIds := NewMockExternalIdentityRepo(ctrl)
	credentials := NewMockCredentialStore(ctrl)
	svc := NewIamService(IamDeps{
		Tx:       &utils.MockTrm{},
		Accounts: accounts, Tenants: tenants, Memberships: memberships,
		ExternalIdentities: extIds, Credentials: credentials,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tenants.EXPECT().CountOwnedBy(ctx, "a1").Return(0, nil)
		memberships.EXPECT().DeleteByAccount(ctx, "a1").Return(nil)
		extIds.EXPECT().DeleteByAccount(ctx, "a1").Return(nil)
		credentials.EXPECT().DeletePassword(ctx, "a1").Return(nil)
		accounts.EXPECT().Delete(ctx, "a1").Return(nil)

		_, err := svc.DeleteAccount(ctx, &api.DeleteAccountRequest{Id: "a1"})
		if err != nil {
			t.Fatalf("DeleteAccount: %v", err)
		}
	})

	t.Run("StillOwnsTenants", func(t *testing.T) {
		tenants.EXPECT().CountOwnedBy(ctx, "a1").Return(2, nil)
		_, err := svc.DeleteAccount(ctx, &api.DeleteAccountRequest{Id: "a1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("CountOwnedByError", func(t *testing.T) {
		tenants.EXPECT().CountOwnedBy(ctx, "a1").Return(0, errors.New("db err"))
		_, err := svc.DeleteAccount(ctx, &api.DeleteAccountRequest{Id: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DeleteMembershipsError", func(t *testing.T) {
		tenants.EXPECT().CountOwnedBy(ctx, "a1").Return(0, nil)
		memberships.EXPECT().DeleteByAccount(ctx, "a1").Return(errors.New("db err"))
		_, err := svc.DeleteAccount(ctx, &api.DeleteAccountRequest{Id: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}
