-- sqld:up
CREATE TABLE "public"."catalog_entries" (
  "id" text NOT NULL,
  "level" text NOT NULL,
  "tenant_id" text NOT NULL DEFAULT ''::text,
  "kind" text NOT NULL,
  "slug" text NOT NULL,
  "version" int4 NOT NULL,
  "origin" text NOT NULL DEFAULT 'native'::text,
  "source_entry_id" text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_catalog_entries_scope" ON "public"."catalog_entries" ("level", "tenant_id", "kind");
CREATE INDEX "idx_catalog_entries_source" ON "public"."catalog_entries" ("source_entry_id");
CREATE UNIQUE INDEX "uq_catalog_entries_scope_slug_version" ON "public"."catalog_entries" ("level", "tenant_id", "kind", "slug", "version");

-- sqld:down
DROP INDEX "public"."uq_catalog_entries_scope_slug_version";
DROP INDEX "public"."idx_catalog_entries_source";
DROP INDEX "public"."idx_catalog_entries_scope";
DROP TABLE "public"."catalog_entries";
