#!/usr/bin/env bash
# End-to-end smoke for the durable-scheduler stack:
#
#   1. compose up everything (postgres + vm + grafana + server)
#   2. wait for /health
#   3. login as admin/admin → grab JWT
#   4. probe the scheduler API surface (/queue, /quotas)
#   5. launch a single minimal postgres run on the docker provider
#   6. poll until run reaches a terminal state
#   7. dump scheduler+job state for inspection
#
# Goal: tiny resource footprint so dev laptops can run it. The run uses
# vus=1, scale=1, duration=10s — enough to exercise every DAG phase but
# finishes in ~30s wall time.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

API="${API:-http://127.0.0.1:8080}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-admin}"
RUN_TIMEOUT_SECONDS="${RUN_TIMEOUT_SECONDS:-900}"

log() { printf '\033[36m▶ %s\033[0m\n' "$*"; }
fail() { printf '\033[31m✘ %s\033[0m\n' "$*"; exit 1; }
ok()  { printf '\033[32m✔ %s\033[0m\n' "$*"; }

require() {
  command -v "$1" >/dev/null 2>&1 || fail "missing dependency: $1"
}

require docker
require curl
require python3

log "bringing up stack (compose up -d)"
docker compose up -d --wait postgres victorialogs vmstorage vminsert vmselect vmagent vmauth grafana >/dev/null
docker compose up -d --build --wait server >/dev/null

log "waiting for /health"
deadline=$(( $(date +%s) + 60 ))
while :; do
  if curl -sf "$API/health" >/dev/null; then break; fi
  [ "$(date +%s)" -gt "$deadline" ] && fail "server did not become healthy in 60s"
  sleep 1
done
ok "server healthy"

log "login → JWT"
TOKEN=$(curl -sf -X POST "$API/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])')
[ -z "$TOKEN" ] && fail "login failed"
ok "login ok"

auth() { curl -sf -H "Authorization: Bearer $TOKEN" "$@"; }

log "verify scheduler endpoints"
auth "$API/api/v1/queue" >/dev/null || fail "/queue unreachable"
auth "$API/api/v1/quotas" >/dev/null || fail "/quotas unreachable"
ok "/queue + /quotas reachable"

log "find postgres single preset"
PRESET_ID=$(auth "$API/api/v1/presets?db_kind=postgres" \
  | python3 -c '
import json,sys
ps=json.load(sys.stdin)
for p in ps:
    if "single" in p["name"].lower():
        print(p["id"]); break')
[ -z "$PRESET_ID" ] && fail "no postgres single preset found"
ok "preset $PRESET_ID"

RUN_ID="smoke-$(date +%s)"
log "launch minimal docker run id=$RUN_ID"
cat > /tmp/smoke-run.json <<JSON
{
  "id": "$RUN_ID",
  "name": "smoke",
  "description": "minimum-cost smoke run",
  "provider": "docker",
  "network": {"cidr": "10.99.0.0/24"},
  "machines": [],
  "database": {"kind": "postgres", "version": "16"},
  "monitor": {},
  "stroppy": {
    "version": "5.1.2",
    "script": "tpcc/procs",
    "duration": "10s",
    "vus": 1,
    "pool_size": 2,
    "scale_factor": 1,
    "k6_mode": "duration",
    "no_thresholds": true
  },
  "preset_id": "$PRESET_ID"
}
JSON

# Retry POST a few times — first run after `compose up --build` can race
# with the scheduler claim loop spinning up. We pin -w to pull the HTTP
# code so we can distinguish "transient 5xx" from a hard 4xx.
START=""
for attempt in 1 2 3 4 5; do
  TMP_OUT=$(mktemp)
  CODE=$(curl -s -o "$TMP_OUT" -w "%{http_code}" \
    -X POST -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" -d @/tmp/smoke-run.json "$API/api/v1/run")
  BODY=$(cat "$TMP_OUT"); rm -f "$TMP_OUT"
  if [ "$CODE" = "202" ] || [ "$CODE" = "200" ]; then
    START="$BODY"; break
  fi
  echo "  attempt $attempt: HTTP $CODE body=$BODY"
  sleep 2
done
[ -z "$START" ] && fail "POST /run failed after retries"
echo "  $START"
STATE=$(echo "$START" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
[ "$STATE" != "queued" ] && fail "expected queued, got $STATE"
ok "run enqueued"

log "polling run status (up to ${RUN_TIMEOUT_SECONDS}s)"
deadline=$(( $(date +%s) + RUN_TIMEOUT_SECONDS ))
while :; do
  S=$(auth "$API/api/v1/run/$RUN_ID/status")
  SUMMARY=$(echo "$S" | python3 -c '
import json,sys
s=json.load(sys.stdin)
nodes=s.get("nodes") or []
done=sum(1 for n in nodes if n["status"]=="done")
fail=sum(1 for n in nodes if n["status"]=="failed")
runn=sum(1 for n in nodes if n["status"]=="running")
pend=sum(1 for n in nodes if n["status"]=="pending")
js=s.get("job_state","")
print(f"job={js} done={done} run={runn} fail={fail} pend={pend} total={len(nodes)}")
if pend==0 and runn==0 and len(nodes)>0:
    print("DONE")
elif js in ("failed","cancelled"):
    print("DONE")
')
  STATUS_LINE=$(echo "$SUMMARY" | head -1)
  printf '  %s\n' "$STATUS_LINE"
  if echo "$SUMMARY" | grep -q '^DONE$'; then break; fi
  [ "$(date +%s)" -gt "$deadline" ] && fail "run did not finish within ${RUN_TIMEOUT_SECONDS}s"
  sleep 5
done

log "final job state"
auth "$API/api/v1/run/$RUN_ID/status" | python3 -c '
import json, sys
s = json.load(sys.stdin)
nodes = s.get("nodes") or []
print("  job_state:", s.get("job_state", ""))
for n in nodes:
    nid = n.get("id", "?")
    status = n.get("status", "?")
    err = (n.get("error") or "")[:200]
    if status == "failed":
        print("  X " + nid + ": " + err)
    else:
        print("  - " + nid + ": " + status)
'

log "/quotas after run (should be zeroed if finished)"
auth "$API/api/v1/quotas" | python3 -m json.tool

ok "smoke completed"
