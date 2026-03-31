#!/bin/bash
# =============================================================================
# compare-branches.sh — Compare performance of two git branches
# =============================================================================
#
# Usage:
#   ./compare-branches.sh <base-branch> <pr-branch>
#
# Example:
#   ./compare-branches.sh main feature/optimize-vacuum
#
# Requirements:
#   - stroppy-cloud binary in PATH (or set STROPPY_BIN)
#   - Stroppy Cloud server running (or set STROPPY_URL)
#   - fpm installed for .deb packaging
#   - Project builds with `make build-deb`
#
# Environment variables:
#   STROPPY_URL    Server URL (default: http://localhost:8080)
#   STROPPY_BIN    Path to stroppy-cloud binary (default: stroppy-cloud)
#   DB_VERSION     Database version (default: 16)
#   WORKLOAD       Benchmark workload (default: tpcb)
#   DURATION       Test duration (default: 5m)
#   WORKERS        Concurrent workers (default: 8)
#   THRESHOLD      Regression threshold % (default: 5)
#   FORMAT         Output format: table, json, junit (default: table)
# =============================================================================
set -euo pipefail

BASE_BRANCH="${1:?Usage: $0 <base-branch> <pr-branch>}"
PR_BRANCH="${2:?Usage: $0 <base-branch> <pr-branch>}"

STROPPY_URL="${STROPPY_URL:-http://localhost:8080}"
STROPPY_BIN="${STROPPY_BIN:-stroppy-cloud}"
DB_VERSION="${DB_VERSION:-16}"
WORKLOAD="${WORKLOAD:-tpcb}"
DURATION="${DURATION:-5m}"
WORKERS="${WORKERS:-8}"
THRESHOLD="${THRESHOLD:-5}"
FORMAT="${FORMAT:-table}"

ORIG_BRANCH=$(git rev-parse --abbrev-ref HEAD)
TIMESTAMP=$(date +%s)

cleanup() {
  echo "Returning to branch $ORIG_BRANCH"
  git checkout "$ORIG_BRANCH" 2>/dev/null || true
}
trap cleanup EXIT

echo "=== Stroppy Branch Comparison ==="
echo "Base:      $BASE_BRANCH"
echo "PR:        $PR_BRANCH"
echo "Server:    $STROPPY_URL"
echo "Workload:  $WORKLOAD / $DURATION / $WORKERS workers"
echo ""

# ---- Step 1: Build base branch package ----
echo ">>> Building base branch ($BASE_BRANCH)..."
git checkout "$BASE_BRANCH"
make build-deb DB_VERSION=$DB_VERSION
mv *.deb /tmp/base-branch.deb
echo "    Built: /tmp/base-branch.deb"

# ---- Step 2: Build PR branch package ----
echo ">>> Building PR branch ($PR_BRANCH)..."
git checkout "$PR_BRANCH"
make build-deb DB_VERSION=$DB_VERSION
mv *.deb /tmp/pr-branch.deb
echo "    Built: /tmp/pr-branch.deb"

# ---- Step 3: Upload packages ----
echo ">>> Uploading packages..."
BASE_PKG_URL=$($STROPPY_BIN upload --server "$STROPPY_URL" --file /tmp/base-branch.deb 2>&1 | grep 'URL:' | awk '{print $2}')
PR_PKG_URL=$($STROPPY_BIN upload --server "$STROPPY_URL" --file /tmp/pr-branch.deb 2>&1 | grep 'URL:' | awk '{print $2}')
echo "    Base: $BASE_PKG_URL"
echo "    PR:   $PR_PKG_URL"

# ---- Step 4: Run base branch benchmark ----
BASE_RUN_ID="cmp-base-${TIMESTAMP}"
echo ">>> Starting base benchmark: $BASE_RUN_ID"

curl -sf -X POST "$STROPPY_URL/api/v1/run" \
  -H 'Content-Type: application/json' \
  -d "{
    \"id\": \"$BASE_RUN_ID\",
    \"provider\": \"docker\",
    \"network\": {\"cidr\": \"10.10.0.0/24\"},
    \"machines\": [],
    \"database\": {\"kind\": \"postgres\", \"version\": \"$DB_VERSION\",
      \"postgres\": {\"master\": {\"role\": \"database\", \"count\": 1, \"cpus\": 2, \"memory_mb\": 4096, \"disk_gb\": 50}}},
    \"monitor\": {},
    \"stroppy\": {\"version\": \"3.1.0\", \"workload\": \"$WORKLOAD\", \"duration\": \"$DURATION\", \"workers\": $WORKERS},
    \"packages\": {\"deb_files\": [\"$BASE_PKG_URL\"]}
  }" > /dev/null

$STROPPY_BIN wait --server "$STROPPY_URL" --run-id "$BASE_RUN_ID" --timeout 30m
echo ""

# ---- Step 5: Run PR branch benchmark ----
PR_RUN_ID="cmp-pr-${TIMESTAMP}"
echo ">>> Starting PR benchmark: $PR_RUN_ID"

curl -sf -X POST "$STROPPY_URL/api/v1/run" \
  -H 'Content-Type: application/json' \
  -d "{
    \"id\": \"$PR_RUN_ID\",
    \"provider\": \"docker\",
    \"network\": {\"cidr\": \"10.20.0.0/24\"},
    \"machines\": [],
    \"database\": {\"kind\": \"postgres\", \"version\": \"$DB_VERSION\",
      \"postgres\": {\"master\": {\"role\": \"database\", \"count\": 1, \"cpus\": 2, \"memory_mb\": 4096, \"disk_gb\": 50}}},
    \"monitor\": {},
    \"stroppy\": {\"version\": \"3.1.0\", \"workload\": \"$WORKLOAD\", \"duration\": \"$DURATION\", \"workers\": $WORKERS},
    \"packages\": {\"deb_files\": [\"$PR_PKG_URL\"]}
  }" > /dev/null

$STROPPY_BIN wait --server "$STROPPY_URL" --run-id "$PR_RUN_ID" --timeout 30m
echo ""

# ---- Step 6: Compare ----
echo ">>> Comparing: $BASE_RUN_ID vs $PR_RUN_ID"
echo ""

$STROPPY_BIN compare \
  --server "$STROPPY_URL" \
  --run-a "$BASE_RUN_ID" \
  --run-b "$PR_RUN_ID" \
  --threshold "$THRESHOLD" \
  --format "$FORMAT"

echo ""
echo "View in UI: $STROPPY_URL/compare?a=$BASE_RUN_ID&b=$PR_RUN_ID"

# ---- Optional: save as baseline ----
# curl -sf -X PUT "$STROPPY_URL/api/v1/admin/baseline/postgres-$DB_VERSION" \
#   -H 'Content-Type: application/json' \
#   -d "{\"run_id\": \"$BASE_RUN_ID\"}"
