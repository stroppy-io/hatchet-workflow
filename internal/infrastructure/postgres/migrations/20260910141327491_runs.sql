-- sqld:up
CREATE SEQUENCE "public"."run_events_id_seq" AS int8 INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START WITH 1 CACHE 1;
CREATE TABLE "public"."favorites" (
  "user_id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "target_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("user_id", "kind", "target_id")
);
CREATE TABLE "public"."run_events" (
  "id" int8 NOT NULL DEFAULT nextval('run_events_id_seq'::regclass),
  "run_id" uuid NOT NULL,
  "graphene_id" int8 NOT NULL DEFAULT 0,
  "at" timestamptz NOT NULL,
  "kind" text NOT NULL,
  "title" text NOT NULL,
  "subject" text NOT NULL DEFAULT ''::text,
  "status" text NOT NULL DEFAULT ''::text,
  "error" text NOT NULL DEFAULT ''::text,
  "attempt" int4 NOT NULL DEFAULT 0,
  "payload" jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."runs" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "phase" text NOT NULL DEFAULT 'queued'::text,
  "status_reason" text NOT NULL DEFAULT ''::text,
  "trigger" text NOT NULL DEFAULT 'manual'::text,
  "suite_run_id" uuid,
  "cell_id" text NOT NULL DEFAULT ''::text,
  "schedule_id" uuid,
  "parent_run_id" uuid,
  "test_id" uuid,
  "test_name" text NOT NULL DEFAULT ''::text,
  "author_id" uuid,
  "snapshot" jsonb NOT NULL,
  "run_spec" jsonb NOT NULL,
  "summary" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "result" jsonb,
  "runtime_state" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "last_event_id" int8 NOT NULL DEFAULT 0,
  "rating_tenant" bool NOT NULL DEFAULT true,
  "rating_global" bool NOT NULL DEFAULT false,
  "keep" text NOT NULL DEFAULT ''::text,
  "keep_until" timestamptz,
  "stand_kept" bool NOT NULL DEFAULT false,
  "notes" text NOT NULL DEFAULT ''::text,
  "labels" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "graphene_namespace" text NOT NULL,
  "pipeline_revision" text NOT NULL DEFAULT ''::text,
  "tps" float8,
  "duration_seconds" float8,
  "idempotency_key" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "started_at" timestamptz,
  "finished_at" timestamptz,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE INDEX "favorites_tenant_idx" ON "public"."favorites" ("tenant_id", "user_id", "kind");
CREATE UNIQUE INDEX "run_events_graphene_idx" ON "public"."run_events" ("run_id", "graphene_id") WHERE (graphene_id <> 0);
CREATE INDEX "run_events_run_idx" ON "public"."run_events" ("run_id", "id");
CREATE UNIQUE INDEX "runs_idempotency_idx" ON "public"."runs" ("tenant_id", "idempotency_key") WHERE (idempotency_key <> ''::text);
CREATE INDEX "runs_live_idx" ON "public"."runs" ("status") WHERE ((deleted_at IS NULL) AND (status = ANY (ARRAY['pending'::text, 'running'::text, 'cancelling'::text])));
CREATE INDEX "runs_tenant_idx" ON "public"."runs" ("tenant_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "runs_test_idx" ON "public"."runs" ("test_id", "created_at" DESC) WHERE (deleted_at IS NULL);
ALTER TABLE "public"."favorites" ADD CONSTRAINT "favorites_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."favorites" ADD CONSTRAINT "favorites_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."profiles" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."run_events" ADD CONSTRAINT "run_events_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "public"."runs" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."runs" ADD CONSTRAINT "runs_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."runs" ADD CONSTRAINT "runs_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."runs" ADD CONSTRAINT "runs_test_id_fkey" FOREIGN KEY ("test_id") REFERENCES "public"."tests" ("id");

-- sqld:down
ALTER TABLE "public"."runs" DROP CONSTRAINT "runs_test_id_fkey";
ALTER TABLE "public"."runs" DROP CONSTRAINT "runs_tenant_id_fkey";
ALTER TABLE "public"."runs" DROP CONSTRAINT "runs_author_id_fkey";
ALTER TABLE "public"."run_events" DROP CONSTRAINT "run_events_run_id_fkey";
ALTER TABLE "public"."favorites" DROP CONSTRAINT "favorites_user_id_fkey";
ALTER TABLE "public"."favorites" DROP CONSTRAINT "favorites_tenant_id_fkey";
DROP INDEX "public"."runs_test_idx";
DROP INDEX "public"."runs_tenant_idx";
DROP INDEX "public"."runs_live_idx";
DROP INDEX "public"."runs_idempotency_idx";
DROP INDEX "public"."run_events_run_idx";
DROP INDEX "public"."run_events_graphene_idx";
DROP INDEX "public"."favorites_tenant_idx";
DROP TABLE "public"."runs";
DROP TABLE "public"."run_events";
DROP TABLE "public"."favorites";
DROP SEQUENCE "public"."run_events_id_seq";
