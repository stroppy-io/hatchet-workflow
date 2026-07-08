-- sqld:up
CREATE TABLE "public"."run_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_run_records_tenant" ON "public"."run_records" ("tenant_id");

-- sqld:down
DROP INDEX "public"."idx_run_records_tenant";
DROP TABLE "public"."run_records";
