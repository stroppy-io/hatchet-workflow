#!/usr/bin/env bash
#
# Bench OrioleDB (.deb) vs vanilla PostgreSQL 17
#
# Prerequisites:
#   - stroppy-cloud server running locally (docker compose up -d)
#   - stroppy-cloud CLI built (make build)
#   - logged in: ./bin/stroppy-cloud cloud login --server http://localhost:8080
#
# Usage:
#   ./examples/bench-orioledb.sh /path/to/orioledb.deb
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CLI="${SCRIPT_DIR}/../bin/stroppy-cloud"

DEB="${1:?Usage: $0 <path-to-orioledb.deb>}"

if [[ ! -f "$DEB" ]]; then
  echo "Error: deb file not found: $DEB"
  exit 1
fi

echo "=== OrioleDB vs PostgreSQL 17 benchmark ==="
echo "  OrioleDB deb: $DEB"
echo ""

# Run bench: baseline = vanilla PG17, candidate = OrioleDB with custom .deb
# Both use the same hardware config, same workload, same duration.
exec "$CLI" cloud bench \
  --baseline  "${SCRIPT_DIR}/bench-pg17-vanilla.json" \
  --candidate "${SCRIPT_DIR}/bench-pg17-vanilla.json" \
  --candidate-deb "$DEB" \
  --format table \
  --timeout 30m
