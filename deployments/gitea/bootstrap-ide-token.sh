#!/usr/bin/env bash
# bootstrap-ide-token.sh — provisions the Gitea service account + access
# token the SP-C git backend / IDE gateway need (see docker-compose.yaml's
# comment above the `gitea` service, and internal/app.Config.GiteaToken).
#
# What it does:
#   1. Creates the "stroppy-bot" admin user inside the running gitea
#      container, with a freshly generated random password, IF it does not
#      already exist (idempotent: a second run skips this step).
#   2. Mints a "server" access token (scopes: write:user, write:repository,
#      write:organization) for that user, IF no token of that name already
#      exists for it (idempotent: a second run reports the token already
#      exists and exits without minting a duplicate — Gitea has no API to
#      recover a previously-issued token's plaintext, so this script cannot
#      "re-print" it; rotate by deleting the token in the Gitea admin UI
#      first, then re-running this script).
#   3. Prints the .env lines to add — NEVER writes them to a file, an image
#      layer, or any log this script does not own. The token only ever
#      appears in THIS script's own stdout, exactly once, at mint time.
#
# Safety:
#   - Never run against the live production stand — this only ever targets
#     whatever `docker compose` resolves to from the current directory
#     (COMPOSE_PROJECT_NAME / -p / docker-compose.yaml discovery), so run it
#     from a throwaway/local checkout, not a directory pointed at a remote
#     production context.
#   - The password/token never touch disk: openssl output is piped directly
#     into `docker compose exec`'s stdin/args, never written to a temp file.
#
# Usage:
#   ./deployments/gitea/bootstrap-ide-token.sh
#   make ide-token

set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yaml}"
GITEA_SERVICE="${GITEA_SERVICE:-gitea}"
BOT_USER="${GITEA_BOT_USER:-stroppy-bot}"
TOKEN_NAME="${GITEA_TOKEN_NAME:-server}"
TOKEN_SCOPES="write:user,write:repository,write:organization"

compose() {
	docker compose -f "$COMPOSE_FILE" "$@"
}

# gitea's official image entrypoint drops privileges to the "git" user for
# the gitea process itself and refuses to run its CLI as root ("Gitea is not
# supposed to be run as root") — exec as git explicitly, matching how the
# entrypoint itself invokes the binary.
compose_gitea_exec() {
	compose exec -T --user git "$GITEA_SERVICE" "$@"
}

if ! compose ps --status running --services 2>/dev/null | grep -qx "$GITEA_SERVICE"; then
	echo "error: '$GITEA_SERVICE' service is not running (docker compose -f $COMPOSE_FILE up -d $GITEA_SERVICE first)" >&2
	exit 1
fi

echo "==> Checking for existing user '$BOT_USER'..."
if compose_gitea_exec gitea admin user list 2>/dev/null | awk '{print $2}' | grep -qx "$BOT_USER"; then
	echo "==> User '$BOT_USER' already exists, skipping creation."
else
	echo "==> Creating user '$BOT_USER'..."
	# Generated locally and piped straight into the container's stdin-less
	# arg list; never written to disk, never echoed by this script.
	bot_password="$(openssl rand -base64 24)"
	compose_gitea_exec gitea admin user create \
		--admin --username "$BOT_USER" --password "$bot_password" \
		--email bootstrap@stroppy.local --must-change-password=false
	echo "==> User '$BOT_USER' created (its password was not printed; it is only needed for interactive Gitea login, not for the token below)."
fi

echo "==> Checking for existing token '$TOKEN_NAME' on '$BOT_USER'..."
# Captured straight into a shell variable (never a file, never an image
# layer): gen_output holds the token in plaintext only for the remainder of
# this process's memory.
set +e
gen_output="$(compose_gitea_exec gitea admin user generate-access-token \
	--username "$BOT_USER" --token-name "$TOKEN_NAME" --scopes "$TOKEN_SCOPES" 2>&1)"
gen_status=$?
set -e

if [ "$gen_status" -eq 0 ]; then
	token="$(printf '%s\n' "$gen_output" | sed -n 's/^Access token was successfully created: //p')"
	if [ -z "$token" ]; then
		echo "error: could not parse the generated token from gitea's output" >&2
		exit 1
	fi
	echo
	echo "==> Token minted. Add these lines to a git-ignored .env (NEVER commit them):"
	echo
	echo "GITEA_TOKEN=$token"
	echo "IDE_MANAGER_ENABLED=true"
	echo
	echo "Then: docker compose -f $COMPOSE_FILE up -d server"
	unset token gen_output
elif printf '%s\n' "$gen_output" | grep -Eqi "already exists|has been used already"; then
	echo "==> A token named '$TOKEN_NAME' already exists for '$BOT_USER'."
	echo "    Gitea cannot re-print an existing token's plaintext. To rotate:"
	echo "      1. Delete it: Gitea UI -> $BOT_USER -> Settings -> Applications -> revoke '$TOKEN_NAME'"
	echo "      2. Re-run this script."
	exit 0
else
	echo "error: token generation failed:" >&2
	printf '%s\n' "$gen_output" >&2
	exit 1
fi
