# OrioleDB scale (replicas + HAProxy) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a streaming-replicated OrioleDB topology — 1 master + N read-replica containers, optionally fronted by an HAProxy that splits writes→primary / reads→replicas via a non-Patroni `pg_is_in_recovery()` health check.

**Architecture:** Extend the existing single-container OrioleDB renderer to a role-dispatched one (master / replica / haproxy). Replicas are PostgreSQL streaming standbys, each its own container bootstrapped via `pg_basebackup`. HAProxy installs natively (apt) like the PostgreSQL engine; a tiny per-DB-node health endpoint on :8008 reports primary/replica so HAProxy `httpchk` routes correctly. No Patroni/etcd/failover (phase 2).

**Tech Stack:** Go (protogen), TypeScript/React, systemd, Docker, PostgreSQL streaming replication, HAProxy, socat. Codegen via `./bin` (easyp).

## Global Constraints

- New proto fields on `OrioledbParams`: `replicas=5` (uint32), `haproxy=6` (uint32), `replica_options=7` (map<string,string>), `haproxy_options=8` (map<string,string>). Keep existing image=1/postgres_options=2/initdb_locale=3/shared_buffers_mb=4.
- NO Patroni / etcd / pgbouncer / sync_replicas (phase 2).
- Roles + priorities: master(global 20,node 20), replica(30,20), haproxy(60,50). Role strings: `"master"`, `"replica"`, `"haproxy"`. haproxy node count capped at 1 in v1.
- Ports: pg 5432; haproxy write 5432, read 5433; per-node health 8008.
- Container flags unchanged from single-node: `--network host --pid host --ipc host --privileged`, PGDATA bind-mount, trust auth (`POSTGRES_HOST_AUTH_METHOD=trust`), `--rm`, `Restart=always`.
- HAProxy `httpchk` contract: `GET /primary`→200 iff `pg_is_in_recovery()`=false; `GET /replica`→200 iff true. Forward-compatible with Patroni REST (phase 2).
- Commits: Conventional Commits, NO `Co-Authored-By`. Branch `ref`.
- Working tree discipline for subagents: work in the main checkout, do NOT create a git worktree, commit on `ref`.
- Go build: `go build ./...` (rtk mangles stderr; full errors in newest `~/.local/share/rtk/tee/*_go_build.log`). Regen cycle (Task 1 only) runs the `./bin` easyp toolchain.

---

## File map

Modify:
- `protocols/cloud/v1/domain/database.proto` — OrioledbParams fields.
- `internal/domain/database/orioledb/orioledb.go` — multi-role topology builder + role/port constants.
- `internal/domain/database/orioledb/deployment.go` — role dispatch: master streaming flags, replica standby unit + wiring, haproxy role, per-node health endpoint.
- `internal/domain/deployment/monitor.go` — `isDatabaseRole` orioledb replica.
- `internal/app/presets_seed.go` — optional "OrioleDB HA" builtin preset.
- `web/src/components/database/DatabaseParamsForm.tsx`, `web/src/services/domainMappers.ts`, `web/src/services/wizard.ts` — FE params + mappers.

Tests:
- `internal/domain/database/orioledb/orioledb_test.go` (topology)
- `internal/domain/database/orioledb/deployment_test.go` (units, wiring)

Regenerated (do not hand-edit): `internal/proto/**`, `web/src/lib/proto/**`, openapi/docs.

---

## Task 1: Proto — OrioledbParams scale fields + regen

**Files:**
- Modify: `protocols/cloud/v1/domain/database.proto`
- Regenerated: `internal/proto/cloud/v1/domain/database.pb.go`, `web/src/lib/proto/...`

**Interfaces:**
- Produces: `OrioledbParams.GetReplicas() uint32`, `GetHaproxy() uint32`, `GetReplicaOptions() map[string]string`, `GetHaproxyOptions() map[string]string`.

- [ ] **Step 1: Add the fields.** In `protocols/cloud/v1/domain/database.proto`, message `OrioledbParams`, after `uint32 shared_buffers_mb = 4;`:

```proto
    // replicas is the streaming-replica container count (0 = no replicas).
    uint32 replicas = 5;
    // haproxy is the dedicated HAProxy LB node count (0 = none; capped at 1).
    uint32 haproxy = 6;
    // replica_options is postgresql.conf applied to replica containers.
    map<string, string> replica_options = 7;
    // haproxy_options tunes haproxy.cfg.
    map<string, string> haproxy_options = 8;
```

- [ ] **Step 2: Regenerate Go + TS.**

Run:
```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/protocols && export PATH="$PWD/../bin:$PWD/../web/node_modules/.bin:$PATH" && \
  ../bin/easyp -cfg easyp.go.yaml generate && \
  ../bin/easyp -cfg easyp.api.go.yaml generate && \
  rm -rf ../web/src/lib/proto && ../bin/easyp -cfg easyp.ts.yaml generate
```
Expected: `code generation completed` ×3.

- [ ] **Step 3: Verify + build.**

Run: `cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && grep -c "GetReplicas\|GetHaproxy\|GetReplicaOptions\|GetHaproxyOptions" internal/proto/cloud/v1/domain/database.pb.go`
Expected: ≥4.
Run: `go build ./... 2>&1 | tail -2`
Expected: `Go build: Success`.

- [ ] **Step 4: Commit.**

```bash
git add protocols/cloud/v1/domain/database.proto internal/proto web/src/lib/proto internal/openapidoc docs/proto
git commit -m "feat(proto): OrioledbParams replicas/haproxy/replica_options/haproxy_options"
```

---

## Task 2: Topology builder — master + replicas + haproxy

**Files:**
- Modify: `internal/domain/database/orioledb/orioledb.go`
- Test: `internal/domain/database/orioledb/orioledb_test.go`

**Interfaces:**
- Consumes: `OrioledbParams.GetReplicas/GetHaproxy` (Task 1); `dbspec.Node/Component/Connection/Tags`.
- Produces: constants `orioledbRoleReplica="replica"`, `orioledbRoleHaproxy="haproxy"`, `haproxyWritePort=5432`, `haproxyReadPort=5433`, `healthPort=8008`; multi-node `BuildTopologySpec`.

- [ ] **Step 1: Write the failing test.** Replace/extend `TestBuildTopologySpecSingleNode` and add a scaled case in `orioledb_test.go`:

```go
func TestBuildTopologySpecScaled(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&domain.OrioledbParams{Replicas: 2, Haproxy: 1})
	if err != nil {
		t.Fatalf("BuildTopologySpec: %v", err)
	}
	roles := map[string]int{}
	for _, c := range spec.GetComponents() {
		roles[c.GetRole()]++
	}
	if roles["master"] != 1 || roles["replica"] != 2 || roles["haproxy"] != 1 {
		t.Fatalf("want 1 master/2 replica/1 haproxy, got %v", roles)
	}
	if len(spec.GetNodes()) != 4 {
		t.Fatalf("want 4 nodes, got %d", len(spec.GetNodes()))
	}
	// every replica connects to the master.
	reps := 0
	for _, cn := range spec.GetConnections() {
		if cn.GetToComponentId() == "orioledb-master-1" {
			reps++
		}
	}
	if reps < 2 {
		t.Fatalf("want >=2 connections to master, got %d", reps)
	}
}
```
Add `import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"` if missing.

- [ ] **Step 2: Run, verify fail.**

Run: `cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/database/orioledb/ -run TestBuildTopologySpecScaled 2>&1 | tail -5`
Expected: FAIL (only 1 component built today).

- [ ] **Step 3: Rewrite the builder.** Replace `orioledb.go` constants block + `BuildTopologySpec`:

```go
const (
	orioledbEngine      = "orioledb"
	orioledbRoleMaster  = "master"
	orioledbRoleReplica = "replica"
	orioledbRoleHaproxy = "haproxy"

	// pgPort is the PostgreSQL wire port exposed by the OrioleDB container.
	pgPort = 5432
	// haproxyWritePort routes to the primary; haproxyReadPort to the replicas.
	haproxyWritePort = 5432
	haproxyReadPort  = 5433
	// healthPort serves the per-node primary/replica check HAProxy probes.
	healthPort = 8008

	// defaultImage is used when OrioledbParams.image is empty.
	defaultImage = "orioledb/orioledb:latest-pg17"

	masterID  = "orioledb-master-1"
	haproxyID = "orioledb-haproxy-1"
)

// BuildTopologySpec builds a master + N streaming replicas, optionally fronted
// by one HAProxy. OrioleDB has no upstream auto-failover yet (Patroni is phase
// 2); replicas are read-scale streaming standbys.
func (d *Database) BuildTopologySpec(input *domain.OrioledbParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	replicas := input.GetReplicas()
	withHAProxy := input.GetHaproxy() > 0

	nodes := []*topology.Node{dbspec.Node(orioledbEngine, masterID, orioledbRoleMaster, 1, []string{masterID})}
	comps := []*topology.Component{dbspec.Component(orioledbEngine, masterID, topology.Component_KIND_DATABASE, orioledbRoleMaster, masterID)}
	var conns []*topology.Connection

	for i := uint32(1); i <= replicas; i++ {
		id := fmt.Sprintf("orioledb-replica-%d", i)
		nodes = append(nodes, dbspec.Node(orioledbEngine, id, orioledbRoleReplica, 1+i, []string{id}))
		comps = append(comps, dbspec.Component(orioledbEngine, id, topology.Component_KIND_DATABASE, orioledbRoleReplica, id))
		// replica streams FROM the master.
		conns = append(conns, dbspec.Connection(orioledbEngine, id, masterID,
			topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_ASYNC,
			"rep", pgPort, false))
	}

	if withHAProxy {
		nodes = append(nodes, dbspec.Node(orioledbEngine, haproxyID, orioledbRoleHaproxy, 100, []string{haproxyID}))
		comps = append(comps, dbspec.Component(orioledbEngine, haproxyID, topology.Component_KIND_DATABASE, orioledbRoleHaproxy, haproxyID))
		// haproxy fronts the master + every replica.
		conns = append(conns, dbspec.Connection(orioledbEngine, haproxyID, masterID,
			topology.Connection_KIND_FLOW, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_SYNC,
			"lb", pgPort, false))
		for i := uint32(1); i <= replicas; i++ {
			conns = append(conns, dbspec.Connection(orioledbEngine, haproxyID, fmt.Sprintf("orioledb-replica-%d", i),
				topology.Connection_KIND_FLOW, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_SYNC,
				"lb", pgPort, false))
		}
	}

	return &topology.TopologySpec{
		Nodes:       nodes,
		Components:  comps,
		Connections: conns,
		Labels: map[string]string{
			"kind":   "database",
			"engine": orioledbEngine,
			"nodes":  strconv.FormatInt(int64(len(nodes)), 10),
		},
		Tags: dbspec.Tags(orioledbEngine),
	}, nil
}
```
Add imports `"fmt"` and `"strconv"` to `orioledb.go`.

- [ ] **Step 4: Run tests.**

Run: `go test ./internal/domain/database/orioledb/ 2>&1 | tail -3`
Expected: PASS (scaled + existing single-node, which now has replicas=0/haproxy=0 → 1 node).

- [ ] **Step 5: Commit.**

```bash
git add internal/domain/database/orioledb/orioledb.go internal/domain/database/orioledb/orioledb_test.go
git commit -m "feat(orioledb): master+replicas+haproxy topology"
```

---

## Task 3: Master streaming flags + role dispatch scaffold

**Files:**
- Modify: `internal/domain/database/orioledb/deployment.go`
- Test: `internal/domain/database/orioledb/deployment_test.go`

**Interfaces:**
- Consumes: role constants (Task 2), `OrioledbParams.GetReplicaOptions`.
- Produces: `orioledbEngineComponent` dispatches on role; master unit carries streaming flags. Helper `streamingMasterOptions(map) map` merging `wal_level=replica,max_wal_senders=10,max_replication_slots=10,hot_standby=on`.

- [ ] **Step 1: Write the failing test.** Append to `deployment_test.go`:

```go
func TestMasterUnitHasStreamingFlags(t *testing.T) {
	opts := streamingMasterOptions(map[string]string{})
	for _, k := range []string{"wal_level", "max_wal_senders", "max_replication_slots", "hot_standby"} {
		if _, ok := opts[k]; !ok {
			t.Fatalf("master options missing %q: %v", k, opts)
		}
	}
	if opts["wal_level"] != "replica" {
		t.Fatalf("wal_level = %q, want replica", opts["wal_level"])
	}
}
```

- [ ] **Step 2: Run, verify fail.**

Run: `go test ./internal/domain/database/orioledb/ -run TestMasterUnitHasStreamingFlags 2>&1 | tail -5`
Expected: FAIL — `streamingMasterOptions` undefined.

- [ ] **Step 3: Implement.** In `deployment.go`:

(a) Add the helper:
```go
// streamingMasterOptions overlays the streaming-master postgresql.conf knobs on
// top of the user's postgres_options (user values win). Enables WAL streaming so
// replicas can attach. Trust auth already permits replication connections.
func streamingMasterOptions(base map[string]string) map[string]string {
	out := map[string]string{
		"wal_level":             "replica",
		"max_wal_senders":       "10",
		"max_replication_slots": "10",
		"hot_standby":           "on",
	}
	for k, v := range base {
		out[k] = v
	}
	return out
}
```

(b) Split `orioledbEngineComponent` to dispatch on role. Replace its body's role guard + the master branch:
```go
func orioledbEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	dbPackage *domain.Package,
	dependencies []string,
	wiring orioledbWiring,
) (deploymentbuilder.EngineComponent, error) {
	switch component.GetRole() {
	case orioledbRoleMaster:
		return orioledbDBComponent(component, database, dbPackage, dependencies, wiring, true)
	case orioledbRoleReplica:
		return orioledbDBComponent(component, database, dbPackage, dependencies, wiring, false)
	case orioledbRoleHaproxy:
		return orioledbHAProxyComponent(component, database, dependencies, wiring)
	default:
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported orioledb component role %q", component.GetRole())
	}
}
```

(c) Rename the existing master-building body into `orioledbDBComponent(component, database, dbPackage, dependencies, wiring, isMaster bool)`. For the master use `streamingMasterOptions(pgOptions(params))`; the replica/haproxy paths come in Tasks 4-5 (for now `orioledbDBComponent` may handle only master and `orioledbHAProxyComponent` can be a stub returning an error — but DO NOT commit a stub; this task only adds the master path + the dispatch shape, with replica handled in the same `orioledbDBComponent` via Task 4). To keep this task self-contained, in this task `orioledbDBComponent` builds the master unit (isMaster true) using `streamingMasterOptions`, and for isMaster=false temporarily returns `fmt.Errorf("replica rendering added in the next task")`. `orioledbHAProxyComponent` likewise returns an explicit "added in a later task" error.

> NOTE: `orioledbWiring` is introduced in Task 4. To avoid a forward reference, in THIS task add a minimal `type orioledbWiring struct{ masterHost string }` and thread it through (RenderComponent/RenderPreview pass an empty `orioledbWiring{}` for now; Task 4 fills resolution). The master path does not use wiring, so an empty struct is fine.

(d) Update `RenderComponent`/`RenderPreview` to build wiring (empty for now) and pass it:
```go
func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil), orioledbWiring{})
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentDeployment(ctx, ec), nil
}
```
(same for RenderPreview, building deps as today and passing `orioledbWiring{}`).

(e) In the master unit options, change `options := pgOptions(params)` to `options := streamingMasterOptions(pgOptions(params))`.

- [ ] **Step 4: Run tests + build.**

Run: `go test ./internal/domain/database/orioledb/ 2>&1 | tail -3 && go build ./... 2>&1 | tail -2`
Expected: orioledb tests PASS (single-node master still works: replicas=0 → only master component), build Success.

- [ ] **Step 5: Commit.**

```bash
git add internal/domain/database/orioledb/deployment.go internal/domain/database/orioledb/deployment_test.go
git commit -m "feat(orioledb): role dispatch + streaming master flags"
```

---

## Task 4: Replica standby renderer + master-endpoint wiring

**Files:**
- Modify: `internal/domain/database/orioledb/deployment.go`
- Test: `internal/domain/database/orioledb/deployment_test.go`

**Interfaces:**
- Consumes: cockroach-style wiring helpers `deploymentbuilder.OwnPrivateEndpoint(ctx)`, `deploymentbuilder.ComponentTargets(ctx, pred, name, port)`, `deploymentbuilder.AddressPort(addr, port)` (see `internal/domain/database/cockroach/deployment.go` for exact usage).
- Produces: `orioledbResolveWiring(ctx) (orioledbWiring, error)` returning `masterHost`; replica unit via `orioledbReplicaUnit(componentID, image, masterHost string, options map[string]string)`.

- [ ] **Step 1: Write the failing test.** Append to `deployment_test.go`:

```go
func TestReplicaUnitBasebackupStandby(t *testing.T) {
	unit := orioledbReplicaUnit("orioledb-replica-1", "orioledb/orioledb:latest-pg17", "10.0.0.5", map[string]string{"hot_standby": "on"})
	for _, want := range []string{
		"docker run",
		"--network host",
		"pg_basebackup",
		"-h 10.0.0.5",
		"standby", // -R writes standby.signal; comment or flag mentions standby
		"docker-entrypoint.sh postgres",
		"orioledb/orioledb:latest-pg17",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("replica unit missing %q:\n%s", want, unit)
		}
	}
}
```

- [ ] **Step 2: Run, verify fail.**

Run: `go test ./internal/domain/database/orioledb/ -run TestReplicaUnitBasebackupStandby 2>&1 | tail -5`
Expected: FAIL — `orioledbReplicaUnit` undefined.

- [ ] **Step 3: Implement wiring + replica unit + replica branch.**

(a) Wiring (mirror `cockroachResolveWiring`; read cockroach/deployment.go for the helper signatures and copy the call shapes):
```go
type orioledbWiring struct {
	masterHost string // private host of the master, "" in preview
}

func orioledbResolveWiring(ctx deploymentbuilder.RenderContext) (orioledbWiring, error) {
	masters, err := deploymentbuilder.ComponentTargets(ctx, func(c *topologypb.Component) bool {
		return c.GetEngine() == orioledbEngine && c.GetRole() == orioledbRoleMaster
	}, "rep", pgPort)
	if err != nil {
		return orioledbWiring{}, err
	}
	if len(masters) == 0 {
		return orioledbWiring{}, nil // preview: no resolved peers
	}
	return orioledbWiring{masterHost: masters[0].Address}, nil
}
```
(If `ComponentTargets`/the target struct's `.Address` field name differs, match cockroach's usage — it uses `node.AddressPort()` / `node.Address`. Use whatever cockroach uses.)

(b) Replica unit — overrides the entrypoint to basebackup-then-start:
```go
func orioledbReplicaUnit(componentID, image, masterHost string, options map[string]string) string {
	var optStr strings.Builder
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&optStr, " -c %s=%s", k, options[k])
	}
	dataDir := deploymentbuilder.DataDir(componentID)
	configDir := deploymentbuilder.ConfigDir(componentID)
	// The container command: basebackup into an empty PGDATA (idempotent), then
	// exec the image's normal postgres entrypoint as a streaming standby (-R wrote
	// standby.signal + primary_conninfo). PGDATA defaults to
	// /var/lib/postgresql/data in the official image.
	inner := fmt.Sprintf(`set -e
if [ ! -s "$PGDATA/PG_VERSION" ]; then
  pg_basebackup -h %s -p %d -U postgres -D "$PGDATA" -R -X stream -P
fi
exec docker-entrypoint.sh postgres%s`, masterHost, pgPort, optStr.String())
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud OrioleDB replica %s
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
EnvironmentFile=%s/orioledb.env
ExecStartPre=-/usr/bin/docker rm -f %s
ExecStartPre=/usr/bin/docker pull %s
ExecStartPre=/bin/mkdir -p %s
ExecStart=/usr/bin/docker run --rm --name %s --network host --pid host --ipc host --privileged \
  -e POSTGRES_HOST_AUTH_METHOD \
  -v %s:/var/lib/postgresql/data \
  --entrypoint /bin/bash %s -c %s
ExecStop=/usr/bin/docker rm -f %s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`,
		componentID, configDir, containerName, image,
		deploymentbuilder.ShellQuote(dataDir), containerName,
		deploymentbuilder.ShellQuote(dataDir), image,
		deploymentbuilder.ShellQuote(inner), containerName,
	)
}
```

(c) In `orioledbDBComponent`, branch on `isMaster`: master uses `orioledbServiceUnit(...)` with `streamingMasterOptions(pgOptions(params))`; replica uses `orioledbReplicaUnit(component.GetId(), image, wiring.masterHost, mergedReplicaOptions)` where `mergedReplicaOptions = mergeMaps(pgOptions(params), params.GetReplicaOptions())`. Add a small `mergeMaps(a,b)` helper (b wins). Replica healthcheck = same retry pg_isready as master.

(d) Wire `orioledbResolveWiring` into `RenderComponent` (replace the empty `orioledbWiring{}`):
```go
wiring, err := orioledbResolveWiring(ctx)
if err != nil {
	return nil, err
}
ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil), wiring)
```
`RenderPreview` keeps `orioledbWiring{}` (no resolved peers in preview).

- [ ] **Step 4: Run tests + build.**

Run: `go test ./internal/domain/database/orioledb/ 2>&1 | tail -3 && go build ./... 2>&1 | tail -2`
Expected: PASS + Success.

- [ ] **Step 5: Commit.**

```bash
git add internal/domain/database/orioledb/deployment.go internal/domain/database/orioledb/deployment_test.go
git commit -m "feat(orioledb): streaming replica standby renderer + master wiring"
```

---

## Task 5: HAProxy role + per-DB-node primary-check health endpoint

**Files:**
- Modify: `internal/domain/database/orioledb/deployment.go`
- Test: `internal/domain/database/orioledb/deployment_test.go`

**Interfaces:**
- Consumes: wiring (Task 4) extended to carry all DB node hosts; `OrioledbParams.GetHaproxyOptions`.
- Produces: `orioledbHAProxyComponent(...)` (native apt haproxy + cfg); `healthEndpointFiles()`/install steps added to every DB node (master + replica).

- [ ] **Step 1: Write the failing tests.** Append to `deployment_test.go`:

```go
func TestHAProxyConfigSplitsWriteRead(t *testing.T) {
	cfg := orioledbHAProxyConfig([]string{"10.0.0.5", "10.0.0.6"}, map[string]string{})
	for _, want := range []string{
		"bind *:5432", // write frontend
		"bind *:5433", // read frontend
		"option httpchk GET /primary",
		"option httpchk GET /replica",
		"10.0.0.5:5432",
		"10.0.0.6:5432",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("haproxy cfg missing %q:\n%s", want, cfg)
		}
	}
}

func TestHealthScriptUsesRecoveryCheck(t *testing.T) {
	s := orioledbHealthScript()
	for _, want := range []string{"pg_is_in_recovery", "/primary", "/replica", "200", "503"} {
		if !strings.Contains(s, want) {
			t.Fatalf("health script missing %q:\n%s", want, s)
		}
	}
}
```

- [ ] **Step 2: Run, verify fail.**

Run: `go test ./internal/domain/database/orioledb/ -run 'TestHAProxy|TestHealth' 2>&1 | tail -5`
Expected: FAIL — undefined funcs.

- [ ] **Step 3: Implement.**

(a) Extend `orioledbWiring` with `dbHosts []string` (master + replicas private IPs) and resolve them in `orioledbResolveWiring` via a `ComponentTargets` predicate matching role master OR replica. (The haproxy component needs every DB host.)

(b) Health endpoint, added to EVERY DB node (master + replica) — in `orioledbDBComponent`, add an `ExtraFiles` entry writing `/usr/local/bin/orioledb-health.sh` and a `PostStartCommands` step that installs socat + starts a health responder unit. The script:
```go
func orioledbHealthScript() string {
	// Reads the request line on stdin, answers HAProxy's /primary and /replica
	// probes from the local container's recovery state. pg_is_in_recovery()=f =>
	// this node is the primary.
	return `#!/bin/bash
read -r line
path=$(echo "$line" | awk '{print $2}')
rec=$(docker exec ` + containerName + ` psql -U postgres -tAc "SELECT pg_is_in_recovery()" 2>/dev/null | tr -d '[:space:]')
ok="HTTP/1.0 200 OK\r\nContent-Length: 2\r\n\r\nok"
no="HTTP/1.0 503 Service Unavailable\r\nContent-Length: 4\r\n\r\nfail"
case "$path" in
  /primary) [ "$rec" = "f" ] && printf "$ok" || printf "$no" ;;
  /replica) [ "$rec" = "t" ] && printf "$ok" || printf "$no" ;;
  *) printf "$no" ;;
esac
`
}
```
Install + run it via a socat listener service (PostStartCommands):
- write `/usr/local/bin/orioledb-health.sh` (ExtraFile, mode 0755),
- write `/etc/systemd/system/orioledb-health.service` (ExtraFile) with `ExecStart=/usr/bin/socat TCP-LISTEN:8008,reuseaddr,fork SYSTEM:/usr/local/bin/orioledb-health.sh`,
- PostStartCommands: `apt-get install -y socat`, `systemctl daemon-reload`, `systemctl enable --now orioledb-health`.

Use `deploymentbuilder.EngineFile{StepID,StepOrder,ArtifactName,File}` for the two files (orders 060/061), mirroring cockroach's ExtraFiles. socat install is a `PostStartCommands` `EngineCommand` (order 300+).

(c) `orioledbHAProxyConfig(dbHosts []string, opts map[string]string) string` — two frontends:
```
global
  maxconn 4096
defaults
  mode tcp
  timeout connect 5s
  timeout client 1m
  timeout server 1m
  option httpchk
frontend write
  bind *:5432
  default_backend primary
frontend read
  bind *:5433
  default_backend replicas
backend primary
  option httpchk GET /primary
  http-check expect status 200
  <for each host>: server db<i> <host>:5432 check port 8008
backend replicas
  option httpchk GET /replica
  http-check expect status 200
  <for each host>: server db<i> <host>:5432 check port 8008
```
Apply `opts` overrides (timeouts/maxconn) where named. Render `server` lines from `dbHosts`.

(d) `orioledbHAProxyComponent(component, database, dependencies, wiring)`:
```go
ec := deploymentbuilder.EngineComponent{
	Engine:          orioledbEngine,
	GlobalPriority:  60,
	NodePriority:    50,
	Dependencies:    dependencies,
	ConfigArtifactID: deploymentbuilder.ArtifactID(component.GetId(), "haproxy.cfg"),
	ConfigFile:      <common.File at /etc/haproxy/haproxy.cfg with orioledbHAProxyConfig(wiring.dbHosts, params.GetHaproxyOptions())>,
	ConfigOrigin:    deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT,
	InstallCommands: []string{"apt-get update", "DEBIAN_FRONTEND=noninteractive apt-get install -y haproxy"},
	ServiceFile:     <native haproxy systemd unit: ExecStart=/usr/sbin/haproxy -f /etc/haproxy/haproxy.cfg -W>,
	Healthcheck:     "systemctl is-active --quiet " + deploymentbuilder.ServiceName(component.GetId()),
}
```
(HAProxy is native, NOT docker — no container.)

- [ ] **Step 4: Run tests + build.**

Run: `go test ./internal/domain/database/orioledb/ 2>&1 | tail -3 && go build ./... 2>&1 | tail -2`
Expected: PASS + Success.

- [ ] **Step 5: Commit.**

```bash
git add internal/domain/database/orioledb/deployment.go internal/domain/database/orioledb/deployment_test.go
git commit -m "feat(orioledb): HAProxy role + per-node primary-check health endpoint"
```

---

## Task 6: Monitor — scrape replica DB nodes

**Files:**
- Modify: `internal/domain/deployment/monitor.go`

**Interfaces:**
- Consumes: role string `"replica"`.

- [ ] **Step 1: Update `isDatabaseRole`.** In `internal/domain/deployment/monitor.go`, the `case "orioledb":` currently returns `role == "master"`. Change to:
```go
	case "orioledb":
		// OrioleDB master + streaming replicas are both scraped via postgres_exporter.
		return role == "master" || role == "replica"
```

- [ ] **Step 2: Build + test.**

Run: `go build ./... 2>&1 | tail -2 && go test ./internal/domain/deployment/ 2>&1 | tail -3`
Expected: Success + PASS.

- [ ] **Step 3: Commit.**

```bash
git add internal/domain/deployment/monitor.go
git commit -m "feat(orioledb): scrape replica DB nodes (postgres_exporter)"
```

---

## Task 7: Frontend — replicas/haproxy params + topology preview

**Files:**
- Modify: `web/src/components/database/DatabaseParamsForm.tsx`, `web/src/services/domainMappers.ts`, `web/src/services/wizard.ts`

**Interfaces:**
- Consumes: regenerated TS `OrioledbParams` with replicas/haproxy/replicaOptions/haproxyOptions (Task 1).

- [ ] **Step 1: Mappers.** In `web/src/services/domainMappers.ts`, the orioledb VM↔proto converters (`orioledbToVM`/`orioledbToProto`) carry the new fields:
```ts
// toVM: replicas: p.replicas ?? 0, haproxy: p.haproxy ?? 0,
//       replicaOptions: {...p.replicaOptions}, haproxyOptions: {...p.haproxyOptions}
// toProto: replicas: v.replicas, haproxy: v.haproxy,
//          replicaOptions: {...v.replicaOptions}, haproxyOptions: {...v.haproxyOptions}
```
Match the exact field names of the generated `OrioledbParams` TS type and the VM shape used by other engines (read the postgres converter in the same file as the template).

- [ ] **Step 2: Blank/default params.** In `web/src/services/wizard.ts`, the orioledb `blankEngineParams`/default gains `replicas: 0, haproxy: 0, replicaOptions: {}, haproxyOptions: {}` (match the discriminated-union member shape).

- [ ] **Step 3: Params form + topology preview.** In `web/src/components/database/DatabaseParamsForm.tsx`, the orioledb `EngineParamsForm` branch adds: a `replicas` number input, a `haproxy` number input (0/1), and `replica_options` / `haproxy_options` map editors — mirror the PostgreSQL branch's controls (read the `case "postgres":` block as the template). `engineNodes` for orioledb returns master + N replicas (+ haproxy when >0) groups (match the `TopologyGroup` shape other engines return; postgres is the model).

- [ ] **Step 4: Typecheck + build.**

Run: `cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/web && npx tsc -b 2>&1 | tail -5`
Expected: `No errors found` / exit 0.

- [ ] **Step 5: Commit.**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud
git add web/src
git commit -m "feat(orioledb): wizard replicas/haproxy params + scaled topology preview"
```

---

## Task 8: Builtin "OrioleDB HA" preset (optional smoke target)

**Files:**
- Modify: `internal/app/presets_seed.go`

- [ ] **Step 1: Add the preset.** In `builtinDatabasePresets()`, alongside "OrioleDB single", add a scaled preset (read the existing orioledb `add(...)` call as the template and only change name/params):
```go
	add("OrioleDB HA", "OrioleDB master + 1 streaming replica + HAProxy.",
		domain.Database_KIND_ORIOLEDB,
		&domain.DatabaseParams{
			Version: "pg17",
			Engine: &domain.DatabaseParams_Orioledb{Orioledb: &domain.OrioledbParams{
				Image:        "orioledb/orioledb:latest-pg17",
				InitdbLocale: "C",
				Replicas:     1,
				Haproxy:      1,
			}},
		})
```
Match the EXACT `add()` signature/wrapping used by the existing orioledb preset.

- [ ] **Step 2: Build + test.**

Run: `go build ./... 2>&1 | tail -2 && go test ./internal/app/ 2>&1 | tail -3`
Expected: Success + PASS.

- [ ] **Step 3: Commit.**

```bash
git add internal/app/presets_seed.go
git commit -m "feat(orioledb): builtin OrioleDB HA (replica + haproxy) preset"
```

---

## Task 9: Full build / vet / test gate

- [ ] **Step 1: Build + cli.**

Run: `go build ./... && go build ./cmd/cli/ 2>&1 | tail -2`
Expected: Success.

- [ ] **Step 2: Tests + vet + gofmt.**

Run: `go test ./internal/domain/database/... ./internal/domain/deployment/ ./internal/app/ 2>&1 | tail -5`
Expected: PASS.
Run: `go vet ./internal/domain/database/orioledb/ 2>&1 | grep -v copylocks | tail`
Expected: clean.
Run: `gofmt -l internal/domain/database/orioledb/ internal/domain/deployment/monitor.go`
Expected: empty.

- [ ] **Step 3: Commit any gofmt fixes (if listed).**

```bash
gofmt -w internal/domain/database/orioledb/ && git add -A && git commit -m "style(orioledb): gofmt" || true
```

---

## Task 10: Stage e2e — master + 1 replica + HAProxy

Follow memory `feedback_staging_deploy_git`: commit (no co-author) → push → pull on box → rebuild. Launch via `scripts/stage-preset-harness.sh` (the proven recipe) filtered to the HA preset; monitor with the Monitor tool (memory `feedback_monitor_not_inline`).

- [ ] **Step 1: Push.** `git push origin ref`

- [ ] **Step 2: Redeploy server on the box.**
```bash
ssh st-postgres@158.160.244.172 'cd ~/stroppy-cloud && git pull --ff-only origin ref && docker compose build server && docker compose up -d server'
```
Expected: server `Up`, no errors in `docker compose logs --tail=30 server`.

- [ ] **Step 3: Launch the HA preset run.**
```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud
STROPPY_BASE_URL=https://stage.cloud.stroppy.io STROPPY_ADMIN_LOGIN=admin \
STROPPY_ADMIN_PASSWORD='<from box .env STROPPY_ADMIN_PASSWORD>' TENANT_SLUG=default \
PRESET_NAME_REGEX='Self-check / OrioleDB HA|OrioleDB HA' \
SMOKE_DURATION=60s SMOKE_VUS=1 SMOKE_POOL_SIZE=2 \
RUN_TIMEOUT_SECONDS=60 RUN_POLL_SECONDS=20 TEMPORAL_DESCRIBE=0 \
REPORT_DIR=.stage-preset-harness/orioledb-ha ./scripts/stage-preset-harness.sh smoke || true
cat .stage-preset-harness/orioledb-ha/report.jsonl | jq -rc 'select(.phase=="audit" or .phase=="launch")|{phase,ready,errors:(.errors//[]),runId}'
```
Expected: audit `ready:true` with `machineCount`=3 (master+replica+haproxy); a `runId`.

- [ ] **Step 4: Monitor the run to terminal.** Use the Monitor tool polling `GetTestRun` (field `runId`) until `STATUS_COMPLETED|FAILED|...`. On failure, pull logs (`QueryLogs`) and fix.

- [ ] **Step 5: Verify scale worked.** On COMPLETED:
  - `GetRunMetrics` (field `runId`): **DB Replication Lag** is now non-null (a replica exists).
  - Logs show the replica did `pg_basebackup` and started as standby; HAProxy `/primary` `/replica` checks pass.
  - On the haproxy node, write traffic hit the master, reads the replica.

- [ ] **Step 6: Update memory.** Append OrioleDB HA/scale outcome to `project_orioledb.md` (replica standby init, haproxy primary-check, any deploy gotchas).

---

## Self-review notes (author)

- Spec coverage: §1→T1, §2→T2, §3 master/replica→T3/T4, §4 haproxy+health→T5, §5 monitor→T6, §6 FE→T7, §7 preset→T8, build/e2e→T9/T10. ✓
- Type consistency: `orioledbWiring{masterHost,dbHosts}` introduced T3 (masterHost) extended T5 (dbHosts); `orioledbEngineComponent` signature gains `wiring` in T3 and is used through T5; role constants from T2 used in T3-T6; `streamingMasterOptions`/`orioledbReplicaUnit`/`orioledbHAProxyConfig`/`orioledbHealthScript` names consistent across tasks and tests.
- Flagged implementer decisions (not placeholders): exact `ComponentTargets`/target-struct field names (T4 — read cockroach), `EngineFile`/`PostStartCommands`/`EngineCommand` exact shapes (T5 — read cockroach/postgres), FE map-editor component + `TopologyGroup`/`add()` shapes (T7/T8 — read postgres). Each names the file to read.
- Naming: container `stroppy-orioledb` (shared master/replica), health unit `orioledb-health`, ports 5432/5433/8008 consistent.
