CREATE TABLE "public"."database_preset_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."favorite_records" (
  "id" text NOT NULL,
  "account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "kind" int4 NOT NULL,
  "target_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_accounts" (
  "id" text NOT NULL,
  "email" text NOT NULL DEFAULT ''::text,
  "nickname" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_api_tokens" (
  "id" text NOT NULL,
  "account_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_credentials" (
  "account_id" text NOT NULL,
  "hash" text NOT NULL,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("account_id")
);
CREATE TABLE "public"."iam_external_identities" (
  "id" text NOT NULL,
  "provider_id" text NOT NULL,
  "subject" text NOT NULL,
  "account_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_identity_providers" (
  "id" text NOT NULL,
  "enabled" bool NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_memberships" (
  "id" text NOT NULL,
  "account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_one_time_tokens" (
  "token" text NOT NULL,
  "purpose" text NOT NULL,
  "account_id" text NOT NULL DEFAULT ''::text,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("token")
);
CREATE TABLE "public"."iam_roles" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_tenants" (
  "id" text NOT NULL,
  "slug" text NOT NULL DEFAULT ''::text,
  "owner_account_id" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."identity_refresh_sessions" (
  "id" text NOT NULL,
  "account_id" text NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."identity_secrets" (
  "namespace" text NOT NULL,
  "key" text NOT NULL,
  "value" text NOT NULL,
  PRIMARY KEY ("namespace", "key")
);
CREATE TABLE "public"."identity_sso_states" (
  "state" text NOT NULL,
  "provider_id" text NOT NULL DEFAULT ''::text,
  "code_verifier" text NOT NULL DEFAULT ''::text,
  "nonce" text NOT NULL DEFAULT ''::text,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("state")
);
CREATE TABLE "public"."package_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."platform_settings" (
  "id" text NOT NULL,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."share_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "token" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."suite_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."suite_run_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."suite_wizard_drafts" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."tenant_settings_records" (
  "tenant_id" text NOT NULL,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("tenant_id")
);
CREATE TABLE "public"."test_preset_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."test_run_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."test_wizard_drafts" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."workload_preset_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_database_preset_records_tenant" ON "public"."database_preset_records" ("tenant_id");
CREATE INDEX "idx_fav_account_kind_target" ON "public"."favorite_records" ("account_id", "kind", "target_id");
CREATE INDEX "idx_favorite_records_tenant" ON "public"."favorite_records" ("tenant_id");
CREATE INDEX "idx_iam_accounts_email" ON "public"."iam_accounts" ("email");
CREATE INDEX "idx_iam_accounts_nickname" ON "public"."iam_accounts" ("nickname");
CREATE INDEX "idx_iam_api_tokens_account" ON "public"."iam_api_tokens" ("account_id");
CREATE INDEX "idx_ext_provider_subject" ON "public"."iam_external_identities" ("provider_id", "subject");
CREATE INDEX "idx_iam_external_identities_account" ON "public"."iam_external_identities" ("account_id");
CREATE INDEX "idx_iam_identity_providers_enabled" ON "public"."iam_identity_providers" ("enabled");
CREATE INDEX "idx_iam_memberships_account" ON "public"."iam_memberships" ("account_id");
CREATE INDEX "idx_iam_memberships_tenant" ON "public"."iam_memberships" ("tenant_id");
CREATE INDEX "idx_iam_one_time_tokens_purpose" ON "public"."iam_one_time_tokens" ("purpose");
CREATE INDEX "idx_iam_roles_tenant" ON "public"."iam_roles" ("tenant_id");
CREATE INDEX "idx_iam_tenants_owner" ON "public"."iam_tenants" ("owner_account_id");
CREATE INDEX "idx_iam_tenants_slug" ON "public"."iam_tenants" ("slug");
CREATE INDEX "idx_package_records_tenant" ON "public"."package_records" ("tenant_id");
CREATE INDEX "idx_share_records_tenant" ON "public"."share_records" ("tenant_id");
CREATE UNIQUE INDEX "idx_share_records_token" ON "public"."share_records" ("token");
CREATE INDEX "idx_suite_records_tenant" ON "public"."suite_records" ("tenant_id");
CREATE INDEX "idx_suite_run_records_tenant" ON "public"."suite_run_records" ("tenant_id");
CREATE INDEX "idx_suite_wizard_drafts_tenant" ON "public"."suite_wizard_drafts" ("tenant_id");
CREATE INDEX "idx_test_preset_records_tenant" ON "public"."test_preset_records" ("tenant_id");
CREATE INDEX "idx_test_run_records_tenant" ON "public"."test_run_records" ("tenant_id");
CREATE INDEX "idx_test_wizard_drafts_tenant" ON "public"."test_wizard_drafts" ("tenant_id");
CREATE INDEX "idx_workload_preset_records_tenant" ON "public"."workload_preset_records" ("tenant_id");

