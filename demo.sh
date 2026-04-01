#!/bin/bash
# =============================================================================
# demo.sh — Launch two test runs in parallel and compare them
# =============================================================================
# Usage: ./demo.sh
# Prerequisites: docker compose stack running (docker compose -f docker-compose.test.yaml up -d)
# =============================================================================
set -euo pipefail

URL="http://localhost:8080"
USER="admin"
PASS="admin"
TS=$(date +%s)

RUN_A="demo-pg-single-$TS"
RUN_B="demo-pg-ha-$TS"

echo "=== Stroppy Cloud Demo ==="
echo ""
echo "  Run A: PostgreSQL single  + TPC-C, 8 workers, 5 min"
echo "  Run B: PostgreSQL HA      + TPC-C, 8 workers, 5 min"
echo ""

# --- Auth ---
echo "[1/5] Authenticating..."
TOKEN=$(curl -sf -X POST "$URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" | jq -r .token)
AUTH="Authorization: Bearer $TOKEN"
echo "  OK"

# --- Fetch HA preset ---
echo ""
echo "[2/5] Starting both runs in parallel..."

# Run A: single topology, tpcc
curl -sf -X POST "$URL/api/v1/run" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{
    \"id\": \"$RUN_A\",
    \"provider\": \"docker\",
    \"network\": {\"cidr\": \"10.10.0.0/24\"},
    \"machines\": [],
    \"database\": {\"kind\": \"postgres\", \"version\": \"16\",
      \"postgres\": {
        \"master\": {\"role\": \"database\", \"count\": 1, \"cpus\": 2, \"memory_mb\": 4096, \"disk_gb\": 50},
        \"pgbouncer\": false, \"patroni\": false, \"etcd\": false, \"sync_replicas\": 0
      }},
    \"monitor\": {},
    \"stroppy\": {\"version\": \"3.1.0\", \"workload\": \"tpcc\", \"duration\": \"5m\", \"workers\": 8}
  }" > /dev/null
echo "  A: $RUN_A started (single)"

# Run B: HA topology (master + 2 replicas + haproxy + etcd), tpcc
curl -sf -X POST "$URL/api/v1/run" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{
    \"id\": \"$RUN_B\",
    \"provider\": \"docker\",
    \"network\": {\"cidr\": \"10.20.0.0/24\"},
    \"machines\": [],
    \"database\": {\"kind\": \"postgres\", \"version\": \"16\",
      \"postgres\": {
        \"master\": {\"role\": \"database\", \"count\": 1, \"cpus\": 2, \"memory_mb\": 4096, \"disk_gb\": 50},
        \"replicas\": [{\"role\": \"database\", \"count\": 2, \"cpus\": 2, \"memory_mb\": 4096, \"disk_gb\": 50}],
        \"haproxy\": {\"role\": \"proxy\", \"count\": 1, \"cpus\": 1, \"memory_mb\": 2048, \"disk_gb\": 20},
        \"pgbouncer\": true, \"patroni\": true, \"etcd\": true, \"sync_replicas\": 1
      }},
    \"monitor\": {},
    \"stroppy\": {\"version\": \"3.1.0\", \"workload\": \"tpcc\", \"duration\": \"5m\", \"workers\": 8}
  }" > /dev/null
echo "  B: $RUN_B started (HA)"

# --- Wait for both ---
echo ""
echo "[3/5] Waiting for both runs..."

A_DONE=false
B_DONE=false
A_FAIL=false
B_FAIL=false

while ! ($A_DONE && $B_DONE); do
  if ! $A_DONE; then
    SA=$(curl -s --fail-with-body "$URL/api/v1/run/$RUN_A/status" -H "$AUTH" 2>/dev/null || echo '{}')
    FA=$(echo "$SA" | jq '[.nodes[]? | select(.status=="failed")] | length' 2>/dev/null || echo 0)
    PA=$(echo "$SA" | jq '[.nodes[]? | select(.status=="pending")] | length' 2>/dev/null || echo 0)
    DA=$(echo "$SA" | jq '[.nodes[]? | select(.status=="done")] | length' 2>/dev/null || echo 0)
    TA=$(echo "$SA" | jq '.nodes? | length' 2>/dev/null || echo 0)
    [ "$FA" -gt 0 ] && { A_DONE=true; A_FAIL=true; }
    [ "$PA" -eq 0 ] && [ "$TA" -gt 0 ] && A_DONE=true
  fi

  if ! $B_DONE; then
    SB=$(curl -s --fail-with-body "$URL/api/v1/run/$RUN_B/status" -H "$AUTH" 2>/dev/null || echo '{}')
    FB=$(echo "$SB" | jq '[.nodes[]? | select(.status=="failed")] | length' 2>/dev/null || echo 0)
    PB=$(echo "$SB" | jq '[.nodes[]? | select(.status=="pending")] | length' 2>/dev/null || echo 0)
    DB_=$(echo "$SB" | jq '[.nodes[]? | select(.status=="done")] | length' 2>/dev/null || echo 0)
    TB=$(echo "$SB" | jq '.nodes? | length' 2>/dev/null || echo 0)
    [ "$FB" -gt 0 ] && { B_DONE=true; B_FAIL=true; }
    [ "$PB" -eq 0 ] && [ "$TB" -gt 0 ] && B_DONE=true
  fi

  A_ST="waiting"
  $A_DONE && A_ST="done"
  $A_FAIL && A_ST="FAILED"
  B_ST="waiting"
  $B_DONE && B_ST="done"
  $B_FAIL && B_ST="FAILED"

  printf "  A: %d/%d (%s)  |  B: %d/%d (%s)     \r" \
    "${DA:-0}" "${TA:-0}" "$A_ST" "${DB_:-0}" "${TB:-0}" "$B_ST"

  ($A_DONE && $B_DONE) || sleep 5
done
echo ""
$A_FAIL && echo "  WARNING: Run A had failures"
$B_FAIL && echo "  WARNING: Run B had failures"
echo "  Both runs finished"

# --- Compare ---
echo ""
echo "[4/5] Comparing..."
echo ""

RESULT=$(curl -sf "$URL/api/v1/compare?a=$RUN_A&b=$RUN_B" -H "$AUTH")

printf "%-35s %12s %12s %10s %8s\n" "METRIC" "RUN A" "RUN B" "DIFF %" "VERDICT"
echo "===================================================================================="
echo "$RESULT" | jq -r '.metrics[] | "\(.name)|\(.avg_a)|\(.avg_b)|\(.diff_avg_pct)|\(.verdict)"' | while IFS='|' read -r name avg_a avg_b diff verdict; do
  printf "%-35s %12.2f %12.2f %+9.1f%% %8s\n" "$name" "$avg_a" "$avg_b" "$diff" "$verdict"
done

echo ""
echo "$RESULT" | jq -r '"Summary: \(.summary.better) better, \(.summary.worse) worse, \(.summary.same) same"'

# --- Links ---
echo ""
echo "[5/5] Demo links:"
echo ""
echo "  UI:        $URL"
echo "  Run A:     $URL/runs/$RUN_A"
echo "  Run B:     $URL/runs/$RUN_B"
echo "  Compare:   $URL/compare?a=$RUN_A&b=$RUN_B"
echo "  Grafana:   http://localhost:3001"
echo ""
echo "=== Done ==="
