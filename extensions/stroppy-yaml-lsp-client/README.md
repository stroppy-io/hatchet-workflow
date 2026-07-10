# Stroppy YAML

Thin VS Code / code-server language client for the stroppy-yaml DSL
(`cluster.yaml` / `workflow.yaml` bundles). Spawns
`/usr/local/bin/stroppy-yaml-lsp` (built from `cmd/stroppy-yaml-lsp` in the
[stroppy-cloud](https://github.com/stroppy-io/stroppy-cloud) repo) as a
stdio language server and surfaces:

- live diagnostics (`textDocument/publishDiagnostics`)
- completion (`textDocument/completion`)
- **Stroppy: Preview Compiled Plan** command, bound to the server's
  `stroppy/previewBundle` `workspace/executeCommand`.

All validation, schema, and compilation logic lives in the Go LSP server and
the compiler it delegates to (`internal/services/dsl`) — this extension adds
none of its own.

Not published to a marketplace; built and shipped alongside the
`stroppy-cloud` server (see `Makefile`'s `ide-extension` target and
`internal/ide.Manager`'s `LSPExtensionPath`).
