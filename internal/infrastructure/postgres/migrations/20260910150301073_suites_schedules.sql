-- sqld:up
CREATE SEQUENCE "public"."schedule_history_id_seq" AS int8 INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START WITH 1 CACHE 1;
CREATE TABLE "public"."schedule_history" (
  "id" int8 NOT NULL DEFAULT nextval('schedule_history_id_seq'::regclass),
  "schedule_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "ref_id" uuid,
  "name" text NOT NULL DEFAULT ''::text,
  "status" text NOT NULL DEFAULT ''::text,
  "error" text NOT NULL DEFAULT ''::text,
  "at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."schedules" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "target_kind" text NOT NULL,
  "target_id" uuid NOT NULL,
  "target_name" text NOT NULL DEFAULT ''::text,
  "cron" text NOT NULL,
  "timezone" text NOT NULL DEFAULT 'UTC'::text,
  "enabled" bool NOT NULL DEFAULT true,
  "overrides" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "next_run_at" timestamptz,
  "last_run" jsonb,
  "author_id" uuid,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."suite_runs" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "suite_id" uuid,
  "suite_name" text NOT NULL DEFAULT ''::text,
  "name" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "status_reason" text NOT NULL DEFAULT ''::text,
  "trigger" text NOT NULL DEFAULT 'manual'::text,
  "schedule_id" uuid,
  "retry_of" uuid,
  "concurrency" int4 NOT NULL DEFAULT 1,
  "cells" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "labels" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "graphene_namespace" text NOT NULL,
  "last_event_id" int8 NOT NULL DEFAULT 0,
  "idempotency_key" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "started_at" timestamptz,
  "finished_at" timestamptz,
  "duration_seconds" float8,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."suites" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "tests" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "axes" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "cells" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "concurrency" int4 NOT NULL DEFAULT 1,
  "defaults" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
ALTER TABLE "public"."runs" ADD COLUMN "graphene_run_id" text NOT NULL DEFAULT ''::text;
CREATE INDEX "schedule_history_idx" ON "public"."schedule_history" ("schedule_id", "at" DESC);
CREATE INDEX "schedules_due_idx" ON "public"."schedules" ("next_run_at") WHERE ((deleted_at IS NULL) AND enabled);
CREATE INDEX "schedules_tenant_idx" ON "public"."schedules" ("tenant_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "suite_runs_idempotency_idx" ON "public"."suite_runs" ("tenant_id", "idempotency_key") WHERE (idempotency_key <> ''::text);
CREATE INDEX "suite_runs_suite_idx" ON "public"."suite_runs" ("suite_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "suite_runs_tenant_idx" ON "public"."suite_runs" ("tenant_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "suites_name_idx" ON "public"."suites" ("tenant_id", "name") WHERE (deleted_at IS NULL);
ALTER TABLE "public"."schedule_history" ADD CONSTRAINT "schedule_history_schedule_id_fkey" FOREIGN KEY ("schedule_id") REFERENCES "public"."schedules" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."schedules" ADD CONSTRAINT "schedules_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."schedules" ADD CONSTRAINT "schedules_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."suite_runs" ADD CONSTRAINT "suite_runs_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."suite_runs" ADD CONSTRAINT "suite_runs_suite_id_fkey" FOREIGN KEY ("suite_id") REFERENCES "public"."suites" ("id");
ALTER TABLE "public"."suite_runs" ADD CONSTRAINT "suite_runs_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."suites" ADD CONSTRAINT "suites_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."suites" ADD CONSTRAINT "suites_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;

-- sqld:down
ALTER TABLE "public"."suites" DROP CONSTRAINT "suites_tenant_id_fkey";
ALTER TABLE "public"."suites" DROP CONSTRAINT "suites_author_id_fkey";
ALTER TABLE "public"."suite_runs" DROP CONSTRAINT "suite_runs_tenant_id_fkey";
ALTER TABLE "public"."suite_runs" DROP CONSTRAINT "suite_runs_suite_id_fkey";
ALTER TABLE "public"."suite_runs" DROP CONSTRAINT "suite_runs_author_id_fkey";
ALTER TABLE "public"."schedules" DROP CONSTRAINT "schedules_tenant_id_fkey";
ALTER TABLE "public"."schedules" DROP CONSTRAINT "schedules_author_id_fkey";
ALTER TABLE "public"."schedule_history" DROP CONSTRAINT "schedule_history_schedule_id_fkey";
DROP INDEX "public"."suites_name_idx";
DROP INDEX "public"."suite_runs_tenant_idx";
DROP INDEX "public"."suite_runs_suite_idx";
DROP INDEX "public"."suite_runs_idempotency_idx";
DROP INDEX "public"."schedules_tenant_idx";
DROP INDEX "public"."schedules_due_idx";
DROP INDEX "public"."schedule_history_idx";
ALTER TABLE "public"."runs" DROP COLUMN "graphene_run_id";
DROP TABLE "public"."suites";
DROP TABLE "public"."suite_runs";
DROP TABLE "public"."schedules";
DROP TABLE "public"."schedule_history";
DROP SEQUENCE "public"."schedule_history_id_seq";
