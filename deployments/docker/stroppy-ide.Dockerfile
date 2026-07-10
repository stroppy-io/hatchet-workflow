# Derived code-server image with the stroppy-yaml language client baked in.
#
# codercom/code-server ships no Dockerfile in this repo to extend, and there
# is no clean way to install a VS Code extension into
# ~/.local/share/code-server/extensions via a bind mount: Docker creates any
# missing leading path components of a bind-mount target as root, and
# code-server (running as the non-root "coder" user, uid 1000) then can't
# create its OWN sibling state directories (User/, Machine/, logs/,
# extensions.json) under that now-root-owned tree — verified locally, see
# .superpowers/sdd/lsp-extension-report.md. Baking the extension in at build
# time avoids all of that: `code-server --install-extension` runs as the same
# uid (1000) the base image already sets via USER, so the installed
# extension directory it creates already has the right ownership, and it
# becomes part of the image layer rather than anything Manager.EnsureRunning
# has to mount per-container-start. A recreated container is just a fresh
# instance of this same image — the extension is always there, no bind mount
# or install step to redo.
#
# The stroppy-yaml-lsp BINARY itself is deliberately NOT baked in here — it
# stays a read-only bind mount via internal/ide.Manager.Config.LSPBinaryPath
# (IDE_LSP_BINARY_PATH), unchanged from the prior task. That keeps the
# server-side compiler binary on the same "operator provisions a host path,
# unset by default" lifecycle as everything else IDE_* in this repo, instead
# of coupling a Go binary rebuild to a Docker image rebuild.
#
# Build:
#   make ide-extension   # compiles + packages extensions/stroppy-yaml-lsp-client
#   make ide-image        # runs this Dockerfile against that package
# Then point IDE_IMAGE (docker-compose.yaml / IdeImage config) at the result
# instead of the stock codercom/code-server tag — everything else about
# internal/ide.Manager is unchanged.
ARG IDE_BASE_IMAGE=codercom/code-server:4.96.4
FROM ${IDE_BASE_IMAGE}

# Build context is extensions/stroppy-yaml-lsp-client/dist/ (see the
# ide-image Makefile target) — its only *.vsix is the packaged extension.
COPY --chown=1000:1000 *.vsix /tmp/stroppy-yaml-lsp-client.vsix
RUN code-server --install-extension /tmp/stroppy-yaml-lsp-client.vsix --force \
    && rm /tmp/stroppy-yaml-lsp-client.vsix
