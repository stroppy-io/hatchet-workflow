# OrioleDB database kind — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add OrioleDB as a selectable database engine end-to-end — a single docker-only PostgreSQL-protocol container, deployed by the agent, with its image pulled through a gateway-hosted pull-through Docker registry.

**Architecture:** OrioleDB requires a patched Postgres build shipped only as `orioledb/orioledb:latest-pgNN`. A new slim `OrioledbParams` describes one container. A new `internal/domain/database/orioledb` package builds a one-node topology and a deployment renderer that installs docker, configures a registry mirror, and runs the container under a systemd unit. The wire protocol is Postgres, so workload/driver/metrics reuse the Postgres path. The cloud stack box runs a `registry:2` pull-through cache to Docker Hub; egress-less nodes reach it via a docker `registry-mirror`.

**Tech Stack:** Go (protobuf/protogen, ogen, connect, graphql-go), TypeScript/React (web), Docker/registry:2, systemd, easyp codegen toolchain (in `./bin`).

## Global Constraints

- New DB kind enum value: `KIND_ORIOLEDB = 9` (next free after `KIND_EXTERNAL = 8`).
- New params field number: `OrioledbParams orioledb = 17` in `DatabaseParams.engine` oneof (16 is the last used — cockroach).
- Default image: `orioledb/orioledb:latest-pg17`. Exposed versions (image tags): `pg17`, `pg16`.
- OrioleDB requires locale C/POSIX/ICU — default `initdb_locale = "C"`, passed as `POSTGRES_INITDB_ARGS="--locale=C"`.
- Wire protocol = PostgreSQL: workload protocol `PROTOCOL_PG`, stroppy driver `postgres`, port `5432`.
- Single container only (no replicas/HA) for v1.
- Commits: Conventional Commits, NO `Co-Authored-By` / AI attribution. Branch `ref`.
- Codegen runs through `./bin` toolchain: regenerate via the cycle in Task 1; never hand-edit generated files under `internal/proto/**` or `web/src/lib/proto/**`.
- Build verification command for Go: `go build ./...` from repo root (note: `rtk` intercepts `go build`; full error list is in `~/.local/share/rtk/tee/<newest>_go_build.log`).

---

## File map

Create:
- `internal/domain/database/orioledb/orioledb.go` — topology builder (one node).
- `internal/domain/database/orioledb/package.go` — PackageResolver (docker.io + registry-mirror).
- `internal/domain/database/orioledb/deployment.go` — DeploymentRenderer (docker-run systemd unit).

Modify (backend):
- `protocols/cloud/v1/domain/database.proto` — enum value + `OrioledbParams` + oneof field.
- `internal/domain/database/builder.go` — topology dispatch case.
- `internal/infrastructure/adapters/suite_wizard_engine.go` — register PackageResolver + DeploymentRenderer.
- `internal/app/presets_seed.go` — `workloadProtocolForDatabase` case + builtin "OrioleDB single" preset + package seeding helper.
- `internal/infrastructure/execution/monitoring.go` — `dbKindString` case.
- `internal/domain/metrics/queries.go` — `MetricsForDB` case (maps to postgres metrics).

Modify (infra):
- `docker-compose.yaml` — `registry:2` pull-through service.
- `deployments/caddy/Caddyfile` — route to the registry (or gateway cmux route).

Modify (frontend):
- `web/src/services/wizard.ts` — `EngineKind`, `ENGINES`, `ENGINE_TO_KIND`/`KIND_TO_ENGINE`, `defaultProtocolFor`, `driverTypeFor`, `blankEngineParams`, `DB_VERSIONS`.
- `web/src/components/library-table/labels.ts` — `DbKind`, `DB_KINDS`, `DB_LABEL`, `DB_COLOR`.
- `web/src/pages/NewRun.tsx` — `ENGINE_ICON`.
- `web/src/components/database/DatabaseParamsForm.tsx` — `EngineParamsForm` branch + `engineNodes` branch + `DB_VERSIONS`.

Regenerated (do not hand-edit): `internal/proto/cloud/v1/domain/*`, `internal/proto/cloud/v1/api/*`, `web/src/lib/proto/**`.

---

## Task 1: Proto — add KIND_ORIOLEDB + OrioledbParams, regenerate

**Files:**
- Modify: `protocols/cloud/v1/domain/database.proto`
- Regenerated: `internal/proto/cloud/v1/domain/database.pb.go`, related api converters, `web/src/lib/proto/cloud/v1/domain/database_pb.ts`

**Interfaces:**
- Produces: `domain.Database_KIND_ORIOLEDB`, `domain.OrioledbParams` (getters `GetImage()`, `GetPostgresOptions()`, `GetInitdbLocale()`, `GetSharedBuffersMb()`), `DatabaseParams.GetOrioledb()`.

- [ ] **Step 1: Add the enum value.** In `protocols/cloud/v1/domain/database.proto`, in `enum Kind` (after `KIND_EXTERNAL = 8;`):

```proto
    KIND_ORIOLEDB = 9;
```

- [ ] **Step 2: Add the params message.** Near the other `*Params` messages in the same file:

```proto
/*
    OrioledbParams configures a single OrioleDB container. OrioleDB is a
    patched-Postgres storage engine shipped docker-only, so there is no native
    package: the node installs Docker and runs the official image. Wire protocol
    is plain PostgreSQL.
 */
message OrioledbParams {
    // image is the full OrioleDB docker image ref. Empty => latest-pg17.
    string image = 1;
    // postgres_options are appended as `-c key=value` to the container command
    // (postgresql.conf overrides).
    map<string, string> postgres_options = 2;
    // initdb_locale must be C, POSIX or an ICU locale (OrioleDB limitation).
    // Empty => "C".
    string initdb_locale = 3;
    // shared_buffers_mb is a convenience tuning knob; 0 => image default.
    uint32 shared_buffers_mb = 4;
}
```

- [ ] **Step 3: Add the oneof field.** In `message DatabaseParams`, inside `oneof engine` (after `CockroachParams cockroach = 16;`):

```proto
    OrioledbParams orioledb = 17;
```

- [ ] **Step 4: Regenerate Go + TS proto.**

Run:
```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/protocols && export PATH="$PWD/../bin:$PWD/../web/node_modules/.bin:$PATH" && \
  ../bin/easyp -cfg easyp.go.yaml generate && \
  ../bin/easyp -cfg easyp.api.go.yaml generate && \
  rm -rf ../web/src/lib/proto && ../bin/easyp -cfg easyp.ts.yaml generate
```
Expected: `code generation completed` for each, exit 0.

- [ ] **Step 5: Verify the symbols exist and module builds.**

Run: `cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && grep -rn "KIND_ORIOLEDB\|OrioledbParams" internal/proto/cloud/v1/domain/database.pb.go | head`
Expected: matches for the enum const and the struct.

Run: `go build ./... 2>&1 | tail -3`
Expected: `Go build: Success` (no new errors).

- [ ] **Step 6: Commit.**

```bash
git add protocols/cloud/v1/domain/database.proto internal/proto web/src/lib/proto
git commit -m "feat(proto): add OrioleDB database kind + OrioledbParams"
```

---

## Task 2: Backend — orioledb topology builder

**Files:**
- Create: `internal/domain/database/orioledb/orioledb.go`
- Test: `internal/domain/database/orioledb/orioledb_test.go`

**Interfaces:**
- Consumes: `domain.OrioledbParams` (Task 1); `dbspec.Component/Node/Tags`, `topology.*` (existing).
- Produces: `orioledb.Database` with `BuildTopologySpec(*domain.OrioledbParams) (*topology.TopologySpec, error)`; constants `orioledbEngine = "orioledb"`, `orioledbRoleMaster = "master"`, `pgPort = 5432`.

- [ ] **Step 1: Write the failing test.** Create `internal/domain/database/orioledb/orioledb_test.go`:

```go
package orioledb

import "testing"

func TestBuildTopologySpecSingleNode(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&dummyParams)
	if err != nil {
		t.Fatalf("BuildTopologySpec: %v", err)
	}
	if len(spec.GetNodes()) != 1 {
		t.Fatalf("want 1 node, got %d", len(spec.GetNodes()))
	}
	if len(spec.GetComponents()) != 1 {
		t.Fatalf("want 1 component, got %d", len(spec.GetComponents()))
	}
	c := spec.GetComponents()[0]
	if c.GetEngine() != orioledbEngine || c.GetRole() != orioledbRoleMaster {
		t.Fatalf("unexpected component engine=%q role=%q", c.GetEngine(), c.GetRole())
	}
	if len(spec.GetConnections()) != 0 {
		t.Fatalf("single node must have no connections, got %d", len(spec.GetConnections()))
	}
}
```

(Add at top of test file, after imports — a package-level var so the test compiles:)
```go
import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

var dummyParams = domain.OrioledbParams{}
```

- [ ] **Step 2: Run test to verify it fails.**

Run: `cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/database/orioledb/ 2>&1 | tail -5`
Expected: FAIL — package/`Database` undefined.

- [ ] **Step 3: Write the implementation.** Create `internal/domain/database/orioledb/orioledb.go`:

```go
package orioledb

import (
	"errors"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	orioledbEngine     = "orioledb"
	orioledbRoleMaster = "master"

	// pgPort is the PostgreSQL wire port exposed by the OrioleDB container.
	pgPort = 5432

	// defaultImage is used when OrioledbParams.image is empty.
	defaultImage = "orioledb/orioledb:latest-pg17"
)

type Database struct{}

func (d *Database) ValidateInput(input *domain.OrioledbParams) error {
	if input == nil {
		return errors.New("orioledb params are required")
	}
	return input.Validate()
}

// BuildTopologySpec returns a single-node topology: one OrioleDB container.
// OrioleDB has no upstream HA yet, so there are no replicas or coordinators.
func (d *Database) BuildTopologySpec(input *domain.OrioledbParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	id := "orioledb-master-1"
	spec := &topology.TopologySpec{
		Nodes: []*topology.Node{
			dbspec.Node(orioledbEngine, id, orioledbRoleMaster, 1, []string{id}),
		},
		Components: []*topology.Component{
			dbspec.Component(orioledbEngine, id, topology.Component_KIND_DATABASE, orioledbRoleMaster, id),
		},
		Connections: []*topology.Connection{},
		Labels: map[string]string{
			"kind":   "database",
			"engine": orioledbEngine,
			"nodes":  "1",
		},
		Tags: dbspec.Tags(orioledbEngine),
	}
	return spec, nil
}
```

- [ ] **Step 4: Run test to verify it passes.**

Run: `go test ./internal/domain/database/orioledb/ 2>&1 | tail -5`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/domain/database/orioledb/orioledb.go internal/domain/database/orioledb/orioledb_test.go
git commit -m "feat(orioledb): single-node topology builder"
```

---

## Task 3: Backend — orioledb package resolver (docker.io + registry mirror)

**Files:**
- Create: `internal/domain/database/orioledb/package.go`
- Test: `internal/domain/database/orioledb/package_test.go`

**Interfaces:**
- Consumes: `domain.Database`, `domain.Package` (existing).
- Produces: `orioledb.PackageResolver` with `SupportsDatabase(*domain.Database) bool` and `ResolveDatabasePackage(*domain.Database) (*domain.Package, error)`; exported `RegistryMirrorEnv = "STROPPY_REGISTRY_MIRROR"` (env var the agent substitutes with the gateway registry URL).

- [ ] **Step 1: Write the failing test.** Create `internal/domain/database/orioledb/package_test.go`:

```go
package orioledb

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestResolveDatabasePackageInstallsDockerAndMirror(t *testing.T) {
	db := &domain.Database{
		Kind:   domain.Database_KIND_ORIOLEDB,
		Params: &domain.DatabaseParams{Engine: &domain.DatabaseParams_Orioledb{Orioledb: &domain.OrioledbParams{}}},
	}
	pkg, err := PackageResolver{}.ResolveDatabasePackage(db)
	if err != nil {
		t.Fatalf("ResolveDatabasePackage: %v", err)
	}
	if got := strings.Join(pkg.GetAptPackages(), ","); !strings.Contains(got, "docker.io") {
		t.Fatalf("want docker.io apt package, got %q", got)
	}
	joined := strings.Join(pkg.GetPreInstall(), "\n")
	if !strings.Contains(joined, "/etc/docker/daemon.json") {
		t.Fatalf("pre_install must write the docker registry mirror config, got:\n%s", joined)
	}
	if !strings.Contains(joined, "systemctl enable --now docker") {
		t.Fatalf("pre_install must start docker, got:\n%s", joined)
	}
}
```

- [ ] **Step 2: Run test to verify it fails.**

Run: `go test ./internal/domain/database/orioledb/ -run TestResolveDatabasePackage 2>&1 | tail -5`
Expected: FAIL — `PackageResolver` undefined.

- [ ] **Step 3: Write the implementation.** Create `internal/domain/database/orioledb/package.go`:

```go
package orioledb

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// RegistryMirrorEnv is the env var the agent expands (from the bootstrap
// ExtraEnv) into the gateway's pull-through registry URL, e.g.
// "http://gateway-host:5000". Empty disables the mirror (direct pull).
const RegistryMirrorEnv = "STROPPY_REGISTRY_MIRROR"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_ORIOLEDB && database.GetParams() != nil
}

// ResolveDatabasePackage installs Docker (from the Ubuntu universe repo, which
// rides the existing apt-cacher proxy) and points dockerd at the gateway's
// pull-through registry mirror so an egress-less node can pull the OrioleDB
// image. The mirror URL is injected at run time via $STROPPY_REGISTRY_MIRROR.
func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/orioledb/docker"
	}
	return &domain.Package{
		Id:          packageID,
		Name:        "OrioleDB (docker)",
		DbKind:      domain.Database_KIND_ORIOLEDB,
		DbVersion:   database.GetParams().GetVersion(),
		IsBuiltin:   true,
		AptPackages: []string{"docker.io"},
		PreInstall:  dockerMirrorPreInstall(),
	}, nil
}

// dockerMirrorPreInstall configures /etc/docker/daemon.json with the registry
// mirror (when $STROPPY_REGISTRY_MIRROR is set) and a matching insecure-registry
// entry (the in-VPC mirror is plain HTTP), then enables dockerd. apt installs
// docker.io itself; these run as the package pre_install steps (order 100).
func dockerMirrorPreInstall() []string {
	const daemonJSON = "/etc/docker/daemon.json"
	return []string{
		"install -d /etc/docker",
		// Write daemon.json only when a mirror is provided; strip the scheme for
		// the insecure-registries entry (docker wants host:port there).
		fmt.Sprintf(`sh -c 'mirror="${%s}"; if [ -n "$mirror" ]; then host="${mirror#http://}"; host="${host#https://}"; printf "{\"registry-mirrors\":[\"%%s\"],\"insecure-registries\":[\"%%s\"]}\n" "$mirror" "$host" > %s; fi'`,
			RegistryMirrorEnv, daemonJSON),
		"systemctl enable --now docker",
		// Apply daemon.json if dockerd was already running.
		"systemctl reload docker 2>/dev/null || systemctl restart docker || true",
	}
}
```

- [ ] **Step 4: Run test to verify it passes.**

Run: `go test ./internal/domain/database/orioledb/ -run TestResolveDatabasePackage 2>&1 | tail -5`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/domain/database/orioledb/package.go internal/domain/database/orioledb/package_test.go
git commit -m "feat(orioledb): package resolver installs docker + registry mirror"
```

---

## Task 4: Backend — orioledb deployment renderer (docker-run systemd unit)

**Files:**
- Create: `internal/domain/database/orioledb/deployment.go`
- Test: `internal/domain/database/orioledb/deployment_test.go`

**Interfaces:**
- Consumes: `deploymentbuilder.{RenderContext,PreviewContext,EngineComponent,RenderComponentDeployment,RenderComponentPreview,ConfigDir,DataDir,ServiceName,ShellQuote,EngineServiceFile,DependencyIDs,ArtifactID}`; `topologypb.Component`; `domain.Database/Package`; `deploymentpb.*`; `common.File`. (All from cockroach's renderer — same helper set.)
- Produces: `orioledb.DeploymentRenderer` implementing `Supports`/`RenderComponent`/`RenderPreview`.

- [ ] **Step 1: Write the failing test.** Create `internal/domain/database/orioledb/deployment_test.go`:

```go
package orioledb

import (
	"strings"
	"testing"

	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestSupportsOnlyOrioledb(t *testing.T) {
	r := DeploymentRenderer{}
	if !r.Supports(&topologypb.Component{Engine: orioledbEngine}) {
		t.Fatal("must support orioledb engine")
	}
	if r.Supports(&topologypb.Component{Engine: "postgres"}) {
		t.Fatal("must not support postgres engine")
	}
}

func TestServiceUnitRunsContainer(t *testing.T) {
	unit := orioledbServiceUnit("orioledb-master-1", "orioledb/orioledb:latest-pg17", "C", map[string]string{"shared_buffers": "512MB"})
	for _, want := range []string{
		"docker run",
		"--network host",
		"orioledb/orioledb:latest-pg17",
		"POSTGRES_INITDB_ARGS=--locale=C",
		"ExecStartPre=", // docker pull
		"-c shared_buffers=512MB",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("service unit missing %q:\n%s", want, unit)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails.**

Run: `go test ./internal/domain/database/orioledb/ -run 'TestSupports|TestServiceUnit' 2>&1 | tail -5`
Expected: FAIL — `DeploymentRenderer`/`orioledbServiceUnit` undefined.

- [ ] **Step 3: Write the implementation.** Create `internal/domain/database/orioledb/deployment.go`:

```go
package orioledb

import (
	"fmt"
	"sort"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const containerName = "stroppy-orioledb"

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == orioledbEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil))
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentDeployment(ctx, ec), nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	deps := deploymentbuilder.DependencyIDs(deploymentbuilder.RenderContext{
		Topology:  ctx.Topology,
		Component: ctx.Component,
		Node:      ctx.Node,
	}, nil)
	ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deps)
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

func orioledbEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	dbPackage *domain.Package,
	dependencies []string,
) (deploymentbuilder.EngineComponent, error) {
	if component.GetRole() != orioledbRoleMaster {
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported orioledb component role %q", component.GetRole())
	}
	params := database.GetParams().GetOrioledb()
	image := params.GetImage()
	if image == "" {
		image = defaultImage
	}
	locale := params.GetInitdbLocale()
	if locale == "" {
		locale = "C"
	}
	options := pgOptions(params)
	configDir := deploymentbuilder.ConfigDir(component.GetId())

	return deploymentbuilder.EngineComponent{
		Engine:          orioledbEngine,
		GlobalPriority:  20,
		NodePriority:    20,
		Dependencies:    dependencies,
		InstallCommands: orioledbInstallCommands(dbPackage),
		ServiceFile:     deploymentbuilder.EngineServiceFile(component.GetId(), orioledbServiceUnit(component.GetId(), image, locale, options)),
		Healthcheck:     "docker exec " + containerName + " pg_isready -h 127.0.0.1 -p " + fmt.Sprint(pgPort),
		ConfigDirOverride: configDir, // ensures the config dir step still runs
	}, nil
}

// pgOptions merges the explicit shared_buffers_mb knob into postgres_options.
func pgOptions(params *domain.OrioledbParams) map[string]string {
	out := map[string]string{}
	for k, v := range params.GetPostgresOptions() {
		out[k] = v
	}
	if mb := params.GetSharedBuffersMb(); mb > 0 {
		out["shared_buffers"] = fmt.Sprintf("%dMB", mb)
	}
	return out
}

// orioledbInstallCommands runs the package pre_install (docker + mirror) then the
// apt install of docker.io. dbPackage is always set for builtin orioledb.
func orioledbInstallCommands(dbPackage *domain.Package) []string {
	var commands []string
	if dbPackage != nil {
		commands = append(commands, dbPackage.GetPreInstall()...)
		if pkgs := dbPackage.GetAptPackages(); len(pkgs) > 0 {
			commands = append(commands,
				"DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(pkgs, " "))
		}
	}
	return commands
}

// orioledbServiceUnit renders a systemd unit that pulls and runs the OrioleDB
// container with host networking (so port 5432 is on the node exactly as native
// Postgres). The agent passes POSTGRES_PASSWORD via the unit EnvironmentFile.
func orioledbServiceUnit(componentID, image, locale string, options map[string]string) string {
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
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud OrioleDB %s
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
ExecStartPre=-/usr/bin/docker rm -f %s
ExecStartPre=/usr/bin/docker pull %s
ExecStartPre=/bin/mkdir -p %s
ExecStart=/usr/bin/docker run --rm --name %s --network host \
  -e POSTGRES_PASSWORD=stroppy \
  -e POSTGRES_INITDB_ARGS=--locale=%s \
  -v %s:/var/lib/postgresql/data \
  %s%s
ExecStop=/usr/bin/docker rm -f %s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`,
		componentID,
		containerName,
		image,
		deploymentbuilder.ShellQuote(dataDir),
		containerName,
		locale,
		deploymentbuilder.ShellQuote(dataDir),
		image,
		optStr.String(),
		containerName,
	)
}
```

> NOTE for the implementer: `EngineComponent` may not have a `ConfigDirOverride` field. Open `internal/domain/deployment/engine.go`, read the `EngineComponent` struct and `RenderComponentDeployment`. OrioleDB needs no rendered config file (config is passed via `-c` flags). If the engine REQUIRES a `ConfigFile`/`ConfigArtifactID` (like cockroach sets), provide a minimal one: a `common.File` at `ConfigDir(id)+"/orioledb.env"` containing `POSTGRES_PASSWORD=stroppy` and reference it from the unit via `EnvironmentFile`. Drop the `ConfigDirOverride` line if no such field exists. Match whatever the struct actually offers — do not invent fields.

- [ ] **Step 4: Reconcile with the real EngineComponent struct.**

Run: `sed -n '/type EngineComponent struct/,/^}/p' internal/domain/deployment/engine.go`
Adjust the returned `EngineComponent` literal to use only fields that exist (see NOTE). If a config file is mandatory, add an `orioledb.env` `ConfigFile`/`ConfigArtifactID` and add `EnvironmentFile=<configDir>/orioledb.env` to the unit, replacing the inline `-e POSTGRES_PASSWORD=stroppy`.

- [ ] **Step 5: Run tests to verify they pass.**

Run: `go test ./internal/domain/database/orioledb/ 2>&1 | tail -5`
Expected: PASS (all orioledb tests).

- [ ] **Step 6: Commit.**

```bash
git add internal/domain/database/orioledb/deployment.go internal/domain/database/orioledb/deployment_test.go
git commit -m "feat(orioledb): docker-run deployment renderer"
```

---

## Task 5: Wire orioledb into the builder + renderer/package registries

**Files:**
- Modify: `internal/domain/database/builder.go`
- Modify: `internal/infrastructure/adapters/suite_wizard_engine.go`

**Interfaces:**
- Consumes: `orioledb.Database`, `orioledb.PackageResolver`, `orioledb.DeploymentRenderer` (Tasks 2-4).

- [ ] **Step 1: Add the topology dispatch case.** In `internal/domain/database/builder.go`, add an import `orioledbdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/orioledb"` and a case before `default:`:

```go
	case domain.Database_KIND_ORIOLEDB:
		if params.GetOrioledb() == nil {
			return nil, fmt.Errorf("orioledb database requires orioledb params")
		}
		return (&orioledbdb.Database{}).BuildTopologySpec(params.GetOrioledb())
```

- [ ] **Step 2: Register the package resolver + renderer.** In `internal/infrastructure/adapters/suite_wizard_engine.go`:

First inspect the two registry literals:
Run: `sed -n '90,115p' internal/infrastructure/adapters/suite_wizard_engine.go`

Add the import (alias matching the file's convention, e.g. `databaseorioledb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/orioledb"`), then add `databaseorioledb.PackageResolver{}` to the package-resolver registry list and `databaseorioledb.DeploymentRenderer{}` to the `deploymentbuilder.NewRegistry(...)` renderer list.

- [ ] **Step 3: Build.**

Run: `go build ./... 2>&1 | tail -3`
Expected: `Go build: Success`.

- [ ] **Step 4: Topology round-trip test.** Append to `internal/domain/database/orioledb/orioledb_test.go`:

```go
func TestBuilderDispatchesOrioledb(t *testing.T) {
	// Guards that the builder switch routes KIND_ORIOLEDB here. Import the
	// builder package locally to avoid an import cycle at file top.
	t.Skip("covered by package-level builder_test in internal/domain/database")
}
```

Instead add the real assertion in the builder's own package — create `internal/domain/database/builder_orioledb_test.go`:

```go
package database

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestBuildTopologySpecOrioledb(t *testing.T) {
	db := &domain.Database{
		Kind:   domain.Database_KIND_ORIOLEDB,
		Params: &domain.DatabaseParams{Engine: &domain.DatabaseParams_Orioledb{Orioledb: &domain.OrioledbParams{}}},
	}
	spec, err := BuildTopologySpec(db)
	if err != nil {
		t.Fatalf("BuildTopologySpec: %v", err)
	}
	if len(spec.GetComponents()) != 1 || spec.GetComponents()[0].GetEngine() != "orioledb" {
		t.Fatalf("expected one orioledb component, got %+v", spec.GetComponents())
	}
}
```

- [ ] **Step 5: Run the test.**

Run: `go test ./internal/domain/database/ -run TestBuildTopologySpecOrioledb 2>&1 | tail -5`
Expected: PASS. (If `domain.Database{}` requires more fields to pass `Validate()`, set `PackageId`/`Params.Version` as needed — read `database.pb.validate.go` for required fields.)

- [ ] **Step 6: Commit.**

```bash
git add internal/domain/database/builder.go internal/domain/database/builder_orioledb_test.go internal/domain/database/orioledb/orioledb_test.go internal/infrastructure/adapters/suite_wizard_engine.go
git commit -m "feat(orioledb): wire topology builder + deploy/package registries"
```

---

## Task 6: Workload protocol, metrics, monitoring, builtin preset

**Files:**
- Modify: `internal/app/presets_seed.go`
- Modify: `internal/infrastructure/execution/monitoring.go`
- Modify: `internal/domain/metrics/queries.go`

**Interfaces:**
- Consumes: `domain.Database_KIND_ORIOLEDB`; existing `postgresMetrics()`, `workloadProtocolForDatabase`, `dbKindString`, `builtinDatabasePresets`.

- [ ] **Step 1: Map the workload protocol.** In `internal/app/presets_seed.go` `workloadProtocolForDatabase`, add (Postgres-compatible):

```go
	case domain.Database_KIND_ORIOLEDB:
		return domain.Workload_PROTOCOL_PG
```
(If `default:` already returns `PROTOCOL_PG`, this is explicit-but-harmless; add it for clarity and to survive a future default change.)

- [ ] **Step 2: Map the metrics kind string.** In `internal/infrastructure/execution/monitoring.go` `dbKindString`, add a case mapping `domain.Database_KIND_ORIOLEDB` to `"orioledb"`.

Run first: `sed -n '/func dbKindString/,/^}/p' internal/infrastructure/execution/monitoring.go` to match the existing switch style; add:
```go
	case domain.Database_KIND_ORIOLEDB:
		return "orioledb"
```

- [ ] **Step 3: Map metrics queries to Postgres.** In `internal/domain/metrics/queries.go` `MetricsForDB`, add `"orioledb"` to the Postgres branch. Since the switch falls through to `postgresMetrics()` in `default`, add an explicit case for documentation:

```go
	case "orioledb":
		return postgresMetrics()
```

- [ ] **Step 4: Add the builtin preset.** In `internal/app/presets_seed.go` `builtinDatabasePresets()`, near the `pg()` helper usage, add an OrioleDB single-node preset. First read the helper signatures:

Run: `sed -n '/func builtinDatabasePresets/,/^}/p' internal/app/presets_seed.go | head -80`

Then add an entry following the existing `add(...)` pattern, e.g.:
```go
	add("OrioleDB single", "Single OrioleDB container (docker).",
		domain.Database_KIND_ORIOLEDB,
		&domain.DatabaseParams{
			Version: "pg17",
			Engine: &domain.DatabaseParams_Orioledb{Orioledb: &domain.OrioledbParams{
				Image:        "orioledb/orioledb:latest-pg17",
				InitdbLocale: "C",
			}},
		})
```
Match the EXACT signature/shape of the existing `add()`/`pg()` calls in this function (they may wrap params differently — copy the closest existing call and swap the engine).

- [ ] **Step 5: Build + test.**

Run: `go build ./... 2>&1 | tail -3 && go test ./internal/domain/metrics/ ./internal/app/ 2>&1 | tail -5`
Expected: build Success; tests pass (or no tests for app — then just build).

- [ ] **Step 6: Commit.**

```bash
git add internal/app/presets_seed.go internal/infrastructure/execution/monitoring.go internal/domain/metrics/queries.go
git commit -m "feat(orioledb): workload protocol, metrics, monitoring, builtin preset"
```

---

## Task 7: Gateway — pull-through Docker registry

**Files:**
- Modify: `docker-compose.yaml`
- Modify: `deployments/caddy/Caddyfile`
- Modify (if mirror URL is injected via bootstrap): `internal/domain/agent/bootstrap.go` (ExtraEnv) and the workflow that builds bootstrap ExtraEnv.

**Interfaces:**
- Produces: a reachable registry mirror URL for nodes; the value placed into `$STROPPY_REGISTRY_MIRROR` (consumed by Task 3's `dockerMirrorPreInstall`).

- [ ] **Step 1: Add the registry service.** In `docker-compose.yaml`, add a service (pull-through cache to Docker Hub). Match the file's existing indentation/network config:

```yaml
  registry:
    image: registry:2
    restart: unless-stopped
    environment:
      REGISTRY_PROXY_REMOTEURL: "https://registry-1.docker.io"
    ports:
      - "127.0.0.1:5000:5000"
    volumes:
      - registry-data:/var/lib/registry
```
And add `registry-data:` under the top-level `volumes:` block.

- [ ] **Step 2: Decide + implement the node-reachable address.** Nodes reach the box via the gateway/Caddy. Two valid wirings — pick the one matching how nodes already reach the gateway:
  - (a) Caddy route: in `deployments/caddy/Caddyfile`, add a handler under the gateway site that reverse-proxies `/v2/*` to `registry:5000`. Nodes set mirror `http://<gateway-host>` (Caddy strips nothing; registry v2 lives at `/v2/`).
  - (b) Dedicated port: expose registry on the box's NAT IP:5000 and set mirror `http://<gateway-host>:5000`.

Read `deployments/caddy/Caddyfile` (already inspected in session: it has `(stroppy_gateway)` with `handle /grafana/*` etc.) and add, inside the gateway site block:
```caddyfile
	handle /v2/* {
		reverse_proxy {$CADDY_REGISTRY_UPSTREAM:127.0.0.1:5000}
	}
```

- [ ] **Step 3: Inject the mirror URL into the node bootstrap.** Find where agent bootstrap `ExtraEnv` is assembled for a run (search):

Run: `grep -rn "ExtraEnv\|AgentServerAddr\|agentBootstrap(" internal/workflows internal/domain/settings internal/app | head`

Set `STROPPY_REGISTRY_MIRROR` in ExtraEnv to the node-facing registry URL derived from the server/gateway address (e.g. reuse the apt-proxy host logic in `internal/domain/agent/bootstrap.go` `AptProxyURL`, swapping the path/port for the registry). Add a config/setting for the registry URL with a sensible default; do not hardcode a host.

- [ ] **Step 4: Build + sanity.**

Run: `go build ./... 2>&1 | tail -3`
Expected: Success.
Run: `docker compose config >/dev/null && echo compose-ok`
Expected: `compose-ok` (compose file parses).

- [ ] **Step 5: Commit.**

```bash
git add docker-compose.yaml deployments/caddy/Caddyfile internal/domain/agent/bootstrap.go internal/workflows
git commit -m "feat(gateway): pull-through Docker registry + node registry-mirror injection"
```

---

## Task 8: Frontend — orioledb engine in the wizard

**Files:**
- Modify: `web/src/services/wizard.ts`
- Modify: `web/src/components/library-table/labels.ts`
- Modify: `web/src/pages/NewRun.tsx`
- Modify: `web/src/components/database/DatabaseParamsForm.tsx`

**Interfaces:**
- Consumes: regenerated `Database_Kind.KIND_ORIOLEDB`, `OrioledbParams` TS types (Task 1).

- [ ] **Step 1: Extend `wizard.ts`.** Read the relevant blocks first:
Run: `grep -n "type EngineKind\|ENGINES\|ENGINE_TO_KIND\|KIND_TO_ENGINE\|defaultProtocolFor\|driverTypeFor\|blankEngineParams\|DB_VERSIONS" web/src/services/wizard.ts`

Then:
  - Add `"orioledb"` to the `EngineKind` union.
  - `ENGINES`: add `{ kind: "orioledb", label: "OrioleDB", hex: "#E8633A", blurb: "Patched-Postgres storage engine (docker).", typed: true }`.
  - `ENGINE_TO_KIND`: `orioledb: Database_Kind.KIND_ORIOLEDB`.
  - `KIND_TO_ENGINE`: `[Database_Kind.KIND_ORIOLEDB]: "orioledb"`.
  - `defaultProtocolFor`: `case "orioledb": return Workload_Protocol.PROTOCOL_PG` (match the existing PG case's enum reference).
  - `driverTypeFor`: `case "orioledb": return "postgres"`.
  - `blankEngineParams`: `case "orioledb": return { kind: "orioledb", orioledb: { image: "", postgresOptions: {}, initdbLocale: "C", sharedBuffersMb: 0 } }` (match the discriminated-union shape used by the other engines).

- [ ] **Step 2: Extend `labels.ts`.**
Run: `grep -n "DbKind\|DB_KINDS\|DB_LABEL\|DB_COLOR" web/src/components/library-table/labels.ts`
Add `"orioledb"` to the `DbKind` union, `DB_KINDS`, and entries `DB_LABEL.orioledb = "OrioleDB"`, `DB_COLOR.orioledb = "#E8633A"`.

- [ ] **Step 3: Add the icon in `NewRun.tsx`.**
Run: `grep -n "ENGINE_ICON" web/src/pages/NewRun.tsx`
Add an `orioledb:` entry to `ENGINE_ICON` (reuse the Postgres icon or a generic DB icon already imported).

- [ ] **Step 4: Add the params form + topology preview branches in `DatabaseParamsForm.tsx`.**
Run: `grep -n "EngineParamsForm\|engineNodes\|DB_VERSIONS\|case \"postgres\"\|case \"cockroach\"" web/src/components/database/DatabaseParamsForm.tsx`
  - `DB_VERSIONS`: add `orioledb: ["pg17", "pg16"]`.
  - `EngineParamsForm`: add a `case "orioledb":` rendering: a version select (DB_VERSIONS), a locale text input (default "C"), a shared_buffers number input, and a key/value map editor for `postgresOptions` (reuse the same map-editor component the Postgres `master_options` uses).
  - `engineNodes`: add a `case "orioledb":` returning a single group `[{ role: "master", count: 1 }]` (match the `TopologyGroup` shape used by other engines).

- [ ] **Step 5: Typecheck + build the web app.**

Run: `cd web && npm run build 2>&1 | tail -15`
Expected: build succeeds, no TS errors referencing orioledb.

- [ ] **Step 6: Commit.**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud
git add web/src
git commit -m "feat(orioledb): wizard engine, params form, labels, topology preview"
```

---

## Task 9: Full build, vet, and module-wide test

**Files:** none (verification gate).

- [ ] **Step 1: Full Go build + cli.**

Run: `go build ./... && go build ./cmd/cli/ 2>&1 | tail -3`
Expected: Success.

- [ ] **Step 2: Vet the touched packages.**

Run: `go vet ./internal/domain/database/orioledb/ ./internal/domain/database/ ./internal/app/ 2>&1 | grep -v copylocks | tail`
Expected: no new issues.

- [ ] **Step 3: Run the DB + app unit tests.**

Run: `go test ./internal/domain/database/... ./internal/domain/metrics/ 2>&1 | tail -10`
Expected: PASS.

- [ ] **Step 4: gofmt.**

Run: `gofmt -l internal/domain/database/orioledb/ internal/app/presets_seed.go internal/infrastructure/execution/monitoring.go`
Expected: empty (all formatted).

- [ ] **Step 5: Commit any formatting fixes (if gofmt listed files).**

```bash
gofmt -w internal/domain/database/orioledb/ && git add -A && git commit -m "style(orioledb): gofmt" || true
```

---

## Task 10: Staging rollout + one e2e OrioleDB run

**Files:** none (deploy + manual verify). Follow memory `feedback_staging_deploy_git`: commit(no co-author) → push → pull on box → rebuild.

- [ ] **Step 1: Push the branch.**

```bash
git push origin ref
```

- [ ] **Step 2: Pull + bring up the new registry service on the box.**

```bash
ssh st-postgres@158.160.244.172 'cd ~/stroppy-cloud && git pull --ff-only origin ref && docker compose up -d registry && docker compose ps registry'
```
Expected: `registry` container `Up`.

- [ ] **Step 3: Rebuild + restart the server.**

```bash
ssh st-postgres@158.160.244.172 'cd ~/stroppy-cloud && docker compose build server && docker compose up -d server'
```
Expected: server `Up`, no errors in `docker compose logs --tail=30 server`.

- [ ] **Step 4: Verify the registry mirror works from the box.**

```bash
ssh st-postgres@158.160.244.172 'curl -fsS http://127.0.0.1:5000/v2/ && echo "" && curl -fsS "http://127.0.0.1:5000/v2/orioledb/orioledb/tags/list" | head -c 300'
```
Expected: `{}` from `/v2/`, and a JSON tags list (proves pull-through to Docker Hub).

- [ ] **Step 5: Launch an OrioleDB run via the API (login → build a run from the "OrioleDB single" preset).**

Use the REST surface (`/api/v1/...`) or the wizard. Minimum smoke: create a TestRun whose database is the OrioleDB builtin preset against the docker provider or a single YC node, start it, and watch logs/metrics. Use the Monitor tool (memory `feedback_monitor_not_inline`) — do not sleep-then-check.

- [ ] **Step 6: Verify on the DB node.**
  - `docker ps` shows `stroppy-orioledb` running.
  - `pg_isready` healthcheck passed (deploy phase green).
  - stroppy connected over `postgresql://...:5432` and produced metrics (postgres metric set, e.g. `pg_stat_database_xact_commit`).

- [ ] **Step 7: Record results + update memory.** Append OrioleDB to the stage invariant matrix and note any registry/mirror gotchas in `project_ogen_generator.md`'s sibling memory or a new `project_orioledb` memory.

---

## Self-review notes (author)

- Spec coverage: §1 proto → Task 1; §2 backend topology/renderer → Tasks 2-5; §3 gateway registry → Task 7; §4 workload/driver/metrics → Task 6; §5 frontend → Task 8; §6 rollout → Task 10. ✓
- Open implementer decisions flagged inline (not placeholders): EngineComponent field reconciliation (Task 4 Step 4), Caddy-vs-port registry wiring (Task 7 Step 2), exact `add()`/`blankEngineParams` shapes (Tasks 6/8) — each names the file to read and the rule to follow.
- Type consistency: `orioledbEngine`/`orioledbRoleMaster`/`pgPort`/`defaultImage`/`containerName`/`RegistryMirrorEnv` defined in Task 2/3 and reused in Task 4; `OrioledbParams` getters from Task 1 used in Tasks 4/6/8.
- Naming: container `stroppy-orioledb`, image default `orioledb/orioledb:latest-pg17`, engine string `"orioledb"` consistent across backend + FE + metrics.
