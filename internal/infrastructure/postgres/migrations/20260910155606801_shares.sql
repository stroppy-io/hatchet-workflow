-- sqld:up
CREATE TABLE "public"."shares" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "token" text NOT NULL,
  "target_kind" text NOT NULL,
  "target_id" uuid,
  "target_name" text NOT NULL DEFAULT ''::text,
  "run_ids" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "scope" text NOT NULL DEFAULT 'overview'::text,
  "title" text NOT NULL DEFAULT ''::text,
  "snapshot" jsonb NOT NULL,
  "captured_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz,
  "revoked_at" timestamptz,
  "view_count" int4 NOT NULL DEFAULT 0,
  "created_by" uuid,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE INDEX "shares_target_idx" ON "public"."shares" ("target_kind", "target_id");
CREATE INDEX "shares_tenant_idx" ON "public"."shares" ("tenant_id", "created_at" DESC);
CREATE UNIQUE INDEX "shares_token_idx" ON "public"."shares" ("token");
ALTER TABLE "public"."shares" ADD CONSTRAINT "shares_created_by_fkey" FOREIGN KEY ("created_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."shares" ADD CONSTRAINT "shares_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;

-- sqld:down
ALTER TABLE "public"."shares" DROP CONSTRAINT "shares_tenant_id_fkey";
ALTER TABLE "public"."shares" DROP CONSTRAINT "shares_created_by_fkey";
DROP INDEX "public"."shares_token_idx";
DROP INDEX "public"."shares_tenant_idx";
DROP INDEX "public"."shares_target_idx";
DROP TABLE "public"."shares";
