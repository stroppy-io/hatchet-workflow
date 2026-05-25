import { create } from "@bufbuild/protobuf";
import { Plus, X } from "lucide-react";

import { SelectField } from "@/components/editors/fields";
import { KeyValueEditor } from "@/components/editors/key-value-editor";
import { StringListEditor } from "@/components/editors/string-list-editor";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  ConfigSchema,
  Config_OverrideSchema,
  Config_Override_KeyValuePatchSchema,
  Config_Override_TextPatchSchema,
  type Config,
  type Config_Override,
} from "@/lib/proto/cloud/v1/runtime/render/config_pb.ts";

const PATCH_OPTIONS = [
  { label: "Text patch (replace content)", value: "textPatch" },
  { label: "Key-value patch (set/remove)", value: "keyValuePatch" },
];

// Editable layer of a render.Config: its `overrides` (user patches on top of the
// rendered items). Items themselves are render output and stay read-only. Covers
// the two common override kinds — full content replace and key/value set/remove.
export function ConfigOverridesEditor({ value, onChange }: { value: Config; onChange: (value: Config) => void }) {
  const setOverrides = (overrides: Config_Override[]) => onChange(create(ConfigSchema, { ...value, overrides }));
  const update = (index: number, next: Config_Override) => setOverrides(value.overrides.map((o, i) => (i === index ? next : o)));

  return (
    <div className="space-y-3">
      {value.overrides.length === 0 ? <p className="text-xs text-muted-foreground">No overrides — the rendered config is used as-is.</p> : null}
      {value.overrides.map((override, index) => {
        const caseName = override.override.case === "keyValuePatch" ? "keyValuePatch" : "textPatch";
        return (
          <div key={index} className="space-y-2 rounded-md border p-3">
            <div className="flex items-center gap-2">
              <Input className="flex-1 font-mono text-xs" value={override.itemId} placeholder="item id" onChange={(event) => update(index, create(Config_OverrideSchema, { ...override, itemId: event.target.value }))} />
              <Button type="button" variant="ghost" size="icon-sm" onClick={() => setOverrides(value.overrides.filter((_, i) => i !== index))}>
                <X className="size-3.5" />
              </Button>
            </div>
            <SelectField
              label="Patch"
              value={caseName}
              options={PATCH_OPTIONS}
              onChange={(next) =>
                update(
                  index,
                  create(Config_OverrideSchema, {
                    ...override,
                    override:
                      next === "keyValuePatch"
                        ? { case: "keyValuePatch", value: create(Config_Override_KeyValuePatchSchema, {}) }
                        : { case: "textPatch", value: create(Config_Override_TextPatchSchema, {}) },
                  }),
                )
              }
            />
            {override.override.case === "textPatch" ? (
              <div className="space-y-1.5">
                <Label>Content</Label>
                <Textarea className="font-mono text-xs" rows={5} value={override.override.value.content} onChange={(event) => update(index, create(Config_OverrideSchema, { ...override, override: { case: "textPatch", value: create(Config_Override_TextPatchSchema, { content: event.target.value }) } }))} />
              </div>
            ) : null}
            {override.override.case === "keyValuePatch"
              ? (() => {
                  const kv = override.override.value;
                  return (
                    <div className="space-y-3">
                      <KeyValueEditor
                        label="Set"
                        value={kv.set}
                        onChange={(set) => update(index, create(Config_OverrideSchema, { ...override, override: { case: "keyValuePatch", value: create(Config_Override_KeyValuePatchSchema, { set, remove: kv.remove }) } }))}
                      />
                      <StringListEditor
                        label="Remove keys"
                        value={kv.remove}
                        onChange={(remove) => update(index, create(Config_OverrideSchema, { ...override, override: { case: "keyValuePatch", value: create(Config_Override_KeyValuePatchSchema, { set: kv.set, remove }) } }))}
                      />
                    </div>
                  );
                })()
              : null}
          </div>
        );
      })}
      <Button type="button" variant="outline" size="sm" onClick={() => setOverrides([...value.overrides, create(Config_OverrideSchema, { override: { case: "textPatch", value: create(Config_Override_TextPatchSchema, {}) } })])}>
        <Plus className="size-3.5" />
        Add override
      </Button>
    </div>
  );
}
