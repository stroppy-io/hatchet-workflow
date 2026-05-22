-- create table platform_settings
CREATE TABLE "public"."platform_settings" (
  "id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  "server_addr" text NOT NULL,
  PRIMARY KEY ("id")
);
