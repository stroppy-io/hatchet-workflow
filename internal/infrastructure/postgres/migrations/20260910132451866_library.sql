-- sqld:up
CREATE TABLE "public"."databases" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "kind" text NOT NULL,
  "version" text NOT NULL,
  "image" text NOT NULL DEFAULT ''::text,
  "params" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "configs" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "external" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."tests" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "database_id" uuid,
  "database_inline" jsonb,
  "workload_id" uuid,
  "workload_inline" jsonb,
  "sizes" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "provider_profile_id" uuid,
  "keep" text NOT NULL DEFAULT ''::text,
  "rating_tenant" bool NOT NULL DEFAULT true,
  "rating_global" bool NOT NULL DEFAULT false,
  "status" text NOT NULL DEFAULT 'draft'::text,
  "validated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."workloads" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "stroppy_version" text NOT NULL,
  "protocol" text NOT NULL,
  "spec" jsonb NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "databases_name_idx" ON "public"."databases" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "databases_tenant_idx" ON "public"."databases" ("tenant_id", "updated_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "tests_database_idx" ON "public"."tests" ("database_id") WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "tests_name_idx" ON "public"."tests" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "tests_tenant_idx" ON "public"."tests" ("tenant_id", "updated_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "tests_workload_idx" ON "public"."tests" ("workload_id") WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "workloads_name_idx" ON "public"."workloads" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "workloads_tenant_idx" ON "public"."workloads" ("tenant_id", "updated_at" DESC) WHERE (deleted_at IS NULL);
ALTER TABLE "public"."databases" ADD CONSTRAINT "databases_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."databases" ADD CONSTRAINT "databases_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_database_id_fkey" FOREIGN KEY ("database_id") REFERENCES "public"."databases" ("id");
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_provider_profile_id_fkey" FOREIGN KEY ("provider_profile_id") REFERENCES "public"."provider_profiles" ("id");
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_workload_id_fkey" FOREIGN KEY ("workload_id") REFERENCES "public"."workloads" ("id");
ALTER TABLE "public"."workloads" ADD CONSTRAINT "workloads_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."workloads" ADD CONSTRAINT "workloads_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;

-- sqld:down
ALTER TABLE "public"."workloads" DROP CONSTRAINT "workloads_tenant_id_fkey";
ALTER TABLE "public"."workloads" DROP CONSTRAINT "workloads_author_id_fkey";
ALTER TABLE "public"."tests" DROP CONSTRAINT "tests_workload_id_fkey";
ALTER TABLE "public"."tests" DROP CONSTRAINT "tests_tenant_id_fkey";
ALTER TABLE "public"."tests" DROP CONSTRAINT "tests_provider_profile_id_fkey";
ALTER TABLE "public"."tests" DROP CONSTRAINT "tests_database_id_fkey";
ALTER TABLE "public"."tests" DROP CONSTRAINT "tests_author_id_fkey";
ALTER TABLE "public"."databases" DROP CONSTRAINT "databases_tenant_id_fkey";
ALTER TABLE "public"."databases" DROP CONSTRAINT "databases_author_id_fkey";
DROP INDEX "public"."workloads_tenant_idx";
DROP INDEX "public"."workloads_name_idx";
DROP INDEX "public"."tests_workload_idx";
DROP INDEX "public"."tests_tenant_idx";
DROP INDEX "public"."tests_name_idx";
DROP INDEX "public"."tests_database_idx";
DROP INDEX "public"."databases_tenant_idx";
DROP INDEX "public"."databases_name_idx";
DROP TABLE "public"."workloads";
DROP TABLE "public"."tests";
DROP TABLE "public"."databases";
