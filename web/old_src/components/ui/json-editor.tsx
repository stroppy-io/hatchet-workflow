import { useMemo } from "react";
import CodeMirror, { EditorView } from "@uiw/react-codemirror";
import { json, jsonParseLinter } from "@codemirror/lang-json";
import { linter, lintGutter } from "@codemirror/lint";

interface JsonEditorProps {
  value: string;
  onChange: (next: string) => void;
  height?: string;
  readOnly?: boolean;
  placeholder?: string;
}

// JsonEditor wraps CodeMirror 6 with the project's dark color palette
// (matches the surrounding zinc-900 textareas) plus JSON syntax highlight
// and a parse linter that puts a red underline on the offending line.
export function JsonEditor({ value, onChange, height = "18rem", readOnly = false, placeholder }: JsonEditorProps) {
  // Font-size override layered on top of oneDark — keeps token colors
  // intact while shrinking text to match the surrounding 11px monospace UI.
  const fontTheme = useMemo(
    () =>
      EditorView.theme({
        "&": { fontSize: "11px" },
        ".cm-content": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" },
        ".cm-gutters": { fontSize: "11px" },
      }),
    [],
  );

  // Memoise extensions so CodeMirror doesn't tear down its editor state on
  // every keystroke (extensions are reference-checked).
  const extensions = useMemo(
    () => [
      json(),
      linter(jsonParseLinter()),
      lintGutter(),
      EditorView.lineWrapping,
      fontTheme,
    ],
    [fontTheme],
  );

  return (
    <div className="border border-zinc-800 focus-within:border-zinc-600 transition-colors">
      <CodeMirror
        value={value}
        height={height}
        readOnly={readOnly}
        placeholder={placeholder}
        onChange={onChange}
        theme="dark"
        extensions={extensions}
        basicSetup={{
          lineNumbers: true,
          highlightActiveLine: true,
          foldGutter: true,
          bracketMatching: true,
          autocompletion: false,
          searchKeymap: true,
        }}
      />
    </div>
  );
}
