package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

/*
	===== IAM Store accessors =====

	Every IAM record is persisted as a protojson blob keyed by id, with the
	queryable envelope fields (email/nickname, slug, account/tenant/provider ids,
	subject, purpose, ...) mirrored into indexed columns so the repos can satisfy
	their lookup-by-secondary-key methods without scanning blobs. Repo calls go
	through the sqld-generated query set bound to db.TxDB so they pick up the
	ambient transaction.
*/

// Accounts returns the iam.AccountRepo.
func (s *Store) Accounts() *AccountRepo { return &AccountRepo{db: s.db} }

// Credentials returns the iam.CredentialStore.
func (s *Store) Credentials() *CredentialRepo { return &CredentialRepo{db: s.db} }

// ApiTokens returns the iam.ApiTokenRepo.
func (s *Store) ApiTokens() *ApiTokenRepo { return &ApiTokenRepo{db: s.db} }

// Tenants returns the iam.TenantRepo.
func (s *Store) Tenants() *TenantRepo { return &TenantRepo{db: s.db} }

// Roles returns the iam.RoleRepo.
func (s *Store) Roles() *RoleRepo { return &RoleRepo{db: s.db} }

// Memberships returns the iam.MembershipRepo.
func (s *Store) Memberships() *MembershipRepo { return &MembershipRepo{db: s.db} }

// IdentityProviders returns the iam.IdentityProviderRepo.
func (s *Store) IdentityProviders() *IdentityProviderRepo { return &IdentityProviderRepo{db: s.db} }

// ExternalIdentities returns the iam.ExternalIdentityRepo.
func (s *Store) ExternalIdentities() *ExternalIdentityRepo { return &ExternalIdentityRepo{db: s.db} }

// OneTimeTokens returns the iam.OneTimeTokens store.
func (s *Store) OneTimeTokens() *OneTimeTokenRepo { return &OneTimeTokenRepo{db: s.db} }

/*
	===== AccountRepo =====
*/

type AccountRepo struct{ db *DB }

var _ iamsvc.AccountRepo = (*AccountRepo)(nil)

func (r *AccountRepo) Create(ctx context.Context, account *iam.Account) error {
	data, err := marshal(account)
	if err != nil {
		return err
	}
	err = r.db.q().CreateIamAccount(ctx, dbgen.CreateIamAccountParams{
		ID:       account.GetId(),
		Email:    account.GetEmail(),
		Nickname: account.GetNickname(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("account", "account already exists")
		}
		return err
	}
	return nil
}

func (r *AccountRepo) Get(ctx context.Context, id string) (*iam.Account, error) {
	row, err := r.db.q().GetIamAccount(ctx, id)
	if err != nil {
		return nil, translatePgErr("account", err)
	}
	return decodeAccount(row.Data)
}

func (r *AccountRepo) GetByEmail(ctx context.Context, email string) (*iam.Account, error) {
	row, err := r.db.q().GetIamAccountByEmail(ctx, email)
	if err != nil {
		return nil, translatePgErr("account", err)
	}
	return decodeAccount(row.Data)
}

func (r *AccountRepo) GetByNickname(ctx context.Context, nickname string) (*iam.Account, error) {
	row, err := r.db.q().GetIamAccountByNickname(ctx, nickname)
	if err != nil {
		return nil, translatePgErr("account", err)
	}
	return decodeAccount(row.Data)
}

func (r *AccountRepo) List(ctx context.Context, _ uint32, _ string) ([]*iam.Account, string, error) {
	rows, err := r.db.q().ListIamAccounts(ctx)
	if err != nil {
		return nil, "", err
	}
	out := make([]*iam.Account, 0, len(rows))
	for _, row := range rows {
		acc, err := decodeAccount(row.Data)
		if err != nil {
			return nil, "", err
		}
		out = append(out, acc)
	}
	return out, "", nil
}

func (r *AccountRepo) Update(ctx context.Context, account *iam.Account) error {
	data, err := marshal(account)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateIamAccount(ctx, dbgen.UpdateIamAccountParams{
		Email:    account.GetEmail(),
		Nickname: account.GetNickname(),
		Data:     data,
		ID:       account.GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("account", "account not found")
	}
	return nil
}

func (r *AccountRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteIamAccount(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("account", "account not found")
	}
	return nil
}

func decodeAccount(data []byte) (*iam.Account, error) {
	rec := &iam.Account{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

/*
	===== CredentialRepo =====
*/

type CredentialRepo struct{ db *DB }

var _ iamsvc.CredentialStore = (*CredentialRepo)(nil)

func (r *CredentialRepo) SetPassword(ctx context.Context, accountID, hash string) error {
	return r.db.q().UpsertIamCredential(ctx, dbgen.UpsertIamCredentialParams{
		AccountID: accountID,
		Hash:      hash,
	})
}

func (r *CredentialRepo) GetPassword(ctx context.Context, accountID string) (string, error) {
	row, err := r.db.q().GetIamCredential(ctx, accountID)
	if err != nil {
		return "", translatePgErr("credential", err)
	}
	return row.Hash, nil
}

func (r *CredentialRepo) DeletePassword(ctx context.Context, accountID string) error {
	n, err := r.db.q().DeleteIamCredential(ctx, accountID)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("credential", "credential not found")
	}
	return nil
}

/*
	===== ApiTokenRepo =====
*/

type ApiTokenRepo struct{ db *DB }

var _ iamsvc.ApiTokenRepo = (*ApiTokenRepo)(nil)

func (r *ApiTokenRepo) Create(ctx context.Context, token *iam.ApiToken) error {
	data, err := marshal(token)
	if err != nil {
		return err
	}
	err = r.db.q().CreateIamAPIToken(ctx, dbgen.CreateIamAPITokenParams{
		ID:        token.GetId(),
		AccountID: token.GetAccountId(),
		Data:      data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("api_token", "token already exists")
		}
		return err
	}
	return nil
}

func (r *ApiTokenRepo) Get(ctx context.Context, id string) (*iam.ApiToken, error) {
	row, err := r.db.q().GetIamAPIToken(ctx, id)
	if err != nil {
		return nil, translatePgErr("api_token", err)
	}
	rec := &iam.ApiToken{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *ApiTokenRepo) ListByAccount(ctx context.Context, accountID string) ([]*iam.ApiToken, error) {
	rows, err := r.db.q().ListIamAPITokensByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]*iam.ApiToken, 0, len(rows))
	for _, row := range rows {
		rec := &iam.ApiToken{}
		if err := unmarshal(row.Data, rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *ApiTokenRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteIamAPIToken(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("api_token", "token not found")
	}
	return nil
}

/*
	===== TenantRepo =====
*/

type TenantRepo struct{ db *DB }

var _ iamsvc.TenantRepo = (*TenantRepo)(nil)

func (r *TenantRepo) Create(ctx context.Context, tenant *iam.Tenant) error {
	data, err := marshal(tenant)
	if err != nil {
		return err
	}
	err = r.db.q().CreateIamTenant(ctx, dbgen.CreateIamTenantParams{
		ID:             tenant.GetId(),
		Slug:           tenant.GetSlug(),
		OwnerAccountID: tenant.GetOwnerAccountId(),
		Data:           data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("tenant", "tenant already exists")
		}
		return err
	}
	return nil
}

func (r *TenantRepo) Get(ctx context.Context, id string) (*iam.Tenant, error) {
	row, err := r.db.q().GetIamTenant(ctx, id)
	if err != nil {
		return nil, translatePgErr("tenant", err)
	}
	return decodeTenant(row.Data)
}

func (r *TenantRepo) GetBySlug(ctx context.Context, slug string) (*iam.Tenant, error) {
	row, err := r.db.q().GetIamTenantBySlug(ctx, slug)
	if err != nil {
		return nil, translatePgErr("tenant", err)
	}
	return decodeTenant(row.Data)
}

// ListByMember returns every tenant the account is a member of (via the
// membership join), unioned with tenants it owns.
func (r *TenantRepo) ListByMember(ctx context.Context, accountID string) ([]*iam.Tenant, error) {
	rows, err := r.db.q().ListIamTenantsByMember(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]*iam.Tenant, 0, len(rows))
	for _, row := range rows {
		t, err := decodeTenant(row.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *TenantRepo) CountOwnedBy(ctx context.Context, accountID string) (int, error) {
	row, err := r.db.q().CountIamTenantsOwnedBy(ctx, accountID)
	if err != nil {
		return 0, err
	}
	return int(row.Count), nil
}

func (r *TenantRepo) Update(ctx context.Context, tenant *iam.Tenant) error {
	data, err := marshal(tenant)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateIamTenant(ctx, dbgen.UpdateIamTenantParams{
		Slug:           tenant.GetSlug(),
		OwnerAccountID: tenant.GetOwnerAccountId(),
		Data:           data,
		ID:             tenant.GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("tenant", "tenant not found")
	}
	return nil
}

func (r *TenantRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteIamTenant(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("tenant", "tenant not found")
	}
	return nil
}

func decodeTenant(data []byte) (*iam.Tenant, error) {
	rec := &iam.Tenant{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

/*
	===== RoleRepo =====
*/

type RoleRepo struct{ db *DB }

var _ iamsvc.RoleRepo = (*RoleRepo)(nil)

func (r *RoleRepo) Create(ctx context.Context, role *iam.Role) error {
	data, err := marshal(role)
	if err != nil {
		return err
	}
	err = r.db.q().CreateIamRole(ctx, dbgen.CreateIamRoleParams{
		ID:       role.GetId(),
		TenantID: role.GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("role", "role already exists")
		}
		return err
	}
	return nil
}

func (r *RoleRepo) Get(ctx context.Context, id string) (*iam.Role, error) {
	row, err := r.db.q().GetIamRole(ctx, id)
	if err != nil {
		return nil, translatePgErr("role", err)
	}
	return decodeRole(row.Data)
}

func (r *RoleRepo) GetMany(ctx context.Context, ids []string) ([]*iam.Role, error) {
	if len(ids) == 0 {
		return []*iam.Role{}, nil
	}
	rows, err := r.db.q().GetManyIamRoles(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*iam.Role, 0, len(rows))
	for _, row := range rows {
		role, err := decodeRole(row.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, nil
}

func (r *RoleRepo) List(ctx context.Context, tenantID string, _ uint32, _ string) ([]*iam.Role, string, error) {
	rows, err := r.db.q().ListIamRoles(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	out := make([]*iam.Role, 0, len(rows))
	for _, row := range rows {
		role, err := decodeRole(row.Data)
		if err != nil {
			return nil, "", err
		}
		out = append(out, role)
	}
	return out, "", nil
}

func (r *RoleRepo) Update(ctx context.Context, role *iam.Role) error {
	data, err := marshal(role)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateIamRole(ctx, dbgen.UpdateIamRoleParams{
		Data: data,
		ID:   role.GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("role", "role not found")
	}
	return nil
}

func (r *RoleRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteIamRole(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("role", "role not found")
	}
	return nil
}

func (r *RoleRepo) DeleteByTenant(ctx context.Context, tenantID string) error {
	return r.db.q().DeleteIamRolesByTenant(ctx, tenantID)
}

func decodeRole(data []byte) (*iam.Role, error) {
	rec := &iam.Role{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

/*
	===== MembershipRepo =====
*/

type MembershipRepo struct{ db *DB }

var _ iamsvc.MembershipRepo = (*MembershipRepo)(nil)

func (r *MembershipRepo) Create(ctx context.Context, membership *iam.Membership) error {
	data, err := marshal(membership)
	if err != nil {
		return err
	}
	err = r.db.q().CreateIamMembership(ctx, dbgen.CreateIamMembershipParams{
		ID:        membership.GetId(),
		AccountID: membership.GetAccountId(),
		TenantID:  membership.GetTenantId(),
		Data:      data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("membership", "membership already exists")
		}
		return err
	}
	return nil
}

func (r *MembershipRepo) Get(ctx context.Context, id string) (*iam.Membership, error) {
	row, err := r.db.q().GetIamMembership(ctx, id)
	if err != nil {
		return nil, translatePgErr("membership", err)
	}
	return decodeMembership(row.Data)
}

func (r *MembershipRepo) GetByAccountTenant(ctx context.Context, accountID, tenantID string) (*iam.Membership, error) {
	row, err := r.db.q().GetIamMembershipByAccountTenant(ctx, dbgen.GetIamMembershipByAccountTenantParams{
		AccountID: accountID,
		TenantID:  tenantID,
	})
	if err != nil {
		return nil, translatePgErr("membership", err)
	}
	return decodeMembership(row.Data)
}

func (r *MembershipRepo) List(ctx context.Context, tenantID string, _ uint32, _ string) ([]*iam.Membership, string, error) {
	rows, err := r.db.q().ListIamMemberships(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	out := make([]*iam.Membership, 0, len(rows))
	for _, row := range rows {
		m, err := decodeMembership(row.Data)
		if err != nil {
			return nil, "", err
		}
		out = append(out, m)
	}
	return out, "", nil
}

func (r *MembershipRepo) Update(ctx context.Context, membership *iam.Membership) error {
	data, err := marshal(membership)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateIamMembership(ctx, dbgen.UpdateIamMembershipParams{
		Data: data,
		ID:   membership.GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("membership", "membership not found")
	}
	return nil
}

func (r *MembershipRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteIamMembership(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("membership", "membership not found")
	}
	return nil
}

func (r *MembershipRepo) DeleteByTenant(ctx context.Context, tenantID string) error {
	return r.db.q().DeleteIamMembershipsByTenant(ctx, tenantID)
}

func (r *MembershipRepo) DeleteByAccount(ctx context.Context, accountID string) error {
	return r.db.q().DeleteIamMembershipsByAccount(ctx, accountID)
}

func decodeMembership(data []byte) (*iam.Membership, error) {
	rec := &iam.Membership{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

/*
	===== IdentityProviderRepo =====
*/

type IdentityProviderRepo struct{ db *DB }

var _ iamsvc.IdentityProviderRepo = (*IdentityProviderRepo)(nil)

func (r *IdentityProviderRepo) Create(ctx context.Context, provider *iam.IdentityProvider) error {
	data, err := marshal(provider)
	if err != nil {
		return err
	}
	err = r.db.q().CreateIamIdentityProvider(ctx, dbgen.CreateIamIdentityProviderParams{
		ID:      provider.GetId(),
		Enabled: !provider.GetDisabled(),
		Data:    data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("identity_provider", "provider already exists")
		}
		return err
	}
	return nil
}

func (r *IdentityProviderRepo) Get(ctx context.Context, id string) (*iam.IdentityProvider, error) {
	row, err := r.db.q().GetIamIdentityProvider(ctx, id)
	if err != nil {
		return nil, translatePgErr("identity_provider", err)
	}
	return decodeProvider(row.Data)
}

func (r *IdentityProviderRepo) ListEnabled(ctx context.Context) ([]*iam.IdentityProvider, error) {
	rows, err := r.db.q().ListEnabledIamIdentityProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*iam.IdentityProvider, 0, len(rows))
	for _, row := range rows {
		p, err := decodeProvider(row.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *IdentityProviderRepo) Update(ctx context.Context, provider *iam.IdentityProvider) error {
	data, err := marshal(provider)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateIamIdentityProvider(ctx, dbgen.UpdateIamIdentityProviderParams{
		Enabled: !provider.GetDisabled(),
		Data:    data,
		ID:      provider.GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("identity_provider", "provider not found")
	}
	return nil
}

func (r *IdentityProviderRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteIamIdentityProvider(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("identity_provider", "provider not found")
	}
	return nil
}

func decodeProvider(data []byte) (*iam.IdentityProvider, error) {
	rec := &iam.IdentityProvider{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

/*
	===== ExternalIdentityRepo =====
*/

type ExternalIdentityRepo struct{ db *DB }

var _ iamsvc.ExternalIdentityRepo = (*ExternalIdentityRepo)(nil)

func (r *ExternalIdentityRepo) Create(ctx context.Context, identity *iam.ExternalIdentity) error {
	data, err := marshal(identity)
	if err != nil {
		return err
	}
	err = r.db.q().CreateIamExternalIdentity(ctx, dbgen.CreateIamExternalIdentityParams{
		ID:         identity.GetId(),
		ProviderID: identity.GetProviderId(),
		Subject:    identity.GetSubject(),
		AccountID:  identity.GetAccountId(),
		Data:       data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("external_identity", "identity already exists")
		}
		return err
	}
	return nil
}

func (r *ExternalIdentityRepo) Get(ctx context.Context, id string) (*iam.ExternalIdentity, error) {
	row, err := r.db.q().GetIamExternalIdentity(ctx, id)
	if err != nil {
		return nil, translatePgErr("external_identity", err)
	}
	return decodeExternalIdentity(row.Data)
}

func (r *ExternalIdentityRepo) GetByProviderSubject(ctx context.Context, providerID, subject string) (*iam.ExternalIdentity, error) {
	row, err := r.db.q().GetIamExternalIdentityByProviderSubject(ctx, dbgen.GetIamExternalIdentityByProviderSubjectParams{
		ProviderID: providerID,
		Subject:    subject,
	})
	if err != nil {
		return nil, translatePgErr("external_identity", err)
	}
	return decodeExternalIdentity(row.Data)
}

func (r *ExternalIdentityRepo) ListByAccount(ctx context.Context, accountID string) ([]*iam.ExternalIdentity, error) {
	rows, err := r.db.q().ListIamExternalIdentitiesByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]*iam.ExternalIdentity, 0, len(rows))
	for _, row := range rows {
		ei, err := decodeExternalIdentity(row.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, ei)
	}
	return out, nil
}

func (r *ExternalIdentityRepo) Delete(ctx context.Context, id string) error {
	n, err := r.db.q().DeleteIamExternalIdentity(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("external_identity", "identity not found")
	}
	return nil
}

func (r *ExternalIdentityRepo) DeleteByAccount(ctx context.Context, accountID string) error {
	return r.db.q().DeleteIamExternalIdentitiesByAccount(ctx, accountID)
}

func decodeExternalIdentity(data []byte) (*iam.ExternalIdentity, error) {
	rec := &iam.ExternalIdentity{}
	if err := unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

/*
	===== OneTimeTokenRepo =====

	Single-use, time-bounded tokens. Mint stores a fresh random token; Consume
	atomically deletes the matching live row and returns its account (single-use).
*/

type OneTimeTokenRepo struct{ db *DB }

var _ iamsvc.OneTimeTokens = (*OneTimeTokenRepo)(nil)

func (r *OneTimeTokenRepo) Mint(ctx context.Context, purpose, accountID string, ttl time.Duration) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	err := r.db.q().CreateIamOneTimeToken(ctx, dbgen.CreateIamOneTimeTokenParams{
		Token:     token,
		Purpose:   purpose,
		AccountID: accountID,
		ExpiresAt: time.Now().Add(ttl),
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Consume deletes the token row matching (purpose, token) and returns its
// account id. The row is removed regardless of expiry (single-use, so a replayed
// token is gone). An unknown / expired token is reported as not-found.
func (r *OneTimeTokenRepo) Consume(ctx context.Context, purpose, token string) (string, error) {
	row, err := r.db.q().ConsumeIamOneTimeToken(ctx, dbgen.ConsumeIamOneTimeTokenParams{
		Purpose: purpose,
		Token:   token,
	})
	if err != nil {
		return "", translatePgErr("one_time_token", err)
	}
	if time.Now().After(row.ExpiresAt) {
		return "", derrors.NotFound("one_time_token", "token expired")
	}
	return row.AccountID, nil
}
