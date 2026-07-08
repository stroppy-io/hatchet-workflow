package iam

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// One-time token purposes (OneTimeTokens.Mint/Consume).
const (
	purposeEmailVerify   = "email_verify"
	purposePasswordReset = "password_reset"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage or crypto of its own: every side effect goes
	through one of these. Implementations live elsewhere and are wired in via
	IamDeps. Persistence methods return derrors.ErrNotFound / derrors.ErrConflict so the handler
	can translate them into status codes.
*/

// Authz resolves a caller's effective permissions in one tenant — the live
// union the request gate computes (is_admin short-circuits to full access).
type Authz interface {
	EffectivePermissions(ctx context.Context, accountID, tenantID string) ([]*iam.Permission, error)
}

// AccountRepo persists Accounts (identity only — never secrets).
type AccountRepo interface {
	Create(ctx context.Context, account *iam.Account) error
	Get(ctx context.Context, id string) (*iam.Account, error)
	GetByEmail(ctx context.Context, email string) (*iam.Account, error)
	GetByNickname(ctx context.Context, nickname string) (*iam.Account, error)
	List(ctx context.Context, pageSize uint32, pageToken string) (accounts []*iam.Account, nextPageToken string, err error)
	Update(ctx context.Context, account *iam.Account) error
	Delete(ctx context.Context, id string) error
}

// RegistrationRequestRepo persists prospective-user access requests captured
// while self-signup is closed. email is the natural key — Upsert collapses
// repeat submissions from the same address onto one row.
type RegistrationRequestRepo interface {
	Upsert(ctx context.Context, req *api.RegistrationRequest) error
	Get(ctx context.Context, id string) (*api.RegistrationRequest, error)
	GetByEmail(ctx context.Context, email string) (*api.RegistrationRequest, error)
	List(ctx context.Context) ([]*api.RegistrationRequest, error)
	Update(ctx context.Context, req *api.RegistrationRequest) error
}

// CredentialStore holds password hashes keyed by account id, separate from the
// Account so identity can travel without credentials. Get returns derrors.ErrNotFound
// for an SSO-only account that has no password.
type CredentialStore interface {
	SetPassword(ctx context.Context, accountID, hash string) error
	GetPassword(ctx context.Context, accountID string) (hash string, err error)
	DeletePassword(ctx context.Context, accountID string) error
}

// PasswordHasher hashes and verifies plaintext passwords.
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(hash, plain string) bool
}

// TokenService mints, rotates and revokes the bearer TokenPair and its
// server-side refresh session. Its persistence (refresh session) participates in
// the ambient ctx transaction, so issuing/rotating commits atomically with the
// surrounding writes.
type TokenService interface {
	IssuePair(ctx context.Context, accountID string, isAdmin bool) (*api.TokenPair, error)
	// Rotate consumes a refresh token (single-use) and returns the account it
	// belonged to; the caller re-mints a pair so is_admin stays current.
	Rotate(ctx context.Context, refreshToken string) (accountID string, err error)
	Revoke(ctx context.Context, refreshToken string) error
}

// OneTimeTokens mints and consumes single-use, time-bounded tokens for the
// email-verification and password-reset flows. Consume is the credential. Both
// methods are DB-backed and participate in the ambient ctx transaction.
type OneTimeTokens interface {
	Mint(ctx context.Context, purpose, accountID string, ttl time.Duration) (token string, err error)
	Consume(ctx context.Context, purpose, token string) (accountID string, err error)
}

// Notifier delivers minted tokens over whatever channel (email, ...). Swapping
// the channel never touches the RPC contract. It is called INSIDE the ambient ctx
// transaction, so an implementation MUST enqueue the message transactionally (an
// outbox row that commits atomically with the surrounding writes) and let a
// relay perform the actual send afterwards — it MUST NOT block on synchronous
// network IO while the transaction is open.
type Notifier interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
}

// PlatformGates exposes the global process flags that fence two flows before
// any RBAC check (see PlatformSettings).
type PlatformGates interface {
	SelfRegistrationAllowed(ctx context.Context) (bool, error)
	MemberTenantCreationAllowed(ctx context.Context) (bool, error)
}

// PermissionCatalog yields the grantable-permission catalog assembled from the
// auth annotations across the build (see IamAPI.ListPermissions).
type PermissionCatalog interface {
	Grantable(ctx context.Context) ([]*api.CatalogEntry, error)
}

// TenantRepo persists Tenants. Delete is the caller's responsibility to cascade
// memberships/roles (done in the handler).
type TenantRepo interface {
	Create(ctx context.Context, tenant *iam.Tenant) error
	Get(ctx context.Context, id string) (*iam.Tenant, error)
	GetBySlug(ctx context.Context, slug string) (*iam.Tenant, error)
	ListByMember(ctx context.Context, accountID string) ([]*iam.Tenant, error)
	CountOwnedBy(ctx context.Context, accountID string) (int, error)
	Update(ctx context.Context, tenant *iam.Tenant) error
	Delete(ctx context.Context, id string) error
}

// RoleRepo persists Roles. GetMany backs membership-role validation.
type RoleRepo interface {
	Create(ctx context.Context, role *iam.Role) error
	Get(ctx context.Context, id string) (*iam.Role, error)
	GetMany(ctx context.Context, ids []string) ([]*iam.Role, error)
	List(ctx context.Context, tenantID string, pageSize uint32, pageToken string) (roles []*iam.Role, nextPageToken string, err error)
	Update(ctx context.Context, role *iam.Role) error
	Delete(ctx context.Context, id string) error
	DeleteByTenant(ctx context.Context, tenantID string) error
}

// MembershipRepo persists the account<->tenant role bindings.
type MembershipRepo interface {
	Create(ctx context.Context, membership *iam.Membership) error
	Get(ctx context.Context, id string) (*iam.Membership, error)
	GetByAccountTenant(ctx context.Context, accountID, tenantID string) (*iam.Membership, error)
	ShareTenant(ctx context.Context, accountID, otherAccountID string) (bool, error)
	RoleInUse(ctx context.Context, roleID string) (bool, error)
	List(ctx context.Context, tenantID string, pageSize uint32, pageToken string) (memberships []*iam.Membership, nextPageToken string, err error)
	Update(ctx context.Context, membership *iam.Membership) error
	Delete(ctx context.Context, id string) error
	DeleteByTenant(ctx context.Context, tenantID string) error
	DeleteByAccount(ctx context.Context, accountID string) error
}

// CatalogSeeder seeds a newly created tenant's org catalog inside the
// ambient CreateTenant transaction (see SeedOrgCatalog's doc for the
// LINKED/fork-on-edit semantics). Implemented by catalog.Service; iam never
// imports package catalog — this is the Deps-of-interfaces shape every other
// IamDeps field already follows, so wiring stays import-cycle-free (catalog
// would otherwise need iam for tenant lifecycle context, and iam would need
// catalog for this call — the interface breaks that cycle).
type CatalogSeeder interface {
	SeedOrgCatalog(ctx context.Context, tenantID string) error
}

// IdentityProviderRepo persists OIDC provider config (secret stored separately).
type IdentityProviderRepo interface {
	Create(ctx context.Context, provider *iam.IdentityProvider) error
	Get(ctx context.Context, id string) (*iam.IdentityProvider, error)
	ListEnabled(ctx context.Context) ([]*iam.IdentityProvider, error)
	Update(ctx context.Context, provider *iam.IdentityProvider) error
	Delete(ctx context.Context, id string) error
}

// ProviderSecrets holds write-only OIDC client secrets keyed by provider id.
type ProviderSecrets interface {
	Set(ctx context.Context, providerID, secret string) error
	Get(ctx context.Context, providerID string) (string, error)
	Delete(ctx context.Context, providerID string) error
}

// ExternalIdentityRepo persists the IdP-subject<->account links.
type ExternalIdentityRepo interface {
	Create(ctx context.Context, identity *iam.ExternalIdentity) error
	Get(ctx context.Context, id string) (*iam.ExternalIdentity, error)
	GetByProviderSubject(ctx context.Context, providerID, subject string) (*iam.ExternalIdentity, error)
	ListByAccount(ctx context.Context, accountID string) ([]*iam.ExternalIdentity, error)
	Delete(ctx context.Context, id string) error
	DeleteByAccount(ctx context.Context, accountID string) error
}

// SSOFlows runs the OIDC authorization-code dance, keeping state/PKCE/nonce
// server-side between Authorize and Exchange. Exchange reports the IdP's
// email_verified claim so the caller never treats an unverified address as
// authoritative.
type SSOFlows interface {
	Authorize(ctx context.Context, provider *iam.IdentityProvider) (redirectURL, state string, err error)
	Exchange(ctx context.Context, provider *iam.IdentityProvider, secret, code, state string) (subject, email string, emailVerified bool, err error)
}

// TokenTTL supplies the lifetimes for the one-time tokens, so the durations are
// configurable rather than hard-coded.
type TokenTTL interface {
	EmailVerificationTTL() time.Duration
	PasswordResetTTL() time.Duration
}

// IamDeps bundles every dependency for the constructor.
type IamDeps struct {
	Authn                utils.Authn
	Authz                Authz
	Accounts             AccountRepo
	RegistrationRequests RegistrationRequestRepo
	Credentials          CredentialStore
	Hasher               PasswordHasher
	Tokens               TokenService
	OneTimeTokens        OneTimeTokens
	Notifier             Notifier
	Gates                PlatformGates
	Catalog              PermissionCatalog
	Tenants              TenantRepo
	Roles                RoleRepo
	Memberships          MembershipRepo
	CatalogSeeder        CatalogSeeder
	Providers            IdentityProviderRepo
	ProviderSecrets      ProviderSecrets
	ExternalIdentities   ExternalIdentityRepo
	SSO                  SSOFlows
	TTL                  TokenTTL
	ApiTokens            ApiTokenRepo
	ApiTokenSecrets      ApiTokenSecrets
	ApiTokenMinter       ApiTokenMinter
	Tx                   tx.Trm
}

type IamService struct {
	*api.UnimplementedIamServiceServer
	tx.Trm
	d IamDeps
}

var _ api.IamServiceServer = (*IamService)(nil)

func NewIamService(deps IamDeps) *IamService {
	return &IamService{UnimplementedIamServiceServer: &api.UnimplementedIamServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *IamService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *IamService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects (outbox
// rows included) and never synchronous external IO.
func (s *IamService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *IamService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

/*
	===== Auth (public) =====
*/

func (s *IamService) Register(ctx context.Context, req *api.RegisterRequest) (*api.RegisterResponse, error) {
	allowed, err := s.d.Gates.SelfRegistrationAllowed(ctx)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if !allowed {
		return nil, status.Error(codes.PermissionDenied, "self-registration is disabled")
	}
	if _, err := s.d.Accounts.GetByEmail(ctx, req.GetEmail()); err == nil {
		return nil, status.Error(codes.AlreadyExists, "email already in use")
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}
	if _, err := s.d.Accounts.GetByNickname(ctx, req.GetNickname()); err == nil {
		return nil, status.Error(codes.AlreadyExists, "nickname already in use")
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}

	hash, err := s.d.Hasher.Hash(req.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	acc := &iam.Account{
		Id:            uuid.NewString(),
		Email:         req.GetEmail(),
		Nickname:      req.GetNickname(),
		EmailVerified: false,
		IsAdmin:       false,
		CreatedAt:     s.now(),
		UpdatedAt:     s.now(),
	}
	pair, err := doTxRet(ctx, s, func(ctx context.Context) (*api.TokenPair, error) {
		if err := s.d.Accounts.Create(ctx, acc); err != nil {
			return nil, utils.MapErr(err)
		}
		if err := s.d.Credentials.SetPassword(ctx, acc.Id, hash); err != nil {
			return nil, utils.MapErr(err)
		}
		if err := s.dispatchVerification(ctx, acc); err != nil {
			return nil, err
		}
		p, err := s.d.Tokens.IssuePair(ctx, acc.Id, acc.IsAdmin)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return p, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.RegisterResponse{Tokens: pair}, nil
}

func (s *IamService) dispatchVerification(ctx context.Context, acc *iam.Account) error {
	token, err := s.d.OneTimeTokens.Mint(ctx, purposeEmailVerify, acc.Id, s.d.TTL.EmailVerificationTTL())
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if err := s.d.Notifier.SendEmailVerification(ctx, acc.Email, token); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func (s *IamService) Login(ctx context.Context, req *api.LoginRequest) (*api.LoginResponse, error) {
	var (
		acc *iam.Account
		err error
	)
	if strings.Contains(req.GetLogin(), "@") {
		acc, err = s.d.Accounts.GetByEmail(ctx, req.GetLogin())
	} else {
		acc, err = s.d.Accounts.GetByNickname(ctx, req.GetLogin())
	}
	if errors.Is(err, derrors.ErrNotFound) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	hash, err := s.d.Credentials.GetPassword(ctx, acc.Id)
	if err != nil || !s.d.Hasher.Verify(hash, req.GetPassword()) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	pair, err := s.d.Tokens.IssuePair(ctx, acc.Id, acc.IsAdmin)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &api.LoginResponse{Tokens: pair}, nil
}

func (s *IamService) Refresh(ctx context.Context, req *api.RefreshRequest) (*api.RefreshResponse, error) {
	pair, err := doTxRet(ctx, s, func(ctx context.Context) (*api.TokenPair, error) {
		accountID, err := s.d.Tokens.Rotate(ctx, req.GetRefreshToken())
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		acc, err := s.d.Accounts.Get(ctx, accountID)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		p, err := s.d.Tokens.IssuePair(ctx, acc.Id, acc.IsAdmin)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return p, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.RefreshResponse{Tokens: pair}, nil
}

func (s *IamService) Logout(ctx context.Context, req *api.LogoutRequest) (*api.LogoutResponse, error) {
	if err := s.d.Tokens.Revoke(ctx, req.GetRefreshToken()); err != nil && !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}
	return &api.LogoutResponse{}, nil
}

func (s *IamService) RequestPasswordReset(ctx context.Context, req *api.RequestPasswordResetRequest) (*api.RequestPasswordResetResponse, error) {
	// Never leak account existence: always return success.
	acc, err := s.d.Accounts.GetByEmail(ctx, req.GetEmail())
	if err != nil {
		return &api.RequestPasswordResetResponse{}, nil
	}
	// Mint (token row) + notify (outbox) must commit together; failures are
	// swallowed to never leak account existence.
	_ = s.doTx(ctx, func(ctx context.Context) error {
		token, err := s.d.OneTimeTokens.Mint(ctx, purposePasswordReset, acc.Id, s.d.TTL.PasswordResetTTL())
		if err != nil {
			return err
		}
		return s.d.Notifier.SendPasswordReset(ctx, acc.Email, token)
	})
	return &api.RequestPasswordResetResponse{}, nil
}

func (s *IamService) ConfirmPasswordReset(ctx context.Context, req *api.ConfirmPasswordResetRequest) (*api.ConfirmPasswordResetResponse, error) {
	hash, err := s.d.Hasher.Hash(req.GetNewPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		accountID, err := s.d.OneTimeTokens.Consume(ctx, purposePasswordReset, req.GetToken())
		if err != nil {
			return status.Error(codes.PermissionDenied, "invalid or expired token")
		}
		return utils.MapErr(s.d.Credentials.SetPassword(ctx, accountID, hash))
	}); err != nil {
		return nil, err
	}
	return &api.ConfirmPasswordResetResponse{}, nil
}

func (s *IamService) VerifyEmail(ctx context.Context, req *api.VerifyEmailRequest) (*api.VerifyEmailResponse, error) {
	if err := s.doTx(ctx, func(ctx context.Context) error {
		accountID, err := s.d.OneTimeTokens.Consume(ctx, purposeEmailVerify, req.GetToken())
		if err != nil {
			return status.Error(codes.PermissionDenied, "invalid or expired token")
		}
		acc, err := s.d.Accounts.Get(ctx, accountID)
		if err != nil {
			return utils.MapErr(err)
		}
		acc.EmailVerified = true
		acc.UpdatedAt = s.now()
		return utils.MapErr(s.d.Accounts.Update(ctx, acc))
	}); err != nil {
		return nil, err
	}
	return &api.VerifyEmailResponse{}, nil
}

/*
	===== Accounts =====
*/

func (s *IamService) CreateAccount(ctx context.Context, req *api.CreateAccountRequest) (*api.CreateAccountResponse, error) {
	if req.Password == nil && req.GetLink() == nil {
		return nil, status.Error(codes.InvalidArgument, "account needs a password or an external identity link")
	}
	if _, err := s.d.Accounts.GetByEmail(ctx, req.GetEmail()); err == nil {
		return nil, status.Error(codes.AlreadyExists, "email already in use")
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}
	if _, err := s.d.Accounts.GetByNickname(ctx, req.GetNickname()); err == nil {
		return nil, status.Error(codes.AlreadyExists, "nickname already in use")
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}

	acc := &iam.Account{
		Id:            uuid.NewString(),
		Email:         req.GetEmail(),
		Nickname:      req.GetNickname(),
		IsAdmin:       req.GetIsAdmin(),
		EmailVerified: true, // admin-created accounts start verified.
		CreatedAt:     s.now(),
		UpdatedAt:     s.now(),
	}
	var hash string
	if req.Password != nil {
		h, err := s.d.Hasher.Hash(req.GetPassword())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		hash = h
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		if err := s.d.Accounts.Create(ctx, acc); err != nil {
			return utils.MapErr(err)
		}
		if req.Password != nil {
			if err := s.d.Credentials.SetPassword(ctx, acc.Id, hash); err != nil {
				return utils.MapErr(err)
			}
		}
		if link := req.GetLink(); link != nil {
			return s.linkIdentity(ctx, acc.Id, link)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &api.CreateAccountResponse{Account: acc}, nil
}

func (s *IamService) linkIdentity(ctx context.Context, accountID string, link *api.ExternalIdentityLink) error {
	if existing, err := s.d.ExternalIdentities.GetByProviderSubject(ctx, link.GetProviderId(), link.GetSubject()); err == nil {
		if existing.GetAccountId() != accountID {
			return status.Error(codes.AlreadyExists, "identity already linked to another account")
		}
		return nil
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return utils.MapErr(err)
	}
	ext := &iam.ExternalIdentity{
		Id:         uuid.NewString(),
		ProviderId: link.GetProviderId(),
		Subject:    link.GetSubject(),
		AccountId:  accountID,
		Email:      link.GetEmail(),
		CreatedAt:  s.now(),
		UpdatedAt:  s.now(),
	}
	return utils.MapErr(s.d.ExternalIdentities.Create(ctx, ext))
}

func (s *IamService) GetAccount(ctx context.Context, req *api.GetAccountRequest) (*api.GetAccountResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := s.d.Accounts.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if err := s.canReadAccount(ctx, c, acc.GetId()); err != nil {
		return nil, err
	}
	return &api.GetAccountResponse{Account: acc}, nil
}

// LookupAccountByEmail resolves an exact email to its account — the
// invite-by-email primitive (e.g. a tenant owner adding a member). Any
// authenticated caller may use it: an exact hit or NotFound, never a listing.
func (s *IamService) LookupAccountByEmail(ctx context.Context, req *api.LookupAccountByEmailRequest) (*api.LookupAccountByEmailResponse, error) {
	if _, err := s.caller(ctx); err != nil {
		return nil, err
	}
	acc, err := s.d.Accounts.GetByEmail(ctx, req.GetEmail())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.LookupAccountByEmailResponse{Account: acc}, nil
}

func (s *IamService) GetMyAccount(ctx context.Context, _ *api.GetMyAccountRequest) (*api.GetMyAccountResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := s.d.Accounts.Get(ctx, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetMyAccountResponse{Account: acc}, nil
}

func (s *IamService) ListAccounts(ctx context.Context, req *api.ListAccountsRequest) (*api.ListAccountsResponse, error) {
	accs, next, err := s.d.Accounts.List(ctx, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListAccountsResponse{Accounts: accs, NextPageToken: next}, nil
}

func (s *IamService) UpdateAccount(ctx context.Context, req *api.UpdateAccountRequest) (*api.UpdateAccountResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if !selfOrAdmin(c, req.GetId()) {
		return nil, status.Error(codes.PermissionDenied, "not allowed to update this account")
	}
	acc, err := s.d.Accounts.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if req.Email != nil && req.GetEmail() != acc.Email {
		if other, err := s.d.Accounts.GetByEmail(ctx, req.GetEmail()); err == nil && other.Id != acc.Id {
			return nil, status.Error(codes.AlreadyExists, "email already in use")
		} else if err != nil && !errors.Is(err, derrors.ErrNotFound) {
			return nil, utils.MapErr(err)
		}
		acc.Email = req.GetEmail()
		acc.EmailVerified = false
	}
	if req.Nickname != nil && req.GetNickname() != acc.Nickname {
		if other, err := s.d.Accounts.GetByNickname(ctx, req.GetNickname()); err == nil && other.Id != acc.Id {
			return nil, status.Error(codes.AlreadyExists, "nickname already in use")
		} else if err != nil && !errors.Is(err, derrors.ErrNotFound) {
			return nil, utils.MapErr(err)
		}
		acc.Nickname = req.GetNickname()
	}
	acc.UpdatedAt = s.now()
	if err := s.d.Accounts.Update(ctx, acc); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.UpdateAccountResponse{Account: acc}, nil
}

func (s *IamService) DeleteAccount(ctx context.Context, req *api.DeleteAccountRequest) (*api.DeleteAccountResponse, error) {
	if err := s.doTx(ctx, func(ctx context.Context) error {
		owned, err := s.d.Tenants.CountOwnedBy(ctx, req.GetId())
		if err != nil {
			return utils.MapErr(err)
		}
		if owned > 0 {
			return status.Error(codes.FailedPrecondition, "account still owns tenants; transfer ownership first")
		}
		if err := derrors.IgnoreNotFound(s.d.Memberships.DeleteByAccount(ctx, req.GetId())); err != nil {
			return utils.MapErr(err)
		}
		if err := derrors.IgnoreNotFound(s.d.ExternalIdentities.DeleteByAccount(ctx, req.GetId())); err != nil {
			return utils.MapErr(err)
		}
		if err := derrors.IgnoreNotFound(s.d.Credentials.DeletePassword(ctx, req.GetId())); err != nil {
			return utils.MapErr(err)
		}
		return utils.MapErr(derrors.IgnoreNotFound(s.d.Accounts.Delete(ctx, req.GetId())))
	}); err != nil {
		return nil, err
	}
	return &api.DeleteAccountResponse{}, nil
}

/*
	===== Passwords / verification =====
*/

func (s *IamService) ChangePassword(ctx context.Context, req *api.ChangePasswordRequest) (*api.ChangePasswordResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	hash, err := s.d.Credentials.GetPassword(ctx, c.GetAccountId())
	if err != nil || !s.d.Hasher.Verify(hash, req.GetOldPassword()) {
		return nil, status.Error(codes.PermissionDenied, "old password does not match")
	}
	newHash, err := s.d.Hasher.Hash(req.GetNewPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.d.Credentials.SetPassword(ctx, c.GetAccountId(), newHash); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ChangePasswordResponse{}, nil
}

func (s *IamService) ResetPassword(ctx context.Context, req *api.ResetPasswordRequest) (*api.ResetPasswordResponse, error) {
	if _, err := s.d.Accounts.Get(ctx, req.GetAccountId()); err != nil {
		return nil, utils.MapErr(err)
	}
	hash, err := s.d.Hasher.Hash(req.GetNewPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.d.Credentials.SetPassword(ctx, req.GetAccountId(), hash); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ResetPasswordResponse{}, nil
}

func (s *IamService) ResendVerification(ctx context.Context, _ *api.ResendVerificationRequest) (*api.ResendVerificationResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := s.d.Accounts.Get(ctx, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if acc.EmailVerified {
		return &api.ResendVerificationResponse{}, nil
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		return s.dispatchVerification(ctx, acc)
	}); err != nil {
		return nil, err
	}
	return &api.ResendVerificationResponse{}, nil
}

/*
	===== Tenants =====
*/

func (s *IamService) CreateTenant(ctx context.Context, req *api.CreateTenantRequest) (*api.CreateTenantResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if !c.GetIsAdmin() {
		allowed, err := s.d.Gates.MemberTenantCreationAllowed(ctx)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if !allowed {
			return nil, status.Error(codes.PermissionDenied, "tenant creation is disabled")
		}
	}
	return doTxRet(ctx, s, func(ctx context.Context) (*api.CreateTenantResponse, error) {
		if _, err := s.d.Tenants.GetBySlug(ctx, req.GetSlug()); err == nil {
			return nil, status.Error(codes.AlreadyExists, "slug already in use")
		} else if !errors.Is(err, derrors.ErrNotFound) {
			return nil, utils.MapErr(err)
		}

		tenant := &iam.Tenant{
			Id:             uuid.NewString(),
			Name:           req.GetName(),
			Slug:           req.GetSlug(),
			OwnerAccountId: c.GetAccountId(),
			CreatedAt:      s.now(),
			UpdatedAt:      s.now(),
		}
		if err := s.d.Tenants.Create(ctx, tenant); err != nil {
			return nil, utils.MapErr(err)
		}
		// Seed an owner role + membership so the creator can enter immediately.
		// It must cover the whole tenant API surface, matching first-boot seeding.
		perms, err := catalogManagePermissions(ctx, s.d.Catalog)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		ownerRole := &iam.Role{
			Id:          uuid.NewString(),
			Name:        "owner",
			Scope:       iam.Scope_SCOPE_TENANT,
			TenantId:    tenant.Id,
			IsSystem:    true,
			Permissions: perms,
			CreatedAt:   s.now(),
			UpdatedAt:   s.now(),
		}
		if err := s.d.Roles.Create(ctx, ownerRole); err != nil {
			return nil, utils.MapErr(err)
		}
		membership := &iam.Membership{
			Id:        uuid.NewString(),
			AccountId: c.GetAccountId(),
			TenantId:  tenant.Id,
			RoleIds:   []string{ownerRole.Id},
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		}
		if err := s.d.Memberships.Create(ctx, membership); err != nil {
			return nil, utils.MapErr(err)
		}
		// Seed the tenant's org catalog with a LINKED row per LEVEL_INSTANCE
		// entry, in the same transaction as the role/membership seeding above
		// — nil-guarded so every IamDeps literal that predates this field
		// (across the existing test suite and any not-yet-updated wiring)
		// keeps compiling and behaving unchanged.
		if s.d.CatalogSeeder != nil {
			if err := s.d.CatalogSeeder.SeedOrgCatalog(ctx, tenant.Id); err != nil {
				return nil, utils.MapErr(err)
			}
		}
		return &api.CreateTenantResponse{Tenant: tenant}, nil
	})
}

func (s *IamService) GetTenant(ctx context.Context, req *api.GetTenantRequest) (*api.GetTenantResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	var (
		tenant *iam.Tenant
	)
	switch {
	case req.GetId() != "":
		tenant, err = s.d.Tenants.Get(ctx, req.GetId())
	case req.GetSlug() != "":
		tenant, err = s.d.Tenants.GetBySlug(ctx, req.GetSlug())
	default:
		return nil, status.Error(codes.InvalidArgument, "id or slug required")
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if !c.GetIsAdmin() {
		if _, err := s.d.Memberships.GetByAccountTenant(ctx, c.GetAccountId(), tenant.GetId()); err != nil {
			if errors.Is(err, derrors.ErrNotFound) {
				return nil, status.Error(codes.PermissionDenied, "not a member of tenant")
			}
			return nil, utils.MapErr(err)
		}
	}
	return &api.GetTenantResponse{Tenant: tenant}, nil
}

func (s *IamService) ListMyTenants(ctx context.Context, _ *api.ListMyTenantsRequest) (*api.ListMyTenantsResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	tenants, err := s.d.Tenants.ListByMember(ctx, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListMyTenantsResponse{Tenants: tenants}, nil
}

func (s *IamService) UpdateTenant(ctx context.Context, req *api.UpdateTenantRequest) (*api.UpdateTenantResponse, error) {
	tenant, err := s.d.Tenants.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if req.Name != nil {
		tenant.Name = req.GetName()
	}
	if req.Slug != nil && req.GetSlug() != tenant.Slug {
		if other, err := s.d.Tenants.GetBySlug(ctx, req.GetSlug()); err == nil && other.Id != tenant.Id {
			return nil, status.Error(codes.AlreadyExists, "slug already in use")
		} else if err != nil && !errors.Is(err, derrors.ErrNotFound) {
			return nil, utils.MapErr(err)
		}
		tenant.Slug = req.GetSlug()
	}
	tenant.UpdatedAt = s.now()
	if err := s.d.Tenants.Update(ctx, tenant); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.UpdateTenantResponse{Tenant: tenant}, nil
}

func (s *IamService) DeleteTenant(ctx context.Context, req *api.DeleteTenantRequest) (*api.DeleteTenantResponse, error) {
	if err := s.doTx(ctx, func(ctx context.Context) error {
		if err := derrors.IgnoreNotFound(s.d.Memberships.DeleteByTenant(ctx, req.GetId())); err != nil {
			return utils.MapErr(err)
		}
		if err := derrors.IgnoreNotFound(s.d.Roles.DeleteByTenant(ctx, req.GetId())); err != nil {
			return utils.MapErr(err)
		}
		return utils.MapErr(derrors.IgnoreNotFound(s.d.Tenants.Delete(ctx, req.GetId())))
	}); err != nil {
		return nil, err
	}
	return &api.DeleteTenantResponse{}, nil
}

func (s *IamService) TransferTenantOwnership(ctx context.Context, req *api.TransferTenantOwnershipRequest) (*api.TransferTenantOwnershipResponse, error) {
	tenant, err := s.d.Tenants.Get(ctx, req.GetTenantId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if tenant.OwnerAccountId == req.GetNewOwnerAccountId() {
		return &api.TransferTenantOwnershipResponse{Tenant: tenant}, nil
	}
	if _, err := s.d.Memberships.GetByAccountTenant(ctx, req.GetNewOwnerAccountId(), tenant.Id); err != nil {
		if errors.Is(err, derrors.ErrNotFound) {
			return nil, status.Error(codes.FailedPrecondition, "new owner must be a member of the tenant")
		}
		return nil, utils.MapErr(err)
	}
	tenant.OwnerAccountId = req.GetNewOwnerAccountId()
	tenant.UpdatedAt = s.now()
	if err := s.d.Tenants.Update(ctx, tenant); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.TransferTenantOwnershipResponse{Tenant: tenant}, nil
}

func (s *IamService) LeaveTenant(ctx context.Context, req *api.LeaveTenantRequest) (*api.LeaveTenantResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	tenant, err := s.d.Tenants.Get(ctx, req.GetTenantId())
	if err != nil {
		if errors.Is(err, derrors.ErrNotFound) {
			return &api.LeaveTenantResponse{}, nil
		}
		return nil, utils.MapErr(err)
	}
	if tenant.OwnerAccountId == c.GetAccountId() {
		return nil, status.Error(codes.FailedPrecondition, "owner must transfer ownership before leaving")
	}
	m, err := s.d.Memberships.GetByAccountTenant(ctx, c.GetAccountId(), tenant.Id)
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.LeaveTenantResponse{}, nil
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if err := derrors.IgnoreNotFound(s.d.Memberships.Delete(ctx, m.Id)); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.LeaveTenantResponse{}, nil
}

/*
	===== Roles =====
*/

func (s *IamService) CreateRole(ctx context.Context, req *api.CreateRoleRequest) (*api.CreateRoleResponse, error) {
	if err := validateScopeTenant(req.GetScope(), req.GetTenantId()); err != nil {
		return nil, err
	}
	role := &iam.Role{
		Id:          uuid.NewString(),
		Name:        req.GetName(),
		Scope:       req.GetScope(),
		TenantId:    req.GetTenantId(),
		Permissions: req.GetPermissions(),
		IsSystem:    false,
		CreatedAt:   s.now(),
		UpdatedAt:   s.now(),
	}
	if err := s.d.Roles.Create(ctx, role); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateRoleResponse{Role: role}, nil
}

func validateScopeTenant(scope iam.Scope, tenantID string) error {
	switch scope {
	case iam.Scope_SCOPE_TENANT:
		if tenantID == "" {
			return status.Error(codes.InvalidArgument, "tenant_id required for SCOPE_TENANT role")
		}
	case iam.Scope_SCOPE_PLATFORM:
		if tenantID != "" {
			return status.Error(codes.InvalidArgument, "tenant_id must be empty for SCOPE_PLATFORM role")
		}
	default:
		return status.Error(codes.InvalidArgument, "scope required")
	}
	return nil
}

func (s *IamService) canReadAccount(ctx context.Context, c *iam.AccessClaims, accountID string) error {
	if selfOrAdmin(c, accountID) {
		return nil
	}
	shared, err := s.d.Memberships.ShareTenant(ctx, c.GetAccountId(), accountID)
	if err != nil {
		return utils.MapErr(err)
	}
	if !shared {
		return status.Error(codes.PermissionDenied, "not allowed to view this account")
	}
	return nil
}

func (s *IamService) GetRole(ctx context.Context, req *api.GetRoleRequest) (*api.GetRoleResponse, error) {
	role, err := s.d.Roles.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetRoleResponse{Role: role}, nil
}

func (s *IamService) ListRoles(ctx context.Context, req *api.ListRolesRequest) (*api.ListRolesResponse, error) {
	roles, next, err := s.d.Roles.List(ctx, req.GetTenantId(), req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListRolesResponse{Roles: roles, NextPageToken: next}, nil
}

func (s *IamService) UpdateRole(ctx context.Context, req *api.UpdateRoleRequest) (*api.UpdateRoleResponse, error) {
	role, err := s.d.Roles.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if role.IsSystem {
		return nil, status.Error(codes.FailedPrecondition, "system roles cannot be edited")
	}
	if req.Name != nil {
		role.Name = req.GetName()
	}
	if req.Permissions != nil {
		role.Permissions = req.GetPermissions()
	}
	role.UpdatedAt = s.now()
	if err := s.d.Roles.Update(ctx, role); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.UpdateRoleResponse{Role: role}, nil
}

func (s *IamService) DeleteRole(ctx context.Context, req *api.DeleteRoleRequest) (*api.DeleteRoleResponse, error) {
	role, err := s.d.Roles.Get(ctx, req.GetId())
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.DeleteRoleResponse{}, nil
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if role.IsSystem {
		return nil, status.Error(codes.FailedPrecondition, "system roles cannot be deleted")
	}
	inUse, err := s.d.Memberships.RoleInUse(ctx, role.Id)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if inUse {
		return nil, status.Error(codes.FailedPrecondition, "role is still assigned to a membership")
	}
	if err := derrors.IgnoreNotFound(s.d.Roles.Delete(ctx, role.Id)); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteRoleResponse{}, nil
}

/*
	===== Memberships =====
*/

func (s *IamService) CreateMembership(ctx context.Context, req *api.CreateMembershipRequest) (*api.CreateMembershipResponse, error) {
	if _, err := s.d.Accounts.Get(ctx, req.GetAccountId()); err != nil {
		return nil, utils.MapErr(err)
	}
	if _, err := s.d.Tenants.Get(ctx, req.GetTenantId()); err != nil {
		return nil, utils.MapErr(err)
	}
	if err := s.validateMembershipRoles(ctx, req.GetTenantId(), req.GetRoleIds()); err != nil {
		return nil, err
	}
	m := &iam.Membership{
		Id:        uuid.NewString(),
		AccountId: req.GetAccountId(),
		TenantId:  req.GetTenantId(),
		RoleIds:   req.GetRoleIds(),
		CreatedAt: s.now(),
		UpdatedAt: s.now(),
	}
	if err := s.d.Memberships.Create(ctx, m); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateMembershipResponse{Membership: m}, nil
}

// validateMembershipRoles checks each role is a SCOPE_PLATFORM role or a
// SCOPE_TENANT role of this same tenant — never a foreign tenant's role.
func (s *IamService) validateMembershipRoles(ctx context.Context, tenantID string, roleIDs []string) error {
	roles, err := s.d.Roles.GetMany(ctx, roleIDs)
	if err != nil {
		return utils.MapErr(err)
	}
	if len(roles) != len(roleIDs) {
		return status.Error(codes.InvalidArgument, "one or more roles do not exist")
	}
	for _, r := range roles {
		if r.GetScope() == iam.Scope_SCOPE_TENANT && r.GetTenantId() != tenantID {
			return status.Error(codes.InvalidArgument, "role belongs to another tenant")
		}
	}
	return nil
}

func (s *IamService) GetMembership(ctx context.Context, req *api.GetMembershipRequest) (*api.GetMembershipResponse, error) {
	m, err := s.d.Memberships.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetMembershipResponse{Membership: m}, nil
}

func (s *IamService) ListMemberships(ctx context.Context, req *api.ListMembershipsRequest) (*api.ListMembershipsResponse, error) {
	ms, next, err := s.d.Memberships.List(ctx, req.GetTenantId(), req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListMembershipsResponse{Memberships: ms, NextPageToken: next}, nil
}

func (s *IamService) UpdateMembership(ctx context.Context, req *api.UpdateMembershipRequest) (*api.UpdateMembershipResponse, error) {
	m, err := s.d.Memberships.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if err := s.validateMembershipRoles(ctx, m.TenantId, req.GetRoleIds()); err != nil {
		return nil, err
	}
	m.RoleIds = req.GetRoleIds()
	m.UpdatedAt = s.now()
	if err := s.d.Memberships.Update(ctx, m); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.UpdateMembershipResponse{Membership: m}, nil
}

func (s *IamService) DeleteMembership(ctx context.Context, req *api.DeleteMembershipRequest) (*api.DeleteMembershipResponse, error) {
	m, err := s.d.Memberships.Get(ctx, req.GetId())
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.DeleteMembershipResponse{}, nil
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	tenant, err := s.d.Tenants.Get(ctx, m.GetTenantId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if tenant.GetOwnerAccountId() == m.GetAccountId() {
		return nil, status.Error(codes.FailedPrecondition, "owner membership cannot be removed; transfer ownership first")
	}
	if err := derrors.IgnoreNotFound(s.d.Memberships.Delete(ctx, req.GetId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteMembershipResponse{}, nil
}

func (s *IamService) GetMyPermissions(ctx context.Context, req *api.GetMyPermissionsRequest) (*api.GetMyPermissionsResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	perms, err := s.d.Authz.EffectivePermissions(ctx, c.GetAccountId(), req.GetTenantId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetMyPermissionsResponse{Permissions: perms}, nil
}

func (s *IamService) ListPermissions(ctx context.Context, _ *api.ListPermissionsRequest) (*api.ListPermissionsResponse, error) {
	entries, err := s.d.Catalog.Grantable(ctx)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListPermissionsResponse{Entries: entries}, nil
}

/*
	===== SSO: provider config (admin-only) =====
*/

func (s *IamService) CreateIdentityProvider(ctx context.Context, req *api.CreateIdentityProviderRequest) (*api.CreateIdentityProviderResponse, error) {
	if req.GetAutoProvision() && len(req.GetAllowedDomains()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "auto_provision requires allowed_domains")
	}
	p := &iam.IdentityProvider{
		Id:             uuid.NewString(),
		Slug:           req.GetSlug(),
		DisplayName:    req.GetDisplayName(),
		Issuer:         req.GetIssuer(),
		ClientId:       req.GetClientId(),
		Scopes:         req.GetScopes(),
		AllowedDomains: req.GetAllowedDomains(),
		AutoProvision:  req.GetAutoProvision(),
		Disabled:       false,
		CreatedAt:      s.now(),
		UpdatedAt:      s.now(),
	}
	if err := s.d.Providers.Create(ctx, p); err != nil {
		return nil, utils.MapErr(err)
	}
	if err := s.d.ProviderSecrets.Set(ctx, p.Id, req.GetClientSecret()); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateIdentityProviderResponse{Provider: p}, nil
}

func (s *IamService) GetIdentityProvider(ctx context.Context, req *api.GetIdentityProviderRequest) (*api.GetIdentityProviderResponse, error) {
	p, err := s.d.Providers.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetIdentityProviderResponse{Provider: p}, nil
}

func (s *IamService) UpdateIdentityProvider(ctx context.Context, req *api.UpdateIdentityProviderRequest) (*api.UpdateIdentityProviderResponse, error) {
	p, err := s.d.Providers.Get(ctx, req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if req.DisplayName != nil {
		p.DisplayName = req.GetDisplayName()
	}
	if req.Issuer != nil {
		p.Issuer = req.GetIssuer()
	}
	if req.ClientId != nil {
		p.ClientId = req.GetClientId()
	}
	if req.Scopes != nil {
		p.Scopes = req.GetScopes()
	}
	if req.AllowedDomains != nil {
		p.AllowedDomains = req.GetAllowedDomains()
	}
	if req.AutoProvision != nil {
		p.AutoProvision = req.GetAutoProvision()
	}
	if req.Disabled != nil {
		p.Disabled = req.GetDisabled()
	}
	if p.AutoProvision && len(p.AllowedDomains) == 0 {
		return nil, status.Error(codes.InvalidArgument, "auto_provision requires allowed_domains")
	}
	p.UpdatedAt = s.now()
	if err := s.d.Providers.Update(ctx, p); err != nil {
		return nil, utils.MapErr(err)
	}
	if req.ClientSecret != nil && req.GetClientSecret() != "" {
		if err := s.d.ProviderSecrets.Set(ctx, p.Id, req.GetClientSecret()); err != nil {
			return nil, utils.MapErr(err)
		}
	}
	return &api.UpdateIdentityProviderResponse{Provider: p}, nil
}

func (s *IamService) DeleteIdentityProvider(ctx context.Context, req *api.DeleteIdentityProviderRequest) (*api.DeleteIdentityProviderResponse, error) {
	if err := derrors.IgnoreNotFound(s.d.ProviderSecrets.Delete(ctx, req.GetId())); err != nil {
		return nil, utils.MapErr(err)
	}
	if err := derrors.IgnoreNotFound(s.d.Providers.Delete(ctx, req.GetId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteIdentityProviderResponse{}, nil
}

/*
	===== SSO: login flow (public) =====
*/

func (s *IamService) ListIdentityProviders(ctx context.Context, _ *api.ListIdentityProvidersRequest) (*api.ListIdentityProvidersResponse, error) {
	providers, err := s.d.Providers.ListEnabled(ctx)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	buttons := make([]*api.SsoButton, 0, len(providers))
	for _, p := range providers {
		buttons = append(buttons, &api.SsoButton{Id: p.Id, Slug: p.Slug, DisplayName: p.DisplayName})
	}
	return &api.ListIdentityProvidersResponse{Buttons: buttons}, nil
}

func (s *IamService) StartSSO(ctx context.Context, req *api.StartSSORequest) (*api.StartSSOResponse, error) {
	p, err := s.d.Providers.Get(ctx, req.GetProviderId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if p.Disabled {
		return nil, status.Error(codes.FailedPrecondition, "provider is disabled")
	}
	redirectURL, state, err := s.d.SSO.Authorize(ctx, p)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &api.StartSSOResponse{RedirectUrl: redirectURL, State: state}, nil
}

func (s *IamService) CompleteSSO(ctx context.Context, req *api.CompleteSSORequest) (*api.CompleteSSOResponse, error) {
	p, err := s.d.Providers.Get(ctx, req.GetProviderId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if p.Disabled {
		return nil, status.Error(codes.FailedPrecondition, "provider is disabled")
	}
	secret, err := s.d.ProviderSecrets.Get(ctx, p.Id)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	subject, email, emailVerified, err := s.d.SSO.Exchange(ctx, p, secret, req.GetCode(), req.GetState())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	var accountID string
	ext, err := s.d.ExternalIdentities.GetByProviderSubject(ctx, p.Id, subject)
	switch {
	case err == nil:
		accountID = ext.GetAccountId()
	case errors.Is(err, derrors.ErrNotFound):
		acc, perr := s.provisionSSOAccount(ctx, p, subject, email, emailVerified)
		if perr != nil {
			return nil, perr
		}
		accountID = acc.Id
	default:
		return nil, utils.MapErr(err)
	}

	acc, err := s.d.Accounts.Get(ctx, accountID)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	pair, err := s.d.Tokens.IssuePair(ctx, acc.Id, acc.IsAdmin)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &api.CompleteSSOResponse{Tokens: pair}, nil
}

// provisionSSOAccount JIT-creates an account + link on first SSO login, gated by
// the provider's auto_provision flag and allowed_domains. It refuses to provision
// when the IdP has not verified the email, or when the email already belongs to
// an account — that user must sign in with their existing credentials and link
// the provider explicitly, so a provider cannot mint a duplicate over a known
// address.
func (s *IamService) provisionSSOAccount(ctx context.Context, p *iam.IdentityProvider, subject, email string, emailVerified bool) (*iam.Account, error) {
	if !p.AutoProvision {
		return nil, status.Error(codes.PermissionDenied, "no account linked to this identity")
	}
	if !emailVerified {
		return nil, status.Error(codes.PermissionDenied, "provider did not verify the email address")
	}
	if !domainAllowed(email, p.AllowedDomains) {
		return nil, status.Error(codes.PermissionDenied, "email domain not allowed for this provider")
	}
	if _, err := s.d.Accounts.GetByEmail(ctx, email); err == nil {
		return nil, status.Error(codes.FailedPrecondition, "an account with this email already exists; sign in and link the provider")
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}
	id := uuid.NewString()
	acc := &iam.Account{
		Id:            id,
		Email:         email,
		Nickname:      sanitizeNickname(email, id),
		EmailVerified: true,
		CreatedAt:     s.now(),
		UpdatedAt:     s.now(),
	}
	ext := &iam.ExternalIdentity{
		Id:         uuid.NewString(),
		ProviderId: p.Id,
		Subject:    subject,
		AccountId:  acc.Id,
		Email:      email,
		CreatedAt:  s.now(),
		UpdatedAt:  s.now(),
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		if err := s.d.Accounts.Create(ctx, acc); err != nil {
			return utils.MapErr(err)
		}
		return utils.MapErr(s.d.ExternalIdentities.Create(ctx, ext))
	}); err != nil {
		return nil, err
	}
	return acc, nil
}

/*
	===== SSO: identity links =====
*/

func (s *IamService) LinkExternalIdentity(ctx context.Context, req *api.LinkExternalIdentityRequest) (*api.LinkExternalIdentityResponse, error) {
	link := req.GetLink()
	if existing, err := s.d.ExternalIdentities.GetByProviderSubject(ctx, link.GetProviderId(), link.GetSubject()); err == nil {
		if existing.GetAccountId() != req.GetAccountId() {
			return nil, status.Error(codes.AlreadyExists, "identity already linked to another account")
		}
		return &api.LinkExternalIdentityResponse{Identity: existing}, nil
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return nil, utils.MapErr(err)
	}
	if _, err := s.d.Accounts.Get(ctx, req.GetAccountId()); err != nil {
		return nil, utils.MapErr(err)
	}
	ext := &iam.ExternalIdentity{
		Id:         uuid.NewString(),
		ProviderId: link.GetProviderId(),
		Subject:    link.GetSubject(),
		AccountId:  req.GetAccountId(),
		Email:      link.GetEmail(),
		CreatedAt:  s.now(),
		UpdatedAt:  s.now(),
	}
	if err := s.d.ExternalIdentities.Create(ctx, ext); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.LinkExternalIdentityResponse{Identity: ext}, nil
}

func (s *IamService) UnlinkExternalIdentity(ctx context.Context, req *api.UnlinkExternalIdentityRequest) (*api.UnlinkExternalIdentityResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	ext, err := s.d.ExternalIdentities.Get(ctx, req.GetId())
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.UnlinkExternalIdentityResponse{}, nil
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if !selfOrAdmin(c, ext.AccountId) {
		return nil, status.Error(codes.PermissionDenied, "not allowed to unlink this identity")
	}
	if err := derrors.IgnoreNotFound(s.d.ExternalIdentities.Delete(ctx, ext.Id)); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.UnlinkExternalIdentityResponse{}, nil
}

func (s *IamService) ListExternalIdentities(ctx context.Context, req *api.ListExternalIdentitiesRequest) (*api.ListExternalIdentitiesResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if !selfOrAdmin(c, req.GetAccountId()) {
		return nil, status.Error(codes.PermissionDenied, "not allowed to view these identities")
	}
	ids, err := s.d.ExternalIdentities.ListByAccount(ctx, req.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListExternalIdentitiesResponse{Identities: ids}, nil
}
