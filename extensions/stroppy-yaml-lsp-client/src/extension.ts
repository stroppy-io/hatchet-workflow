// Stroppy YAML language client.
//
// This extension is a THIN client: it spawns cmd/stroppy-yaml-lsp
// (internal/ide/lsp.Server) as a stdio subprocess and forwards the standard
// LSP surface (diagnostics, completion) to `vscode-languageclient`. It
// contains no DSL validation, schema, or compiler logic of its own — every
// domain answer comes from the Go LSP, which itself delegates strictly to
// internal/services/dsl.DslService (see internal/ide/lsp/server.go). Do NOT
// add parsing/validation here; a second, drifting implementation is exactly
// the failure this project is trying to avoid (see the launch-form
// regression this codebase's memory already tracks).
//
// The LSP binary and this extension both run INSIDE the org's code-server
// container (see internal/ide.Manager.EnsureRunning) — this file therefore
// never talks to Gitea, never sees a Gitea token, and never reaches outside
// the container's own filesystem. Isolation between orgs is enforced by
// internal/ide.Session.ResolvePath on the Go side, not by anything here.

import * as fs from "fs";
import * as vscode from "vscode";
import {
  DocumentSelector,
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

// previewCommand MUST match internal/ide/lsp/server.go's previewCommand
// constant ("stroppy/previewBundle") exactly — it is the workspace/
// executeCommand id the Go server registers in its ExecuteCommandProvider
// capability and is the only command it accepts.
const previewCommand = "stroppy/previewBundle";

// documentSelector matches the bundle file shapes internal/ide/lsp/bundle.go
// looks for: cluster.yaml/workflow.yaml at a bundle root, plus any YAML file
// under that bundle's components/** subtree (spec: "activates on
// cluster.yaml / workflow.yaml (and the bundle's components/**)").
const documentSelector: DocumentSelector = [
  { scheme: "file", pattern: "**/cluster.yaml" },
  { scheme: "file", pattern: "**/workflow.yaml" },
  { scheme: "file", pattern: "**/components/**/*.yaml" },
  { scheme: "file", pattern: "**/components/**/*.yml" },
];

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const serverPath = vscode.workspace
    .getConfiguration("stroppyYaml")
    .get<string>("serverPath", "/usr/local/bin/stroppy-yaml-lsp");

  // The binary is only present when internal/ide.Manager.Config.
  // LSPBinaryPath was configured for this scope (IDE_LSP_BINARY_PATH) — off
  // by default. Fail soft, not loud: a code-server session with no LSP
  // mounted should still be a perfectly usable plain editor, exactly like
  // every deployment before this feature existed.
  if (!fs.existsSync(serverPath)) {
    vscode.window.setStatusBarMessage(
      `Stroppy YAML: language server not found at ${serverPath} — diagnostics disabled.`,
      10_000,
    );
    return;
  }

  const serverOptions: ServerOptions = {
    command: serverPath,
    args: [],
    transport: TransportKind.stdio,
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector,
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher(
        "**/{cluster.yaml,workflow.yaml,components/**}",
      ),
    },
  };

  client = new LanguageClient(
    "stroppyYaml",
    "Stroppy YAML Language Server",
    serverOptions,
    clientOptions,
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("stroppyYaml.previewBundle", () =>
      previewActiveBundle(context),
    ),
  );

  await client.start();
  context.subscriptions.push({ dispose: () => client?.stop() });
}

export async function deactivate(): Promise<void> {
  await client?.stop();
}

// PreviewResult mirrors internal/ide/lsp/preview.go's PreviewResult 1:1
// ({"plan": dslpb.CompiledPlan | null, "diagnostics": dslpb.Diagnostic[]}) —
// this extension does not interpret either field beyond pretty-printing
// them; RecipeEditor.tsx already owns real plan rendering, and duplicating
// that here would be exactly the parallel-implementation drift this
// codebase's docs warn against.
interface PreviewResult {
  plan: unknown;
  diagnostics: unknown[];
}

async function previewActiveBundle(context: vscode.ExtensionContext): Promise<void> {
  if (!client) {
    vscode.window.showWarningMessage("Stroppy YAML: language server is not running.");
    return;
  }
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showWarningMessage("Stroppy YAML: open a cluster.yaml or workflow.yaml first.");
    return;
  }

  let result: PreviewResult;
  try {
    result = await client.sendRequest("workspace/executeCommand", {
      command: previewCommand,
      arguments: [editor.document.uri.toString()],
    });
  } catch (err) {
    vscode.window.showErrorMessage(`Stroppy YAML: preview failed: ${String(err)}`);
    return;
  }

  const panel = vscode.window.createWebviewPanel(
    "stroppyYamlPreview",
    "Stroppy: Compiled Plan Preview",
    vscode.ViewColumn.Beside,
    { enableScripts: false },
  );
  panel.webview.html = renderPreviewHtml(result);
  context.subscriptions.push(panel);
}

function renderPreviewHtml(result: PreviewResult): string {
  const escape = (s: string) =>
    s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const diagCount = Array.isArray(result.diagnostics) ? result.diagnostics.length : 0;
  const body = escape(JSON.stringify(result, null, 2));
  return `<!doctype html>
<html>
<body style="font-family: var(--vscode-editor-font-family, monospace);">
  <h3>Compiled plan ${result.plan ? "" : "(compilation failed)"} — ${diagCount} diagnostic(s)</h3>
  <pre>${body}</pre>
</body>
</html>`;
}
