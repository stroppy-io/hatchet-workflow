-- create table accounts
CREATE TABLE "public"."accounts" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "email" text NOT NULL,
  "nickname" text NOT NULL,
  "is_admin" bool NOT NULL,
  "password_hash" text NOT NULL,
  PRIMARY KEY ("id")
);
-- create table tenants
CREATE TABLE "public"."tenants" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create table agents
CREATE TABLE "public"."agents" (
  "id" text NOT NULL,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "machine_id" text NOT NULL,
  "agent_component_id" text NOT NULL,
  "status" text NOT NULL,
  "host" text NOT NULL,
  "port" int4 NOT NULL,
  "version" text NOT NULL,
  "boot_id" text NOT NULL,
  "registered_at" timestamptz NOT NULL,
  "last_seen_at" timestamptz NOT NULL,
  "lease_expires_at" timestamptz NOT NULL,
  "error" text NOT NULL,
  "runtime" text NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index agents_status_seen_idx
CREATE INDEX "agents_status_seen_idx" ON "public"."agents" USING btree ("status", "last_seen_at");
-- create index agents_tenant_machine_idx
CREATE INDEX "agents_tenant_machine_idx" ON "public"."agents" USING btree ("tenant_id", "machine_id");
-- create table api_tokens
CREATE TABLE "public"."api_tokens" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "name" text NOT NULL,
  "role" text NOT NULL,
  "expires_at" timestamptz,
  "token_hash" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index api_tokens_hash_uniq
CREATE UNIQUE INDEX "api_tokens_hash_uniq" ON "public"."api_tokens" USING btree ("token_hash");
-- create table dags
CREATE TABLE "public"."dags" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "status" text NOT NULL,
  "payload" text NOT NULL DEFAULT '{}'::jsonb,
  "processor" text NOT NULL DEFAULT '{}'::jsonb,
  "admission" text NOT NULL DEFAULT '{}'::jsonb,
  "not_before" timestamptz,
  "claim_priority" int4 NOT NULL,
  "lease_expires_at" timestamptz,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index dags_claim_idx
CREATE INDEX "dags_claim_idx" ON "public"."dags" USING btree ("status", "not_before", "lease_expires_at", "claim_priority");
-- create index dags_status_updated_at_idx
CREATE INDEX "dags_status_updated_at_idx" ON "public"."dags" USING btree ("status", "updated_at");
-- create index dags_tenant_status_idx
CREATE INDEX "dags_tenant_status_idx" ON "public"."dags" USING btree ("tenant_id", "status");
-- create table packages
CREATE TABLE "public"."packages" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "name" text NOT NULL,
  "db_kind" text NOT NULL,
  "db_version" text NOT NULL,
  "deb_object_uri" text NOT NULL,
  "checksum" text NOT NULL,
  "is_builtin" bool NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create table presets
CREATE TABLE "public"."presets" (
  "preset_workload_preset" jsonb,
  "preset_database_preset" jsonb,
  "preset_test_preset" jsonb,
  "preset_case" text NOT NULL,
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "tags" text NOT NULL,
  "kind" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create table settings_items
CREATE TABLE "public"."settings_items" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "part" text NOT NULL,
  "key" text NOT NULL,
  "value" text NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index settings_items_tenant_part_key_uniq
CREATE UNIQUE INDEX "settings_items_tenant_part_key_uniq" ON "public"."settings_items" USING btree ("tenant_id", "part", "key");
-- create table suites
CREATE TABLE "public"."suites" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "name" text,
  "description" text,
  "preset" text NOT NULL,
  "cron" text NOT NULL,
  "next_fire_at" timestamptz,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index suites_cron_idx
CREATE INDEX "suites_cron_idx" ON "public"."suites" USING btree ("next_fire_at");
-- create table tenant_members
CREATE TABLE "public"."tenant_members" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "tenant_id" text NOT NULL,
  "account_id" text NOT NULL,
  "role" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create table webhooks
CREATE TABLE "public"."webhooks" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "url" text NOT NULL,
  "events" _text NOT NULL,
  "enabled" bool NOT NULL,
  "secret" text,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create table network_allocations
CREATE TABLE "public"."network_allocations" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "dag_id" text NOT NULL,
  "provider" text NOT NULL,
  "cidr" text NOT NULL,
  "zone" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "lease_expires_at" timestamptz,
  "tags" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("dag_id") REFERENCES "public"."dags" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index netalloc_lease_idx
CREATE INDEX "netalloc_lease_idx" ON "public"."network_allocations" USING btree ("lease_expires_at");
-- create index netalloc_provider_idx
CREATE INDEX "netalloc_provider_idx" ON "public"."network_allocations" USING btree ("provider");
-- create index netalloc_tenant_idx
CREATE INDEX "netalloc_tenant_idx" ON "public"."network_allocations" USING btree ("tenant_id");
-- create table suite_runs
CREATE TABLE "public"."suite_runs" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "suite_id" text NOT NULL,
  "dag" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("dag") REFERENCES "public"."dags" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("suite_id") REFERENCES "public"."suites" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create table test_runs
CREATE TABLE "public"."test_runs" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "owner_account_id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "name" text,
  "description" text,
  "test_preset" text NOT NULL,
  "dag" text NOT NULL,
  "suite_run_id" text,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("dag") REFERENCES "public"."dags" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("owner_account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("suite_run_id") REFERENCES "public"."suite_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
