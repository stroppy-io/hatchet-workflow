-- Authoritative schema for the Postgres control-plane store. sqld reads this to
-- generate gen/db (typed query funcs), gen/bob (bob models) and the bootstrap
-- migration. Storage model: every record type is one table carrying the
-- queryable envelope columns (id/tenant_id/created_at/updated_at plus any
-- secondary keys) and the full proto message in a `data jsonb` column.

-- ===== record tables (store.go / settings.go / records.go) =====

CREATE TABLE run_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_run_records_tenant ON run_records (tenant_id);

CREATE TABLE platform_settings (
  id         text PRIMARY KEY,
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);

CREATE TABLE registration_requests (
  id         text PRIMARY KEY,
  email      text NOT NULL,
  status     text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE UNIQUE INDEX idx_registration_requests_email ON registration_requests (email);
CREATE INDEX idx_registration_requests_status ON registration_requests (status);

CREATE TABLE recipe_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  name       text NOT NULL,
  version    integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_recipe_records_tenant ON recipe_records (tenant_id);
CREATE UNIQUE INDEX uq_recipe_records_tenant_name_version ON recipe_records (tenant_id, name, version);

CREATE TABLE catalog_entries (
  id               text PRIMARY KEY,
  level            text NOT NULL,          -- 'instance' | 'org'
  tenant_id        text NOT NULL DEFAULT '', -- '' for level='instance'
  kind             text NOT NULL,          -- 'provider' | 'workflow'
  slug             text NOT NULL,
  version          integer NOT NULL,
  origin           text NOT NULL DEFAULT 'native',
  source_entry_id  text NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  data             jsonb NOT NULL
);
CREATE UNIQUE INDEX uq_catalog_entries_scope_slug_version
  ON catalog_entries (level, tenant_id, kind, slug, version);
CREATE INDEX idx_catalog_entries_scope ON catalog_entries (level, tenant_id, kind);
CREATE INDEX idx_catalog_entries_source ON catalog_entries (source_entry_id);

CREATE TABLE share_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  token      text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_share_records_tenant ON share_records (tenant_id);
CREATE UNIQUE INDEX idx_share_records_token ON share_records (token);

CREATE TABLE favorite_records (
  id         text PRIMARY KEY,
  account_id text NOT NULL,
  tenant_id  text NOT NULL,
  kind       integer NOT NULL,
  target_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_favorite_records_tenant ON favorite_records (tenant_id);
CREATE INDEX idx_fav_account_kind_target ON favorite_records (account_id, kind, target_id);

CREATE TABLE package_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_package_records_tenant ON package_records (tenant_id);

CREATE TABLE tenant_settings_records (
  tenant_id  text PRIMARY KEY,
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);

-- ===== quota snapshots + reservation ledger (quota.go) =====

CREATE TABLE quota_snapshots (
  tenant_id          text NOT NULL,
  provider           integer NOT NULL,
  resource_type      text NOT NULL,
  resource_id        text NOT NULL,
  service            text NOT NULL DEFAULT '',
  quota_name         text NOT NULL,
  units              text NOT NULL DEFAULT '',
  provider_used      numeric NOT NULL DEFAULT 0,
  quota_limit        numeric NOT NULL DEFAULT 0,
  provider_available numeric NOT NULL DEFAULT 0,
  observed_at        timestamptz NOT NULL,
  stale_after        timestamptz NOT NULL,
  raw                jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (tenant_id, provider, resource_type, resource_id, quota_name)
);
CREATE INDEX idx_quota_snapshots_tenant_provider ON quota_snapshots (tenant_id, provider);
CREATE INDEX idx_quota_snapshots_stale ON quota_snapshots (stale_after);

CREATE TABLE quota_reservations (
  id            text PRIMARY KEY,
  tenant_id     text NOT NULL,
  run_id        text NOT NULL,
  node_id       text NOT NULL,
  provider      integer NOT NULL,
  resource_type text NOT NULL,
  resource_id   text NOT NULL,
  service       text NOT NULL DEFAULT '',
  quota_name    text NOT NULL,
  units         text NOT NULL DEFAULT '',
  amount        numeric NOT NULL,
  status        integer NOT NULL,
  workflow_id   text NOT NULL DEFAULT '',
  expires_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, run_id, node_id, provider, resource_type, resource_id, quota_name)
);
CREATE INDEX idx_quota_reservations_run ON quota_reservations (tenant_id, run_id);
CREATE INDEX idx_quota_reservations_scope_status ON quota_reservations (tenant_id, provider, resource_type, resource_id, status);

-- ===== IAM tables (iam.go) =====

CREATE TABLE iam_accounts (
  id         text PRIMARY KEY,
  email      text NOT NULL DEFAULT '',
  nickname   text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_iam_accounts_email ON iam_accounts (email);
CREATE INDEX idx_iam_accounts_nickname ON iam_accounts (nickname);

CREATE TABLE iam_credentials (
  account_id text PRIMARY KEY,
  hash       text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE iam_api_tokens (
  id         text PRIMARY KEY,
  account_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_iam_api_tokens_account ON iam_api_tokens (account_id);

CREATE TABLE iam_tenants (
  id               text PRIMARY KEY,
  slug             text NOT NULL DEFAULT '',
  owner_account_id text NOT NULL DEFAULT '',
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  data             jsonb NOT NULL
);
CREATE INDEX idx_iam_tenants_slug ON iam_tenants (slug);
CREATE INDEX idx_iam_tenants_owner ON iam_tenants (owner_account_id);

CREATE TABLE iam_roles (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_iam_roles_tenant ON iam_roles (tenant_id);

CREATE TABLE iam_memberships (
  id         text PRIMARY KEY,
  account_id text NOT NULL,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_iam_memberships_account ON iam_memberships (account_id);
CREATE INDEX idx_iam_memberships_tenant ON iam_memberships (tenant_id);

CREATE TABLE iam_identity_providers (
  id         text PRIMARY KEY,
  enabled    boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_iam_identity_providers_enabled ON iam_identity_providers (enabled);

CREATE TABLE iam_external_identities (
  id          text PRIMARY KEY,
  provider_id text NOT NULL,
  subject     text NOT NULL,
  account_id  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  data        jsonb NOT NULL
);
CREATE INDEX idx_ext_provider_subject ON iam_external_identities (provider_id, subject);
CREATE INDEX idx_iam_external_identities_account ON iam_external_identities (account_id);

CREATE TABLE iam_one_time_tokens (
  token      text PRIMARY KEY,
  purpose    text NOT NULL,
  account_id text NOT NULL DEFAULT '',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_iam_one_time_tokens_purpose ON iam_one_time_tokens (purpose);

-- ===== identity backing stores (identity.go) =====

CREATE TABLE identity_secrets (
  namespace text NOT NULL,
  key       text NOT NULL,
  value     text NOT NULL,
  PRIMARY KEY (namespace, key)
);

CREATE TABLE identity_refresh_sessions (
  id         text PRIMARY KEY,
  account_id text NOT NULL,
  expires_at timestamptz NOT NULL
);

CREATE TABLE identity_sso_states (
  state         text PRIMARY KEY,
  provider_id   text NOT NULL DEFAULT '',
  code_verifier text NOT NULL DEFAULT '',
  nonce         text NOT NULL DEFAULT '',
  expires_at    timestamptz NOT NULL
);
