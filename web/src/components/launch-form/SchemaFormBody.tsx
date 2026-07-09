// The actual schemapb-driven form body. Split out of LaunchFormRenderer.tsx
// so it can be `lazy()`-loaded (see that file's header comment) — this is
// the module that statically imports @stroppy-io/schemapb-react and, through
// it, the ~22MB schemapb WASM engine.
//
// Rendering contract (per schemapb.Schema.Filed docs, schemapb/schema.proto):
//   - an inactive field (`when` false) MUST NOT be rendered — enforced via
//     form.fieldActive(path) before every field, including container kinds
//     (list/object), which gate their whole subtree.
//   - Computed fields are never user-editable: they are read from
//     form.computed (values resolved by the engine), not form.register.
//   - Ref fields that survive to the browser are unresolved identity-refs
//     (normal Refs are resolved server-side before the schema ships) — we
//     render nothing rather than guess at their shape.
//   - Kinds outside the spec's named list (float/int32/uint32/uint64/
//     duration/timestamp/oneOf) fall back to a plain string field so an
//     author sees SOME input rather than a silently missing one.
import type { ReactNode } from "react";
import { useSchemaForm, getPath } from "@stroppy-io/schemapb-react";
import type { Schema_Filed } from "@stroppy-io/schemapb";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import type { LaunchFormRendererProps } from "./LaunchFormRenderer";
import { StringField, Int64Field, DoubleField, BoolField, EnumField } from "./fields";

export default function SchemaFormBody({
  schema,
  onSubmit,
  onInvalid,
  submitLabel = "Launch",
}: LaunchFormRendererProps) {
  const form = useSchemaForm({ schema });

  function renderField(f: Schema_Filed, path: string): ReactNode {
    if (!form.fieldActive(path)) return null;
    const label = path;

    switch (f.kind.case) {
      case "string":
        return <StringField key={path} label={label} field={form.register(path)} />;
      case "int64":
        return (
          <Int64Field
            key={path}
            label={label}
            field={form.register(path)}
            gt={f.kind.value.gt}
            gte={f.kind.value.gte}
            lt={f.kind.value.lt}
            lte={f.kind.value.lte}
          />
        );
      case "double":
        return (
          <DoubleField
            key={path}
            label={label}
            field={form.register(path)}
            gt={f.kind.value.gt}
            gte={f.kind.value.gte}
            lt={f.kind.value.lt}
            lte={f.kind.value.lte}
          />
        );
      case "bool":
        return <BoolField key={path} label={label} field={form.register(path)} />;
      case "enum":
        return (
          <EnumField
            key={path}
            label={label}
            field={form.register(path)}
            values={f.kind.value.values}
            options={form.enumOptions(path)}
          />
        );
      case "computed": {
        const value = getPath(form.computed, path);
        return (
          <div key={path} className="flex flex-col gap-1">
            <Label>{label}</Label>
            <p aria-label={label} className="text-sm text-muted-foreground">
              {value === undefined || value === null ? "—" : String(value)}
            </p>
          </div>
        );
      }
      case "list": {
        const count = form.listCount(path);
        const item = f.kind.value.items[0];
        return (
          <div key={path} aria-label={label} className="flex flex-col gap-2">
            <Label>{label}</Label>
            {item && Array.from({ length: count }, (_, i) => renderField(item, `${path}.${i}`))}
          </div>
        );
      }
      case "object": {
        const nested = f.kind.value.schema;
        return (
          <fieldset key={path} aria-label={label} className="flex flex-col gap-2 border border-border p-3">
            <legend className="text-xs uppercase text-muted-foreground">{label}</legend>
            {nested?.fields.map((nf) => renderField(nf, `${path}.${nf.name}`))}
          </fieldset>
        );
      }
      case "ref":
        return null;
      default:
        return <StringField key={path} label={label} field={form.register(path)} />;
    }
  }

  return (
    <form onSubmit={form.handleSubmit(onSubmit, onInvalid)} className="flex flex-col gap-4">
      {schema.fields.map((f) => renderField(f, f.name))}
      <Button type="submit" disabled={!form.ready}>
        {submitLabel}
      </Button>
    </form>
  );
}
