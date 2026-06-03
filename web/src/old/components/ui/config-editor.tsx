import { useMemo } from "react";
import CodeMirror, { EditorView, type Extension } from "@uiw/react-codemirror";
import { yaml } from "@codemirror/lang-yaml";
import { iniLike } from "./cm-modes/ini-like";
import { haproxyMode } from "./cm-modes/haproxy";
import { pgHbaMode } from "./cm-modes/pg-hba";

interface ConfigEditorProps {
  /**
   * Filename (or rendered-config key like "postgresql.conf:master") used to
   * pick a language by extension. Examples:
   *   - postgresql.conf, pg_hba.conf  → properties
   *   - my.cnf:primary                → properties
   *   - haproxy.cfg                   → properties
   *   - pgbouncer.ini                 → properties
   *   - ydb.yaml:storage              → yaml
   *   - patroni.yml                   → yaml
   * Anything else falls back to plain text + JSON-style highlight off.
   */
  filename?: string;
  value: string;
  onChange: (next: string) => void;
  height?: string;
  readOnly?: boolean;
}

function pickLanguage(filename: string | undefined): Extension | null {
  if (!filename) return null;
  // Rendered-config keys can be "<file>:<scope>"; trim the scope before
  // matching the extension.
  const head = filename.split(":")[0].toLowerCase();
  if (head.endsWith(".yaml") || head.endsWith(".yml")) {
    return yaml();
  }
  // HAProxy gets its own dialect — sections (frontend/backend/…), directives
  // (bind/server/option/timeout/…), and IP:port literals are all handled.
  if (head.endsWith(".cfg") && head.includes("haproxy")) {
    return haproxyMode;
  }
  // pg_hba.conf is column-based (type/db/user/address/method), not
  // key=value, so the generic INI mode mis-tokenizes the columns. Route
  // it to a purpose-built mode.
  if (head.endsWith("pg_hba.conf") || head === "pg_hba.conf") {
    return pgHbaMode;
  }
  // Generic INI-style covers postgresql.conf, pg_hba.conf, my.cnf,
  // pgbouncer.ini, haproxy.cfg fallback. Recognises section headers,
  // numbers with units (4MB, 1d, 100ms), CIDR addresses, comments, etc.
  if (
    head.endsWith(".conf") ||
    head.endsWith(".cfg") ||
    head.endsWith(".ini") ||
    head.endsWith(".properties") ||
    head.endsWith(".cnf")
  ) {
    return iniLike;
  }
  return null;
}

// ConfigEditor wraps CodeMirror 6 + vscodeDark for arbitrary config
// formats (yaml, ini-style .conf/.cfg/.cnf, .properties). Mirrors
// JsonEditor but with a language picker keyed off the filename so a
// preview tab can render PostgreSQL conf, HAProxy cfg, YDB yaml, etc.
// with the right highlight without each call site knowing which mode to
// pass.
export function ConfigEditor({ filename, value, onChange, height = "16rem", readOnly = false }: ConfigEditorProps) {
  const lang = useMemo(() => pickLanguage(filename), [filename]);
  const fontTheme = useMemo(
    () =>
      EditorView.theme({
        "&": { fontSize: "11px" },
        ".cm-content": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" },
        ".cm-gutters": { fontSize: "11px" },
      }),
    [],
  );
  const extensions = useMemo(() => {
    // theme="dark" on <CodeMirror> installs oneDark (settings + HighlightStyle).
    // Without explicit `theme`, @uiw/react-codemirror falls back to its
    // bundled light theme, producing a white background.
    const exts: Extension[] = [
      EditorView.lineWrapping,
      fontTheme,
    ];
    if (lang) exts.unshift(lang);
    return exts;
  }, [lang, fontTheme]);

  return (
    <div className="border border-zinc-800 focus-within:border-zinc-600 transition-colors">
      <CodeMirror
        value={value}
        height={height}
        readOnly={readOnly}
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
