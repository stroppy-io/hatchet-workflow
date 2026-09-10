-- sqld:up
CREATE SEQUENCE "public"."audit_log_id_seq" AS int8 INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START WITH 1 CACHE 1;
CREATE TABLE "public"."api_tokens" (
  "id" uuid NOT NULL,
  "kind" text NOT NULL,
  "name" text NOT NULL,
  "prefix" text NOT NULL,
  "secret_hash" bytea NOT NULL,
  "tenant_id" uuid NOT NULL,
  "role" text NOT NULL,
  "owner_id" uuid,
  "expires_at" timestamptz,
  "last_used_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "revoked_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."audit_log" (
  "id" int8 NOT NULL DEFAULT nextval('audit_log_id_seq'::regclass),
  "at" timestamptz NOT NULL DEFAULT now(),
  "tenant_id" uuid,
  "actor_kind" text NOT NULL,
  "actor_id" text NOT NULL DEFAULT ''::text,
  "actor_name" text NOT NULL DEFAULT ''::text,
  "action" text NOT NULL,
  "target_kind" text NOT NULL DEFAULT ''::text,
  "target_id" text NOT NULL DEFAULT ''::text,
  "target_name" text NOT NULL DEFAULT ''::text,
  "details" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "request_id" text NOT NULL DEFAULT ''::text,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."iam_denylist" (
  "session_id" text NOT NULL,
  "user_id" uuid NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("session_id")
);
CREATE TABLE "public"."iam_webhook_events" (
  "id" text NOT NULL,
  "received_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."profiles" (
  "id" uuid NOT NULL,
  "email" text NOT NULL DEFAULT ''::text,
  "display_name" text NOT NULL DEFAULT ''::text,
  "avatar" text NOT NULL DEFAULT ''::text,
  "is_platform_admin" bool NOT NULL DEFAULT false,
  "preferences" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "notifications" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."provider_profiles" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "kind" text NOT NULL,
  "settings" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "secret_names" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "status" text NOT NULL DEFAULT 'verifying'::text,
  "status_reason" text NOT NULL DEFAULT ''::text,
  "verified_at" timestamptz,
  "verify_run_id" text NOT NULL DEFAULT ''::text,
  "quotas" jsonb,
  "quotas_observed_at" timestamptz,
  "created_by" uuid,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."tenant_invites" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "email" text NOT NULL,
  "role" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "invited_by" uuid,
  "message" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz NOT NULL,
  "resolved_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."tenant_members" (
  "tenant_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "role" text NOT NULL,
  "joined_at" timestamptz NOT NULL DEFAULT now(),
  "last_seen_at" timestamptz,
  PRIMARY KEY ("tenant_id", "user_id")
);
CREATE TABLE "public"."tenant_settings" (
  "tenant_id" uuid NOT NULL,
  "run_retention_days" int4 NOT NULL DEFAULT 90,
  "rating_tenant" bool NOT NULL DEFAULT true,
  "rating_global" bool NOT NULL DEFAULT false,
  "default_keep" text NOT NULL DEFAULT '0s'::text,
  "notification_emails" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "limits_override" jsonb,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id")
);
CREATE TABLE "public"."tenants" (
  "id" uuid NOT NULL,
  "slug" text NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "public_name" text,
  "status" text NOT NULL DEFAULT 'active'::text,
  "owner_id" uuid NOT NULL,
  "graphene_namespace" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."webhook_deliveries" (
  "id" uuid NOT NULL,
  "webhook_id" uuid NOT NULL,
  "event" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "attempts" int4 NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_attempt_at" timestamptz,
  "response_status" int4,
  "error" text NOT NULL DEFAULT ''::text,
  "payload" jsonb NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."webhooks" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "url" text NOT NULL,
  "events" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "enabled" bool NOT NULL DEFAULT true,
  "description" text NOT NULL DEFAULT ''::text,
  "secret" text NOT NULL,
  "prev_secret" text NOT NULL DEFAULT ''::text,
  "prev_expires_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE INDEX "api_tokens_owner_idx" ON "public"."api_tokens" ("owner_id") WHERE (revoked_at IS NULL);
CREATE UNIQUE INDEX "api_tokens_prefix_idx" ON "public"."api_tokens" ("prefix");
CREATE INDEX "api_tokens_tenant_idx" ON "public"."api_tokens" ("tenant_id") WHERE (revoked_at IS NULL);
CREATE INDEX "audit_log_tenant_idx" ON "public"."audit_log" ("tenant_id", "id" DESC);
CREATE INDEX "iam_denylist_expires_at_idx" ON "public"."iam_denylist" ("expires_at");
CREATE UNIQUE INDEX "provider_profiles_name_idx" ON "public"."provider_profiles" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "provider_profiles_tenant_idx" ON "public"."provider_profiles" ("tenant_id") WHERE (deleted_at IS NULL);
CREATE INDEX "tenant_invites_email_idx" ON "public"."tenant_invites" ("email") WHERE (status = 'pending'::text);
CREATE UNIQUE INDEX "tenant_invites_pending_idx" ON "public"."tenant_invites" ("tenant_id", "email") WHERE (status = 'pending'::text);
CREATE INDEX "tenant_members_user_idx" ON "public"."tenant_members" ("user_id");
CREATE UNIQUE INDEX "tenants_owner_live_idx" ON "public"."tenants" ("owner_id") WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "tenants_slug_live_idx" ON "public"."tenants" ("slug") WHERE (deleted_at IS NULL);
CREATE INDEX "webhook_deliveries_due_idx" ON "public"."webhook_deliveries" ("next_attempt_at") WHERE (status = 'pending'::text);
CREATE INDEX "webhook_deliveries_webhook_idx" ON "public"."webhook_deliveries" ("webhook_id", "created_at");
CREATE INDEX "webhooks_tenant_idx" ON "public"."webhooks" ("tenant_id");
ALTER TABLE "public"."api_tokens" ADD CONSTRAINT "api_tokens_owner_id_fkey" FOREIGN KEY ("owner_id") REFERENCES "public"."profiles" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."api_tokens" ADD CONSTRAINT "api_tokens_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."provider_profiles" ADD CONSTRAINT "provider_profiles_created_by_fkey" FOREIGN KEY ("created_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."provider_profiles" ADD CONSTRAINT "provider_profiles_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenant_invites" ADD CONSTRAINT "tenant_invites_invited_by_fkey" FOREIGN KEY ("invited_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."tenant_invites" ADD CONSTRAINT "tenant_invites_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenant_members" ADD CONSTRAINT "tenant_members_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenant_members" ADD CONSTRAINT "tenant_members_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."profiles" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenant_settings" ADD CONSTRAINT "tenant_settings_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenants" ADD CONSTRAINT "tenants_owner_id_fkey" FOREIGN KEY ("owner_id") REFERENCES "public"."profiles" ("id");
ALTER TABLE "public"."webhook_deliveries" ADD CONSTRAINT "webhook_deliveries_webhook_id_fkey" FOREIGN KEY ("webhook_id") REFERENCES "public"."webhooks" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."webhooks" ADD CONSTRAINT "webhooks_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;

-- sqld:down
ALTER TABLE "public"."webhooks" DROP CONSTRAINT "webhooks_tenant_id_fkey";
ALTER TABLE "public"."webhook_deliveries" DROP CONSTRAINT "webhook_deliveries_webhook_id_fkey";
ALTER TABLE "public"."tenants" DROP CONSTRAINT "tenants_owner_id_fkey";
ALTER TABLE "public"."tenant_settings" DROP CONSTRAINT "tenant_settings_tenant_id_fkey";
ALTER TABLE "public"."tenant_members" DROP CONSTRAINT "tenant_members_user_id_fkey";
ALTER TABLE "public"."tenant_members" DROP CONSTRAINT "tenant_members_tenant_id_fkey";
ALTER TABLE "public"."tenant_invites" DROP CONSTRAINT "tenant_invites_tenant_id_fkey";
ALTER TABLE "public"."tenant_invites" DROP CONSTRAINT "tenant_invites_invited_by_fkey";
ALTER TABLE "public"."provider_profiles" DROP CONSTRAINT "provider_profiles_tenant_id_fkey";
ALTER TABLE "public"."provider_profiles" DROP CONSTRAINT "provider_profiles_created_by_fkey";
ALTER TABLE "public"."api_tokens" DROP CONSTRAINT "api_tokens_tenant_id_fkey";
ALTER TABLE "public"."api_tokens" DROP CONSTRAINT "api_tokens_owner_id_fkey";
DROP INDEX "public"."webhooks_tenant_idx";
DROP INDEX "public"."webhook_deliveries_webhook_idx";
DROP INDEX "public"."webhook_deliveries_due_idx";
DROP INDEX "public"."tenants_slug_live_idx";
DROP INDEX "public"."tenants_owner_live_idx";
DROP INDEX "public"."tenant_members_user_idx";
DROP INDEX "public"."tenant_invites_pending_idx";
DROP INDEX "public"."tenant_invites_email_idx";
DROP INDEX "public"."provider_profiles_tenant_idx";
DROP INDEX "public"."provider_profiles_name_idx";
DROP INDEX "public"."iam_denylist_expires_at_idx";
DROP INDEX "public"."audit_log_tenant_idx";
DROP INDEX "public"."api_tokens_tenant_idx";
DROP INDEX "public"."api_tokens_prefix_idx";
DROP INDEX "public"."api_tokens_owner_idx";
DROP TABLE "public"."webhooks";
DROP TABLE "public"."webhook_deliveries";
DROP TABLE "public"."tenants";
DROP TABLE "public"."tenant_settings";
DROP TABLE "public"."tenant_members";
DROP TABLE "public"."tenant_invites";
DROP TABLE "public"."provider_profiles";
DROP TABLE "public"."profiles";
DROP TABLE "public"."iam_webhook_events";
DROP TABLE "public"."iam_denylist";
DROP TABLE "public"."audit_log";
DROP TABLE "public"."api_tokens";
DROP SEQUENCE "public"."audit_log_id_seq";
