-- sqld:up
CREATE TABLE "public"."recipe_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "name" text NOT NULL,
  "version" int4 NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_recipe_records_tenant" ON "public"."recipe_records" ("tenant_id");
CREATE UNIQUE INDEX "uq_recipe_records_tenant_name_version" ON "public"."recipe_records" ("tenant_id", "name", "version");

-- sqld:down
DROP INDEX "public"."uq_recipe_records_tenant_name_version";
DROP INDEX "public"."idx_recipe_records_tenant";
DROP TABLE "public"."recipe_records";
