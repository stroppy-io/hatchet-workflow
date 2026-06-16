# Stage preset smoke handoff

Date: 2026-06-08
Branch: `ref`
Stage host: `st-postgres@158.160.244.172`
Stage repo: `/home/st-postgres/stroppy-cloud`
Stage URL: `https://stage.cloud.stroppy.io`
Tenant: `abbfbf2d-5bf9-4371-8ce2-2a0957f8e376`

## Current code status

Committed and pushed:

- `0a7b79c fix: relay apt proxy through caddy https`
- `a9cbebc fix: catch https proxy requests in caddy`
- `2f47d0d fix: fetch database binaries through gateway cache`
- `462d96b fix: expand gateway binary urls in install commands`
- `6548aa0 fix: bootstrap self-hosted ydb presets`
- `f3e168d fix: compact workflow run state queries`
- `c188bc9 fix: cap workflow runtime error previews`
- `563769d fix: finalize cancelled runs after workflow closes`

Stage is fast-forwarded to `563769d`, server was rebuilt and restarted. Health check passed:

```bash
curl -fsS https://stage.cloud.stroppy.io/healthz
```

Local worktree still has an unrelated dirty file that was not committed:

- `internal/domain/workload/deployment.go`

## What was fixed today

### Caddy / gateway path

The agent now goes through the public HTTPS Caddy endpoint for control plane traffic and apt/cache traffic. Caddy no longer tries to issue local HTTPS certs by default in local/docker mode, and stage works through `https://stage.cloud.stroppy.io`.

Validated earlier with Postgres single run:

- run id: `7c0cb9d9-4809-4fd6-aff2-127caf599a80`
- status: `STATUS_COMPLETED`

### Binary cache

Gateway binary cache now supports:

- YDB server binaries via `/api/binaries/ydbd/{version}/{file}`
- Cockroach binaries via `/api/binaries/cockroach/{version}/{file}`

YDB/Cockroach package URLs are rendered through `${STROPPY_SERVER_ADDR%/}/api/binaries/...`, and install command quoting now allows that env expression to expand on the VM.

### Self-hosted YDB bootstrap

The YDB default flow previously started only a static storage node and then ran workload against `/Root/testdb`, but no tenant database was created.

The latest commit adds:

- generic renderer support for post-start commands;
- YDB `240_init_database` step on `ydb-storage-1`;
- `ydbd admin blobstorage config init --yaml-file ...`;
- idempotent `ydbd admin database /Root/testdb create ssd:1`;
- `YDB single` preset changed to `1 storage node + 1 database node`.

### Runtime overview / cancelling cleanup

The live Temporal `GetRunState` query exceeded Temporal's 2 MB query result limit after a stage kept a 1.4 MB error message in workflow state:

```text
workflow_state_unavailable: query result size (3192705) exceeds limit (2000000)
```

Fixes now deployed:

- `GetRunState` returns compact runtime projection instead of full workflow state.
- Runtime projection truncates `Stage.ErrorMessage`, operation summaries, and output summaries/targets.
- Repeated `CancelTestRun` finalizes records stuck in `STATUS_CANCELLING` when the Temporal workflow is already closed/terminated.

Tests passed locally:

```bash
rtk go test ./internal/domain/deployment ./internal/domain/database/ydb ./internal/app
rtk go test ./...
```

## Current stage run

Latest YDB single smoke was launched after deploying `6548aa0`.

- run id: `677b3b44-2e87-4136-bf2a-9a7f4960f571`
- report dir on stage: `/tmp/stroppy-preset-smoke-ydb-bootstrap-20260608T171922Z`
- final status after cleanup: `STATUS_CANCELLED`
- final overview degraded reasons: none
- harness process was stopped manually so no SSH command is left running.

Check status:

```bash
ssh -l st-postgres 158.160.244.172
cd /home/st-postgres/stroppy-cloud
set -a
. ./.env
set +a

base=${PUBLIC_SERVER_ADDR:-https://stage.cloud.stroppy.io}
password=${STROPPY_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}
token=$(curl -fsS -H "Content-Type: application/json" \
  --data-binary "$(jq -cn --arg login "${STROPPY_ADMIN_LOGIN:-admin}" --arg password "$password" '{login:$login,password:$password}')" \
  "$base/cloud.v1.api.IamService/Login" | jq -r ".tokens.accessToken")

curl -fsS -H "Content-Type: application/json" -H "Authorization: Bearer $token" \
  --data-binary "$(jq -cn --arg tenant "${STROPPY_TENANT_ID:-abbfbf2d-5bf9-4371-8ce2-2a0957f8e376}" --arg id "677b3b44-2e87-4136-bf2a-9a7f4960f571" '{tenantId:$tenant,id:$id}')" \
  "$base/cloud.v1.api.TestRunService/GetTestRun" \
  | jq -c '{id:(.run.entity.id // .run.id), name:(.run.entity.name // .run.name), status:.run.status, summary:.run.summary, error:.run.error}'
```

## Known open issues

### YDB

Before `6548aa0`, YDB install/start got past binary download but workload failed with:

- `can't initialize shared driver`
- `failed to dial "10.61.0.24:2136": context deadline exceeded`

That was likely caused by missing tenant DB and missing dynamic node. The current run should confirm whether the new bootstrap is enough or whether the generated `config.yaml` is still incomplete for real self-hosted YDB.

After `6548aa0`, the focused run reached the new `240_init_database` step and failed there. Raw VictoriaLogs showed:

```text
component/ydb-storage-1/step/240_init_database
failed to parse config from file: ...
YDB database /Root/testdb was not created after retries
```

So the next YDB task is not Caddy/binary download anymore; it is the generated self-hosted YDB `config.yaml`. It is too incomplete or invalid for `ydbd admin blobstorage config init --yaml-file`.

If the current run fails, inspect these first:

- step `component/ydb-storage-1/step/240_init_database`;
- step `component/ydb-database-1/step/220_enable_start`;
- step `component/workload-runner-1/step/900_run_stroppy`;
- raw VictoriaLogs by `run_id`.

### CockroachDB

Cockroach is still blocked separately:

- stage gateway URL returned upstream `403 Forbidden`;
- direct `https://binaries.cockroachdb.com/cockroach-v23.2.5.linux-amd64.tgz` also returned 403 from this environment.

This needs either a different pinned upstream/mirror or a staged artifact source. It is not fixed by the YDB work.

### Logs

VictoriaLogs raw lines contain useful data, but UI/query rendering often truncates long `_msg` values. Also YDB node logs are polluted by:

```text
missing _msg field; see https://docs.victoriametrics...
```

The xlog/vector sync still needs a separate fix so server/agent/systemd logs are reliably queryable by `run_id`, `component_id`, and `step_id`.

## API-loop methodology

Use the harness instead of manual UI clicks. It keeps token usage low because it emits compact JSONL and only pulls logs for failed runs.

Audit all system presets:

```bash
ssh -l st-postgres 158.160.244.172
cd /home/st-postgres/stroppy-cloud
set -a
. ./.env
set +a
./scripts/stage-preset-harness.sh audit
```

Run one focused smoke:

```bash
PRESET_NAME_REGEX="YDB single$" \
RUN_TIMEOUT_SECONDS=3600 \
RUN_POLL_SECONDS=20 \
REPORT_DIR=/tmp/stroppy-preset-smoke-ydb-$(date -u +%Y%m%dT%H%M%SZ) \
./scripts/stage-preset-harness.sh smoke
```

Run a small matrix:

```bash
PRESET_NAME_REGEX="single$" \
PRESET_LIMIT=6 \
RUN_TIMEOUT_SECONDS=3600 \
RUN_POLL_SECONDS=20 \
REPORT_DIR=/tmp/stroppy-preset-smoke-single-$(date -u +%Y%m%dT%H%M%SZ) \
./scripts/stage-preset-harness.sh smoke
```

Read compact results:

```bash
dir=$(ls -td /tmp/stroppy-preset-smoke-* | head -n1)
jq -r '[.phase,.name,(.runId // ""),(.status // ""),(.class // ""),(.error // "")] | @tsv' "$dir/report.jsonl"
```

Query raw VictoriaLogs for one run and step:

```bash
run_id=677b3b44-2e87-4136-bf2a-9a7f4960f571
docker compose --profile prod exec -T victorialogs sh -c \
  "wget -qO- --post-data=\"query=run_id:\\\"$run_id\\\" | sort by (_time) desc\" http://127.0.0.1:9428/select/logsql/query"
```

Recommended loop:

1. Pick one failing default preset, not the whole catalog.
2. Run harness smoke with a dedicated `REPORT_DIR`.
3. If it fails, classify by stage: infra, deployment step, workload, teardown.
4. Pull only the failed step logs and Temporal describe.
5. Fix the smallest renderer/preset/provider issue.
6. Run local tests.
7. Commit without co-author, push, deploy stage.
8. Re-run the same focused smoke.
9. Move to the next default preset only after the current failure class is understood.
