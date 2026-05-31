# Per-Database Params & Deployment Map (`main`)

Source-derived reference: **which params each database uses + how it is deployed.**
Companion to `run-build-rules-map.md` (which covers the conditional rules). This doc
answers: for each DB engine — what topology fields drive it, what config defaults it
gets, what gets installed/configured/started on which node, and how stroppy connects.

Scope = `internal/domain/{types,run,agent}` on `main`.

## 0. Common model (applies to every DB)

### 0.1 Execution unit — `Command` / `Action`
There is **no generic `Step` struct**. The dispatch unit is `Command` (`agent/protocol.go`):
```go
type Command struct { ID string; Action Action; Config any }  // Config = action-specific payload
```
`Action` is a string enum. Each DB has an `install_*` + `config_*` (+ `init_*` / `start_*`) action.
The agent's `Executor.Run()` switches on `Action`, runs `/bin/bash -c <script>`, streams
output to a `LogCallback`, returns a `Report{Status: running|completed|failed}`.

### 0.2 How a node is provisioned & reached
- **Yandex:** Terraform creates VMs; cloud-init installs `stroppy-agent` as a systemd unit; agent **long-polls** `POST /api/agent/poll` (JWT `Bearer`), runs the `Command`, posts `Report` + batched logs. Server never dials the agent.
- **Docker:** `DockerDeployer` runs privileged `stroppy-agent:latest` containers (real systemd PID 1, cgroup mounts), env injected via CopyToContainer. Same agent poll loop. Host = container name (bridge DNS); `pdisk.data` files instead of block devices.
- Same `Executor` + `Command`/`Report` protocol both providers. Durability: commands persisted to `agent_commands` table; orphan-reaper revives on server restart.

### 0.3 Install mechanism (`installPackage`)
Every `install_*` runs `installPackage(pkg)`:
1. `CustomRepoKey` → import GPG key to `/etc/apt/trusted.gpg.d/`
2. `CustomRepo` → write `sources.list.d/custom.list` + `apt-get update`
3. `PreInstall[]` → shell cmds (repo setup)
4. `DebFilename` → `curl [Bearer] → apt-get install -y <deb>`
5. `AptPackages[]` → `apt-get install -y --no-install-recommends ...`

Bootstrap (once/node): policy-rc.d=101, apt lock-timeout 600, mask unattended-upgrades,
install `curl wget ca-certificates gnupg lsb-release sudo tar gzip python3-pip`.

### 0.4 Config-render flow
Defaults map (`db_defaults.go`) → merged with user `*Options` (user wins, per-key) → rendered
file (`dbconfig/*.go`) → written by agent. `%`-values (`25%`, `75%`) resolved against node
RAM. Any `RenderedConfigOverrides["<file>:<role>"]` replaces the rendered body verbatim
(placeholders still substituted). Daemons started via `systemd-run --unit=<name>`.

### 0.5 MachineSpec (shared by all topologies)
`role, count, cpus, memory_mb, disk_gb, disk_type, secondary_disks[], placement{strategy,zones}`.
`secondary_disks` only consumed by YDB. `placement` round-robin/single across zones.

### 0.6 Protocol registry (`protocol.go`) — how stroppy connects

| Protocol | Port | Driver | URL | Kinds (default*) |
|---|---|---|---|---|
| `pg` | 5432 | postgres | `postgresql://postgres@<h>:<p>/postgres?sslmode=disable` | postgres* |
| `mysql` | 3306 | mysql | `root@tcp(<h>:<p>)/` (DSN) | mysql*, mariadb* |
| `picodata` | 5432 | picodata | `postgres://admin:T0psecret@<h>:<p>?sslmode=disable` | picodata* |
| `ydb-grpc` | 2136 | ydb | `grpc://<h>:<p>/Root/testdb` | ydb* |
| `ydb-grpcs` | 2135 | ydb | `grpcs://<h>:<p>/?database=<path>` | ydb-managed* |
| `ydb-pgwire` | 5432 | postgres | `postgresql://<h>:<p>/local?sslmode=disable` | ydb (2nd) |
| `cockroach` | 26257 | postgres | `postgresql://<h>:<p>/defaultdb?sslmode=disable` | cockroach* |

`pg`/`picodata`/`ydb-grpcs` URLs hardcoded in `dbDriverURL` (registry URLTail unused).

### 0.7 ScriptCompat (which workloads each kind+protocol allows)

| Kind:proto | scripts |
|---|---|
| postgres:pg, mysql:mysql, mariadb:mysql | tpcc/procs, tpcc/tx, tpcb/procs, tpcb/tx, tpch/tx |
| picodata:picodata, ydb:ydb-grpc, ydb-managed:ydb-grpcs, cockroach:cockroach | tpcc/tx, tpcb/tx, tpch/tx |
| ydb:ydb-pgwire | tpcc/tx-ydb-pgwire, tpcb/tx-ydb-pgwire |

Only pg/mysql/mariadb support `/procs` (stored procedures). cockroach/ydb-pgwire have no `tpch/tx`... (ydb-pgwire), cockroach no tpch.

### 0.8 Stroppy workload config defaults (`BuildStroppyConfigJSON`)
script `tpcc/procs` · VUs `1` (VUs→VUSScale→Workers) · poolSize `100` · scaleFactor `1` ·
duration `60s` · iterations `1` · k6Mode `duration` · quiet `true` · insert `native`.
Env always: `SCALE_FACTOR`, `POOL_SIZE`; `LOAD_WORKERS` if Machine.CPUs>0. OTLP injected
when `OTLPEndpoint` set (user exporter wins). `ConfigOverrideJSON` bypasses build (only
`__STROPPY_DB_HOST__`/`__STROPPY_DB_PORT__` substituted).

### 0.9 Monitoring (every DB)
Install on all nodes: `node_exporter` v1.9.1, `vmagent` v1.139.0, `vector`. DB-specific exporter:
postgres/cockroach/picodata/ydb-pgwire → `postgres_exporter`; mysql/mariadb → `mysqld_exporter`;
ydb/ydb-managed → native `/metrics` (8765 storage / 8766 db). Configure: derives
metrics/logs endpoints from `monitoringURL`, scrape targets = InternalHost list, `IsYDBCombined`
flag → scrape both 8765+8766 on storage VMs.

---

## 1. PostgreSQL

### A. Topology (`PostgresTopology`)
| Field | Type | Default(single/ha) | Controls | Consumed by |
|---|---|---|---|---|
| Master | MachineSpec | 2c/4G/50 · 4c/8G/100 | primary node | install/config_db |
| Replicas | []MachineSpec | – / 2×4c/8G/100 | streaming/Patroni standbys | config_db, patroni |
| HAProxy | *MachineSpec | nil / 1×2c/2G/20 | proxy tier (nil=none) | needsProxy→proxy phases |
| PgBouncer | bool | false/true | colocated pooler :6432 | needsPgBouncer |
| Patroni | bool | false/true | HA manager; replaces configure_db | needsPatroni |
| Etcd | bool | false/true | DCS on ≤3 DB nodes | needsEtcd |
| SyncReplicas | int | 0/1 (scale 2) | Patroni `synchronous_node_count`; 0=async | patroni |
| MasterOptions/ReplicaOptions | map | shared_buffers 25%, max_conn 200, work_mem 64MB | postgresql.conf overrides | config_db / patroni |
| PgBouncerOptions | map | pool_mode transaction, max_client_conn 1000 | only `auth_type` read explicitly | config_pgbouncer |
| PatroniOptions/EtcdOptions/HAProxyOptions | map | ttl30/loop10/retry10 | **stored, NOT forwarded** (template hardcodes) | — |

### B. Default postgresql.conf (`PostgresDefaults`)
shared_buffers `25%`, max_connections `200`, max_wal_size `4GB`, effective_cache_size `75%`,
work_mem `64MB`, maintenance_work_mem `512MB`, wal_buffers `64MB`, checkpoint_completion_target `0.9`,
random_page_cost `1.1`, effective_io_concurrency `200`, listen_addresses `'*'`.
**v17:** + `summarize_wal=on`.
Replication knobs (only when Patroni=false): `wal_level=replica`, `max_wal_senders=10`,
`max_replication_slots=10`, `hot_standby=on`(replica). With Patroni these go into patroni.yml.
**Packages:** v16 `postgresql-16`+client, v17 `postgresql-17`+client (PGDG repo, key ACCC4CF8).

### C. Deployment
Phases: `install_db → configure_db → configure_monitor → run_stroppy`. Variants add phases:
- **plain/replicas:** config_db serial, node0=master (`systemctl restart postgresql`, `pg_isready`); replicas `pg_ctlcluster stop` → wipe → `pg_basebackup -h <master> -R` → start. pg_hba.conf = trust all.
- **+Etcd:** install_etcd (tarball→/usr/local/bin) + configure_etcd on first ≤3 DB nodes; `/etc/default/etcd` + `systemd-run etcd`; client 2379 / peer 2380; wait `etcdctl endpoint health`.
- **+Patroni:** install_patroni (`pip3 install patroni[etcd3] psycopg2-binary`) + configure_patroni serial; writes `/etc/patroni/patroni.yml` (scope `stroppy-pg`, restapi :8008, etcd3 hosts, bootstrap.dcs params, sync if SyncReplicas>0); stop PG, wipe data, `systemd-run --uid=postgres patroni`; wait `/health` :8008. **configure_db becomes noop.**
- **+PgBouncer:** install (`apt-get install pgbouncer`) + configure; `/etc/pgbouncer/pgbouncer.ini` (:6432→127.0.0.1:5432, pool_mode transaction, auth trust) + userlist; `su pgbouncer -c "pgbouncer -d"`. Stroppy waits.
- **+HAProxy:** `apt-get install haproxy`; `configHAProxyPostgres` writes `/etc/haproxy/haproxy.cfg` — write :5000 / read :5001 → backends :5432. Health: Patroni→httpchk `/primary` & `/replica` on :8008, else tcp-check. `haproxy -f ... -D`. Stroppy waits.

### D. Stroppy connect
`postgresql://postgres@<host>:<port>/postgres?sslmode=disable`, driver `postgres`. host/port =
DBEndpoint (HAProxy node :5000 write if present, else master :5432). PgBouncer :6432 only via explicit route.

---

## 2. MySQL / MariaDB (shared `MySQLTopology`)

### A. Topology
| Field | Type | Default(single/replica/group) | Controls |
|---|---|---|---|
| Primary | MachineSpec | 2c/4G/50 · 4c/8G/100 · 8c/16G/200 | primary node |
| Replicas | []MachineSpec | – / 2×… / 2×… | replicas (Replicas[0] = spec for all) |
| ProxySQL | *MachineSpec | nil / 1×… / 2×… | dedicated proxy (nil=none) |
| GroupRepl | bool | f/f/**t** | InnoDB Group Replication |
| SemiSync | bool | f/**t**/f | semi-sync (when no GR); GR XOR SemiSync |
| PrimaryOptions/ReplicaOptions | map | innodb_buffer_pool 25%, max_conn 200/500 | my.cnf (server-id silently dropped) |
| ProxySQLOptions | map | threads 4, max_conn 2048 | **stored, NOT forwarded** (renderer hardcodes) |

### B. Default my.cnf (`MySQLDefaults`)
innodb_buffer_pool_size `25%`, max_connections `200`, innodb_log_file_size `1G`,
innodb_flush_method `O_DIRECT`, innodb_flush_log_at_trx_commit `1`, innodb_io_capacity `2000`/max `4000`,
read/write_io_threads `8`, table_open_cache `4000`, thread_cache_size `64`, bind_address `0.0.0.0`.
**v8.4:** + `default_authentication_plugin=caching_sha2_password`.
**Always injected (locked):** `server-id=__node+1__` (hard-locked), `gtid_mode=ON`, `enforce_gtid_consistency=ON`, `log_bin=mysql-bin`.
SemiSync → `plugin-load-add=semisync_*.so` + enable. GroupRepl → `binlog_checksum=NONE`, `plugin-load-add=group_replication.so`, `report_host=__local__`.
**Packages:** mysql8.0/8.4 `mysql-server-X`+client (repo.mysql.com, key 2023, component mysql-8.0/8.4-lts); mariadb10.11/11.4 `mariadb-server`+client (`mariadb_repo_setup --version=X`).

### C. Deployment
`install_db → configure_db → [proxy] → monitor → run_stroppy`. MySQL ≡ MariaDB logic (same `configMySQL`, different package).
- config_db serial, node0=primary: write `/etc/mysql/my.cnf` (placeholders `__SERVER_ID__`=idx+1, `__LOCAL_HOST__`=IP), `systemctl restart mysql`, `mysqladmin ping`. Root any-host. Primary creates `repl`/`monitor` users.
- **GroupRepl:** all GR setup via `SET GLOBAL group_replication_*` after start (group_name UUID `aaaa...`, local_address `:33061`, seeds all-nodes:33061). node0 `bootstrap_group=ON; START`; replicas wait+`START GROUP_REPLICATION`.
- **SemiSync/async replica:** `CHANGE REPLICATION SOURCE TO ... SOURCE_AUTO_POSITION=1; START REPLICA`.
- **ProxySQL:** download `proxysql_2.7.3-ubuntu22_amd64.deb`→dpkg; `/etc/proxysql.cnf` (listen :6033, admin :6032, writer hg 10 / reader hg 20, monitor user); `proxysql --initial -f -D ...`.

### D. Stroppy connect
`root@tcp(<host>:<port>)/`, driver `mysql`. With ProxySQL → proxy :6033; else primary :3306.

---

## 3. Picodata

### A. Topology (`PicodataTopology`)
| Field | Type | Default(single/cluster/scale) | Controls |
|---|---|---|---|
| Instances | []MachineSpec | 1×2c/4G/50 · 3×4c/8G/100 · 6×8c/16G/200 | instance groups (flattened to dbTargets) |
| HAProxy | *MachineSpec | nil / 1× / 2× | pgproto LB |
| Replication | int | 1/2/3 | `cluster.tier.default.replication_factor` |
| Shards | int | 1/3/6 | **stored, NOT emitted to YAML** (effective-config only) |
| Tiers | []PicodataTier{name,rf,can_vote,count} | – / – / compute+storage | **stored, NOT wired to renderer** (always single `default` tier) |
| InstanceOptions | map | memtx/vinyl/log_level | picodata.yaml overrides |

### B. Config (`PicodataDefaults` v25.3, version-invariant)
memtx_memory `2048MB`, vinyl_memory `25%`, net_msg_max `1024`, readahead `16384`, log_level `info`, listen `0.0.0.0:3301`.
Rendered `/etc/picodata/picodata.yaml`: cluster.name `stroppy-cluster` (locked), tier default rf=Replication can_vote=true,
iproto :3301, pg :5432 ssl=false, http :8081, memtx.memory. **Locked structural:** cluster/tier name, ports, ssl.
**Package:** `picodata` (download.picodata.io repo).

### C. Deployment
`install_db → configure_db → [proxy] → monitor → run_stroppy`. No etcd/patroni/pgbouncer.
- config_db serial per node: write yaml (`__ADVERTISE_HOST__` subst), `systemd-run --setenv PICODATA_ADMIN_PASSWORD=T0psecret picodata run --config ... --peer <peer0>:3301 --instance-name instance-N`; wait `http://localhost:8081/api/v1/health/ready`.
- **HAProxy:** `configHAProxyPicodata` write :4327 / read :4328 → backends `<host>:4327` (⚠️ but yaml iproto=3301/pg=5432 — port mismatch in code), tcp-check.
- **Picodata NOT redirected to proxy** — stroppy always hits node0 directly (driver does topology discovery).

### D. Stroppy connect
`postgres://admin:T0psecret@<host>:5432?sslmode=disable`, driver `picodata`, always node0 :5432 (no proxy redirect).
Ports: 3301 iproto/peer · 5432 pgwire · 8081 health/metrics.

---

## 4. YDB (self-hosted)

### A. Topology (`YDBTopology`)
| Field | Type | Default | Controls |
|---|---|---|---|
| Storage | MachineSpec | required | static storage nodes |
| Database | *MachineSpec | nil=**combined** | dynamic/compute nodes (nil→ydbd-database colocated on storage) |
| HAProxy | *MachineSpec | nil | LB |
| FaultTolerance | string | none | erasure: none / block-4-2 / mirror-3-dc |
| FailureDomainType | string | "" | "" / disk |
| DefaultDiskType | string | ""→ssd | pool kind for `database create` |
| StorageGroups | int | 0→1 | `create <pool>:<groups>` |
| AutoSizePdisks | bool | false | dry-run resizes secondary disks |
| DatabasePath | string | /Root/testdb | `database create`, `--tenant` |
| StorageOptions/DatabaseOptions | map | | config render |

**Combined** (Database==nil): both ydbd-storage+ydbd-database on storage VMs; **Storage.MemoryMB halved** (each daemon hard_limit=mem×0.85, prevents OOM). **Split:** separate compute VMs, Database.MemoryMB unhalved.
**pdisk auto-size:** `CalculateYDBStoragePdiskGB` = ceil(rawGB/93)×93×2; rawGB: tpcc 100MB/wh, tpcb 15MB/unit.
**Placement zones:** YC round-robin `[region-a,-b,-d]`; instance i → zones[i%len]; single→zones[0].
**Package:** none — ydbd tarball from `binaries.ydb.tech`.

### B. Deployment
`install_db → configure_db → init_ydb → start_ydb_db → monitor → run_stroppy`.
- **install:** download ydbd tarball → `/opt/ydb`; user `ydb`; dirs `/opt/ydb/cfg /ydb_data`.
- **configure (storage, parallel):** render `/opt/ydb/cfg/config.yaml` (hosts+zones, BlockDevicePaths `/dev/disk/by-id/virtio-<dev>` or file `pdisk.data` size=max(DiskGB-2,10)), `ydbd admin bs disk obliterate`, `hostnamectl set-hostname <ip>`, `systemd-run ydbd-storage server --grpc-port 2136 --ic-port 19001 --mon-port 8765 [--pgwire-port N] --node static`; wait :2136.
- **init (node0):** `ydbd -s grpc://h:2136 admin blobstorage config init --yaml-file` (retry 30); `ydbd ... admin database /Root/testdb create <pool>:<groups>` (retry 15).
- **start_db (split=compute / combined=storage):** render `database.yaml` (`__YDB_HOST_i__` storage IPs), `systemd-run ydbd-database server --ic-port 19002 --mon-port 8766 --tenant /Root/testdb --node-broker grpc://<storage>:2136 ...`; wait :2136.

### C. Stroppy connect
ydb-grpc `grpc://<host>:2136/Root/testdb` driver ydb · ydb-pgwire `postgresql://<host>:5432/local?sslmode=disable` driver postgres (`--pgwire-port 5432` added to both daemons when selected).
HAProxy `configHAProxyYDB` write :2136 / read :2137 → backends :2136, tcp.

---

## 5. YDB Managed (Yandex Cloud)

### A. Topology (`YDBManagedTopology`)
| Field | Type | Default | Controls |
|---|---|---|---|
| Type | serverless/dedicated | required | terraform resource flavor |
| ComputeType | oltp/olap | oltp | UI preset filter only (NOT to terraform) |
| ResourcePresetID | string | medium | dedicated hw class |
| NodeCount | int | 1 | dedicated fixed_scale.size |
| AutoScale {min,max,cpu%} | * | nil | switches to auto_scale (cpu% default 70); label enable_autoscaling=1 |
| StorageGroups | int | 1 | dedicated group_count |
| StorageType | string | ssd | dedicated storage_type_id |
| ThrottlingRCUs | int | 0 | serverless throttling (omitted when 0) |
| Client | MachineSpec | 8c/16G/50 stroppy | runner VM (SA ydb.editor attached) |
| DatabasePath/Endpoint/TerraformOutput | – | runtime | filled from TF output |

**Resource presets:** small-m8 4/8, small 4/16, medium 8/32, medium-m64 8/64, medium-m96 8/96, large 12/48, xlarge 16/64, oltp-c16-m128 16/128, olap-medium 8/32, olap-large 12/48 (cores/GB).

### B. Deployment — NO install on VMs (install/config = noop)
Terraform module `deployments/terraform/yandex_managed_ydb` (env `YC_TOKEN/CLOUD_ID/FOLDER_ID/ZONE`):
- network.tf: 3 subnets (a/b/d, /20 from /16) + SG; iam.tf: SA `<db>-sa` + `ydb.editor` folder binding.
- serverless: `yandex_ydb_database_serverless` (+throttling block if RCU>0).
- dedicated: `yandex_ydb_database_dedicated` (resource_preset, fixed_scale.size OR auto_scale, storage_config group_count+type).
- vm.tf: client VM(s) with SA attached (driver pulls token+CA from metadata).
- location_id = zone with suffix trimmed (`ru-central1-b`→`ru-central1`); db name `stroppy-<runID>` ≤63.
Outputs: vm_ips, ydb_endpoint, ydb_database_path, ydb_managed snapshot. Endpoint parse default port 2135.

### C. Stroppy connect
`grpcs://<host>:<port>/?database=<DatabasePath>`, driver ydb, no creds (SA metadata fallback).

---

## 6. CockroachDB

### A. Topology (`CockroachTopology`)
| Field | Type | Default(single/3/6) | Controls |
|---|---|---|---|
| Nodes | MachineSpec | 1×2c/4G/50 · 3×4c/8G/100 · 6×8c/16G/200 | homogeneous N nodes (no master/replica) |
| Options | map | empty | post-init `SET CLUSTER SETTING k='v'` |

No defaults map. **Package:** none — tarball from `binaries.cockroachdb.com` (24.2→24.2.4 etc.).

### B. Deployment
`install_db → configure_db → init_cockroach → run_stroppy`.
- install: tarball → `/opt/cockroach` → link `/usr/local/bin/cockroach`; user+`/var/lib/cockroach`.
- configure (parallel): `systemd-run cockroach start --insecure --advertise-addr=<ih>:26257 --listen-addr=0.0.0.0:26257 --http-addr=0.0.0.0:8080 --store=/var/lib/cockroach --cache=<mem/4>MiB --max-sql-memory=<mem/4>MiB [--join=<peers>] --background`; peers = all DBTargets InternalHost (own stripped); wait :26257.
- init (node0): `cockroach init --insecure --host=<h>:26257` (idempotent on "already initialized"); then `SET CLUSTER SETTING` per Options.
All `--insecure` (no TLS/auth).

### C. Stroppy connect
`postgresql://<InternalHost>:26257/defaultdb?sslmode=disable`, driver `postgres`. No proxy. Admin UI :8080 (unrouted).

---

## 7. Cross-DB comparison

### 7.1 Install method
| DB | source | mechanism |
|---|---|---|
| postgres | apt (PGDG) | postgresql-N |
| mysql | apt (repo.mysql.com) | mysql-server-X |
| mariadb | apt (mariadb_repo_setup) | mariadb-server |
| picodata | apt (download.picodata.io) | picodata |
| ydb | tarball binaries.ydb.tech | /opt/ydb |
| cockroach | tarball binaries.cockroachdb.com | /opt/cockroach |
| ydb-managed | terraform | YC managed (no node install) |

### 7.2 HA / clustering
| DB | mechanism | proxy | proxy ports |
|---|---|---|---|
| postgres | Patroni+etcd (or streaming) | HAProxy | w5000/r5001 → 5432, hc 8008 |
| mysql/mariadb | GroupRepl XOR SemiSync | ProxySQL | 6033 (admin 6032), hg 10/20 |
| picodata | raft (rf×shards) | HAProxy (not used by stroppy) | 4327/4328 |
| ydb | erasure (none/block-4-2/mirror-3-dc) | HAProxy | w2136/r2137 |
| cockroach | gossip --join, init once | none | — |
| ydb-managed | YC managed | — | — |

### 7.3 Daemon launch & ports
| DB | unit | ports |
|---|---|---|
| postgres | systemctl postgresql / systemd-run patroni | 5432, patroni 8008, etcd 2379/2380, pgbouncer 6432 |
| mysql | systemctl mysql | 3306, GR 33061 |
| picodata | systemd-run picodata | 3301 iproto, 5432 pg, 8081 http |
| ydb | systemd-run ydbd-storage/-database | 2136 grpc, 19001/19002 ic, 8765/8766 mon, 5432 pgwire |
| cockroach | systemd-run cockroach | 26257 sql, 8080 ui |

### 7.4 Options forwarding gaps (declared but NOT wired)
- postgres: `PatroniOptions`, `EtcdOptions`, `HAProxyOptions` — template hardcodes
- mysql: `ProxySQLOptions` — renderer hardcodes
- picodata: `Shards` (not in yaml), `Tiers` (always single default tier), `HAProxyOptions`
- mysql locked: `server-id` (hard); others injected-then-overridable

---

*Generated from `main` @ `cf4a5c8`. `internal/domain/{types,run,agent}`. Line numbers/values approximate — re-verify against source. Companion: `run-build-rules-map.md`.*
