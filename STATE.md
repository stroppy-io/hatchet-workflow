# Stroppy Cloud — Docker Self-Check Test Progress

## Goal
Run all Docker-compatible database+topology presets through test runs via API, fix failures iteratively, and confirm each with stroppy metrics.

## Constraints
- Docker provider only (PROVIDER_DOCKER), no Yandex Cloud
- Each preset tested one at a time: start run -> terminal status -> fix if needed -> re-run -> verify stroppy metrics
- Task done per-preset only when run is STATUS_COMPLETED and stroppy metrics confirmed
- Server API uses Connect protocol (JSON-over-HTTP): endpoints are `/cloud.v1.api.<Service>/<Method>` on port 8080
- Auth: `POST /cloud.v1.api.IamService/Login {"login":"admin@stroppy.local","password":"admin"}` -> JWT Bearer token

## Test Results

### Completed (8/18)

| Preset | Run ID | Stroppy series | Notes |
|---|---|---|---|
| PostgreSQL single | 3aa4e3bd-9ca8-4216-8e60-a504c8474b9a | 1568 | |
| PostgreSQL HA | ba45218a-30eb-4e43-ba09-0e73fa1ccfe4 | 6 | |
| MySQL single | bc3cbf7f-1f47-4a3d-adbc-107f964b7dee | 5 | |
| MySQL replica | 184da06b-f2f3-4bd6-a123-00681e6921a8 | 6 | |
| MySQL group | 1f8f02db-2b60-417a-9130-5dc207e50c4a | 6 | |
| MariaDB single | f4210d48-c93a-4db1-a50c-ca22955c60d6 | 5 | |
| MariaDB replica | 2b076f83-e753-429b-90c4-8c88ff647378 | 5 | |
| MariaDB group | 82491b6f-56a3-4b36-8e2f-2d406979e120 | 5 | |

### Blocked — External Dependencies (7/18)

| Preset | Blocker |
|---|---|
| Picodata single | No public picodata deb package, repo.picodata.io redirect loop |
| Picodata cluster | Same as above |
| YDB single | YDB is a Yandex Cloud managed service, not installable in Docker |
| YDB mirror3dc-3x32 | Same as above |
| YDB mirror3dc-3x64 | Same as above |
| YDB mirror3dc-9x32 | Same as above |
| CockroachDB single | binaries.cockroachdb.com returns 403 for all versions |
| CockroachDB cluster-3 | Same as above |
| CockroachDB cluster-6 | Same as above |

### Blocked — Technical (2/18)

| Preset | Blocker |
|---|---|
| PostgreSQL scale | Temporal workflow payload size exceeds 512KB limit (1.2MB). Needs architectural fix (state externalization or payload compression) |
| Picodata scale | Temporal payload limit + no public deb |

Note: YDB presets also include "Managed dedicated" and "Managed serverless" variants which are cloud-only and not Docker-compatible.

## Code Changes Made

### `internal/app/presets_seed.go`
- `innodb_buffer_pool_size` changed from `25%` to `256M` for MySQL replica/group and MariaDB presets (MySQL 8.0.46/MariaDB 10.6 don't support percent syntax)

### `internal/domain/database/mysql/mysql.go`
- `mysqlConfigContent` now takes `isMariaDB bool` parameter
- MariaDB uses `gtid_domain_id` + `log_bin_trust_function_creators` instead of MySQL's `gtid_mode` + `enforce_gtid_consistency`
- Semi-sync replication: added `plugin_load_add = "semisync_master.so;semisync_slave.so"` (MySQL 8.0.46 requires explicit plugin load)

### `internal/domain/database/mysql/deployment.go`
- MariaDB: `mysql_install_db` instead of `--initialize-insecure` (not supported by MariaDB)
- MariaDB: creates `/run/mysqld` socket directory via ExecStartPre
- MariaDB: uses `mariadb` client instead of `mysql` for ExecStartPost commands
- ProxySQL install: downloads deb from GitHub releases (repo.proxysql.com is unreachable/slow from containers)
- ProxySQL config: added `mysql_users` section with `root` user (workload needs passwordless access)
- ProxySQL: `command -v proxysql` guard to skip install if pre-installed in agent image

### `internal/domain/database/postgres/deployment.go`
- Added retry loop (30x3s) for `pg_basebackup` in replica ExecStartPre
- Added `chmod 0700` on data dir after `pg_basebackup`
- Patroni config: added `name`, `restapi.listen`, `postgresql.listen`, `postgresql.bin_dir`, `postgresql.data_dir`, `postgresql.config_dir`, `postgresql.pgpass`, `postgresql.use_pg_rewind: false`
- PgBouncer: runs as `postgres` user via `runuser`
- PgBouncer auth: `auth_type = trust` with `users.txt`
- PgBouncer: added `users.txt` file write step
- `pg_hba.conf`: `trust` for all hosts (was `scram-sha-256`)
- Healthcheck retry: 60x3s=180s for all postgres components (was 30x2s)

### `internal/domain/workload/stroppy_config.go`
- MySQL workload `urlTail` changed from `/` to `/stroppy`
- `postgresPgbouncerConfig` now takes `componentID` parameter

### `internal/infrastructure/quotas/docker.go`
- Docker quota minimums bumped: disk 500GiB, CPU 64 cores, memory 256GiB

### `deployments/docker/agent.Dockerfile`
- Pre-installs ProxySQL from GitHub releases in agent image

### `docker-compose.yaml`
- Local server image build configuration

## Key Context

- **Tenant ID**: `798b2c01-6142-44e6-b553-068f09717dae`
- **Admin account ID**: `33bf3422-d1ae-4630-91f9-8d812f28741d`
- **Docker self-check suite ID**: `c48b12c4-1ad3-4126-9585-8e787343a7c2`
- **Quota**: After server restart, run `UPDATE quota_snapshots SET stale_after = now() - interval '1 hour'` to force refresh
- **VictoriaLogs**: `GET http://localhost:8427/select/logsql/query` with `Authorization: Bearer stroppy-monitoring-secret`
- **VictoriaMetrics**: `GET http://localhost:8427/select/0/prometheus/api/v1/query` with same auth
- **Stroppy metrics**: `stroppy_<run_id_underscored>_<metric_name>`
- **Preset IDs**:
  - PG-single=`fbc923d9`, PG-ha=`a9c116ac`, PG-scale=`868491e1`
  - MySQL-single=`321f61cf`, MySQL-replica=`ba6e2a0e`, MySQL-group=`fc6218d2`
  - MariaDB-single=`aefea115`, MariaDB-replica=`9b02dfca`, MariaDB-group=`cf8076e3`
  - Picodata-single=`d86f28e1`, Picodata-cluster=`2522b352`, Picodata-scale=`6acd37b6`
  - YDB-single=`848bc31f`, YDB-m3dc-no-arb=`7e1eda8b`, YDB-m3dc-arb=`66937fb0`
  - CRDB-single=`61e21fad`, CRDB-cluster-3=`e1b2ea22`, CRDB-cluster-6=`9e6c5347`
- **Test wizard flow**: 3 API calls: StartTestWizard -> PatchTestWizard -> FinishTestWizard
- **Automation script**: `/tmp/opencode/run_test.sh`
- **MariaDB uses same renderer as MySQL**: `(&mysqldb.Database{}).BuildTopologySpec(params.GetMariadb())` in `builder.go`
- **Seed reconciliation**: Server re-seeds presets on every boot from `presets_seed.go` — DB edits to preset data are overwritten on restart
- **Do NOT restart server mid-run** — kills Temporal workflows, leaves orphaned containers

## Debugging Patterns

- **Error diagnosis**: Check VictoriaLogs for `run_id:"<id>" component_id:"<comp>" (_msg:"*error*" OR _msg:"*failed*")`
- **Service journal**: VictoriaLogs captures systemd journal output tagged `stream:"stdout"`
- **Container inspection**: Containers are removed after terminal status — use logs, not docker exec
- **Key pattern**: Failed stage names from `SELECT elem FROM test_run_records, jsonb_array_elements(data->'runtimeState'->'stages') elem WHERE elem->>'status' = 'STATUS_FAILED'`
