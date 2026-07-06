-- sqld:up
DROP TABLE "public"."database_preset_records";
DROP TABLE "public"."network_reservations";
DROP TABLE "public"."suite_records";
DROP TABLE "public"."suite_run_records";
DROP TABLE "public"."suite_wizard_drafts";
DROP TABLE "public"."test_preset_records";
DROP TABLE "public"."test_wizard_drafts";
DROP TABLE "public"."workload_preset_records";

-- sqld:down
-- WARNING: data loss on rollback
CREATE TABLE "public"."workload_preset_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- WARNING: data loss on rollback
CREATE TABLE "public"."test_wizard_drafts" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- WARNING: data loss on rollback
CREATE TABLE "public"."test_preset_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- WARNING: data loss on rollback
CREATE TABLE "public"."suite_wizard_drafts" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- WARNING: data loss on rollback
CREATE TABLE "public"."suite_run_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- WARNING: data loss on rollback
CREATE TABLE "public"."suite_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- WARNING: data loss on rollback
CREATE TABLE "public"."network_reservations" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "run_id" text NOT NULL,
  "provider" int4 NOT NULL,
  "resource_type" text NOT NULL,
  "resource_id" text NOT NULL,
  "cidr" text NOT NULL,
  "status" int4 NOT NULL,
  "workflow_id" text NOT NULL DEFAULT ''::text,
  "expires_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- WARNING: data loss on rollback
CREATE TABLE "public"."database_preset_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
