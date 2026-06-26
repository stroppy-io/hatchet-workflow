# OrioleDB scale (replicas + HAProxy) — design

Date: 2026-06-26
Branch: ref
Status: approved (pre-implementation)

## Goal

Add horizontal scale to the OrioleDB engine — a streaming-replicated topology
(1 master + N read replicas) fronted by an optional HAProxy that splits writes
to the primary and reads to the replicas — modeled on the existing PostgreSQL
engine's parameter and topology shape, but adapted to OrioleDB's docker-only
deployment.

This is **scale v1**. Automatic failover / HA (Patroni + etcd + synchronous
replicas) is explicitly a later phase; the HAProxy primary-check contract here
is chosen to be drop-in compatible with a future Patroni REST backend.

## Decisions (locked)

- v1 scope = **replicas + HAProxy read/write split**, NO Patroni / etcd /
  auto-failover / synchronous replicas (phase 2).
- Replicas are plain PostgreSQL streaming standbys, each its own OrioleDB
  container, initialized via `pg_basebackup` from the master.
- HAProxy routing without Patroni uses a **lightweight per-DB-node health
  endpoint** that reports primary vs replica via `pg_is_in_recovery()`; HAProxy
  `option httpchk` splits write→primary, read→replicas.
- Params mirror the PostgreSQL subset; supporting roles (HAProxy) install
  natively via apt like the PostgreSQL engine, only the DB engine is dockerized.

## 1. Proto — extend OrioledbParams

File `protocols/cloud/v1/domain/database.proto`, message `OrioledbParams`
(existing fields image=1, postgres_options=2, initdb_locale=3,
shared_buffers_mb=4) — add:

```proto
    // replicas is the streaming-replica container count (0 = no replicas).
    uint32 replicas = 5;
    // haproxy is the dedicated HAProxy LB node count (0 = none).
    uint32 haproxy = 6;
    // replica_options is postgresql.conf applied to replica containers.
    map<string, string> replica_options = 7;
    // haproxy_options tunes haproxy.cfg.
    map<string, string> haproxy_options = 8;
```

Patroni / etcd / pgbouncer / sync_replicas are intentionally omitted (phase 2).

## 2. Topology builder

File `internal/domain/database/orioledb/orioledb.go` — `BuildTopologySpec` grows
from one node to:

- `orioledb-master-1` — role `master`, one container.
- `orioledb-replica-{1..N}` — role `replica`, one container each (N = replicas).
- `orioledb-haproxy-1` — role `haproxy`, only when `haproxy > 0` (cap at 1 for v1).

Roles + priorities (mirror postgres ordering, no etcd/patroni tiers):
- master: globalPriority 20, nodePriority 20
- replica: globalPriority 30, nodePriority 20
- haproxy: globalPriority 60, nodePriority 50

Connections:
- each replica → master, KIND_COORDINATION / PROTOCOL_TCP / MODE_ASYNC, "rep",
  port 5432 (streaming source).
- haproxy → master and each replica, KIND_FLOW / TCP, port 5432.

New role constants: `orioledbRoleReplica = "replica"`, `orioledbRoleHaproxy =
"haproxy"`; ports `pgPort = 5432` (exists), `haproxyWritePort = 5432`,
`haproxyReadPort = 5433`, `healthPort = 8008`.

`dbspec.Component/Node/Connection/Tags` used exactly as cockroach/postgres do.

## 3. Deployment renderer (per role)

File `internal/domain/database/orioledb/deployment.go`. `RenderComponent`
dispatches on `component.GetRole()`.

### master (extends current single-container unit)
Same docker-run systemd unit as today, plus streaming master config appended as
`-c` flags merged into postgres_options:
`wal_level=replica`, `max_wal_senders=10`, `max_replication_slots=10`,
`hot_standby=on`. Trust auth (`POSTGRES_HOST_AUTH_METHOD=trust`) already permits
replication connections (the postgres image's trust mode writes a
`host replication all all trust` pg_hba line). Container flags unchanged
(`--network host --pid host --ipc host --privileged`, PGDATA bind mount).

### replica
A docker-run systemd unit whose container command bootstraps a standby before
starting postgres. The image's default entrypoint runs initdb on an empty
PGDATA, which we must NOT do for a standby — so the replica overrides the
command:

```
docker run --rm --name stroppy-orioledb --network host --pid host --ipc host --privileged \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -v <dataDir>:/var/lib/postgresql/data \
  --entrypoint /bin/bash <image> -c '
    set -e
    if [ ! -s "$PGDATA/PG_VERSION" ]; then
      pg_basebackup -h <master-ip> -p 5432 -U postgres -D "$PGDATA" -R -X stream -P
    fi
    exec docker-entrypoint.sh postgres <replica -c flags>'
```

Notes:
- `<master-ip>` comes from runtime wiring (resolve the master component's
  private endpoint, like cockroach's join-addrs / OwnPrivateEndpoint pattern).
- `-R` writes `standby.signal` + `primary_conninfo` (host=master), so on start
  the container streams from the master.
- `$PGDATA` defaults to `/var/lib/postgresql/data` in the official image.
- replica_options are appended as `-c` flags (e.g. `hot_standby=on` implied).
- Healthcheck: `for i in $(seq 1 60); do docker exec stroppy-orioledb pg_isready -h 127.0.0.1 -p 5432 && exit 0; sleep 3; done; exit 1` (same retry shape as master).

The renderer needs the master's address at render time — add an
`orioledbResolveWiring(ctx)` returning the master private endpoint (mirrors
`cockroachResolveWiring`). Preview (no infra) renders the replica unit with a
placeholder master host.

### haproxy (native apt, not docker)
Mirror the PostgreSQL HAProxy role: install `haproxy` via apt, write
`haproxy.cfg`, run as a systemd service. Two frontends:
- `:5432` (write) → backend with only the **primary** (httpchk `/primary`).
- `:5433` (read) → backend with the **replicas** (httpchk `/replica`).
Backends list every DB node (master + replicas) by private IP; the health check
decides membership. `haproxy_options` tune timeouts/maxconn.

## 4. HAProxy primary-check without Patroni

Each DB node (master + replica) runs a tiny HTTP health endpoint on `:8008`
that answers the two paths HAProxy probes:
- `GET /primary` → 200 if `pg_is_in_recovery()` is false (this node is primary),
  else 503.
- `GET /replica` → 200 if `pg_is_in_recovery()` is true (standby), else 503.

Implementation: a small shell responder behind `socat`/`xinetd` installed as a
deploy step on every DB node (master + replica), querying the local container
via `docker exec stroppy-orioledb psql -tAc "SELECT pg_is_in_recovery()"`. The
endpoint runs on the host (the DB roles are docker, but the health responder is
a host-side step in the component's deploy, ordered after the DB healthcheck).

This contract (`/primary`, `/replica` returning 200/503) is exactly what
Patroni's REST API serves, so phase-2 HA swaps the responder for Patroni with no
HAProxy.cfg change.

## 5. Monitoring

`internal/domain/deployment/monitor.go` `isDatabaseRole`: the `orioledb` case
must also return true for role `"replica"` (currently only `"master"`). dbKind
still normalizes to `"postgres"`, so postgres_exporter scrapes every DB node and
the **Replication Lag** metric becomes meaningful (was null on a single node).
HAProxy nodes are not DB nodes (node_exporter only). No other monitor change.

## 6. Frontend

- `web/src/components/database/DatabaseParamsForm.tsx`: extend the orioledb
  `EngineParamsForm` branch with `replicas` (number) and `haproxy` (number)
  inputs plus `replica_options` / `haproxy_options` map editors — mirror the
  PostgreSQL branch's controls. `engineNodes` for orioledb returns master +
  N replicas (+ haproxy) groups for the topology preview.
- `web/src/services/domainMappers.ts`: orioledb VM↔proto converters carry the
  new fields (replicas, haproxy, replicaOptions, haproxyOptions).
- `web/src/services/wizard.ts`: `blankEngineParams` / default for orioledb gains
  `replicas:0, haproxy:0, replicaOptions:{}, haproxyOptions:{}`.
- No new DbKind/label/icon work (orioledb already wired).

## 7. Builtin preset (optional)

Add a builtin test preset "Self-check / OrioleDB HA" (master + 1 replica +
haproxy) alongside the existing single, for stage smoke of the scaled topology.
Reuses the same workload.

## Exhaustive change sites

Backend:
- `protocols/cloud/v1/domain/database.proto` (params).
- `internal/domain/database/orioledb/orioledb.go` (multi-node topology + roles).
- `internal/domain/database/orioledb/deployment.go` (master streaming flags,
  replica standby unit + wiring, haproxy role + health endpoint).
- `internal/domain/deployment/monitor.go` (`isDatabaseRole` orioledb replica).
- `internal/app/presets_seed.go` (optional HA preset).

Frontend:
- `web/src/components/database/DatabaseParamsForm.tsx`,
  `web/src/services/domainMappers.ts`, `web/src/services/wizard.ts`.

Regenerated: `internal/proto/**`, `web/src/lib/proto/**`, openapi/docs.

## Build order

proto + codegen → topology builder (roles) → master streaming flags → replica
standby renderer + wiring → haproxy role + health endpoint → monitor replica →
frontend → optional HA preset → stage e2e (master+1replica+haproxy, verify
streaming + replication-lag metric + haproxy write/read split).

## Out of scope (phase 2 — HA)

- Patroni-managed automatic failover, etcd DCS, synchronous replicas
  (`sync_replicas`), PgBouncer. The HAProxy `/primary` `/replica` httpchk
  contract is forward-compatible with Patroni's REST so phase 2 is additive.
- Replica auto-reseed on divergence, cascading replication, multi-AZ placement.
