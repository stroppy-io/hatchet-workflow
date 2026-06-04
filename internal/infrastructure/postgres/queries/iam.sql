-- ===== iam_accounts (iam.go AccountRepo) =====

-- name: CreateIamAccount :exec
insert into iam_accounts (id, email, nickname, created_at, updated_at, data)
values (@id, @email, @nickname, now(), now(), @data);

-- name: GetIamAccount :one
select data from iam_accounts where id = @id;

-- name: GetIamAccountByEmail :one
select data from iam_accounts where email = @email;

-- name: GetIamAccountByNickname :one
select data from iam_accounts where nickname = @nickname;

-- name: ListIamAccounts :many
select data from iam_accounts;

-- name: UpdateIamAccount :execrows
update iam_accounts set email = @email, nickname = @nickname, data = @data, updated_at = now()
where id = @id;

-- name: DeleteIamAccount :execrows
delete from iam_accounts where id = @id;

-- ===== iam_credentials (iam.go CredentialRepo) =====

-- name: UpsertIamCredential :exec
insert into iam_credentials (account_id, hash, updated_at)
values (@account_id, @hash, now())
on conflict (account_id) do update set hash = excluded.hash, updated_at = now();

-- name: GetIamCredential :one
select hash from iam_credentials where account_id = @account_id;

-- name: DeleteIamCredential :execrows
delete from iam_credentials where account_id = @account_id;

-- ===== iam_api_tokens (iam.go ApiTokenRepo) =====

-- name: CreateIamApiToken :exec
insert into iam_api_tokens (id, account_id, created_at, updated_at, data)
values (@id, @account_id, now(), now(), @data);

-- name: GetIamApiToken :one
select data from iam_api_tokens where id = @id;

-- name: ListIamApiTokensByAccount :many
select data from iam_api_tokens where account_id = @account_id;

-- name: DeleteIamApiToken :execrows
delete from iam_api_tokens where id = @id;

-- ===== iam_tenants (iam.go TenantRepo) =====

-- name: CreateIamTenant :exec
insert into iam_tenants (id, slug, owner_account_id, created_at, updated_at, data)
values (@id, @slug, @owner_account_id, now(), now(), @data);

-- name: GetIamTenant :one
select data from iam_tenants where id = @id;

-- name: GetIamTenantBySlug :one
select data from iam_tenants where slug = @slug;

-- name: ListIamTenantsByMember :many
select data from iam_tenants
where owner_account_id = @account_id
   or id in (select tenant_id from iam_memberships where account_id = @account_id);

-- name: CountIamTenantsOwnedBy :one
select count(*) from iam_tenants where owner_account_id = @account_id;

-- name: UpdateIamTenant :execrows
update iam_tenants set slug = @slug, owner_account_id = @owner_account_id, data = @data, updated_at = now()
where id = @id;

-- name: DeleteIamTenant :execrows
delete from iam_tenants where id = @id;

-- ===== iam_roles (iam.go RoleRepo) =====

-- name: CreateIamRole :exec
insert into iam_roles (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetIamRole :one
select data from iam_roles where id = @id;

-- name: GetManyIamRoles :many
select data from iam_roles where id = any(@ids::text[]);

-- name: ListIamRoles :many
select data from iam_roles where tenant_id = @tenant_id;

-- name: UpdateIamRole :execrows
update iam_roles set data = @data, updated_at = now() where id = @id;

-- name: DeleteIamRole :execrows
delete from iam_roles where id = @id;

-- name: DeleteIamRolesByTenant :exec
delete from iam_roles where tenant_id = @tenant_id;

-- ===== iam_memberships (iam.go MembershipRepo) =====

-- name: CreateIamMembership :exec
insert into iam_memberships (id, account_id, tenant_id, created_at, updated_at, data)
values (@id, @account_id, @tenant_id, now(), now(), @data);

-- name: GetIamMembership :one
select data from iam_memberships where id = @id;

-- name: GetIamMembershipByAccountTenant :one
select data from iam_memberships where account_id = @account_id and tenant_id = @tenant_id;

-- name: ListIamMemberships :many
select data from iam_memberships where tenant_id = @tenant_id;

-- name: UpdateIamMembership :execrows
update iam_memberships set data = @data, updated_at = now() where id = @id;

-- name: DeleteIamMembership :execrows
delete from iam_memberships where id = @id;

-- name: DeleteIamMembershipsByTenant :exec
delete from iam_memberships where tenant_id = @tenant_id;

-- name: DeleteIamMembershipsByAccount :exec
delete from iam_memberships where account_id = @account_id;

-- ===== iam_identity_providers (iam.go IdentityProviderRepo) =====

-- name: CreateIamIdentityProvider :exec
insert into iam_identity_providers (id, enabled, created_at, updated_at, data)
values (@id, @enabled, now(), now(), @data);

-- name: GetIamIdentityProvider :one
select data from iam_identity_providers where id = @id;

-- name: ListEnabledIamIdentityProviders :many
select data from iam_identity_providers where enabled = true;

-- name: UpdateIamIdentityProvider :execrows
update iam_identity_providers set enabled = @enabled, data = @data, updated_at = now()
where id = @id;

-- name: DeleteIamIdentityProvider :execrows
delete from iam_identity_providers where id = @id;

-- ===== iam_external_identities (iam.go ExternalIdentityRepo) =====

-- name: CreateIamExternalIdentity :exec
insert into iam_external_identities (id, provider_id, subject, account_id, created_at, updated_at, data)
values (@id, @provider_id, @subject, @account_id, now(), now(), @data);

-- name: GetIamExternalIdentity :one
select data from iam_external_identities where id = @id;

-- name: GetIamExternalIdentityByProviderSubject :one
select data from iam_external_identities where provider_id = @provider_id and subject = @subject;

-- name: ListIamExternalIdentitiesByAccount :many
select data from iam_external_identities where account_id = @account_id;

-- name: DeleteIamExternalIdentity :execrows
delete from iam_external_identities where id = @id;

-- name: DeleteIamExternalIdentitiesByAccount :exec
delete from iam_external_identities where account_id = @account_id;

-- ===== iam_one_time_tokens (iam.go OneTimeTokenRepo) =====

-- name: CreateIamOneTimeToken :exec
insert into iam_one_time_tokens (token, purpose, account_id, expires_at, created_at)
values (@token, @purpose, @account_id, @expires_at, now());

-- name: ConsumeIamOneTimeToken :one
delete from iam_one_time_tokens where purpose = @purpose and token = @token
returning account_id, expires_at;

-- ===== identity_secrets (identity.go SecretStore) =====

-- name: UpsertIdentitySecret :exec
insert into identity_secrets (namespace, key, value)
values (@namespace, @key, @value)
on conflict (namespace, key) do update set value = excluded.value;

-- name: GetIdentitySecret :one
select value from identity_secrets where namespace = @namespace and key = @key;

-- name: DeleteIdentitySecret :exec
delete from identity_secrets where namespace = @namespace and key = @key;

-- ===== identity_refresh_sessions (identity.go RefreshSessionStore) =====

-- name: CreateIdentityRefreshSession :exec
insert into identity_refresh_sessions (id, account_id, expires_at)
values (@id, @account_id, @expires_at);

-- name: GetIdentityRefreshSession :one
select id, account_id, expires_at from identity_refresh_sessions where id = @id;

-- name: DeleteIdentityRefreshSession :exec
delete from identity_refresh_sessions where id = @id;

-- ===== identity_sso_states (identity.go SSOStateStore) =====

-- name: SaveIdentitySsoState :exec
insert into identity_sso_states (state, provider_id, code_verifier, nonce, expires_at)
values (@state, @provider_id, @code_verifier, @nonce, @expires_at);

-- name: ConsumeIdentitySsoState :one
delete from identity_sso_states where state = @state
returning state, provider_id, code_verifier, nonce, expires_at;

-- ===== registration_requests (registration_requests.go RegistrationRequestRepo) =====

-- name: UpsertRegistrationRequest :exec
insert into registration_requests (id, email, status, created_at, updated_at, data)
values (@id, @email, @status, now(), now(), @data)
on conflict (email) do update set status = excluded.status, data = excluded.data, updated_at = now();

-- name: GetRegistrationRequest :one
select data from registration_requests where id = @id;

-- name: GetRegistrationRequestByEmail :one
select data from registration_requests where email = @email;

-- name: ListRegistrationRequests :many
select data from registration_requests order by created_at desc;

-- name: UpdateRegistrationRequest :execrows
update registration_requests set status = @status, data = @data, updated_at = now()
where id = @id;
