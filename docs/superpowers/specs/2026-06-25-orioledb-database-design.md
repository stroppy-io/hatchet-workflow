# OrioleDB as a new database kind — design

Date: 2026-06-25
Branch: ref
Status: approved (pre-implementation)

## Goal

Add OrioleDB as a new selectable database engine, end to end: proto/backend,
deploy/recipe, parameters, frontend, and staging rollout.

OrioleDB is a storage-engine extension that requires a **patched** PostgreSQL
build (it refuses to compile against stock Postgres). Therefore it cannot be
installed via apt like the other engines — it ships **docker-only** via the
official image `orioledb/orioledb:latest-pg17`. Its wire protocol is plain
PostgreSQL (libpq/psql), so workload/driver/metrics reuse the Postgres path.

Active-active replication is not yet available upstream, so v1 is a **single
container**.

## Decisions (locked)

- **Topology / params:** single docker container, a new slim `OrioledbParams`
  (NOT a reuse of `PostgresParams` — patroni/haproxy/etcd/pgbouncer/replicas do
  not apply to a single container).
- **Image pull:** the gateway runs as a **pull-through Docker registry** to
  Docker Hub; benchmark nodes (no public egress) reach it via a docker
  `registry-mirror`.

## 1. Proto & data model

File `protocols/cloud/v1/domain/database.proto`:

- `Database.Kind`: add `KIND_ORIOLEDB = 9`.
- New message:
  ```proto
  message OrioledbParams {
    string image            = 1; // default "orioledb/orioledb:latest-pg17"
    map<string,string> postgres_options = 2; // -> postgresql.conf (-c flags)
    string initdb_locale    = 3; // default "C" (OrioleDB requires C/POSIX/ICU)
    uint32 shared_buffers_mb = 4; // common tuning, optional
  }
  ```
- `DatabaseParams.engine` oneof: add `OrioledbParams orioledb = 17;`
- The image tag (`pg17`, `pg16`) is the "version" surfaced in the FE; no extra
  proto field for version.

Rationale for slim params: OrioleDB v1 is one container; HA primitives are not
applicable and would be dead config.

## 2. Backend: topology + deploy renderer

New package `internal/domain/database/orioledb/` mirroring
`internal/domain/database/postgres/` but emitting docker steps:

- `orioledb.go` `BuildTopologySpec(*domain.OrioledbParams)`: one node, one
  component `engine="orioledb", role="master"`. No replica/proxy/coordinator.
- `internal/domain/database/builder.go`: add
  `case domain.Database_KIND_ORIOLEDB: return (&orioledbdb.Database{}).BuildTopologySpec(params.GetOrioledb())`.
- `package.go` `PackageResolver.ResolveDatabasePackage`: apt packages
  `["docker.io"]`; `pre_install` writes `/etc/docker/daemon.json` with
  `registry-mirrors: ["http://<gateway-registry>"]` (+ `insecure-registries`
  if plain HTTP), then `systemctl enable --now docker`. docker.io comes from
  the Ubuntu universe repo, so it flows through the existing apt-cacher proxy.
- `deployment.go` `DeploymentRenderer` (implements `Supports`/`RenderComponent`/
  `RenderPreview`) emits, in the existing engine.go ordering:
  - order 100 (install): install docker.io, write daemon.json mirror, start docker.
  - order 200 (write_service): a systemd unit wrapping the container —
    `ExecStartPre=docker pull <image>`,
    `ExecStart=docker run --rm --name stroppy-orioledb --network host \
      -e POSTGRES_PASSWORD=<pw> -e POSTGRES_INITDB_ARGS="--locale=<locale>" \
      -v <conf-dir>:/etc/orioledb <image> -c config_file=... <postgres_options>`,
    `ExecStop=docker rm -f stroppy-orioledb`.
  - order 230 (healthcheck): `pg_isready -h 127.0.0.1 -p 5432` (or
    `docker exec stroppy-orioledb pg_isready`).
- Register in `internal/infrastructure/adapters/suite_wizard_engine.go`: add
  `orioledb.PackageResolver{}` to the package resolver registry and
  `orioledb.DeploymentRenderer{}` to the renderer registry.

Key point: no new deploy phase. The generic deploy engine
(`internal/domain/deployment/engine.go`: install -> systemd -> start ->
healthcheck) already models this; "install docker before the DB" is simply the
order-100 install command preceding the order-200 docker-run unit. The
`--network host` keeps port 5432 on the node so the stroppy workload connects
exactly as it does to native Postgres.

## 3. Gateway: pull-through Docker registry

Private benchmark nodes have no public egress; they only reach the gateway. So
the gateway mediates the image pull.

- **Server side** (the cloud stack box, which has egress): add a `registry:2`
  service to `docker-compose.yaml` configured as a pull-through cache
  (`REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io`). Expose it to nodes
  either via Caddy (a new route/host) or via the gateway cmux `/v2/` route.
- **Node side**: docker `daemon.json` sets
  `registry-mirrors: ["http://<gateway-registry>"]` at docker install time
  (§2 pre_install). `docker pull orioledb/orioledb:latest-pg17` then transparently
  resolves through the mirror, which fetches from Docker Hub once and caches.

Notes / to resolve in the plan:
- docker `registry-mirror` only applies to Docker Hub images — exactly our case.
- TLS: the mirror will likely be plain HTTP inside the VPC (mirroring the
  existing apt http-proxy choice), requiring `insecure-registries` on nodes.
  Confirm during implementation whether Caddy terminates TLS for it instead.

## 4. Workload / driver / metrics (reuse Postgres)

- `internal/app/presets_seed.go` `workloadProtocolForDatabase`:
  `KIND_ORIOLEDB -> PROTOCOL_PG`.
- stroppy driver `postgres`, port 5432, scheme `postgresql` (already wired for PG
  in `internal/domain/workload/stroppy_config.go`).
- `internal/domain/metrics/queries.go` `MetricsForDB` and
  `internal/infrastructure/execution/monitoring.go` `dbKindString`:
  `orioledb -> postgresMetrics()` (OrioleDB exposes Postgres metrics via
  postgres_exporter).
- Add a builtin preset "OrioleDB single" in `builtinDatabasePresets()`.

## 5. Frontend

- `web/src/lib/proto/cloud/v1/domain/database_pb.ts`: regenerated from proto.
- `web/src/services/wizard.ts`: add `"orioledb"` to `EngineKind`; add an
  `ENGINES` entry (label "OrioleDB", hex, blurb, `typed:true`); add to
  `ENGINE_TO_KIND` / `KIND_TO_ENGINE`; `defaultProtocolFor -> PG`;
  `driverTypeFor -> "postgres"`; `blankEngineParams`; `DB_VERSIONS:
  {orioledb: ["pg17","pg16"]}`.
- `web/src/components/library-table/labels.ts`: add to `DbKind`, `DB_KINDS`,
  `DB_LABEL`, `DB_COLOR`.
- `web/src/pages/NewRun.tsx`: add `ENGINE_ICON` entry.
- `web/src/components/database/DatabaseParamsForm.tsx`: add an
  `EngineParamsForm` branch for orioledb (image/version select +
  `postgres_options` map + locale), and an `engineNodes` branch returning a
  single master node for the topology preview.

## 6. Rollout (OrioleDB-specific)

- Before deploying a stand: the box must run the `registry:2` pull-through
  service (part of the cloud compose; lands via the usual git pull +
  `docker compose up -d`).
- The OrioleDB container is installed on the node by the agent per the recipe
  (docker.io -> mirror -> pull -> run). No manual node prep.

## Exhaustive list of switch/enum sites to touch

Backend:
- `protocols/cloud/v1/domain/database.proto` (enum + oneof).
- `internal/domain/database/builder.go` (topology dispatch switch).
- `internal/infrastructure/adapters/suite_wizard_engine.go` (two registries).
- `internal/app/presets_seed.go` (`workloadProtocolForDatabase`, builtin preset,
  package seeding helper).
- `internal/domain/metrics/queries.go` (`MetricsForDB`).
- `internal/infrastructure/execution/monitoring.go` (`dbKindString`).

Frontend:
- `web/src/services/wizard.ts`, `web/src/components/library-table/labels.ts`,
  `web/src/pages/NewRun.tsx`, `web/src/components/database/DatabaseParamsForm.tsx`.

Infra:
- `docker-compose.yaml` (registry:2 service), Caddy/gateway route for the
  registry.

## Build order

proto + codegen (Go api/ts) -> backend orioledb package + registries + switch
sites -> gateway registry (compose + route) -> frontend -> preset/metrics ->
roll the registry to stage -> one e2e orioledb run.

## Out of scope (v1)

- OrioleDB replicas / HA (active-active not upstream-ready).
- Non-Docker-Hub registries through the mirror.
- Custom patched-postgres builds outside the official image.
