-- sqld:up
CREATE TABLE "public"."pipeline_pushes" (
  "namespace" text NOT NULL,
  "revision" text NOT NULL DEFAULT ''::text,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "error" text NOT NULL DEFAULT ''::text,
  "pushed_at" timestamptz,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("namespace")
);

-- sqld:down
DROP TABLE "public"."pipeline_pushes";
