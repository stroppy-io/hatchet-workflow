-- sqld:up
CREATE TABLE "public"."registration_requests" (
  "id" text NOT NULL,
  "email" text NOT NULL,
  "status" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "idx_registration_requests_email" ON "public"."registration_requests" ("email");
CREATE INDEX "idx_registration_requests_status" ON "public"."registration_requests" ("status");

-- sqld:down
DROP INDEX "public"."idx_registration_requests_status";
DROP INDEX "public"."idx_registration_requests_email";
DROP TABLE "public"."registration_requests";
