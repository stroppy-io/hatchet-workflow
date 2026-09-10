-- sqld:up
CREATE TABLE "public"."system_settings" (
  "id" int4 NOT NULL DEFAULT 1,
  "tenant_creation" text NOT NULL DEFAULT 'anyone'::text,
  "public_rating_enabled" bool NOT NULL DEFAULT true,
  "examples_enabled" bool NOT NULL DEFAULT true,
  "default_limits" jsonb,
  "run_retention_max_days" int4 NOT NULL DEFAULT 365,
  "stroppy_catalog" jsonb,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "updated_by" uuid,
  PRIMARY KEY ("id")
);
ALTER TABLE "public"."tenants" ADD COLUMN "suspended_reason" text NOT NULL DEFAULT ''::text;
ALTER TABLE "public"."system_settings" ADD CONSTRAINT "system_settings_id_check" CHECK (id = 1);
ALTER TABLE "public"."system_settings" ADD CONSTRAINT "system_settings_updated_by_fkey" FOREIGN KEY ("updated_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;

-- sqld:down
ALTER TABLE "public"."system_settings" DROP CONSTRAINT "system_settings_updated_by_fkey";
ALTER TABLE "public"."system_settings" DROP CONSTRAINT "system_settings_id_check";
ALTER TABLE "public"."tenants" DROP COLUMN "suspended_reason";
DROP TABLE "public"."system_settings";
