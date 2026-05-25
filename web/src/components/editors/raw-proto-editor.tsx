import { useState } from "react";
import { fromJsonString, toJsonString, type DescMessage, type MessageShape } from "@bufbuild/protobuf";

import { JsonEditor } from "@/components/ui/json-editor";
import { Button } from "@/components/ui/button";

// Editable raw-proto JSON editor for deep messages where field-by-field forms
// add little value (rendered/materialized artifacts: DeploymentIntent, configs).
// Holds local text so typing isn't fought by re-serialization; commits on Apply
// via fromJsonString, surfacing parse/validation errors.
export function RawProtoEditor<Desc extends DescMessage>({
  schema,
  value,
  onChange,
  minHeight = "280px",
}: {
  schema: Desc;
  value: MessageShape<Desc>;
  onChange: (value: MessageShape<Desc>) => void;
  minHeight?: string;
}) {
  const [text, setText] = useState(() => toJsonString(schema, value, { prettySpaces: 2 }));
  const [error, setError] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);

  function apply() {
    try {
      onChange(fromJsonString(schema, text));
      setError(null);
      setDirty(false);
    } catch (parseError) {
      setError(parseError instanceof Error ? parseError.message : "Invalid JSON");
    }
  }

  return (
    <div className="space-y-2">
      <JsonEditor
        value={text}
        minHeight={minHeight}
        onChange={(next) => {
          setText(next);
          setDirty(true);
        }}
      />
      <div className="flex items-center gap-3">
        <Button type="button" size="sm" variant="outline" onClick={apply} disabled={!dirty}>
          Apply
        </Button>
        {error ? <span className="text-xs text-destructive">{error}</span> : dirty ? <span className="text-xs text-muted-foreground">Unapplied changes</span> : null}
      </div>
    </div>
  );
}
