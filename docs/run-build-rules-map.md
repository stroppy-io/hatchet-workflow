# Run-Build Rules Map (`main`)

Source-derived map of **every conditional** (`if` / `switch` / guard / default-fallback)
that decides **how a run is built** — from API request to DAG. Purpose: align the
**wizard UI rules** with the **code rules** so the form never offers an invalid combo
and never silently diverges from what the backend does.

Scope = the `run` build path only:
`internal/domain/api/*` (resolve) → `internal/domain/run/*` (validate, machines, DAG, tasks).

**Two parts:**
- **Part I (§0–6)** — backend rules (Go).
- **Part II (§7–9)** — frontend rules (`web/src`, React/TSX) + **parity/drift table** (§9) where the wizard and the backend disagree.

> ⚠️ **Architectural fact:** the frontend does **not** fetch run-config rules from the
> backend. `KIND_PROTOCOLS`, `SCRIPT_COMPAT`, `DB_VERSIONS`, all per-DB validation, and all
> defaults are **hardcoded a second time** in `web/src`. Only stroppy versions/commits,
> presets, packages, quotas, and probe metadata come from the API. → **Two sources of
> truth.** Every rule below exists twice and can drift. §9 lists the drifts found.

## Pipeline overview

```
HTTP request (RunConfig)
  │
  ├─ 1. RESOLVE (api/)                        ← presets, packages, defaults, probe
  │     server.runStart / suite.runSuiteOnce
  │       ↳ resolveRunPreset    (preset_handlers.go)
  │       ↳ resolveRunPackage   (package_handlers.go)
  │       ↳ probeRunWorkload    (probe_handler.go)
  │       ↳ FillMachinesFromTopology (run/machines.go)
  │       ↳ ValidateQuotas      (types/settings.go)
  │
  ├─ 2. VALIDATE (run/validate.go)            ← reject invalid config
  │
  ├─ 3. EXPAND / SIZE (run/machines, placement, pdisk_size, cost)
  │
  ├─ 4. BUILD DAG (run/builder.go)            ← which phases exist + dependency wiring
  │
  └─ 5. BUILD TASKS (run/task_*.go, effective_config, rendered_configs)
            ← provider dispatch, per-DB install/config, stroppy, configs
```

Each conditional has a stable ID. Wizard fields that drive a rule are named in the
"Driving input" column — those are the inputs the form must gate on.

---

## 0. The decision axes (what the wizard collects)

Every rule keys off one of these inputs. The wizard form is essentially these axes:

| Axis | Field | Values / notes |
|---|---|---|
| **DB kind** | `Database.Kind` | `postgres`, `mysql`, `mariadb`, `picodata`, `ydb`, `ydb-managed`, `cockroach` |
| **Provider** | `Provider` | `yandex`, `docker` |
| **External DB** | `ExternalDB{Endpoint}` | bring-your-own-DB toggle; if set, almost everything is skipped |
| **Preset** | `PresetID` | applies a stored topology *only if no topology given* |
| **Package** | `PackageID` | resolved binary; skipped for YDB / YDB-managed |
| **Protocol** | `Stroppy.Protocol` | gated by DB kind (`KindSupportsProtocol`) |
| **Workload/script** | `Stroppy.Script` / `Workload` | gated by (kind, protocol) via `ScriptSupported` |
| **Topology flags** | per-kind | Patroni, Etcd, PgBouncer, HAProxy/ProxySQL, GroupRepl/SemiSync, YDB split/combined, fault tolerance |
| **Machine spec** | per-role + `MachineOverride` | cpus/mem/disk/diskType; override wins per-field if `>0` |
| **Stroppy params** | VUs, PoolSize, ScaleFactor, Duration, Iterations, K6Mode, Quiet, NoThresholds | range-validated + defaulted |

---

## 1. RESOLVE stage (`internal/domain/api/`)

Ordered sequence — single-run (`runStart`) and suite (`runSuiteOnce`) share sub-steps:

| # | Step | Where |
|---|---|---|
| 1 | decode RunConfig, gen ID if empty | `server.go:584,592` |
| 2a | `resolveRunPreset` — apply preset topology if none given | `preset_handlers.go:236` |
| 2b | `resolveRunPackage` — resolve binary (skip YDB) | `package_handlers.go:322` |
| 2c | `probeRunWorkload` — probe script (skip if override JSON) | `probe_handler.go:202` |
| 2d | `FillMachinesFromTopology` — explode topology → machines | `run/machines.go` |
| 2e | `ValidateQuotas` — per-tenant limits | `types/settings.go:177` |
| 2f–2h | queue-depth + duplicate-run checks | `server.go:636-649` |
| 2i | `EstimateRunCost` | `server.go:651` |
| 2j | enqueue | `server.go:659` |

Suite path additionally expands a **DB-preset × workload matrix** and skips cells where
`ScriptSupported(kind, proto, script)` is false (`suite_handlers.go:693`).

### 1.1 Preset application

| ID | file:line | Driving input | Condition | TRUE | FALSE/ELSE | Cat |
|---|---|---|---|---|---|---|
| RS01 | server.go:592 | `cfg.ID` | `== ""` | gen `run-<unixms>` | keep | default |
| RS02 | preset_handlers.go:237 | `PresetID` | `== ""` | skip preset | fetch preset | guard |
| RS03 | preset_handlers.go:248 | `Database.Kind` | `== ""` | take kind from preset row | keep | default |
| RS04 | preset_handlers.go:253 | any topology ptr | any non-nil | **request topology wins, preset skipped** | apply preset | preset-apply |
| RS05 | preset_handlers.go:265 (switch DbKind) | preset `DbKind` | per-kind case | set matching `Database.<Kind>` from preset | — | preset-apply |

**Wizard rule:** preset only fills topology when the user supplied none. If wizard sends a
partial topology, the preset is ignored entirely (not merged).

### 1.2 Package resolution

| ID | file:line | Driving input | Condition | TRUE | FALSE/ELSE | Cat |
|---|---|---|---|---|---|---|
| RS06 | package_handlers.go:327 | `Database.Kind` | `ydb` or `ydb-managed` | **no package** (YDB self-downloads / cloud-managed) | resolve package | package |
| RS07 | package_handlers.go:336 | `PackageID` | `!= ""` | fetch by explicit ID | fall to default | package |
| RS08 | package_handlers.go:339 | `PackageID` empty | — | `ListPackagesByKindVersion(kind,version)` → rows[0] (built-ins first) | — | package |
| RS09 | package_handlers.go:343 | lookup result | err or empty | error `no package found for <kind> <version>` | continue | guard |
| RS10 | package_handlers.go:373 | `pkg.DebFilename` | `!= ""` | build deb download URL + token | empty | package |
| RS11 | package_handlers.go:375 | `Cloud.ServerAddr` | `== ""` | fallback to docker host addr | use configured | default |
| RS12 | package_handlers.go:383 | jwtIssuer | non-nil | gen 2h deb token | none | package |

### 1.3 Workload probe + defaults

| ID | file:line | Driving input | Condition | TRUE | FALSE/ELSE | Cat |
|---|---|---|---|---|---|---|
| RS13 | probe_handler.go:206 | `ConfigOverrideJSON` | `!= ""` | parse override, **skip normal probe** | run probe | probe |
| RS14 | probe_handler.go:214 | `Script`/`Workload` | both empty | default `tpcc/procs` | use given | default |
| RS15 | probe_handler.go:221 | `Protocol` | `== ""` | `DefaultProtocol(kind)` (first in `KindProtocols[kind]`) | keep | default |
| RS16 | probe_handler.go:225 | `Protocols[proto].DriverType` | non-empty | driver = meta.DriverType | driver = `string(kind)` | probe |
| RS17 | probe_handler.go:103/124 | `PoolSize`/`ScaleFactor` | `>0` | set driver pool / `SCALE_FACTOR`,`POOL_SIZE` env | absent | probe |
| RS18 | stroppy_preview_handler.go:18 / server.go:732 | `ConfigOverrideJSON` | `!= ""` | preview/dry-run returns override verbatim | build from fields | probe |

### 1.4 Suite-matrix specifics

| ID | file:line | Driving input | Condition | TRUE | FALSE/ELSE | Cat |
|---|---|---|---|---|---|---|
| RS19 | suite_handlers.go:629 | `Policy.Mode` | `== ""` | `DefaultSuitePolicy()` | stored policy | default |
| RS20 | suite_handlers.go:614 | `ConcurrentPolicy`+`LastBatchID` | `forbid` & active | 409 previous batch running | continue | guard |
| RS21 | suite_handlers.go:654 | `Network.CIDR` | `== ""` | default `10.10.0.0/24` | decoded | default |
| RS22 | suite_handlers.go:657 | `StroppyMachine` | set & `!= "{}"` | use suite stroppy machine | nil | default |
| RS23 | suite_handlers.go:685 | `workload.Protocol` | `== ""` | `DefaultProtocol(kind)` | explicit | default |
| RS24 | suite_handlers.go:693 | `ScriptSupported(kind,proto,script)` | false | **skip cell**, add to `skipped` | enqueue | guard |
| RS25 | suite_handlers.go:701 | suite stroppy vs item | item has none | item inherits suite stroppy machine | item own | default |
| RS26 | suite_handlers.go:788 (switch) | `kind` `defaultDBVersion` | per-kind | pg`17` my`8.4` maria`11.4` pico`25.3` ydb`25.2` managed`managed` crdb`24.2` | `latest` | default |

### 1.5 Quotas (hard gate before enqueue)

| ID | file:line | Driving input | Condition | Effect if violated | Cat |
|---|---|---|---|---|
| RS27 | settings.go:177 | `AllowedDBKinds` vs kind | kind not allowed | error | guard |
| RS28 | settings.go:188 | `AllowedProviders` vs provider | provider not allowed | error | guard |
| RS29 | settings.go:201 | per DB-role machine | non-DB role skipped; DB role checked | — | guard |
| RS30 | settings.go:205-216 | `MaxNodes/CPUs/Mem/Disk` per node | exceeds (when limit `>0`) | error | guard |
| RS31 | server.go:636 | `MaxQueueDepth` | depth `>=` limit | 429 | guard |
| RS32 | server.go:642/645 | runs / job_runs dup | already exists/queued | 409 | guard |
| RS33 | server.go:656 | scheduler | nil | 503 | guard |

---

## 2. VALIDATE stage (`internal/domain/run/validate.go`)

The canonical config-validity contract. **The wizard's client-side validation should
mirror this table exactly.**

### 2.1 Required / topology consistency

| ID | file:line | Driving input | Condition | Error | Cat |
|---|---|---|---|---|---|
| V01 | validate.go:19 | `Database.Kind` | `== ""` | `database.kind is required` | required |
| V02 | validate.go:26 | `ExternalDB.Endpoint` | externalDB set & endpoint empty | `external_db.endpoint is required` | required |
| V03 | validate.go:29 | all topology ptrs + PresetID | no externalDB & all nil & no preset | `database topology or preset_id is required` | required |
| V04–V10 | validate.go:50 | `Kind` vs topology ptrs | kind=X but a non-X topology ptr set | `database.kind is X but non-X topology is set` (one per kind) | consistency |

**Wizard rule:** exactly one topology block, matching the selected kind. Switching kind
must clear other kinds' topology.

### 2.2 Protocol / script compatibility

| ID | file:line | Driving input | Condition | Error | Cat |
|---|---|---|---|---|---|
| V11 | validate.go:65 | `Protocol`,`Kind` | proto set & `!KindSupportsProtocol(kind,proto)` | `protocol %q not supported by database %q` | protocol |
| V12 | validate.go:68 | `Script`,`Kind`,`Protocol` | known script & not in `ScriptCompat[{kind,proto}]` | `script %q not compatible with %q on %q` | script |

**Wizard rule:** protocol dropdown filtered by kind; script dropdown filtered by (kind, protocol).

### 2.3 Stroppy value ranges

| ID | file:line | Field | Condition | Error |
|---|---|---|---|---|
| V13 | validate.go:87 | Duration | len<2 | `invalid duration` |
| V14 | validate.go:89 | Duration | not end s/m/h | `must end with s, m, or h` |
| V15 | validate.go:96 | VUs | `<0` | `vus must be >= 0` |
| V16 | validate.go:99 | Iterations | `<0` | `iterations must be >= 0` |
| V17 | validate.go:102 | K6Mode | not `""/duration/iterations` | `k6_mode must be duration or iterations` |
| V18 | validate.go:107 | PoolSize | `<0` | `pool_size must be >= 0` |
| V19 | validate.go:110 | ScaleFactor | `<0` | `scale_factor must be >= 0` |

### 2.4 Stroppy version

| ID | file:line | Condition | Error |
|---|---|---|---|
| V20 | validate.go:121 | version empty | `stroppy.version is required (minimum …)` |
| V21 | validate.go:125 | `commit:` SHA len <7 or >40 | `SHA must be 7–40 hex` |
| V22 | validate.go:128 | SHA non-hex | `SHA must be lowercase hex` |
| V23 | validate.go:134 | semver parse fail | `not valid semver` |
| V24 | validate.go:138 | `< MinStroppyVersion` | `below minimum` |

### 2.5 Stroppy runner machine

| ID | file:line | Field | Condition | Error |
|---|---|---|---|---|
| V25 | validate.go:146 | CPUs | `<1` | `cpus must be >= 1` |
| V26 | validate.go:149 | MemoryMB | `<512` | `memory must be >= 512 MB` |
| V27 | validate.go:152 | DiskGB | `<10` | `disk must be >= 10 GB` |

### 2.6 YDB topology (only when kind=ydb)

| ID | file:line | Field | Condition | Error |
|---|---|---|---|---|
| V29 | validate.go:167 | FaultTolerance | not `""/none/block-4-2/mirror-3-dc` | enum error |
| V30 | validate.go:172 | FailureDomainType | not `""/disk` | enum error |
| V31 | validate.go:177 | DefaultDiskType | not `SSD/NVME/ROT` | enum error |
| V32 | validate.go:184 | StorageGroups | `<0` | range error |
| V33 | validate.go:187 | Storage.Placement.Strategy | not `single/round-robin` | enum error |
| V34 | validate.go:190 | Storage.boot size | type `network-ssd-io-m3` & size % 93 ≠ 0 | `must be multiple of 93 GB` |
| V35 | validate.go:193 | Storage.secondary disks | same %93 rule | same |
| V36 | validate.go:198 | Database.Placement.Strategy | enum | enum error |
| V37 | validate.go:205 | Database.boot size | %93 rule | same |
| V38 | validate.go:206 | mirror-3-dc + disk + Storage.Count | count `<3` | `requires ≥3 storage nodes` |
| V39 | validate.go:210 | mirror-3-dc + disk + secondary disks | `<3` | `requires ≥3 secondary disks per node` |

**Wizard rule:** the `network-ssd-io-m3` disk type forces all YDB pdisk sizes to multiples
of 93 GB; `mirror-3-dc` + `disk` failure domain forces ≥3 storage nodes and ≥3 secondary disks.

---

## 3. EXPAND / SIZE stage

### 3.1 Machine explosion (`machines.go` — `FillMachinesFromTopology`)

| ID | file:line | Driving input | Condition | TRUE | FALSE/ELSE |
|---|---|---|---|---|---|
| M01 | machines.go:85 | `Machines` | non-empty | use user machines as-is (no explode) | derive from topology |
| M02 | machines.go:92 | `ExternalDB` | non-nil | no DB nodes; only stroppy runner | normal explode |
| M04/M05 | machines.go:109 | kind=postgres | + Postgres!=nil | 1 aggregated DB spec (master+replicas) | skip |
| M06 | machines.go:121 | Postgres.HAProxy | non-nil | +HAProxy machine | none |
| M07/M08 | machines.go:125 | kind=mysql/mariadb | mariadb→MariaDB ptr else MySQL | aggregated DB spec | skip |
| M10 | machines.go:141 | ProxySQL | non-nil | +ProxySQL machine | none |
| M11/M12 | machines.go:145 | kind=picodata | Picodata!=nil | one spec per instance group | skip |
| M13 | machines.go:154 | Picodata.HAProxy | non-nil | +HAProxy | none |
| M14/M15 | machines.go:158 | kind=ydb | YDB!=nil | YDB storage/db specs +HAProxy | skip |
| M16 | machines.go:165 | YDB.Database | non-nil | **split mode**: compute nodes first, then storage | **combined**: storage only (db co-located) |
| M17 | machines.go:179 | YDB.HAProxy | non-nil | +HAProxy | none |
| M18/M19 | machines.go:183 | kind=cockroach | Cockroach!=nil | N-node DB spec | skip |
| M20/M21 | machines.go:192 | kind=ydb-managed | client CPUs>0 & no stroppy machine | synth stroppy from `YDBManaged.Client` | keep |
| M22 | machines.go:210 | Stroppy.Machine | non-nil | runner spec (role=Stroppy,count=1) | **default 2cpu/4096MB/20GB** |

### 3.2 MachineOverride (per-field, override wins if `>0` / non-empty)

`BakeMachineOverrideIntoTopology` (dry-run, folds into topology) + inline apply during fill.

| ID | file:line | Field | Rule |
|---|---|---|---|
| OV13–18 | machines.go:7-36 | CPUs/MemoryMB/DiskGB/DiskType | override applies only when CPUs>0 / Mem>0 / Disk>0 / Type≠"" |
| OV01 | machines.go:21 | override nil | no-op |
| OV02–12 | machines.go:40-77 | switch kind | applies to that kind's master+replicas / instances / storage(+db) / nodes |

### 3.3 Placement / zones (`placement.go`)

| ID | file:line | Driving input | Condition | TRUE | FALSE/ELSE |
|---|---|---|---|---|---|
| PL01–03 | placement.go:12 | `Placement.Zones` | non-empty (after filter) | use explicit zones | default `[zone-a,zone-b,zone-c]` |
| PL04 | placement.go:39 | Zones (yandex) | explicit | use them | strategy-based |
| PL05 | placement.go:42 | `Strategy` | not `round-robin` (or nil) | single-zone `[defaultZone]` | round-robin from region prefix |
| PL08 | placement.go:51 | defaultZone suffix | no `-x` suffix | `[defaultZone]` | build `[region-a,-b,-d]` |
| PL09 | placement.go:67 | `Strategy` | `single` | always `zones[0]` | `zones[idx%len]` round-robin |

### 3.4 YDB pdisk auto-size (`pdisk_size.go`)

| ID | file:line | Driving input | Condition | TRUE | FALSE/ELSE |
|---|---|---|---|---|---|
| PD06 | pdisk_size.go:73 | kind / YDB | not ydb or YDB nil | no adjust | continue |
| PD07 | pdisk_size.go:77 | `AutoSizePdisks` | false | no adjust (manual) | continue |
| PD08 | pdisk_size.go:79 | SecondaryDisks | empty | no adjust | size every secondary disk |
| PD04 | pdisk_size.go:40 | `Script` | prefix `tpcb` | 15 MB/unit | 100 MB/unit (tpcc/default) |
| PD01 | pdisk_size.go:19 | diskType | `network-ssd-io-m3` & disk>0 | round up to 93 GiB chunk | unchanged |

### 3.5 Cost estimate (`cost.go`)

| ID | file:line | Rule |
|---|---|---|
| CO01/02 | cost.go:25 | externalDB → only stroppy runner counted |
| CO03/07 | cost.go:36 | machine `Count<=0` clamped to 1 |
| CO04 | cost.go:43 | secondary disks summed into DiskGB |
| CO05/06 | cost.go:52 | yandex: stroppy runner counted once (dedup by role) |

---

## 4. BUILD DAG stage (`internal/domain/run/builder.go`)

Entry: `Build(cfg, deps)` → `builder.build()` (normal) or `buildExternalDB()`.

### 4.1 Phase-inclusion / dependency conditionals

| ID | file:line | Driving input (wizard flag) | Condition | TRUE | FALSE/ELSE | Cat |
|---|---|---|---|---|---|---|
| B01 | builder.go:80 | `ExternalDB` | non-nil | **whole DB stack skipped** → only network, machines, stroppy, teardown | full DAG | guard |
| B02 | builder.go:211 | `ExternalDB.Endpoint` | host empty after split | error `must be host:port` | continue | guard |
| B03 | builder.go:95 `needsEtcd` | postgres + `Postgres.Etcd` | true | +InstallEtcd +ConfigureEtcd; etcd→configDB dep | none | phase |
| B04 | builder.go:108 `needsPatroni` | postgres + `Postgres.Patroni` | true | +Install/ConfigurePatroni; **ConfigureDB becomes noop dep on Patroni** | real ConfigureDB | phase |
| B07 | builder.go:144 `needsPgBouncer` | postgres + `Postgres.PgBouncer` | true | +Install/ConfigurePgBouncer; stroppy waits on it | none | phase |
| B08 | builder.go:149 `needsProxy` | per-kind proxy ptr (see 4.2) | true | +Install/ConfigureProxy; stroppy waits | none | phase |
| B09 | builder.go:154 `needsYDBInit` | ydb + YDB!=nil | true | +InitYDBCluster +StartYDBDatabase; stroppy+monitor wait | none | phase |
| B10 | builder.go:159 `needsCockroachInit` | cockroach + Cockroach!=nil | true | +InitCockroach; stroppy+monitor wait | none | phase |
| B11 | builder.go:177 | `Deps.MonitoringURL` (server) | `!= ""` | configure OTLP export in stroppy task | defaults | wiring |
| B13 | builder.go:317 | `Stroppy.Protocol` | ≠ `ydb-pgwire` | pgwire port = 0 (listener off) | pass pgwire port | wiring |

### 4.2 `needsProxy` per-kind (builder.go:295)

Proxy phases added when the kind's proxy topology field is non-nil:
postgres→`HAProxy`, mysql→`ProxySQL`, mariadb→`ProxySQL`, picodata→`HAProxy`, ydb→`HAProxy`.

### 4.3 `dbTasks()` switch (builder.go:390) — install/config task per kind

| Kind | Install | Config | Note |
|---|---|---|---|
| postgres | pgInstall | pgConfig | |
| mysql | mysqlInstall | mysqlConfig | |
| mariadb | mysqlInstall | mysqlConfig | reuses MySQL tasks |
| picodata | picoInstall | picoConfig | |
| ydb | ydbInstall | ydbConfig | pgwirePort from B13 |
| ydb-managed | noop | noop | YC manages DB |
| cockroach | cockroachInstall | cockroachConfig | **error if Cockroach topology nil** |
| default | — | — | error `unsupported database kind` |

### 4.4 Full phase list & gate

| Phase | Normal | ExternalDB | Gate |
|---|---|---|---|
| network, machines | always | always | — |
| install_etcd / configure_etcd | cond | never | B03 |
| install_db | always | never | — |
| install_patroni / configure_patroni | cond | never | B04 |
| configure_db | always (real or noop) | never | noop when B04 |
| install_monitor / configure_monitor | always | never | extra deps via B09/B10 |
| install_pgbouncer / configure_pgbouncer | cond | never | B07 |
| install_proxy / configure_proxy | cond | never | B08 |
| init_ydb_cluster / start_ydb_database | cond | never | B09 |
| init_cockroach | cond | never | B10 |
| install_stroppy, run_stroppy | always | always | run deps vary |
| teardown | always (AlwaysRun) | always | runs even on upstream failure |

---

## 5. BUILD TASKS stage

### 5.1 Provider dispatch (`task_infra.go` — biggest conditional file, ~108)

Every infra step switches on **provider**:

| Step | docker | yandex | default |
|---|---|---|---|
| network | create bridge | no-op (VPC via TF) | skip |
| machines | deploy containers | run Terraform | error |
| teardown | stop containers + rm network | `terraform destroy` | warn+skip |

Key yandex sub-rules (defaults / guards):
- `kind=ydb-managed` → fully delegated to `yandexManagedYDBMachines` (task_infra.go:322).
- VM defaults: cores 0→2, memGB 0→4, **memGB rounded up to multiple of cores** (YC), disk 0→50, diskType ""→`network-ssd`.
- `SoftwareAcceleratedNetwork` → `software_accelerated` else `standard`.
- PlatformID: run-level → settings → `standard-v2`.
- multi-zone (`len(subnetZones)>1`) → per-zone /24 subnet map.
- NatIP empty → fall back to InternalIP.

Role dispatch (both docker & yandex), determines targets + DB endpoint:
- `RoleDatabase/YDBStorage/YDBDatabase` → dbTargets (+ sub-lists for YDB).
- first dbTarget = master → `SetDBEndpoint`. YDB prefers a `YDBDatabase` (compute) target.
- `RoleProxy` → proxyTargets; `RoleStroppy` → `SetStroppyTarget`.

**Proxy endpoint override** (when proxyTargets present & kind≠picodata):
postgres→`:5000` (HAProxy write), mysql/mariadb→`:6033` (ProxySQL), ydb→`:2136`.

### 5.2 Stroppy task (`task_stroppy.go`)

Protocol → URL/driver:
- proto ""→`DefaultProtocol(kind)`; unknown→generic `host:port` + kind as driver.
- `pg`→`postgresql://postgres@host:port/postgres?sslmode=disable`.
- `picodata`→`postgres://admin:T0psecret@host:port?sslmode=disable`.
- `ydb-grpcs`→`grpcs://host:port/?database=<path>` (managed path if set, else `DBPathPlaceholder`).

Config-override branch: `ConfigOverrideJSON != ""` → substitute `DBHost/DBPort` placeholders,
inject OTLP, skip field-based build entirely.

Stroppy param defaults (only applied when zero/empty):
VUs: `VUSScale`→`Workers`→`1`; PoolSize 0→100; ScaleFactor 0→1; Duration ""→`60s`;
Iterations ≤0→1; K6Mode ""→`duration`; Quiet nil→true; InsertMethod ""→`native`;
script ""→`Workload`→`tpcc/procs`.

k6 args: quiet→`-q`; mode iterations→`--iterations N` else `--duration D`; NoThresholds→`--no-thresholds`.
Env: `Machine.CPUs>0`→`LOAD_WORKERS`; user env keys upper-cased.

OTLP injection skipped entirely if `OTLPEndpoint==""`; user-provided exporter/env wins.

### 5.3 Effective + rendered config (`effective_config.go`, `rendered_configs.go`)

Both switch on **DB kind**, then emit keys conditionally:

- **YDB**: disk 0→80, mem 0→4096, cpus 0→2, pdisk floor 10, FT ""→`none`.
  **Combined mode** (`YDB.Database==nil`) halves storage memory (OOM guard); split mode uses Database spec.
- **Postgres**: `Patroni`→`ha=patroni+etcd`; `PgBouncer`→`pooler=pgbouncer`; replicas→replica conf;
  HAProxy health-check = `patroni`(:8008) if Patroni else `tcp`. SyncReplicas>0 → Patroni sync mode.
- **MySQL/MariaDB**: `GroupRepl`→`replication=group`, else `SemiSync`→`semi-sync`, else none.
- Master/Primary/Instance/Node **Options maps merged verbatim** (user overrides win, RC01).
- Stroppy bench keys emitted only when set; K6Mode ""→`duration`; Quiet nil→true.

### 5.4 Per-DB task rules (`task_*.go`)

| File | Key rules |
|---|---|
| task_ydb.go | FT ""→none; combined halves mem; pdisk floor 10; dbPath ""→`/Root/testdb`; storageGroups 0→1; diskType ""→`ssd`; compute targets fall back to storage in combined |
| task_proxy.go | install: pg/pico/ydb→HAProxy, mysql/maria→ProxySQL; config routed per kind; HAProxy health = patroni(:8008) if Patroni else tcp; ProxySQL reads GroupRepl |
| task_mysql.go | node[0]=primary else replica; replica uses ReplicaOptions+Replicas[0]; GroupRepl→group seeds `host:33061` |
| task_postgres.go | node[0]=master(MasterOptions) else replica(ReplicaOptions); reports ha/pooler |
| task_picodata.go | instance spec = Instances[0] or zero-value |
| task_cockroach.go | init on first node only; InternalHost→Host fallback |
| task_patroni.go | etcd targets capped at 3; pgVersion ""→`16`; SyncReplicas>0→sync mode |
| task_pgbouncer.go | auth_type from `PgBouncerOptions["auth_type"]` else `trust` |
| task_etcd.go | install/config targets capped at 3 (quorum) |
| task_monitor.go | metrics/logs endpoints derived from monitoringURL if unset; YDB combined-mode detection |

---

## 6. Wizard ⇄ code rule checklist (the actionable map)

These are the rules the **wizard form must enforce** so requests never get rejected or
silently reshaped by the backend:

1. **One topology, matching kind.** Selecting a kind must clear other kinds' topology (V04–V10).
2. **Protocol filtered by kind** (`KindSupportsProtocol`, V11); **script filtered by (kind,protocol)** (`ScriptSupported`, V12, RS24).
3. **Preset is fill-only.** A partial topology disables the preset entirely (RS04) — don't show "merge" semantics.
4. **No package picker for YDB / YDB-managed** (RS06).
5. **External DB toggle** hides the whole DB topology + proxy/HA UI; only endpoint + stroppy runner remain (B01, M02). Endpoint must be `host:port` (V02, B02).
6. **Postgres feature flags** (Etcd, Patroni, PgBouncer, HAProxy) each add phases (B03/04/07/08). Patroni replaces plain configure-db; PgBouncer/Proxy gate stroppy start.
7. **YDB constraints:** `network-ssd-io-m3` → disk size multiple of 93 (V34/35/37); `mirror-3-dc`+`disk` → ≥3 storage nodes & ≥3 secondary disks (V38/39); split vs combined mode toggled by presence of a Database node spec (M16) — combined halves memory.
8. **Stroppy ranges** (V13–V27): duration suffix s/m/h; VUs/Iterations/PoolSize/ScaleFactor ≥0; runner cpus≥1, mem≥512, disk≥10; version ≥ min semver or 7–40-hex commit.
9. **MachineOverride is per-field, >0 wins** (OV13–18) — a 0/empty field means "keep topology value", not "set to 0".
10. **Defaults the backend will silently apply** (so the wizard should preview them): protocol, script `tpcc/procs`, PoolSize 100, ScaleFactor 1, Duration 60s, K6Mode duration, Quiet true, runner 2cpu/4096/20, db-version per kind (RS26), CIDR 10.10.0.0/24.
11. **Replication mode** for MySQL/MariaDB: GroupRepl XOR SemiSync (group wins if both set).
12. **Quotas** (RS27–30): kind/provider allowlists + per-node max cpu/mem/disk/nodes — mirror as form limits.

---

# Part II — Frontend rules (`web/src`)

React/TSX wizard. Run created via `POST /api/v1/run` (`client.ts:281 startRun`);
validated via `POST /api/v1/validate`, `POST /api/v1/dry-run`.

## 7. Frontend file map

| File | LoC | Role in run-build |
|---|---|---|
| `api/types.ts` | 701 | **Source of truth (frontend copy):** `KIND_PROTOCOLS`, `SCRIPT_COMPAT`, `ALL_DB_KINDS`, `YDB_MANAGED_RESOURCE_PRESETS`, `DEFAULT_SUITE_POLICY`, all RunConfig types |
| `api/client.ts` | 850 | HTTP: startRun, validateRun, dryRun, previewStroppyConfig, create/update preset/run-preset/suite/item, probeScript |
| `pages/NewRun.tsx` | 1775 | 4-step run wizard (Infra→DB→Workload→Review); **assembles + POSTs RunConfig**; `DB_VERSIONS` lives here |
| `pages/PresetDesigner.tsx` | 1679 | topology-preset editor; per-kind forms + `validatePostgres/MySQL/Picodata/YDB`; locked/default option maps |
| `pages/SuiteBuilder.tsx` | 1484 | suite = preset×workload matrix; `defaultDBVersionTS()` |
| `components/WorkloadForm.tsx` | 861 | script picker (SCRIPT_COMPAT filtered), stroppy params, env, runner sizing, live probe |
| `components/InfrastructureForm.tsx` | 122 | provider tiles + platform select (yandex only) |
| `components/ui/sliders.tsx` | 289 | `CPU_STEPS`, `DISK_STEPS`, `IO_M3_CHUNK_GB=93`, `YC_PLATFORMS`, `DISK_SPECS`; step snapping |

**Payload assembly:** `NewRun.tsx:315-372` (useMemo build) → `buildLaunchConfig()` `:448`
(folds dbConfigDraft, stroppyConfigDraft, rendered overrides) → `handleSubmit()` `:530` → `startRun()`.

## 8. Frontend rule tables (condensed)

Full per-rule tables held by the analysis; key conditionals grouped by axis. IDs map to
the verbose tables (NR-*/WF-*/PD-*/SB-*/IF-*).

### 8.1 Hardcoded constant maps (frontend copy)

| Const | file:line | Value |
|---|---|---|
| `KIND_PROTOCOLS` | types.ts:21 | pg→[pg], mysql→[mysql], mariadb→[mysql], picodata→[picodata], ydb→[ydb-grpc,ydb-pgwire], ydb-managed→[ydb-grpcs], cockroach→[cockroach] |
| `SCRIPT_COMPAT` | types.ts:33 | pg:pg & mysql & mariadb → tpcc/procs,tpcc/tx,tpcb/procs,tpcb/tx,tpch/tx; picodata/ydb-grpc/ydb-managed/cockroach → tpcc/tx,tpcb/tx,tpch/tx; **ydb:ydb-pgwire → tpcc/tx-ydb-pgwire,tpcb/tx-ydb-pgwire** |
| `DB_VERSIONS` | NewRun.tsx:90 | pg[17,16,15] my[8.4,8.0] maria[11.4,10.11] pico[25.3] ydb[25.2,25.1,24.4,24.3] managed[managed] crdb[24.2,24.1,23.2] |
| `defaultDBVersionTS` | SuiteBuilder.tsx | pg17 my8.4 maria11.4 pico25.3 ydb25.2 managed crdb24.2 (else `latest`) |
| `CPU_STEPS` | sliders.tsx:111 | [2,4,8,12,16,24,32,48,64,96,128,192,256] |
| `IO_M3_CHUNK_GB` | sliders.tsx:116 | 93 |
| `YC_PLATFORMS` | sliders.tsx:148 | v2: 80c/640G; v3: 96c/768G; highfreq-v3: 96c/768G |
| YDB fault tolerance | PresetDesigner.tsx:1078 | none, block-4-2, mirror-3-dc |
| YDB disk types | PresetDesigner.tsx:1110 | SSD, NVME, ROT |
| placement strategies | PresetDesigner.tsx:942 | single, round-robin |

### 8.2 Visibility / option-filter (kind & provider driven)

| Axis | Rule |
|---|---|
| protocol picker | shown only when `KIND_PROTOCOLS[kind].length > 1` → in practice **only YDB** (NR-29) |
| package picker | hidden when `kind === "ydb"` (NR-31) — note: backend also skips ydb-managed (RS06), frontend hides only ydb |
| script grid | filtered by `SCRIPT_COMPAT["kind:protocol"]` (WF-01) |
| external DB | toggle hides topology editor; shows endpoint/db/user/pass/ssl (NR-32/33) |
| topology editor | `editorSupported` = pg/mysql/mariadb/picodata/ydb/ydb-managed; **cockroach has NO editor** (NR-34) |
| provider=yandex | shows platform select (IF-04) + stroppy runner machine sizing (WF-05) + platform_id in payload |
| disk io-m3 | `network-ssd-io-m3` → step ladder = multiples of 93 (OF-07/sliders) |
| quotas | allowed_db_kinds/providers filter tiles; max cpu/mem/disk per node cap sliders (NR-01/02, WF-06/07/08) |
| YDB managed | `type==dedicated` → compute/preset/scaling; serverless → throttling RCU (PD-41/42/43) |

### 8.3 Frontend defaults (set on mount)

| Field | Default | file:line |
|---|---|---|
| provider | `docker` | NewRun.tsx:181 |
| platform_id | `standard-v3` | NewRun.tsx:182 |
| script | `tpcc/procs` | NewRun.tsx:191 |
| duration | **`5m`** | NewRun.tsx:195 |
| k6Mode | `duration` | NewRun.tsx:196 |
| iterations | 1 | NewRun.tsx:197 |
| quiet | `true` | NewRun.tsx:198 |
| insert method | `native` | NewRun.tsx:200 |
| vus | **`100`** | NewRun.tsx:205 |
| poolSize | **`150`** | NewRun.tsx:206 |
| scaleFactor | **`500`** | NewRun.tsx:207 |
| stroppy cpus | **`32`** | NewRun.tsx:221 |
| stroppy mem | **`65536`** | NewRun.tsx:222 |
| stroppy disk | **`100`** | NewRun.tsx:223 |
| stroppy disk type | `network-ssd` | NewRun.tsx:224 |

### 8.4 Frontend client-side validation

| Rule | file:line | Note vs backend |
|---|---|---|
| Patroni requires Etcd | PresetDesigner.tsx:82 | **frontend-only — backend has no such rule** |
| sync_replicas requires Patroni + ≤ replicas | PresetDesigner.tsx:87 | frontend-only |
| master count = 1 | PresetDesigner.tsx:69 | frontend-only |
| GroupRepl XOR SemiSync | PresetDesigner.tsx:127 | frontend-only (backend just prefers group if both) |
| GroupRepl → 3–9 nodes | PresetDesigner.tsx:132 | frontend-only |
| Picodata instances ≥ rf×shards | PresetDesigner.tsx:181 | frontend-only |
| YDB storage disk ≥ 80 GB | PresetDesigner.tsx:219 | frontend-only (backend just defaults to 80) |
| YDB io-m3 size %93==0 | PresetDesigner.tsx:225 | **matches backend V34/35/37** |
| YDB mirror-3-dc+disk → ≥3 nodes & ≥3 disks | PresetDesigner.tsx:229 | **matches backend V38/39** |
| machine cpus≥1/mem≥512/disk≥10 | PresetDesigner.tsx:53 | **matches backend V25-27** (but FE applies to DB nodes, BE only to stroppy runner) |
| duration required (non-iterations) | NewRun.tsx:531 | matches V13/14 loosely (FE checks non-empty, not s/m/h suffix) |
| iterations ≥ 1 | NewRun.tsx:532 | stricter than backend (BE allows ≥0) |

### 8.5 Payload shaping (NewRun.tsx)

- topology inline `database[subKey]=edited` **OR** `preset_id` — edits win, preset dropped (NR-14/15). `subKey` = `ydb_managed` for managed (NR-16).
- yandex → `stroppy.machine` + `platform_id` included; docker → omitted (NR-13/18).
- `external_db` only if `externalEnabled && endpoint` (NR-21).
- optional stroppy keys (sql/iterations/env/files/steps/no_steps) only when set (NR-07..12).
- `config_override_json` only when user edited stroppy config (NR-26).

---

## 9. Parity / DRIFT table (frontend ⇄ backend)

The actionable payoff. ✅ = aligned, ⚠️ = drift to reconcile.

| # | Rule | Frontend | Backend | Status |
|---|---|---|---|---|
| 1 | **Source of truth** | hardcoded copy in `web/src` | `internal/domain/types` + `validate.go` | ⚠️ duplicated, no sync mechanism — every row below can silently diverge |
| 2 | protocols per kind | `KIND_PROTOCOLS` (types.ts) | `KindProtocols` / `KindSupportsProtocol` | ⚠️ verify byte-for-byte; FE drives the picker, BE rejects mismatch (V11) |
| 3 | script compat | `SCRIPT_COMPAT` (types.ts) | `ScriptCompat` / `ScriptSupported` | ⚠️ two hand-maintained maps; suite skips (RS24) and BE validate (V12) use BE map, wizard uses FE map |
| 4 | default DB version | DB_VERSIONS[0] per kind | `defaultDBVersion` (suite_handlers.go) | ✅ values match (pg17/my8.4/maria11.4/pico25.3/ydb25.2/crdb24.2) |
| 5 | **default duration** | `5m` (NewRun:195) | `60s` (task_stroppy) | ⚠️ different — FE always sends a value so BE default rarely hits, but the two disagree |
| 6 | **default poolSize** | `150` | `100` (task_stroppy:243) | ⚠️ |
| 7 | **default scaleFactor** | `500` | `1` (task_stroppy:246) | ⚠️ large gap; FE always sends 500 |
| 8 | **default stroppy runner** | 32c/64GB/100GB (yandex) | 2c/4GB/20GB (machines.go:210) | ⚠️ docker path uses BE default (FE sends no machine on docker) |
| 9 | default vus | `100` | `1` (task_stroppy:239) | ⚠️ FE always sends 100 |
| 10 | default script | `tpcc/procs` | `tpcc/procs` | ✅ |
| 11 | k6Mode / quiet / insert | duration / true / native | duration / true / native | ✅ |
| 12 | YDB io-m3 %93 | enforced (PD:225) | enforced (V34/35/37) | ✅ |
| 13 | YDB mirror-3-dc ≥3 | enforced (PD:229) | enforced (V38/39) | ✅ |
| 14 | **Patroni→Etcd required** | FE error (PD:82) | no such check | ⚠️ FE stricter; BE builds etcd only if `Postgres.Etcd` flag set (B03), independent of Patroni — possible Patroni-without-etcd run via API |
| 15 | **master count = 1** | FE error (PD:69) | not validated | ⚠️ FE-only |
| 16 | **GroupRepl XOR SemiSync** | FE error (PD:127) | BE prefers group if both (effective_config:119) | ⚠️ BE silently picks, FE blocks |
| 17 | **GroupRepl 3–9 nodes** | FE error (PD:132) | not validated | ⚠️ FE-only |
| 18 | **stroppy version min** | not checked | semver ≥ Min, or 7–40 hex commit (V20-24) | ⚠️ **FE misses it** — below-min version passes wizard, BE rejects |
| 19 | **external endpoint shape** | non-empty only (NR-21) | must be `host:port` (V02, B02) | ⚠️ FE accepts bad endpoint, BE rejects |
| 20 | duration suffix s/m/h | not checked (only non-empty) | enforced (V14) | ⚠️ FE-only gap |
| 21 | iterations range | FE `≥1`, BE `≥0` | — | ⚠️ minor; FE stricter |
| 22 | package skip | FE hides for `ydb` only (NR-31) | BE skips `ydb` AND `ydb-managed` (RS06) | ⚠️ FE may show package picker for ydb-managed |
| 23 | cockroach topology editor | none in FE (NR-34) | BE supports cockroach (dbTasks C15g) | ⚠️ wizard can't build cockroach topology; only via preset/API |
| 24 | YDB storage disk ≥80 | FE error (PD:219) | BE defaults 80, no min check | ⚠️ FE stricter (harmless) |
| 25 | machine min cpu/mem/disk | FE checks DB nodes (PD:53) | BE checks only stroppy runner (V25-27) | ⚠️ different target; DB-node undersize not caught by BE |
| 26 | quotas (kinds/providers/per-node) | FE filters UI (NR-01/02, WF-06/07/08) | BE hard-rejects (RS27-30) | ✅ same intent, BE authoritative |
| 27 | YDB combined vs split | FE: presence of Database node (PD-39) | BE: `YDB.Database==nil` → combined, halves mem (M16, RC19) | ✅ same toggle |
| 28 | placement strategy | FE single/round-robin (PD:942) | BE single/round-robin (placement.go, V33/36) | ✅ |

### 9.1 Recommended reconciliation (priority order)

1. **Kill the duplication (root cause).** Serve `KIND_PROTOCOLS`, `SCRIPT_COMPAT`,
   `DB_VERSIONS`, fault-tolerance/disk-type enums, and defaults from one backend endpoint
   (e.g. `GET /api/v1/run-schema`) and have the wizard consume it. Removes drifts #2–#11 structurally.
2. **Align defaults** (#5–#9): pick one set. Current gap means wizard-preview ≠ what a
   bare API call produces.
3. **Add backend validation the FE already enforces** (#14–#17) — or drop the FE rules if
   they're not real constraints. Today an API client bypasses them.
4. **Add FE validation the backend enforces** (#18–#20): min stroppy version, `host:port`
   endpoint, duration suffix — so the wizard fails fast instead of round-tripping a 400.
5. **Cockroach + ydb-managed gaps** (#22, #23): finish the wizard surfaces or document the
   API-only path.

---

*Generated from `main` @ `cf4a5c8`. Backend = `internal/domain/{api,run,types}`; frontend = `web/src`. Line numbers approximate — re-verify against source before relying on a single rule. §9 drifts are the actionable items.*
