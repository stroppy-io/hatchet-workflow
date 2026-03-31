---
sidebar_position: 10
---

# CI Integration

This guide shows how to use Stroppy Cloud in CI pipelines for automated database performance testing.

## Overview

A typical CI flow:

1. **Build** your custom database package (.deb/.rpm)
2. **Upload** the package to the Stroppy Cloud server
3. **Start** a test run referencing the uploaded package
4. **Wait** for the run to complete
5. **Compare** results against a baseline run
6. **Report** pass/fail based on performance regression thresholds

## Prerequisites

- Stroppy Cloud server running and accessible from CI (e.g., `https://stroppy.internal:8080`)
- Authentication token or credentials
- `curl` and `jq` available in CI environment

## Step 1: Authenticate

```bash
export STROPPY_URL="http://stroppy.internal:8080"

TOKEN=$(curl -sf -X POST "$STROPPY_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ci-user","password":"$CI_PASSWORD"}' | jq -r .token)

export AUTH="Authorization: Bearer $TOKEN"
```

## Step 2: Build a Custom Package

Build your database package so it conforms to the standard layout. Stroppy Cloud agents install packages via `dpkg -i` (for .deb) or `rpm -i` (for .rpm).

### .deb package requirements

The package should:
- Install the database binary to a standard path (`/usr/bin/`, `/usr/lib/postgresql/`)
- Include systemd service files if needed
- Not start the service on install (agents manage startup)

Example build (for a patched PostgreSQL):

```bash
# Build from source
./configure --prefix=/usr/lib/postgresql/16
make -j$(nproc)
make DESTDIR=./pkg install

# Package
fpm -s dir -t deb \
  -n postgresql-custom-16 \
  -v "16.0-$(git rev-parse --short HEAD)" \
  -C ./pkg \
  --deb-no-default-config-files \
  usr/
```

### .rpm package requirements

Same as .deb but in RPM format:

```bash
fpm -s dir -t rpm \
  -n postgresql-custom-16 \
  -v "16.0.$(git rev-parse --short HEAD)" \
  -C ./pkg \
  usr/
```

## Step 3: Upload the Package

```bash
UPLOAD_RESULT=$(curl -sf -X POST "$STROPPY_URL/api/v1/upload/deb" \
  -H "$AUTH" \
  -F file=@postgresql-custom-16_16.0-abc1234_amd64.deb)

PACKAGE_URL=$(echo "$UPLOAD_RESULT" | jq -r .url)
echo "Package uploaded: $PACKAGE_URL"
```

## Step 4: Start a Test Run

Create a RunConfig JSON that references your uploaded package:

```bash
RUN_ID="ci-$(date +%s)-${CI_COMMIT_SHORT_SHA:-manual}"

cat > /tmp/run-config.json << EOF
{
  "id": "$RUN_ID",
  "provider": "docker",
  "network": {"cidr": "10.10.0.0/24"},
  "machines": [],
  "database": {
    "kind": "postgres",
    "version": "16",
    "postgres": {
      "master": {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}
    }
  },
  "monitor": {},
  "stroppy": {
    "version": "3.1.0",
    "workload": "tpcb",
    "duration": "5m",
    "workers": 8
  },
  "packages": {
    "deb_files": ["$PACKAGE_URL"]
  }
}
EOF

curl -sf -X POST "$STROPPY_URL/api/v1/run" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d @/tmp/run-config.json
```

## Step 5: Wait for Completion

Poll the status endpoint until the run finishes:

```bash
while true; do
  STATUS=$(curl -sf "$STROPPY_URL/api/v1/run/$RUN_ID/status" -H "$AUTH")
  PENDING=$(echo "$STATUS" | jq '[.nodes[] | select(.status == "pending")] | length')
  FAILED=$(echo "$STATUS" | jq '[.nodes[] | select(.status == "failed")] | length')

  if [ "$FAILED" -gt 0 ]; then
    echo "FAIL: Run has $FAILED failed nodes"
    echo "$STATUS" | jq '.nodes[] | select(.status == "failed")'
    exit 1
  fi

  if [ "$PENDING" -eq 0 ]; then
    echo "Run completed successfully"
    break
  fi

  echo "Waiting... ($PENDING nodes pending)"
  sleep 10
done
```

## Step 6: Compare Against Baseline

Compare the current run against a known-good baseline run:

```bash
BASELINE_RUN="baseline-postgres-16"  # a previously saved successful run ID

COMPARE=$(curl -sf "$STROPPY_URL/api/v1/compare?a=$BASELINE_RUN&b=$RUN_ID" \
  -H "$AUTH")

echo "Comparison results:"
echo "$COMPARE" | jq '.metrics[] | {name, avg_a, avg_b, diff_avg_pct, verdict}'

# Check for regressions
WORSE=$(echo "$COMPARE" | jq '.summary.worse')
BETTER=$(echo "$COMPARE" | jq '.summary.better')

echo "Better: $BETTER, Worse: $WORSE"

if [ "$WORSE" -gt 2 ]; then
  echo "FAIL: Performance regression detected ($WORSE metrics worse)"
  exit 1
fi

echo "PASS: No significant regressions"
```

### Comparison verdict logic

The compare endpoint uses a 5% threshold by default:
- **better**: metric improved by >5%
- **worse**: metric degraded by >5%
- **same**: within 5% of baseline

Metrics where "higher is better": `db_qps`, `stroppy_ops`
Metrics where "lower is better": `db_latency_p99`, `cpu_usage`, `memory_usage`, `stroppy_latency_p99`, `stroppy_errors`

## Step 7: Cleanup

Delete the run after extracting results:

```bash
curl -sf -X DELETE "$STROPPY_URL/api/v1/run/$RUN_ID" -H "$AUTH"
```

## Full CI Script Example

```bash
#!/bin/bash
set -euo pipefail

STROPPY_URL="${STROPPY_URL:-http://stroppy.internal:8080}"
BASELINE_RUN="${BASELINE_RUN:-baseline-postgres-16}"
WORKLOAD="${WORKLOAD:-tpcb}"
DURATION="${DURATION:-5m}"
WORKERS="${WORKERS:-8}"

# 1. Auth
TOKEN=$(curl -sf -X POST "$STROPPY_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$CI_USER\",\"password\":\"$CI_PASSWORD\"}" | jq -r .token)
AUTH="Authorization: Bearer $TOKEN"

# 2. Upload package
PKG_URL=$(curl -sf -X POST "$STROPPY_URL/api/v1/upload/deb" \
  -H "$AUTH" -F file=@"$DEB_PATH" | jq -r .url)

# 3. Start run
RUN_ID="ci-$(date +%s)"
curl -sf -X POST "$STROPPY_URL/api/v1/run" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{
    \"id\": \"$RUN_ID\",
    \"provider\": \"docker\",
    \"network\": {\"cidr\": \"10.10.0.0/24\"},
    \"machines\": [],
    \"database\": {\"kind\": \"postgres\", \"version\": \"16\",
      \"postgres\": {\"master\": {\"role\": \"database\", \"count\": 1, \"cpus\": 2, \"memory_mb\": 4096, \"disk_gb\": 50}}},
    \"monitor\": {},
    \"stroppy\": {\"version\": \"3.1.0\", \"workload\": \"$WORKLOAD\", \"duration\": \"$DURATION\", \"workers\": $WORKERS},
    \"packages\": {\"deb_files\": [\"$PKG_URL\"]}
  }"

# 4. Wait
while true; do
  S=$(curl -sf "$STROPPY_URL/api/v1/run/$RUN_ID/status" -H "$AUTH")
  F=$(echo "$S" | jq '[.nodes[] | select(.status=="failed")] | length')
  P=$(echo "$S" | jq '[.nodes[] | select(.status=="pending")] | length')
  [ "$F" -gt 0 ] && { echo "FAIL"; echo "$S" | jq '.nodes[] | select(.status=="failed")'; exit 1; }
  [ "$P" -eq 0 ] && break
  sleep 10
done

# 5. Compare
C=$(curl -sf "$STROPPY_URL/api/v1/compare?a=$BASELINE_RUN&b=$RUN_ID" -H "$AUTH")
echo "$C" | jq '.metrics[] | {name, diff_avg_pct, verdict}'
WORSE=$(echo "$C" | jq '.summary.worse')
[ "$WORSE" -gt 2 ] && { echo "Regression: $WORSE metrics worse"; exit 1; }

echo "OK: no regressions"
```

## PR Branch Comparison

The most powerful CI use case: compare the performance of a PR branch against the base branch. Each branch builds its own .deb package, and stroppy runs identical benchmarks on both.

### How it works

```
┌─────────────┐    ┌─────────────┐
│ Base branch  │    │ PR branch   │
│ (main)       │    │ (feature/X) │
└──────┬───────┘    └──────┬──────┘
       │                   │
   make build-deb      make build-deb
       │                   │
   base.deb            pr.deb
       │                   │
   upload to server    upload to server
       │                   │
   stroppy run         stroppy run
   (run-base-123)      (run-pr-123)
       │                   │
       └─────────┬─────────┘
                 │
          compare API
                 │
          ┌──────┴──────┐
          │ 3 better    │
          │ 1 worse     │
          │ 12 same     │
          └─────────────┘
```

### Shell script (manual)

```bash
./examples/ci/compare-branches.sh main feature/optimize-vacuum
```

This script:
1. Checks out each branch and runs `make build-deb`
2. Uploads both .deb files to the server
3. Runs identical stroppy benchmarks on each
4. Uses `stroppy-cloud wait` to poll until completion
5. Uses `stroppy-cloud compare` to diff the results
6. Prints a table and exits non-zero on regression

### Using stroppy-cloud CLI

```bash
# Upload packages
stroppy-cloud upload --file base.deb --server http://localhost:8080
stroppy-cloud upload --file pr.deb --server http://localhost:8080

# Start runs (via curl or future `stroppy-cloud run` with --server flag)
# ...

# Wait for both
stroppy-cloud wait --run-id ci-base-123 --server http://localhost:8080 --timeout 30m
stroppy-cloud wait --run-id ci-pr-123 --server http://localhost:8080 --timeout 30m

# Compare
stroppy-cloud compare --run-a ci-base-123 --run-b ci-pr-123 --threshold 5 --format table
stroppy-cloud compare --run-a ci-base-123 --run-b ci-pr-123 --format junit > report.xml
```

### GitHub Actions

A full workflow is provided in [`examples/ci/github-actions.yml`](https://github.com/stroppy-io/stroppy-cloud/tree/main/examples/ci/github-actions.yml). It:

1. **build-base** / **build-pr** — parallel jobs building .deb artifacts
2. **benchmark** — sequential runs (to avoid resource contention), then compare
3. **comment** — posts a PR comment with comparison link using `actions/github-script`

Key features:
- `mikepenz/action-junit-report` for test summary in PR checks
- Outputs `compare_url` for shareable link
- Uses GitHub Actions secrets for credentials

### .deb package requirements

Your project needs a `make build-deb` target (or equivalent) that produces a .deb file. Example using [fpm](https://fpm.readthedocs.io/):

```makefile
build-deb:
	./configure --prefix=/usr/lib/postgresql/$(DB_VERSION)
	make -j$$(nproc)
	make DESTDIR=./pkg install
	fpm -s dir -t deb \
		-n postgresql-custom-$(DB_VERSION) \
		-v "$(DB_VERSION).0-$$(git rev-parse --short HEAD)" \
		-C ./pkg \
		--deb-no-default-config-files \
		usr/
```

Requirements:
- Installs to standard paths (`/usr/lib/postgresql/`, `/usr/bin/`)
- Does NOT start services on install (agents manage lifecycle)
- Naming convention: `<name>_<version>_<arch>.deb`

For RPM: same but `fpm -t rpm` and `.rpm` extension. Upload via `POST /api/v1/upload/rpm`.

### Baseline management

Instead of comparing two branches every time, you can save a "golden" baseline and compare PR runs against it:

```bash
# Save a run as the baseline
curl -X PUT "$STROPPY_URL/api/v1/admin/baseline/postgres-16" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"run_id": "ci-base-12345"}'

# Later, compare a new PR run against the saved baseline
BASELINE_ID=$(curl -s "$STROPPY_URL/api/v1/admin/baseline/postgres-16" -H "$AUTH" | jq -r .run_id)
stroppy-cloud compare --run-a "$BASELINE_ID" --run-b ci-pr-67890 --format table

# List all baselines
curl "$STROPPY_URL/api/v1/admin/baselines" -H "$AUTH" | jq
```

## ROADMAP

### Implemented

| Feature | CLI | API | Notes |
|---|---|---|---|
| `stroppy-cloud compare` | `--run-a`, `--run-b`, `--format table/json/junit`, `--threshold` | `GET /compare` | Exits non-zero if >2 metrics regress |
| `stroppy-cloud upload` | `--file`, `--server` | `POST /upload/deb`, `POST /upload/rpm` | Supports .deb and .rpm |
| `stroppy-cloud wait` | `--run-id`, `--timeout`, `--interval` | Polls `/run/{id}/status` | Live progress counter |
| Configurable threshold | `--threshold 10` | `?threshold=10` query param | Default 5% |
| JUnit XML output | `--format junit` | N/A | For GitLab/Jenkins/GitHub Actions |
| Baseline management | N/A | `PUT/GET /admin/baseline/{name}` | Named baselines in BadgerDB |
| RPM upload | `--file my.rpm` | `POST /upload/rpm` | Same handler as .deb |
| Webhook notifications | N/A | `webhooks` in ServerSettings | Events: run.started/completed/failed, node.done/failed |
| Per-run stroppy_run_id | N/A | Auto via OTEL_RESOURCE_ATTRIBUTES | `stroppy.run.id` label in K6 metrics |

### Remaining

#### `stroppy-cloud run --server` CLI command
**Status**: Not implemented.
Start a run on a remote server from CLI (currently requires `curl` or the SPA).
Would allow: `stroppy-cloud run -c config.json --server http://stroppy:8080`.

#### `stroppy-cloud baseline` CLI subcommand
**Status**: Not implemented.
Manage baselines from CLI. Would allow: `stroppy-cloud baseline set --name postgres-16 --run-id X --server http://...`.
**Workaround**: Use `curl -X PUT /api/v1/admin/baseline/{name}`.

#### Webhook retry with exponential backoff
**Status**: Not implemented.
Currently webhooks are fire-and-forget (one attempt per URL). Failed deliveries are logged but not retried.

#### TAP output format
**Status**: Not implemented.
JUnit XML is supported. TAP (Test Anything Protocol) could be added as `--format tap` for Unix-native CI tools.
