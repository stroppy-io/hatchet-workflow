-- sqld:up
DROP TABLE "public"."test_run_records";

-- sqld:down
-- WARNING: data loss on rollback
CREATE TABLE "public"."test_run_records" (
  "id" text NOT NULL,
  "tenant_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
