import { useEffect, useMemo, useRef } from "react";
import CodeMirror, { EditorView, type Extension } from "@uiw/react-codemirror";
import { yaml } from "@codemirror/lang-yaml";
import { linter, lintGutter, type Diagnostic as CmDiagnostic } from "@codemirror/lint";
import type { Text } from "@codemirror/state";
import { checkBundle, type DiagnosticVM } from "@/services/recipe";

interface DslEditorProps {
  /** The bundle-relative path of the file currently open in this editor
   * (e.g. "cluster.yaml", "components/postgres.yaml"). Diagnostics from
   * checkBundle are filtered down to this path. */
  path: string;
  value: string;
  onChange: (v: string) => void;
  /** The WHOLE bundle (path -> file text) — checkBundle needs every file to
   * resolve cross-file references, not just the one currently open. */
  files: Record<string, string>;
  readOnly?: boolean;
}

function pickLanguage(path: string): Extension | null {
  const head = path.toLowerCase();
  if (head.endsWith(".yaml") || head.endsWith(".yml")) {
    return yaml();
  }
  return null;
}

// Converts a server DiagnosticVM (1-based line/col) into a CodeMirror
// Diagnostic (0-based char offsets), clamped to the document's bounds so a
// stale or out-of-range line/col (e.g. the bundle changed underneath an
// in-flight check) can never throw inside doc.line().
function toCmDiagnostic(d: DiagnosticVM, doc: Text): CmDiagnostic {
  const lineNo = Math.min(Math.max(d.line, 1), doc.lines);
  const lineInfo = doc.line(lineNo);
  const from = Math.min(Math.max(lineInfo.from + Math.max(0, d.col - 1), lineInfo.from), lineInfo.to);
  const to = Math.min(from + 1, lineInfo.to);
  return {
    from,
    to: to > from ? to : from,
    severity: d.severity,
    message: d.message,
    source: d.module || undefined,
  };
}

// DslEditor wraps CodeMirror 6 (mirrors JsonEditor's setup) with a
// checkBundle-backed linter: every ~400ms of editing quiet, it re-checks the
// WHOLE bundle and surfaces the diagnostics that belong to this file as
// inline squiggles + a lint gutter.
export function DslEditor({ path, value, onChange, files, readOnly = false }: DslEditorProps) {
  // The linter extension is created exactly once (see dslLinter below) so
  // CodeMirror doesn't tear down/rebuild lint state on every keystroke. Its
  // async source closure would otherwise capture the `files`/`path` props
  // from whichever render created it — this ref is the escape hatch, kept
  // current via an effect so the source always reads the latest bundle.
  const latestRef = useRef({ path, files });
  useEffect(() => {
    latestRef.current = { path, files };
  }, [path, files]);

  const fontTheme = useMemo(
    () =>
      EditorView.theme({
        "&": { fontSize: "11px" },
        ".cm-content": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" },
        ".cm-gutters": { fontSize: "11px" },
      }),
    [],
  );

  const lang = useMemo(() => pickLanguage(path), [path]);

  const dslLinter = useMemo(
    () =>
      linter(
        async (view): Promise<CmDiagnostic[]> => {
          const { path: currentPath, files: currentFiles } = latestRef.current;
          let diagnostics: DiagnosticVM[];
          try {
            diagnostics = await checkBundle(currentFiles);
          } catch (err) {
            // Don't let a check RPC failure (e.g. transient network error,
            // server down) crash the editor — just drop the diagnostics for
            // this pass and try again on the next edit.
            console.warn("DslEditor: checkBundle failed", err);
            return [];
          }
          return diagnostics
            .filter((d) => d.path === currentPath)
            .map((d) => toCmDiagnostic(d, view.state.doc));
        },
        { delay: 400 },
      ),
    [],
  );

  const extensions = useMemo(() => {
    const exts: Extension[] = [dslLinter, lintGutter(), EditorView.lineWrapping, fontTheme];
    if (lang) exts.unshift(lang);
    return exts;
  }, [lang, dslLinter, fontTheme]);

  return (
    <div className="border border-zinc-800 focus-within:border-zinc-600 transition-colors">
      <CodeMirror
        value={value}
        height="100%"
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
