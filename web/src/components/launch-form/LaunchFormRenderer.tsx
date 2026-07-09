// Public entry point for SP-D's generated launch form (D2). Renders any
// schemapb.Schema as a controlled form via @stroppy-io/schemapb-react's
// useSchemaForm — schemapb (validate/compute/bake/fieldActive/enumOptions/
// listCount) is the single source of validation truth; this module never
// hand-rolls form logic.
//
// The actual rendering (SchemaFormBody) is loaded via a dynamic import()
// rather than a static one. @stroppy-io/schemapb-react statically imports
// @stroppy-io/schemapb, which ships a ~22MB WASM engine (schemapb.wasm).
// A static `import { useSchemaForm } from "@stroppy-io/schemapb-react"` at
// the top of a module that itself gets statically imported into the app's
// entry chunk would pull that whole dependency chain into the main bundle.
// `lazy()` makes Vite/Rollup code-split SchemaFormBody (and everything it
// imports) into its own async chunk, fetched only once a LaunchFormRenderer
// actually mounts. See .superpowers/sdd/task-2-brief.md and the T1 review
// carry-forward note for the constraint this satisfies.
import { lazy, Suspense } from "react";
import type { Schema, BakedJson, FieldErrorJson } from "@stroppy-io/schemapb";

export interface LaunchFormRendererProps {
  schema: Schema;
  onSubmit: (baked: BakedJson) => void;
  onInvalid?: (errors: Record<string, FieldErrorJson>) => void;
  submitLabel?: string;
}

const SchemaFormBody = lazy(() => import("./SchemaFormBody"));

export function LaunchFormRenderer(props: LaunchFormRendererProps) {
  return (
    <Suspense fallback={<div className="text-sm text-muted-foreground">Loading form…</div>}>
      <SchemaFormBody {...props} />
    </Suspense>
  );
}
