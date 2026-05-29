package iam

// sso_test.go: tests for SSO identity provider management and login flows in service.go:
// CreateIdentityProvider, GetIdentityProvider, UpdateIdentityProvider, DeleteIdentityProvider,
// ListIdentityProviders, StartSSO, CompleteSSO, LinkExternalIdentity,
// UnlinkExternalIdentity, ListExternalIdentities.

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

// -------- CreateIdentityProvider --------

func TestCreateIdentityProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providers := NewMockIdentityProviderRepo(ctrl)
	secrets := NewMockProviderSecrets(ctrl)
	svc := NewIamService(IamDeps{
		Tx:        &utils.MockTrm{},
		Providers: providers, ProviderSecrets: secrets,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		req := &api.CreateIdentityProviderRequest{DisplayName: "Google", Slug: "google", ClientSecret: "secret"}
		providers.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		secrets.EXPECT().Set(ctx, gomock.Any(), "secret").Return(nil)
		resp, err := svc.CreateIdentityProvider(ctx, req)
		if err != nil {
			t.Fatalf("CreateIdentityProvider: %v", err)
		}
		if resp.Provider.DisplayName != "Google" {
			t.Errorf("expected Google, got %s", resp.Provider.DisplayName)
		}
	})

	t.Run("AutoProvision_NoAllowedDomains", func(t *testing.T) {
		req := &api.CreateIdentityProviderRequest{AutoProvision: true}
		_, err := svc.CreateIdentityProvider(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("AutoProvision_WithAllowedDomains", func(t *testing.T) {
		req := &api.CreateIdentityProviderRequest{AutoProvision: true, AllowedDomains: []string{"ex.com"}, ClientSecret: "sec"}
		providers.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		secrets.EXPECT().Set(ctx, gomock.Any(), "sec").Return(nil)
		_, err := svc.CreateIdentityProvider(ctx, req)
		if err != nil {
			t.Fatalf("CreateIdentityProvider with allowed domains: %v", err)
		}
	})

	t.Run("CreateError", func(t *testing.T) {
		req := &api.CreateIdentityProviderRequest{DisplayName: "G", ClientSecret: "s"}
		providers.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.CreateIdentityProvider(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("SetSecretError", func(t *testing.T) {
		req := &api.CreateIdentityProviderRequest{DisplayName: "G", ClientSecret: "s"}
		providers.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		secrets.EXPECT().Set(ctx, gomock.Any(), "s").Return(errors.New("vault err"))
		_, err := svc.CreateIdentityProvider(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- GetIdentityProvider --------

func TestGetIdentityProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providers := NewMockIdentityProviderRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Providers: providers})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1", DisplayName: "Google"}, nil)
		resp, err := svc.GetIdentityProvider(ctx, &api.GetIdentityProviderRequest{Id: "p1"})
		if err != nil {
			t.Fatalf("GetIdentityProvider: %v", err)
		}
		if resp.Provider.Id != "p1" {
			t.Errorf("expected p1, got %s", resp.Provider.Id)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		providers.EXPECT().Get(ctx, "p1").Return(nil, derrors.ErrNotFound)
		_, err := svc.GetIdentityProvider(ctx, &api.GetIdentityProviderRequest{Id: "p1"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})
}

// -------- UpdateIdentityProvider --------

func TestUpdateIdentityProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providers := NewMockIdentityProviderRepo(ctrl)
	secrets := NewMockProviderSecrets(ctrl)
	svc := NewIamService(IamDeps{
		Tx:        &utils.MockTrm{},
		Providers: providers, ProviderSecrets: secrets,
	})
	ctx := context.Background()

	t.Run("Success_WithNewSecret", func(t *testing.T) {
		newSecret := "new_secret"
		req := &api.UpdateIdentityProviderRequest{Id: "p1", DisplayName: ssoStrPtr("Google Updated"), ClientSecret: &newSecret}
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1"}, nil)
		providers.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		secrets.EXPECT().Set(ctx, "p1", "new_secret").Return(nil)
		resp, err := svc.UpdateIdentityProvider(ctx, req)
		if err != nil {
			t.Fatalf("UpdateIdentityProvider: %v", err)
		}
		if resp.Provider.DisplayName != "Google Updated" {
			t.Errorf("expected Google Updated, got %s", resp.Provider.DisplayName)
		}
	})

	t.Run("Success_NoSecretUpdate", func(t *testing.T) {
		name := "Updated"
		req := &api.UpdateIdentityProviderRequest{Id: "p1", DisplayName: &name}
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1"}, nil)
		providers.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		_, err := svc.UpdateIdentityProvider(ctx, req)
		if err != nil {
			t.Fatalf("UpdateIdentityProvider no secret: %v", err)
		}
	})

	t.Run("AutoProvision_NoAllowedDomains_ValidationFail", func(t *testing.T) {
		autoProv := true
		req := &api.UpdateIdentityProviderRequest{Id: "p1", AutoProvision: &autoProv}
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1", AllowedDomains: []string{}}, nil)
		_, err := svc.UpdateIdentityProvider(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("GetError", func(t *testing.T) {
		providers.EXPECT().Get(ctx, "p1").Return(nil, errors.New("db err"))
		_, err := svc.UpdateIdentityProvider(ctx, &api.UpdateIdentityProviderRequest{Id: "p1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("UpdateAllFields", func(t *testing.T) {
		issuer := "https://accounts.google.com"
		clientID := "new-client-id"
		scopes := []string{"openid", "email"}
		domains := []string{"example.com"}
		autoProv := true
		disabled := true

		req := &api.UpdateIdentityProviderRequest{
			Id:             "p1",
			Issuer:         &issuer,
			ClientId:       &clientID,
			Scopes:         scopes,
			AllowedDomains: domains,
			AutoProvision:  &autoProv,
			Disabled:       &disabled,
		}
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1"}, nil)
		providers.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		_, err := svc.UpdateIdentityProvider(ctx, req)
		if err != nil {
			t.Fatalf("UpdateIdentityProvider all fields: %v", err)
		}
	})
}

// -------- DeleteIdentityProvider --------

func TestDeleteIdentityProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providers := NewMockIdentityProviderRepo(ctrl)
	secrets := NewMockProviderSecrets(ctrl)
	svc := NewIamService(IamDeps{
		Tx:        &utils.MockTrm{},
		Providers: providers, ProviderSecrets: secrets,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		secrets.EXPECT().Delete(ctx, "p1").Return(nil)
		providers.EXPECT().Delete(ctx, "p1").Return(nil)
		_, err := svc.DeleteIdentityProvider(ctx, &api.DeleteIdentityProviderRequest{Id: "p1"})
		if err != nil {
			t.Fatalf("DeleteIdentityProvider: %v", err)
		}
	})

	t.Run("SecretNotFound_Success", func(t *testing.T) {
		// IgnoreNotFound semantics: ErrNotFound on secret delete is ok
		secrets.EXPECT().Delete(ctx, "p1").Return(derrors.ErrNotFound)
		providers.EXPECT().Delete(ctx, "p1").Return(nil)
		_, err := svc.DeleteIdentityProvider(ctx, &api.DeleteIdentityProviderRequest{Id: "p1"})
		if err != nil {
			t.Fatalf("DeleteIdentityProvider secret not found: %v", err)
		}
	})

	t.Run("ProviderNotFound_Success", func(t *testing.T) {
		secrets.EXPECT().Delete(ctx, "p1").Return(nil)
		providers.EXPECT().Delete(ctx, "p1").Return(derrors.ErrNotFound)
		_, err := svc.DeleteIdentityProvider(ctx, &api.DeleteIdentityProviderRequest{Id: "p1"})
		if err != nil {
			t.Fatalf("DeleteIdentityProvider provider not found: %v", err)
		}
	})

	t.Run("SecretDeleteError", func(t *testing.T) {
		secrets.EXPECT().Delete(ctx, "p1").Return(errors.New("vault err"))
		_, err := svc.DeleteIdentityProvider(ctx, &api.DeleteIdentityProviderRequest{Id: "p1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ProviderDeleteError", func(t *testing.T) {
		secrets.EXPECT().Delete(ctx, "p1").Return(nil)
		providers.EXPECT().Delete(ctx, "p1").Return(errors.New("db err"))
		_, err := svc.DeleteIdentityProvider(ctx, &api.DeleteIdentityProviderRequest{Id: "p1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ListIdentityProviders --------

func TestListIdentityProviders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providers := NewMockIdentityProviderRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Providers: providers})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		providers.EXPECT().ListEnabled(ctx).Return([]*iampb.IdentityProvider{
			{Id: "p1", Slug: "google", DisplayName: "Google"},
		}, nil)
		resp, err := svc.ListIdentityProviders(ctx, &api.ListIdentityProvidersRequest{})
		if err != nil {
			t.Fatalf("ListIdentityProviders: %v", err)
		}
		if len(resp.Buttons) != 1 {
			t.Errorf("expected 1 button, got %d", len(resp.Buttons))
		}
	})

	t.Run("ListError", func(t *testing.T) {
		providers.EXPECT().ListEnabled(ctx).Return(nil, errors.New("db err"))
		_, err := svc.ListIdentityProviders(ctx, &api.ListIdentityProvidersRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- StartSSO --------

func TestStartSSO(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providers := NewMockIdentityProviderRepo(ctrl)
	sso := NewMockSSOFlows(ctrl)
	svc := NewIamService(IamDeps{
		Tx:        &utils.MockTrm{},
		Providers: providers, SSO: sso,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1"}, nil)
		sso.EXPECT().Authorize(ctx, gomock.Any()).Return("auth_url", "state", nil)
		resp, err := svc.StartSSO(ctx, &api.StartSSORequest{ProviderId: "p1"})
		if err != nil {
			t.Fatalf("StartSSO: %v", err)
		}
		if resp.RedirectUrl != "auth_url" {
			t.Errorf("expected auth_url, got %s", resp.RedirectUrl)
		}
	})

	t.Run("ProviderDisabled", func(t *testing.T) {
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1", Disabled: true}, nil)
		_, err := svc.StartSSO(ctx, &api.StartSSORequest{ProviderId: "p1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("GetError", func(t *testing.T) {
		providers.EXPECT().Get(ctx, "p1").Return(nil, errors.New("db err"))
		_, err := svc.StartSSO(ctx, &api.StartSSORequest{ProviderId: "p1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("AuthorizeError", func(t *testing.T) {
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1"}, nil)
		sso.EXPECT().Authorize(ctx, gomock.Any()).Return("", "", errors.New("oauth err"))
		_, err := svc.StartSSO(ctx, &api.StartSSORequest{ProviderId: "p1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// -------- CompleteSSO --------

func TestCompleteSSO(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providers := NewMockIdentityProviderRepo(ctrl)
	secrets := NewMockProviderSecrets(ctrl)
	sso := NewMockSSOFlows(ctrl)
	accounts := NewMockAccountRepo(ctrl)
	identities := NewMockExternalIdentityRepo(ctrl)
	tokens := NewMockTokenService(ctrl)

	svc := NewIamService(IamDeps{
		Tx:        &utils.MockTrm{},
		Providers: providers, ProviderSecrets: secrets,
		SSO: sso, Accounts: accounts, ExternalIdentities: identities, Tokens: tokens,
	})
	ctx := context.Background()

	t.Run("ExistingUser", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		p := &iampb.IdentityProvider{Id: "p1"}
		providers.EXPECT().Get(ctx, "p1").Return(p, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("secret", nil)
		sso.EXPECT().Exchange(ctx, p, "secret", "code", "state").Return("sub", "user@example.com", true, nil)
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(&iampb.ExternalIdentity{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", IsAdmin: false}, nil)
		tokens.EXPECT().IssuePair(ctx, "a1", false).Return(&api.TokenPair{AccessToken: "at"}, nil)

		resp, err := svc.CompleteSSO(ctx, req)
		if err != nil {
			t.Fatalf("CompleteSSO existing user: %v", err)
		}
		if resp.Tokens.AccessToken != "at" {
			t.Errorf("expected at, got %s", resp.Tokens.AccessToken)
		}
	})

	t.Run("ProvisionSuccess", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		p := &iampb.IdentityProvider{Id: "p1", AutoProvision: true, AllowedDomains: []string{"example.com"}}
		providers.EXPECT().Get(ctx, "p1").Return(p, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("secret", nil)
		sso.EXPECT().Exchange(ctx, p, "secret", "code", "state").Return("sub", "new@example.com", true, nil)
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByEmail(ctx, "new@example.com").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		identities.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		accounts.EXPECT().Get(ctx, gomock.Any()).Return(&iampb.Account{Id: "a2", IsAdmin: false}, nil)
		tokens.EXPECT().IssuePair(ctx, "a2", false).Return(&api.TokenPair{AccessToken: "at2"}, nil)

		resp, err := svc.CompleteSSO(ctx, req)
		if err != nil {
			t.Fatalf("CompleteSSO provision: %v", err)
		}
		if resp.Tokens.AccessToken != "at2" {
			t.Errorf("expected at2, got %s", resp.Tokens.AccessToken)
		}
	})

	t.Run("ProvisionDenied_NoAuto", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		p := &iampb.IdentityProvider{Id: "p1", AutoProvision: false}
		providers.EXPECT().Get(ctx, "p1").Return(p, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("secret", nil)
		sso.EXPECT().Exchange(ctx, p, "secret", "code", "state").Return("sub", "new@example.com", true, nil)
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		_, err := svc.CompleteSSO(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("ProvisionDenied_NotVerified", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		p := &iampb.IdentityProvider{Id: "p1", AutoProvision: true}
		providers.EXPECT().Get(ctx, "p1").Return(p, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("secret", nil)
		sso.EXPECT().Exchange(ctx, p, "secret", "code", "state").Return("sub", "new@example.com", false, nil)
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		_, err := svc.CompleteSSO(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("ProvisionDenied_DomainNotAllowed", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		p := &iampb.IdentityProvider{Id: "p1", AutoProvision: true, AllowedDomains: []string{"other.com"}}
		providers.EXPECT().Get(ctx, "p1").Return(p, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("secret", nil)
		sso.EXPECT().Exchange(ctx, p, "secret", "code", "state").Return("sub", "new@example.com", true, nil)
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		_, err := svc.CompleteSSO(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("ProvisionDenied_EmailExists", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		p := &iampb.IdentityProvider{Id: "p1", AutoProvision: true, AllowedDomains: []string{"example.com"}}
		providers.EXPECT().Get(ctx, "p1").Return(p, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("secret", nil)
		sso.EXPECT().Exchange(ctx, p, "secret", "code", "state").Return("sub", "existing@example.com", true, nil)
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByEmail(ctx, "existing@example.com").Return(&iampb.Account{Id: "a1"}, nil)
		_, err := svc.CompleteSSO(ctx, req)
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("ProviderDisabled", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1", Disabled: true}, nil)
		_, err := svc.CompleteSSO(ctx, req)
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("GetSecretError", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		providers.EXPECT().Get(ctx, "p1").Return(&iampb.IdentityProvider{Id: "p1"}, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("", errors.New("vault err"))
		_, err := svc.CompleteSSO(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ExchangeError", func(t *testing.T) {
		req := &api.CompleteSSORequest{ProviderId: "p1", Code: "code", State: "state"}
		p := &iampb.IdentityProvider{Id: "p1"}
		providers.EXPECT().Get(ctx, "p1").Return(p, nil)
		secrets.EXPECT().Get(ctx, "p1").Return("secret", nil)
		sso.EXPECT().Exchange(ctx, p, "secret", "code", "state").Return("", "", false, errors.New("oauth err"))
		_, err := svc.CompleteSSO(ctx, req)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}

// -------- LinkExternalIdentity --------

func TestLinkExternalIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	identities := NewMockExternalIdentityRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:       &utils.MockTrm{},
		Accounts: accounts, ExternalIdentities: identities,
	})
	ctx := context.Background()

	t.Run("Success_NewLink", func(t *testing.T) {
		req := &api.LinkExternalIdentityRequest{
			AccountId: "a1",
			Link:      &api.ExternalIdentityLink{ProviderId: "p1", Subject: "sub", Email: "user@example.com"},
		}
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		identities.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		_, err := svc.LinkExternalIdentity(ctx, req)
		if err != nil {
			t.Fatalf("LinkExternalIdentity: %v", err)
		}
	})

	t.Run("AlreadyLinkedToSameAccount_Success", func(t *testing.T) {
		req := &api.LinkExternalIdentityRequest{
			AccountId: "a1",
			Link:      &api.ExternalIdentityLink{ProviderId: "p1", Subject: "sub"},
		}
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(&iampb.ExternalIdentity{AccountId: "a1"}, nil)
		resp, err := svc.LinkExternalIdentity(ctx, req)
		if err != nil {
			t.Fatalf("LinkExternalIdentity already linked: %v", err)
		}
		if resp.Identity.AccountId != "a1" {
			t.Errorf("expected a1, got %s", resp.Identity.AccountId)
		}
	})

	t.Run("AlreadyLinkedToOther_Error", func(t *testing.T) {
		req := &api.LinkExternalIdentityRequest{
			AccountId: "a1",
			Link:      &api.ExternalIdentityLink{ProviderId: "p1", Subject: "sub"},
		}
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(&iampb.ExternalIdentity{AccountId: "a2"}, nil)
		_, err := svc.LinkExternalIdentity(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("GetByProviderSubjectError", func(t *testing.T) {
		req := &api.LinkExternalIdentityRequest{
			AccountId: "a1",
			Link:      &api.ExternalIdentityLink{ProviderId: "p1", Subject: "sub"},
		}
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, errors.New("db err"))
		_, err := svc.LinkExternalIdentity(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("AccountGetError", func(t *testing.T) {
		req := &api.LinkExternalIdentityRequest{
			AccountId: "a1",
			Link:      &api.ExternalIdentityLink{ProviderId: "p1", Subject: "sub"},
		}
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Get(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.LinkExternalIdentity(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("CreateError", func(t *testing.T) {
		req := &api.LinkExternalIdentityRequest{
			AccountId: "a1",
			Link:      &api.ExternalIdentityLink{ProviderId: "p1", Subject: "sub"},
		}
		identities.EXPECT().GetByProviderSubject(ctx, "p1", "sub").Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		identities.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.LinkExternalIdentity(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- UnlinkExternalIdentity --------

func TestUnlinkExternalIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	identities := NewMockExternalIdentityRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Authn: authn, ExternalIdentities: identities,
	})
	ctx := context.Background()

	t.Run("Success_Self", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		identities.EXPECT().Get(ctx, "ei1").Return(&iampb.ExternalIdentity{Id: "ei1", AccountId: "a1"}, nil)
		identities.EXPECT().Delete(ctx, "ei1").Return(nil)
		_, err := svc.UnlinkExternalIdentity(ctx, &api.UnlinkExternalIdentityRequest{Id: "ei1"})
		if err != nil {
			t.Fatalf("UnlinkExternalIdentity: %v", err)
		}
	})

	t.Run("Success_Admin", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "admin", IsAdmin: true}, nil)
		identities.EXPECT().Get(ctx, "ei1").Return(&iampb.ExternalIdentity{Id: "ei1", AccountId: "a1"}, nil)
		identities.EXPECT().Delete(ctx, "ei1").Return(nil)
		_, err := svc.UnlinkExternalIdentity(ctx, &api.UnlinkExternalIdentityRequest{Id: "ei1"})
		if err != nil {
			t.Fatalf("UnlinkExternalIdentity admin: %v", err)
		}
	})

	t.Run("NotFound_Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		identities.EXPECT().Get(ctx, "ei1").Return(nil, derrors.ErrNotFound)
		_, err := svc.UnlinkExternalIdentity(ctx, &api.UnlinkExternalIdentityRequest{Id: "ei1"})
		if err != nil {
			t.Fatalf("UnlinkExternalIdentity not found: %v", err)
		}
	})

	t.Run("NotOwner_PermissionDenied", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a2"}, nil)
		identities.EXPECT().Get(ctx, "ei1").Return(&iampb.ExternalIdentity{Id: "ei1", AccountId: "a1"}, nil)
		_, err := svc.UnlinkExternalIdentity(ctx, &api.UnlinkExternalIdentityRequest{Id: "ei1"})
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.UnlinkExternalIdentity(ctx, &api.UnlinkExternalIdentityRequest{Id: "ei1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetIdentityError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		identities.EXPECT().Get(ctx, "ei1").Return(nil, errors.New("db err"))
		_, err := svc.UnlinkExternalIdentity(ctx, &api.UnlinkExternalIdentityRequest{Id: "ei1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ListExternalIdentities --------

func TestListExternalIdentities(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	identities := NewMockExternalIdentityRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Authn: authn, ExternalIdentities: identities,
	})
	ctx := context.Background()

	t.Run("Success_Self", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		identities.EXPECT().ListByAccount(ctx, "a1").Return([]*iampb.ExternalIdentity{{Id: "ei1"}}, nil)
		resp, err := svc.ListExternalIdentities(ctx, &api.ListExternalIdentitiesRequest{AccountId: "a1"})
		if err != nil {
			t.Fatalf("ListExternalIdentities: %v", err)
		}
		if len(resp.Identities) != 1 {
			t.Errorf("expected 1 identity, got %d", len(resp.Identities))
		}
	})

	t.Run("NotOwner_PermissionDenied", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a2"}, nil)
		_, err := svc.ListExternalIdentities(ctx, &api.ListExternalIdentitiesRequest{AccountId: "a1"})
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("Admin_CanListOthers", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "admin", IsAdmin: true}, nil)
		identities.EXPECT().ListByAccount(ctx, "a1").Return([]*iampb.ExternalIdentity{{Id: "ei1"}}, nil)
		_, err := svc.ListExternalIdentities(ctx, &api.ListExternalIdentitiesRequest{AccountId: "a1"})
		if err != nil {
			t.Fatalf("ListExternalIdentities admin: %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.ListExternalIdentities(ctx, &api.ListExternalIdentitiesRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		identities.EXPECT().ListByAccount(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.ListExternalIdentities(ctx, &api.ListExternalIdentitiesRequest{AccountId: "a1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

func ssoStrPtr(s string) *string { return &s }
