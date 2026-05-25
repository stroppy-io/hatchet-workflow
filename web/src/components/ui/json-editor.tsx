import CodeMirror from "@uiw/react-codemirror";
import { json } from "@codemirror/lang-json";
import { oneDark } from "@codemirror/theme-one-dark";

type JsonEditorProps = {
  value: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
  minHeight?: string;
};

// CodeMirror JSON editor. Plain string in/out — callers serialize proto messages
// via toJsonString/fromJsonString (keeps this component schema-agnostic).
export function JsonEditor({ value, onChange, readOnly = false, minHeight = "240px" }: JsonEditorProps) {
  return (
    <div className="overflow-hidden rounded-md border text-xs">
      <CodeMirror
        value={value}
        theme={oneDark}
        extensions={[json()]}
        editable={!readOnly}
        onChange={onChange}
        minHeight={minHeight}
        basicSetup={{ lineNumbers: true, foldGutter: false, highlightActiveLine: false, highlightActiveLineGutter: false }}
      />
    </div>
  );
}
