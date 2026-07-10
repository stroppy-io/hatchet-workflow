# Derived code-server image with the stroppy-yaml language client AND
# Terraform support baked in.
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
# or install step to redo. The same reasoning applies to the Terraform
# extension added below: it is installed the same way, as the same uid,
# for the same reason.
#
# The stroppy-yaml-lsp BINARY itself is deliberately NOT baked in here — it
# stays a read-only bind mount via internal/ide.Manager.Config.LSPBinaryPath
# (IDE_LSP_BINARY_PATH), unchanged from the prior task. That keeps the
# server-side compiler binary on the same "operator provisions a host path,
# unset by default" lifecycle as everything else IDE_* in this repo, instead
# of coupling a Go binary rebuild to a Docker image rebuild.
#
# Terraform support: provider repos (see storage model) hold real .tf files
# under module/, so authors need real Terraform tooling, not just YAML
# support. code-server does NOT resolve extensions from the Microsoft
# Marketplace (Microsoft's marketplace ToS forbids non-Microsoft products
# from using it) — it resolves from Open VSX by default, and that was
# verified locally against this exact base image
# (codercom/code-server:4.96.4): `code-server --install-extension
# hashicorp.terraform` succeeds and pulls hashicorp.terraform from
# open-vsx.org. HashiCorp publishes this same extension to Open VSX
# (MPL-2.0 licensed, "verified" publisher), so it is both available and
# license-clean.
#
# The platform-specific (linux-x64) build of hashicorp.terraform bundles
# its own terraform-ls binary at extension/bin/terraform-ls — it does NOT
# fetch it at runtime. That binary was verified locally to be byte-for-byte
# identical (matching sha256) to HashiCorp's own official
# terraform-ls_0.38.8_linux_amd64.zip release from releases.hashicorp.com,
# so baking in the bundled copy is equivalent to baking in the upstream
# release binary — see .superpowers/sdd/ide-terraform-report.md for the
# exact checksums. We symlink it onto PATH at build time (see the second
# RUN below) so `terraform-ls` resolves like any other CLI tool, with zero
# network access needed at container start.
#
# Build:
#   make ide-extension            # compiles + packages extensions/stroppy-yaml-lsp-client
#   make ide-terraform-extension  # downloads + checksum-verifies the pinned hashicorp.terraform VSIX
#   make ide-image                # runs this Dockerfile against both of the above
# Then point IDE_IMAGE (docker-compose.yaml / IdeImage config) at the result
# instead of the stock codercom/code-server tag — everything else about
# internal/ide.Manager is unchanged.
ARG IDE_BASE_IMAGE=codercom/code-server:4.96.4
FROM ${IDE_BASE_IMAGE}

# Build context is build/ide-image/ (assembled by the ide-image Makefile
# target from ide-extension's dist/ output + ide-terraform-extension's
# pinned download) — exactly these two *.vsix files, named explicitly so a
# stray third file in the build dir can't get installed by accident.
COPY --chown=1000:1000 stroppy-yaml-lsp-client.vsix /tmp/stroppy-yaml-lsp-client.vsix
COPY --chown=1000:1000 hashicorp.terraform.vsix /tmp/hashicorp.terraform.vsix
RUN code-server --install-extension /tmp/stroppy-yaml-lsp-client.vsix --force \
    && code-server --install-extension /tmp/hashicorp.terraform.vsix --force \
    && rm /tmp/stroppy-yaml-lsp-client.vsix /tmp/hashicorp.terraform.vsix

# Put the terraform-ls binary bundled inside the just-installed extension
# onto PATH. This needs root because /usr/local/bin is root-owned in the
# base image (world-readable/executable, so the coder user can still run
# whatever lands there); the extensions directory itself stays owned by
# coder from the RUN above. Switch back to uid 1000 afterwards to match
# the base image's runtime user.
USER root
RUN set -eux; \
    ext_dir="$(find /home/coder/.local/share/code-server/extensions -maxdepth 1 -iname 'hashicorp.terraform-*' -print -quit)"; \
    test -n "$ext_dir"; \
    install -m 0755 "$ext_dir/bin/terraform-ls" /usr/local/bin/terraform-ls
USER 1000
