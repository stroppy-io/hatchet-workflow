-- sqld:up
ALTER TABLE "public"."recipe_records" ADD COLUMN "source_ref" text NOT NULL DEFAULT ''::text;

-- sqld:down
ALTER TABLE "public"."recipe_records" DROP COLUMN "source_ref";
