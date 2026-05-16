-- create table binary_artifacts
CREATE TABLE "public"."binary_artifacts" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "kind" text NOT NULL,
  "name" text NOT NULL,
  "version" text NOT NULL,
  "filename" text NOT NULL,
  "arch" text NOT NULL,
  "os" text NOT NULL,
  "origin_url" text NOT NULL,
  "storage_uri" text NOT NULL,
  "sha_256" text NOT NULL,
  "size_bytes" int8 NOT NULL,
  "content_type" text NOT NULL,
  "fetched_at" timestamptz,
  "last_served_at" timestamptz,
  "serve_count" int8 NOT NULL,
  PRIMARY KEY ("id")
);
-- create index binary_artifacts_kind_idx
CREATE INDEX "binary_artifacts_kind_idx" ON "public"."binary_artifacts" USING btree ("kind");
-- create index binary_artifacts_lookup_uniq
CREATE UNIQUE INDEX "binary_artifacts_lookup_uniq" ON "public"."binary_artifacts" USING btree ("name", "version", "filename");
-- create index binary_artifacts_sha_idx
CREATE INDEX "binary_artifacts_sha_idx" ON "public"."binary_artifacts" USING btree ("sha_256");
-- create table dags
CREATE TABLE "public"."dags" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "graph" text NOT NULL DEFAULT '{}'::jsonb,
  "metadata" json NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id")
);
-- create table schedules
CREATE TABLE "public"."schedules" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "cron_expr" text NOT NULL,
  "hook_name" text NOT NULL,
  "payload" text NOT NULL,
  "enabled" bool NOT NULL,
  "catchup" text NOT NULL,
  "misfire_policy" text NOT NULL,
  "max_misfire_age_seconds" int4 NOT NULL,
  "last_fired_at" timestamptz,
  "next_fire_at" timestamptz,
  "lease_owner" text NOT NULL,
  "lease_expires_at" timestamptz,
  "consecutive_failures" int4 NOT NULL,
  "last_failure_error" text NOT NULL,
  "metadata" json NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id")
);
-- create index schedules_fire_idx
CREATE INDEX "schedules_fire_idx" ON "public"."schedules" USING btree ("enabled", "next_fire_at");
-- create table tenants
CREATE TABLE "public"."tenants" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  PRIMARY KEY ("id")
);
-- create table users
CREATE TABLE "public"."users" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "email" text NOT NULL,
  "nickname" text NOT NULL,
  "platform_role" text NOT NULL DEFAULT 'PLATFORM_ROLE_NONE'::text,
  "password_hash" text NOT NULL,
  PRIMARY KEY ("id")
);
-- create index users_email_uniq
CREATE UNIQUE INDEX "users_email_uniq" ON "public"."users" USING btree ("email");
-- create index users_nickname_uniq
CREATE UNIQUE INDEX "users_nickname_uniq" ON "public"."users" USING btree ("nickname");
-- create table dag_runs
CREATE TABLE "public"."dag_runs" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "dag_id" text NOT NULL,
  "status" text NOT NULL,
  "priority" int4 NOT NULL,
  "scheduled_at" timestamptz,
  "started_at" timestamptz,
  "finished_at" timestamptz,
  "attempt" int4 NOT NULL,
  "previous_attempt_id" text,
  "error" text NOT NULL,
  "cancel_requested" bool NOT NULL DEFAULT false,
  "metadata" json NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("dag_id") REFERENCES "public"."dags" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  FOREIGN KEY ("previous_attempt_id") REFERENCES "public"."dag_runs" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- create index dag_runs_dag_idx
CREATE INDEX "dag_runs_dag_idx" ON "public"."dag_runs" USING btree ("dag_id");
-- create index dag_runs_queue_idx
CREATE INDEX "dag_runs_queue_idx" ON "public"."dag_runs" USING btree ("status", "priority", "created_at");
-- create index dag_runs_status_idx
CREATE INDEX "dag_runs_status_idx" ON "public"."dag_runs" USING btree ("status");
-- create table agents
CREATE TABLE "public"."agents" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "dag_run_id" text NOT NULL,
  "machine_id" text NOT NULL,
  "role" text NOT NULL,
  "internal_ip" text NOT NULL,
  "public_ip" text,
  "agent_version" text NOT NULL,
  "capabilities" _text NOT NULL,
  "status" text NOT NULL,
  "last_heartbeat" timestamptz,
  "missed_heartbeats" int4 NOT NULL,
  "last_error" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index agents_dag_run_idx
CREATE INDEX "agents_dag_run_idx" ON "public"."agents" USING btree ("dag_run_id");
-- create index agents_heartbeat_idx
CREATE INDEX "agents_heartbeat_idx" ON "public"."agents" USING btree ("last_heartbeat");
-- create index agents_machine_uniq
CREATE UNIQUE INDEX "agents_machine_uniq" ON "public"."agents" USING btree ("tenant_id", "machine_id");
-- create index agents_status_idx
CREATE INDEX "agents_status_idx" ON "public"."agents" USING btree ("status");
-- create index agents_tenant_idx
CREATE INDEX "agents_tenant_idx" ON "public"."agents" USING btree ("tenant_id");
-- create table quota_counters
CREATE TABLE "public"."quota_counters" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "resource_id" text NOT NULL,
  "limit_value" int8 NOT NULL,
  "used" int8 NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index quota_counters_tenant_resource_uniq
CREATE UNIQUE INDEX "quota_counters_tenant_resource_uniq" ON "public"."quota_counters" USING btree ("tenant_id", "resource_id");
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
-- create table api_tokens
CREATE TABLE "public"."api_tokens" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_by" text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "scopes" _text NOT NULL,
  "expires_at" timestamptz,
  "last_used_at" timestamptz,
  "token_hash" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index api_tokens_hash_uniq
CREATE UNIQUE INDEX "api_tokens_hash_uniq" ON "public"."api_tokens" USING btree ("token_hash");
-- create index api_tokens_tenant_idx
CREATE INDEX "api_tokens_tenant_idx" ON "public"."api_tokens" USING btree ("tenant_id");
-- create table database_presets
CREATE TABLE "public"."database_presets" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  "created_by" text,
  "database" text NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index database_presets_tenant_idx
CREATE INDEX "database_presets_tenant_idx" ON "public"."database_presets" USING btree ("tenant_id");
-- create table packages
CREATE TABLE "public"."packages" (
  "id" text NOT NULL,
  "tenant_id" text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  "created_by" text,
  "db_kind" text NOT NULL,
  "db_version" text NOT NULL,
  "is_builtin" bool NOT NULL,
  "source" text NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index packages_kind_version_idx
CREATE INDEX "packages_kind_version_idx" ON "public"."packages" USING btree ("db_kind", "db_version");
-- create index packages_tenant_idx
CREATE INDEX "packages_tenant_idx" ON "public"."packages" USING btree ("tenant_id");
-- create table refresh_tokens
CREATE TABLE "public"."refresh_tokens" (
  "id" text NOT NULL,
  "user_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "token_hash" text NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "family_id" text NOT NULL,
  "replaced_by" text,
  "jti" text NOT NULL,
  "revoked_at" timestamptz,
  "revocation_reason" text NOT NULL,
  "user_agent" text NOT NULL,
  "source_ip" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("replaced_by") REFERENCES "public"."refresh_tokens" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index refresh_tokens_expiry_idx
CREATE INDEX "refresh_tokens_expiry_idx" ON "public"."refresh_tokens" USING btree ("expires_at");
-- create index refresh_tokens_family_idx
CREATE INDEX "refresh_tokens_family_idx" ON "public"."refresh_tokens" USING btree ("family_id");
-- create index refresh_tokens_hash_uniq
CREATE UNIQUE INDEX "refresh_tokens_hash_uniq" ON "public"."refresh_tokens" USING btree ("token_hash");
-- create index refresh_tokens_user_idx
CREATE INDEX "refresh_tokens_user_idx" ON "public"."refresh_tokens" USING btree ("user_id");
-- create table tenant_members
CREATE TABLE "public"."tenant_members" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "user_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "role" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index tenant_members_tenant_idx
CREATE INDEX "tenant_members_tenant_idx" ON "public"."tenant_members" USING btree ("tenant_id");
-- create index tenant_members_user_idx
CREATE INDEX "tenant_members_user_idx" ON "public"."tenant_members" USING btree ("user_id");
-- create index tenant_members_user_tenant_uniq
CREATE UNIQUE INDEX "tenant_members_user_tenant_uniq" ON "public"."tenant_members" USING btree ("user_id", "tenant_id");
-- create table test_suites
CREATE TABLE "public"."test_suites" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  "created_by" text,
  "matrix" text NOT NULL DEFAULT '{}'::jsonb,
  "policy" text NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index test_suites_tenant_idx
CREATE INDEX "test_suites_tenant_idx" ON "public"."test_suites" USING btree ("tenant_id");
-- create table webhooks
CREATE TABLE "public"."webhooks" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  "created_by" text,
  "url" text NOT NULL,
  "events" _text NOT NULL,
  "secret" text NOT NULL,
  "headers" text NOT NULL,
  "timeout_seconds" int4 NOT NULL,
  "max_retries" int4 NOT NULL,
  "enabled" bool NOT NULL,
  "last_success_at" timestamptz,
  "last_failure_at" timestamptz,
  "last_failure_error" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index webhooks_tenant_idx
CREATE INDEX "webhooks_tenant_idx" ON "public"."webhooks" USING btree ("tenant_id");
-- create table workload_presets
CREATE TABLE "public"."workload_presets" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  "created_by" text,
  "workload" text NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index workload_presets_tenant_idx
CREATE INDEX "workload_presets_tenant_idx" ON "public"."workload_presets" USING btree ("tenant_id");
-- create table dag_run_state_entries
CREATE TABLE "public"."dag_run_state_entries" (
  "id" text NOT NULL,
  "dag_run_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "key" text NOT NULL,
  "value" text NOT NULL,
  "version" int8 NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("dag_run_id") REFERENCES "public"."dag_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index dag_run_state_entries_run_key_uniq
CREATE UNIQUE INDEX "dag_run_state_entries_run_key_uniq" ON "public"."dag_run_state_entries" USING btree ("dag_run_id", "key");
-- create table node_runs
CREATE TABLE "public"."node_runs" (
  "id" text NOT NULL,
  "dag_run_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "node_id" text NOT NULL,
  "status" text NOT NULL,
  "attempt" int4 NOT NULL,
  "error" text NOT NULL,
  "started_at" timestamptz,
  "finished_at" timestamptz,
  "output" text NOT NULL,
  "metadata" json NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("dag_run_id") REFERENCES "public"."dag_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index node_runs_dag_run_idx
CREATE INDEX "node_runs_dag_run_idx" ON "public"."node_runs" USING btree ("dag_run_id");
-- create index node_runs_dag_run_node_uniq
CREATE UNIQUE INDEX "node_runs_dag_run_node_uniq" ON "public"."node_runs" USING btree ("dag_run_id", "node_id");
-- create index node_runs_dag_run_status_idx
CREATE INDEX "node_runs_dag_run_status_idx" ON "public"."node_runs" USING btree ("dag_run_id", "status");
-- create index node_runs_status_idx
CREATE INDEX "node_runs_status_idx" ON "public"."node_runs" USING btree ("status");
-- create table agent_commands
CREATE TABLE "public"."agent_commands" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "agent_id" text NOT NULL,
  "node_run_id" text NOT NULL,
  "command_payload" bytea NOT NULL,
  "state" text NOT NULL,
  "attempt" int4 NOT NULL,
  "issued_at" timestamptz,
  "delivered_at" timestamptz,
  "reported_at" timestamptz,
  "result_payload" bytea NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("agent_id") REFERENCES "public"."agents" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index agent_commands_agent_state_idx
CREATE INDEX "agent_commands_agent_state_idx" ON "public"."agent_commands" USING btree ("agent_id", "state");
-- create index agent_commands_node_run_idx
CREATE INDEX "agent_commands_node_run_idx" ON "public"."agent_commands" USING btree ("node_run_id");
-- create table test_suite_runs
CREATE TABLE "public"."test_suite_runs" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "suite_id" text NOT NULL,
  "test_run_ids" _text NOT NULL,
  "dag_run_id" text,
  "matrix" text NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("dag_run_id") REFERENCES "public"."dag_runs" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("suite_id") REFERENCES "public"."test_suites" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index test_suite_runs_suite_idx
CREATE INDEX "test_suite_runs_suite_idx" ON "public"."test_suite_runs" USING btree ("suite_id");
-- create index test_suite_runs_tenant_idx
CREATE INDEX "test_suite_runs_tenant_idx" ON "public"."test_suite_runs" USING btree ("tenant_id");
-- create table webhook_deliveries
CREATE TABLE "public"."webhook_deliveries" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "webhook_id" text NOT NULL,
  "event" text NOT NULL,
  "payload" bytea NOT NULL,
  "state" text NOT NULL,
  "attempts" int4 NOT NULL,
  "next_attempt_at" timestamptz,
  "delivered_at" timestamptz,
  "last_error" text NOT NULL,
  "last_status_code" int4,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("webhook_id") REFERENCES "public"."webhooks" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index webhook_deliveries_drain_idx
CREATE INDEX "webhook_deliveries_drain_idx" ON "public"."webhook_deliveries" USING btree ("state", "next_attempt_at");
-- create index webhook_deliveries_webhook_idx
CREATE INDEX "webhook_deliveries_webhook_idx" ON "public"."webhook_deliveries" USING btree ("webhook_id");
-- create table test_run_templates
CREATE TABLE "public"."test_run_templates" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  "created_by" text,
  "database_variant_database_preset_id" text,
  "database_variant_database" jsonb,
  "database_variant_case" text NOT NULL,
  "workload_variant_workload_preset_id" text,
  "workload_variant_workload" jsonb,
  "workload_variant_case" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("database_variant_database_preset_id") REFERENCES "public"."database_presets" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("workload_variant_workload_preset_id") REFERENCES "public"."workload_presets" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- create index test_run_templates_tenant_idx
CREATE INDEX "test_run_templates_tenant_idx" ON "public"."test_run_templates" USING btree ("tenant_id");
-- create table shared_suite_runs
CREATE TABLE "public"."shared_suite_runs" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "test_suite_run_id" text NOT NULL,
  "created_by" text,
  "token" text NOT NULL,
  "snapshot" json NOT NULL DEFAULT '{}'::jsonb,
  "metrics" json NOT NULL DEFAULT '{}'::jsonb,
  "expires_at" timestamptz,
  "view_count" int8 NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("test_suite_run_id") REFERENCES "public"."test_suite_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index shared_suite_runs_run_idx
CREATE INDEX "shared_suite_runs_run_idx" ON "public"."shared_suite_runs" USING btree ("test_suite_run_id");
-- create index shared_suite_runs_tenant_idx
CREATE INDEX "shared_suite_runs_tenant_idx" ON "public"."shared_suite_runs" USING btree ("tenant_id");
-- create index shared_suite_runs_token_uniq
CREATE UNIQUE INDEX "shared_suite_runs_token_uniq" ON "public"."shared_suite_runs" USING btree ("token");
-- create table test_runs
CREATE TABLE "public"."test_runs" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "name" text NOT NULL,
  "description" text,
  "label" _text NOT NULL,
  "suite_run_id" text,
  "template_id" text,
  "dag_run_id" text,
  "created_by" text,
  "database_variant_database_preset_id" text,
  "database_variant_database" jsonb,
  "database_variant_case" text NOT NULL,
  "workload_variant_workload_preset_id" text,
  "workload_variant_workload" jsonb,
  "workload_variant_case" text NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("dag_run_id") REFERENCES "public"."dag_runs" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("database_variant_database_preset_id") REFERENCES "public"."database_presets" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  FOREIGN KEY ("suite_run_id") REFERENCES "public"."test_suite_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("template_id") REFERENCES "public"."test_run_templates" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("workload_variant_workload_preset_id") REFERENCES "public"."workload_presets" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- create index test_runs_suite_run_idx
CREATE INDEX "test_runs_suite_run_idx" ON "public"."test_runs" USING btree ("suite_run_id");
-- create index test_runs_template_idx
CREATE INDEX "test_runs_template_idx" ON "public"."test_runs" USING btree ("template_id");
-- create index test_runs_tenant_idx
CREATE INDEX "test_runs_tenant_idx" ON "public"."test_runs" USING btree ("tenant_id");
-- create table shared_test_runs
CREATE TABLE "public"."shared_test_runs" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "test_run_id" text NOT NULL,
  "created_by" text,
  "token" text NOT NULL,
  "snapshot" json NOT NULL DEFAULT '{}'::jsonb,
  "metrics" json NOT NULL DEFAULT '{}'::jsonb,
  "expires_at" timestamptz,
  "view_count" int8 NOT NULL,
  PRIMARY KEY ("id"),
  FOREIGN KEY ("created_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  FOREIGN KEY ("test_run_id") REFERENCES "public"."test_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index shared_test_runs_run_idx
CREATE INDEX "shared_test_runs_run_idx" ON "public"."shared_test_runs" USING btree ("test_run_id");
-- create index shared_test_runs_tenant_idx
CREATE INDEX "shared_test_runs_tenant_idx" ON "public"."shared_test_runs" USING btree ("tenant_id");
-- create index shared_test_runs_token_uniq
CREATE UNIQUE INDEX "shared_test_runs_token_uniq" ON "public"."shared_test_runs" USING btree ("token");
