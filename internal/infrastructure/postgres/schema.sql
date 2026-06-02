-- Authoritative schema for the Postgres control-plane store. sqld reads this to
-- generate gen/db (typed query funcs), gen/bob (bob models) and the bootstrap
-- migration. Storage model: every record type is one table carrying the
-- queryable envelope columns (id/tenant_id/created_at/updated_at plus any
-- secondary keys) and the full proto message in a `data jsonb` column.

-- ===== record tables (store.go / settings.go / records.go) =====

CREATE TABLE test_run_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_test_run_records_tenant ON test_run_records (tenant_id);

CREATE TABLE test_wizard_drafts (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_test_wizard_drafts_tenant ON test_wizard_drafts (tenant_id);

CREATE TABLE test_preset_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_test_preset_records_tenant ON test_preset_records (tenant_id);

CREATE TABLE platform_settings (
  id         text PRIMARY KEY,
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);

CREATE TABLE suite_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_suite_records_tenant ON suite_records (tenant_id);

CREATE TABLE suite_run_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_suite_run_records_tenant ON suite_run_records (tenant_id);

CREATE TABLE suite_wizard_drafts (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_suite_wizard_drafts_tenant ON suite_wizard_drafts (tenant_id);

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

CREATE TABLE database_preset_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_database_preset_records_tenant ON database_preset_records (tenant_id);

CREATE TABLE workload_preset_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_workload_preset_records_tenant ON workload_preset_records (tenant_id);

CREATE TABLE tenant_settings_records (
  tenant_id  text PRIMARY KEY,
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);

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
