package iam

// auth_flow_test.go: tests for public authentication flows in service.go:
// Register, Login, Logout, Refresh, RequestPasswordReset, ConfirmPasswordReset,
// VerifyEmail, ChangePassword, ResetPassword, ResendVerification.

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// -------- Register --------

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gates := NewMockPlatformGates(ctrl)
	accounts := NewMockAccountRepo(ctrl)
	hasher := NewMockPasswordHasher(ctrl)
	credentials := NewMockCredentialStore(ctrl)
	oneTimeTokens := NewMockOneTimeTokens(ctrl)
	notifier := NewMockNotifier(ctrl)
	tokens := NewMockTokenService(ctrl)
	ttl := NewMockTokenTTL(ctrl)

	deps := IamDeps{
		Tx:    &utils.MockTrm{},
		Gates: gates, Accounts: accounts, Hasher: hasher,
		Credentials: credentials, OneTimeTokens: oneTimeTokens,
		Notifier: notifier, Tokens: tokens, TTL: ttl,
	}
	svc := NewIamService(deps)
	ctx := context.Background()
	req := &api.RegisterRequest{Email: "test@example.com", Nickname: "testuser", Password: "password123"}

	t.Run("Success", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash(req.Password).Return("hashed", nil)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		credentials.EXPECT().SetPassword(ctx, gomock.Any(), "hashed").Return(nil)
		ttl.EXPECT().EmailVerificationTTL().Return(time.Hour)
		oneTimeTokens.EXPECT().Mint(ctx, purposeEmailVerify, gomock.Any(), time.Hour).Return("tok", nil)
		notifier.EXPECT().SendEmailVerification(ctx, req.Email, "tok").Return(nil)
		tokens.EXPECT().IssuePair(ctx, gomock.Any(), false).Return(&api.TokenPair{AccessToken: "at", RefreshToken: "rt"}, nil)

		resp, err := svc.Register(ctx, req)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}
		if resp.Tokens.AccessToken != "at" {
			t.Errorf("expected access token at, got %s", resp.Tokens.AccessToken)
		}
	})

	t.Run("Disabled", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(false, nil)
		_, err := svc.Register(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("GatesError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(false, errors.New("gates err"))
		_, err := svc.Register(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("EmailAlreadyExists", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(&iampb.Account{}, nil)
		_, err := svc.Register(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("EmailCheckError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, errors.New("db err"))
		_, err := svc.Register(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("NicknameAlreadyExists", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(&iampb.Account{}, nil)
		_, err := svc.Register(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("NicknameCheckError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, errors.New("db err"))
		_, err := svc.Register(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("HashError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash(req.Password).Return("", errors.New("hash err"))
		_, err := svc.Register(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("AccountCreateError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash(req.Password).Return("hashed", nil)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.Register(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("CredentialSetError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash(req.Password).Return("hashed", nil)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		credentials.EXPECT().SetPassword(ctx, gomock.Any(), "hashed").Return(errors.New("db err"))
		_, err := svc.Register(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DispatchVerificationMintError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash(req.Password).Return("hashed", nil)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		credentials.EXPECT().SetPassword(ctx, gomock.Any(), "hashed").Return(nil)
		ttl.EXPECT().EmailVerificationTTL().Return(time.Hour)
		oneTimeTokens.EXPECT().Mint(ctx, purposeEmailVerify, gomock.Any(), time.Hour).Return("", errors.New("mint err"))
		_, err := svc.Register(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("DispatchVerificationNotifyError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash(req.Password).Return("hashed", nil)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		credentials.EXPECT().SetPassword(ctx, gomock.Any(), "hashed").Return(nil)
		ttl.EXPECT().EmailVerificationTTL().Return(time.Hour)
		oneTimeTokens.EXPECT().Mint(ctx, purposeEmailVerify, gomock.Any(), time.Hour).Return("tok", nil)
		notifier.EXPECT().SendEmailVerification(ctx, req.Email, "tok").Return(errors.New("notify err"))
		_, err := svc.Register(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("IssuePairError", func(t *testing.T) {
		gates.EXPECT().SelfRegistrationAllowed(ctx).Return(true, nil)
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		accounts.EXPECT().GetByNickname(ctx, req.Nickname).Return(nil, derrors.ErrNotFound)
		hasher.EXPECT().Hash(req.Password).Return("hashed", nil)
		accounts.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		credentials.EXPECT().SetPassword(ctx, gomock.Any(), "hashed").Return(nil)
		ttl.EXPECT().EmailVerificationTTL().Return(time.Hour)
		oneTimeTokens.EXPECT().Mint(ctx, purposeEmailVerify, gomock.Any(), time.Hour).Return("tok", nil)
		notifier.EXPECT().SendEmailVerification(ctx, req.Email, "tok").Return(nil)
		tokens.EXPECT().IssuePair(ctx, gomock.Any(), false).Return(nil, errors.New("issue err"))
		_, err := svc.Register(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// -------- Login --------

func TestLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	hasher := NewMockPasswordHasher(ctrl)
	credentials := NewMockCredentialStore(ctrl)
	tokens := NewMockTokenService(ctrl)

	svc := NewIamService(IamDeps{
		Tx:       &utils.MockTrm{},
		Accounts: accounts, Hasher: hasher, Credentials: credentials, Tokens: tokens,
	})
	ctx := context.Background()

	t.Run("SuccessEmail", func(t *testing.T) {
		req := &api.LoginRequest{Login: "test@example.com", Password: "pass"}
		accounts.EXPECT().GetByEmail(ctx, req.Login).Return(&iampb.Account{Id: "a1", IsAdmin: false}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("hash", nil)
		hasher.EXPECT().Verify("hash", req.Password).Return(true)
		tokens.EXPECT().IssuePair(ctx, "a1", false).Return(&api.TokenPair{AccessToken: "at"}, nil)

		resp, err := svc.Login(ctx, req)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if resp.Tokens.AccessToken != "at" {
			t.Errorf("expected at, got %s", resp.Tokens.AccessToken)
		}
	})

	t.Run("SuccessNickname", func(t *testing.T) {
		req := &api.LoginRequest{Login: "testuser", Password: "pass"}
		accounts.EXPECT().GetByNickname(ctx, req.Login).Return(&iampb.Account{Id: "a1", IsAdmin: false}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("hash", nil)
		hasher.EXPECT().Verify("hash", req.Password).Return(true)
		tokens.EXPECT().IssuePair(ctx, "a1", false).Return(&api.TokenPair{AccessToken: "at"}, nil)

		resp, err := svc.Login(ctx, req)
		if err != nil {
			t.Fatalf("Login nickname failed: %v", err)
		}
		if resp.Tokens.AccessToken != "at" {
			t.Errorf("expected at, got %s", resp.Tokens.AccessToken)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := &api.LoginRequest{Login: "nobody@example.com", Password: "pass"}
		accounts.EXPECT().GetByEmail(ctx, req.Login).Return(nil, derrors.ErrNotFound)
		_, err := svc.Login(ctx, req)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("AccountLookupError", func(t *testing.T) {
		req := &api.LoginRequest{Login: "fail@example.com", Password: "pass"}
		accounts.EXPECT().GetByEmail(ctx, req.Login).Return(nil, errors.New("db err"))
		_, err := svc.Login(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("WrongPassword", func(t *testing.T) {
		req := &api.LoginRequest{Login: "test@example.com", Password: "wrong"}
		accounts.EXPECT().GetByEmail(ctx, req.Login).Return(&iampb.Account{Id: "a1"}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("hash", nil)
		hasher.EXPECT().Verify("hash", "wrong").Return(false)
		_, err := svc.Login(ctx, req)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("GetPasswordError", func(t *testing.T) {
		req := &api.LoginRequest{Login: "test@example.com", Password: "pass"}
		accounts.EXPECT().GetByEmail(ctx, req.Login).Return(&iampb.Account{Id: "a1"}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("", errors.New("cred err"))
		_, err := svc.Login(ctx, req)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}

// -------- Logout --------

func TestLogout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokens := NewMockTokenService(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Tokens: tokens})
	ctx := context.Background()
	req := &api.LogoutRequest{RefreshToken: "rt1"}

	t.Run("Success", func(t *testing.T) {
		tokens.EXPECT().Revoke(ctx, "rt1").Return(nil)
		_, err := svc.Logout(ctx, req)
		if err != nil {
			t.Fatalf("Logout failed: %v", err)
		}
	})

	t.Run("NotFoundIsSuccess", func(t *testing.T) {
		tokens.EXPECT().Revoke(ctx, "rt1").Return(derrors.ErrNotFound)
		_, err := svc.Logout(ctx, req)
		if err != nil {
			t.Fatalf("Logout not-found should succeed: %v", err)
		}
	})

	t.Run("RevokeError", func(t *testing.T) {
		tokens.EXPECT().Revoke(ctx, "rt1").Return(errors.New("db err"))
		_, err := svc.Logout(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- Refresh --------

func TestRefresh(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	tokens := NewMockTokenService(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Accounts: accounts, Tokens: tokens})
	ctx := context.Background()
	req := &api.RefreshRequest{RefreshToken: "rt1"}

	t.Run("Success", func(t *testing.T) {
		tokens.EXPECT().Rotate(ctx, "rt1").Return("a1", nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", IsAdmin: false}, nil)
		tokens.EXPECT().IssuePair(ctx, "a1", false).Return(&api.TokenPair{AccessToken: "at2", RefreshToken: "rt2"}, nil)

		resp, err := svc.Refresh(ctx, req)
		if err != nil {
			t.Fatalf("Refresh failed: %v", err)
		}
		if resp.Tokens.AccessToken != "at2" {
			t.Errorf("expected at2, got %s", resp.Tokens.AccessToken)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		tokens.EXPECT().Rotate(ctx, "rt1").Return("", errors.New("invalid"))
		_, err := svc.Refresh(ctx, req)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("AccountGetError", func(t *testing.T) {
		tokens.EXPECT().Rotate(ctx, "rt1").Return("a1", nil)
		accounts.EXPECT().Get(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.Refresh(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("IssuePairError", func(t *testing.T) {
		tokens.EXPECT().Rotate(ctx, "rt1").Return("a1", nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", IsAdmin: false}, nil)
		tokens.EXPECT().IssuePair(ctx, "a1", false).Return(nil, errors.New("issue err"))
		_, err := svc.Refresh(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// -------- RequestPasswordReset --------

func TestRequestPasswordReset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	oneTimeTokens := NewMockOneTimeTokens(ctrl)
	notifier := NewMockNotifier(ctrl)
	ttl := NewMockTokenTTL(ctrl)

	svc := NewIamService(IamDeps{
		Tx:       &utils.MockTrm{},
		Accounts: accounts, OneTimeTokens: oneTimeTokens, Notifier: notifier, TTL: ttl,
	})
	ctx := context.Background()
	req := &api.RequestPasswordResetRequest{Email: "test@example.com"}

	t.Run("Success", func(t *testing.T) {
		acc := &iampb.Account{Id: "a1", Email: req.Email}
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(acc, nil)
		ttl.EXPECT().PasswordResetTTL().Return(time.Hour)
		oneTimeTokens.EXPECT().Mint(ctx, purposePasswordReset, "a1", time.Hour).Return("reset_tok", nil)
		notifier.EXPECT().SendPasswordReset(ctx, req.Email, "reset_tok").Return(nil)

		_, err := svc.RequestPasswordReset(ctx, req)
		if err != nil {
			t.Fatalf("RequestPasswordReset failed: %v", err)
		}
	})

	t.Run("NotFoundIsSuccess", func(t *testing.T) {
		accounts.EXPECT().GetByEmail(ctx, req.Email).Return(nil, derrors.ErrNotFound)
		_, err := svc.RequestPasswordReset(ctx, req)
		if err != nil {
			t.Fatalf("expected success: %v", err)
		}
	})
}

// -------- ConfirmPasswordReset --------

func TestConfirmPasswordReset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oneTimeTokens := NewMockOneTimeTokens(ctrl)
	hasher := NewMockPasswordHasher(ctrl)
	credentials := NewMockCredentialStore(ctrl)

	svc := NewIamService(IamDeps{
		Tx:            &utils.MockTrm{},
		OneTimeTokens: oneTimeTokens, Hasher: hasher, Credentials: credentials,
	})
	ctx := context.Background()
	req := &api.ConfirmPasswordResetRequest{Token: "tok", NewPassword: "newpw"}

	t.Run("Success", func(t *testing.T) {
		hasher.EXPECT().Hash("newpw").Return("h", nil)
		oneTimeTokens.EXPECT().Consume(ctx, purposePasswordReset, "tok").Return("a1", nil)
		credentials.EXPECT().SetPassword(ctx, "a1", "h").Return(nil)
		_, err := svc.ConfirmPasswordReset(ctx, req)
		if err != nil {
			t.Fatalf("ConfirmPasswordReset failed: %v", err)
		}
	})

	t.Run("HashError", func(t *testing.T) {
		hasher.EXPECT().Hash("newpw").Return("", errors.New("hash err"))
		_, err := svc.ConfirmPasswordReset(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		hasher.EXPECT().Hash("newpw").Return("h", nil)
		oneTimeTokens.EXPECT().Consume(ctx, purposePasswordReset, "tok").Return("", errors.New("invalid"))
		_, err := svc.ConfirmPasswordReset(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("SetPasswordError", func(t *testing.T) {
		hasher.EXPECT().Hash("newpw").Return("h", nil)
		oneTimeTokens.EXPECT().Consume(ctx, purposePasswordReset, "tok").Return("a1", nil)
		credentials.EXPECT().SetPassword(ctx, "a1", "h").Return(errors.New("db err"))
		_, err := svc.ConfirmPasswordReset(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- VerifyEmail --------

func TestVerifyEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	oneTimeTokens := NewMockOneTimeTokens(ctrl)

	svc := NewIamService(IamDeps{
		Tx:       &utils.MockTrm{},
		Accounts: accounts, OneTimeTokens: oneTimeTokens,
	})
	ctx := context.Background()
	req := &api.VerifyEmailRequest{Token: "verify_tok"}

	t.Run("Success", func(t *testing.T) {
		oneTimeTokens.EXPECT().Consume(ctx, purposeEmailVerify, "verify_tok").Return("a1", nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", EmailVerified: false}, nil)
		accounts.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		_, err := svc.VerifyEmail(ctx, req)
		if err != nil {
			t.Fatalf("VerifyEmail failed: %v", err)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		oneTimeTokens.EXPECT().Consume(ctx, purposeEmailVerify, "verify_tok").Return("", errors.New("invalid"))
		_, err := svc.VerifyEmail(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("AccountGetError", func(t *testing.T) {
		oneTimeTokens.EXPECT().Consume(ctx, purposeEmailVerify, "verify_tok").Return("a1", nil)
		accounts.EXPECT().Get(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.VerifyEmail(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("AccountUpdateError", func(t *testing.T) {
		oneTimeTokens.EXPECT().Consume(ctx, purposeEmailVerify, "verify_tok").Return("a1", nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", EmailVerified: false}, nil)
		accounts.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.VerifyEmail(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ChangePassword --------

func TestChangePassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	credentials := NewMockCredentialStore(ctrl)
	hasher := NewMockPasswordHasher(ctrl)

	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Authn: authn, Credentials: credentials, Hasher: hasher,
	})
	ctx := context.Background()
	req := &api.ChangePasswordRequest{OldPassword: "old", NewPassword: "new"}

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("hashed_old", nil)
		hasher.EXPECT().Verify("hashed_old", "old").Return(true)
		hasher.EXPECT().Hash("new").Return("hashed_new", nil)
		credentials.EXPECT().SetPassword(ctx, "a1", "hashed_new").Return(nil)
		_, err := svc.ChangePassword(ctx, req)
		if err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.ChangePassword(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("WrongOldPassword", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("hash", nil)
		hasher.EXPECT().Verify("hash", "old").Return(false)
		_, err := svc.ChangePassword(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("GetPasswordError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("", errors.New("db err"))
		_, err := svc.ChangePassword(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("HashError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("hashed_old", nil)
		hasher.EXPECT().Verify("hashed_old", "old").Return(true)
		hasher.EXPECT().Hash("new").Return("", errors.New("hash err"))
		_, err := svc.ChangePassword(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("SetPasswordError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		credentials.EXPECT().GetPassword(ctx, "a1").Return("hashed_old", nil)
		hasher.EXPECT().Verify("hashed_old", "old").Return(true)
		hasher.EXPECT().Hash("new").Return("hashed_new", nil)
		credentials.EXPECT().SetPassword(ctx, "a1", "hashed_new").Return(errors.New("db err"))
		_, err := svc.ChangePassword(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ResetPassword --------

func TestResetPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := NewMockAccountRepo(ctrl)
	credentials := NewMockCredentialStore(ctrl)
	hasher := NewMockPasswordHasher(ctrl)

	svc := NewIamService(IamDeps{
		Tx:       &utils.MockTrm{},
		Accounts: accounts, Credentials: credentials, Hasher: hasher,
	})
	ctx := context.Background()
	req := &api.ResetPasswordRequest{AccountId: "a1", NewPassword: "new"}

	t.Run("Success", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		hasher.EXPECT().Hash("new").Return("hashed", nil)
		credentials.EXPECT().SetPassword(ctx, "a1", "hashed").Return(nil)
		_, err := svc.ResetPassword(ctx, req)
		if err != nil {
			t.Fatalf("ResetPassword failed: %v", err)
		}
	})

	t.Run("AccountNotFound", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "a1").Return(nil, derrors.ErrNotFound)
		_, err := svc.ResetPassword(ctx, req)
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("HashError", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		hasher.EXPECT().Hash("new").Return("", errors.New("hash err"))
		_, err := svc.ResetPassword(ctx, req)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("SetPasswordError", func(t *testing.T) {
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1"}, nil)
		hasher.EXPECT().Hash("new").Return("hashed", nil)
		credentials.EXPECT().SetPassword(ctx, "a1", "hashed").Return(errors.New("db err"))
		_, err := svc.ResetPassword(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ResendVerification --------

func TestResendVerification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	accounts := NewMockAccountRepo(ctrl)
	oneTimeTokens := NewMockOneTimeTokens(ctrl)
	notifier := NewMockNotifier(ctrl)
	ttl := NewMockTokenTTL(ctrl)

	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Authn: authn, Accounts: accounts, OneTimeTokens: oneTimeTokens, Notifier: notifier, TTL: ttl,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", Email: "a@b.com", EmailVerified: false}, nil)
		ttl.EXPECT().EmailVerificationTTL().Return(time.Hour)
		oneTimeTokens.EXPECT().Mint(ctx, purposeEmailVerify, "a1", time.Hour).Return("tok", nil)
		notifier.EXPECT().SendEmailVerification(ctx, "a@b.com", "tok").Return(nil)
		_, err := svc.ResendVerification(ctx, &api.ResendVerificationRequest{})
		if err != nil {
			t.Fatalf("ResendVerification failed: %v", err)
		}
	})

	t.Run("AlreadyVerified", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(&iampb.Account{Id: "a1", EmailVerified: true}, nil)
		_, err := svc.ResendVerification(ctx, &api.ResendVerificationRequest{})
		if err != nil {
			t.Fatalf("expected success: %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.ResendVerification(ctx, &api.ResendVerificationRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("AccountGetError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		accounts.EXPECT().Get(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.ResendVerification(ctx, &api.ResendVerificationRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})
}
