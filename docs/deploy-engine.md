# Deploy engine (Temporal) — full design

End-to-end: a baked `domain.TestRun` → Temporal workflows → real infra (Yandex
Terraform / Docker) → agent installs+configures the DB → runs stroppy → tears
down. This rebuilds the OLD `internal/domain/{run,agent}` engine on Temporal,
using the proto model (`workflow/*`, `topology/*`, `deployment/*`) and the new
schema/expand layer.

Legend for "who/where": **bake** = server, deterministic, at wizard Finish /
StartTestRun. **wf** = a Temporal workflow. **act** = a Temporal activity. **agent**
= code running on the target host (a Temporal worker).

---

## 0. The three responsibilities (clean split)

| Concern | Provider-agnostic? | Where built | Stored in |
|---|---|---|---|
| **logical topology** (roles + counts) | yes | bake (`expand`) | — |
| **abstract machines** (cores/mem/disk) | yes | bake (`expand`) | `Instance.machine_info` |
| **deploy recipe** (config files + commands per component) | yes (placeholders for runtime) | bake (`recipe` builder) | `Component.deployment_strategy` |
| **provider overrides** (disk class/zone/platform) | no | bake (user-edited) | `Instance.provider_parms` |
| **provider materialization** (docker container / tf vars) | no | wf (render) | `Docker.Input` / `Terraform.Input` |
| **runtime facts** (IPs, endpoints, quotas) | no | act (deployment) | `Instance` (filled), `deployed_topology` |

Principle: **bake produces a complete, deterministic, provider-agnostic recipe
with placeholders; the workflow materializes it for the provider and resolves
placeholders with runtime IPs.** `TestWorkflow` never re-derives — it executes.

---

## 1. Topology model (proto change)

**Instance = a concrete VM that HOSTS components.** Components move INTO the
instance (colocation: e.g. db + pgbouncer on one VM; or etcd quorum on its own).

```proto
message Topology {
  message Instance {
    string id;
    common.Status status;
    deployment.MachineInfo machine_info;        // abstract shape (bake / expand)
    optional schemapb.Baked provider_parms;     // overrides (bake / user)
    repeated Component components;               // ← NEW: roles + recipes hosted here
    // runtime (filled by deployment):
    repeated deployment.Quota.Request quota_requests;
    repeated deployment.Quota.Allocation allocated_quotas;
    optional schemapb.Baked deployment_parms;   // runtime facts (IP, etc.)
    common.Tags tags;
  }
  repeated Instance instances;                   // ≥1
  repeated Connection connections;               // edges between component ids
  repeated Component external_components;        // managed services (no instance)
  common.Tags tags;
}
```

- `Component{ id, kind, deployment_strategy{config_files[], commands[]}, provider_parms?, status }`
  — `allocated_on_instance_id` becomes redundant (it lives inside its instance);
  keep it optional for external components or drop it.
- `Connection.from/to` reference **component ids**, which now exist (inside
  instances + external). Resolves the current hole.
- One VM = one or many components. `expand` decides colocation (today: 1 role per
  VM; later: colocate pgbouncer/etcd if desired).

---

## 2. Bake — server, deterministic (wizard Finish / StartTestRun)

Input: db config (Filled), workload (Filled), provider_type, tenant
ProviderSettings. Output: `domain.TestRun`.

```
bake(dbFilled, wlFilled, provider_type, tenantProv):
  1. database = dbFilled.Bake()            → domain.Database
     workload = wlFilled.Bake()            → domain.Workload
     provider = ProviderSettings(provider_type, tenantProv.Bake())
  2. roles    = expand.Expand(kind, db)    → []VM (role, shape)   [+ WithWorkload]
  3. instances = for each VM: Instance{ machine_info=shape,
                  provider_parms=override values,
                  components=[ Component{ kind=role,
                     deployment_strategy = recipe.Build(kind, role, db, workload, refs) } ] }
  4. connections = recipe.Wire(kind, instances)   → []Connection (ports/protocols per role)
  5. topology = Topology{instances, connections, external_components?}
  6. TestRun{ id, provider, topology, database, workload }
```

- **recipe.Build** is the per-db builder (§5) — produces config files (with
  placeholders like `__DB_HOST__`, `__ETCD_HOSTS__`) + the install/config command
  list. Provider-agnostic; deterministic from db config.
- **recipe.Wire** produces connections from roles (replica→master REPLICATION
  :5432, stroppy→endpoint, haproxy→backends :5000/:8008, etcd peers :2380…).
- The resulting `TestRun.topology` has NO IPs/quotas yet (filled at deploy).

`StartTestRun(run)` persists `TestRunRecord` + launches `TestWorkflow(run)`.

---

## 3. TestWorkflow — orchestration (wf, `workflow/test.proto`)

```
TestWorkflow(test_run):                         // id = test-run/<id>, no retry, no timeout
  deployed = ProcessDeploymentWorkflow(provider, topology)     // → topology with IPs/quotas
  InstallStroppyWorkflow(deployed)                              // stroppy onto runner(s)
  if database not external:
     InstallDatabaseWorkflow(deployed, database)                // per-db install+config
  RunWorkloadWorkflow(deployed, workload)                       // stroppy load (unbounded)
  // teardown always (defer): TerraformDestroy / DockerDown
```

Matches the proto: children, retry policies, no execution timeout, heartbeat
liveness. Teardown runs even on upstream failure (AlwaysRun).

---

## 4. Deployment — provider materialization (wf+act, `workflow/deployment.proto`)

```
ProcessDeploymentWorkflow(provider, topology):
  quotas      = CalculateQuotasWorkflow(topology)               // pure: shapes → Quota.Request
  AcquireNetworkActivity(providerSettings)                      // VPC/subnets (yandex) / bridge (docker)
  AcquireQuotasActivity(quota_requests)                         // provider quota grant
  switch provider:
    YANDEX:  in  = RenderTerraformVariablesWorkflow(topology)   // topology → Terraform.Input (compute.vms)
             TerraformPlanActivity(in); TerraformApplyActivity(in) → outputs (IPs)
    DOCKER:  in  = RenderDockerInputWorkflow(topology)          // topology → Docker.Input (containers)
             DockerPullActivity(in); DockerUpActivity(in)       → container IPs
  return deployed_topology   // instances filled with IPs (deployment_parms), allocated_quotas
```

- **Render** = topology (machine_info + provider_parms + components) →
  provider input. Yandex: each Instance → a `compute.vms` entry (cores/mem/disk
  from machine_info, disk_type/zone/platform from provider_parms; the terraform
  module `deployments/terraform/yandex` already takes this shape). Docker: each
  Instance → a container (image per component kind, mounts = config files).
- **deployed_topology** carries the runtime IPs back; the engine substitutes them
  into the baked recipe placeholders before install.

---

## 5. Recipe builder — the per-DB engine (bake; rebuild of old `task_*`/`dbconfig`)

A registry parallel to `expand`, per db kind:

```go
type Recipe interface {
  // Strategy for one component (role) — config files (placeholders) + commands.
  Build(role Role, db, workload, refs) Component.Strategy
  // Connections for the whole cluster (ports/protocols per role).
  Wire(instances) []Connection
}
var recipes = map[kind]Recipe{ "postgres": pgRecipe, ... }
```

This is the bulk of the old engine, per db:
- **postgres**: pg install (PGDG apt), postgresql.conf + pg_hba (from tuning/auth
  schema values, `%`/`${var}` resolved against `machine_info` RAM at render),
  replica `pg_basebackup` from `__MASTER_IP__`, +Patroni (patroni.yml, etcd hosts
  `__ETCD_HOSTS__`), +etcd, +pgbouncer, +HAProxy (write :5000/read :5001, hc :8008).
  Connections: replica→master REPLICATION, etcd peers COORDINATION :2380, stroppy
  →endpoint, haproxy→backends. (Old `task_postgres/patroni/etcd/pgbouncer/proxy`.)
- mysql/mariadb, picodata, ydb, cockroach, ydb-managed (terraform-only, no agent
  install): each a `Recipe`. Source of truth = `docs/db-params-deployment-map.md`.
- Monitoring (node_exporter/vmagent/vector + per-db exporter) = an ADDON component
  recipe added to every instance.

Placeholders (`__DB_HOST__`, `__MASTER_IP__`, `__ETCD_HOSTS__`, …) are resolved
from `deployed_topology` right before each install/config command runs.

---

## 6. Agent — Temporal worker on the host (`workflow/agent.proto`)

The agent is a **Temporal worker** polling a per-host task queue. The server
schedules the `AgentCommandService` activities onto that queue; the agent
executes them locally and heartbeats.

```
on provision (cloud-init / docker entrypoint): start stroppy-agent →
  AgentRegistryService.Register + Heartbeat   (server learns the task queue, marks online)
EnsureAgentOnlineActivity   → blocks until registered/online (heartbeat poll)
CreateDir / CreateTempDir / WriteFile (config files) / CallCmd (install/config/stroppy)
  → run locally, stream logs via AgentLogService.ShipLogs → VictoriaLogs
```

- `Component.deployment_strategy.{configuration_files, deployment_commands}` are
  executed via `WriteFileActivity` + `CallCmdActivity` against the component's
  instance agent.
- `CallCmdActivity` runs the long stroppy load (30d cap, 60s heartbeat).
- Reverse shell (`agent/shell.proto`) is the admin debug channel, unchanged.

This replaces the old "agent long-polls `/api/agent/poll`" with Temporal's own
poll loop (the activity task queue). Same `Cmd`/`Report` semantics.

---

## 7. Runtime info flow (placeholders → IPs → stroppy)

1. bake: recipe baked with placeholders, `TestRun.topology` has no IPs.
2. ProcessDeployment: provider apply → `deployed_topology` instances get IPs
   (`deployment_parms`).
3. InstallDatabase: per component, substitute `__MASTER_IP__`/`__ETCD_HOSTS__`
   into config files (from `deployed_topology`), `WriteFile` + `CallCmd` via agent.
4. RunWorkload: stroppy DB endpoint = the write connection (HAProxy :5000 if
   present, else master :5432, or pooler) resolved from `deployed_topology` +
   connections; substitute `__STROPPY_DB_HOST__`/`__STROPPY_DB_PORT__` into the
   stroppy config, `CallCmd` the load.
5. Metrics/logs ship to VictoriaMetrics/Logs keyed by run id; monitor protos
   project the live view.

---

## 8. What gets built (component inventory)

- **proto**: `Topology.Instance.components[]` (the §1 change); regen.
- **bake** (`internal/.../bake`): `expand` (have) + `recipe` registry (NEW, big) +
  assemble `domain.TestRun`; `WizardEngine.Compute` calls it for preview/finish.
- **recipe** (`internal/schemas/recipe` or `internal/deploy/recipe`): per-db
  `Recipe` (NEW, the old `task_*`/`dbconfig`). Source: db-params doc.
- **workflows** (`internal/.../workflows`): Test/Suite/Deployment orchestration +
  activities (render docker/tf, terraform plan/apply/destroy, docker up/down,
  acquire net/quota) — implement the temporal stubs.
- **provider deployers**: terraform runner (module exists) + docker deployer.
- **agent**: Temporal worker (register/heartbeat + Cmd activities + log ship).

---

## 9. Open design decisions (to align before building)

1. **Recipe at bake vs render-in-workflow.** Proposed: recipe (config+commands,
   placeholders) at BAKE → baked into `TestRun.topology` → reproducible, no
   re-derivation. Provider materialization (docker/tf) + placeholder resolution in
   the workflow. Alternative: bake only roles/shapes, a `RenderConfigsWorkflow`
   builds recipes at runtime (leaner TestRun, but non-deterministic spec).
2. **Agent = Temporal worker (per-host task queue)** vs server-side activity that
   dials the agent. Proposed: Temporal worker (native, heartbeat liveness, matches
   `agent.proto` activities). Needs a task-queue-per-host (or routing) scheme.
3. **Colocation.** Keep 1 role = 1 instance for v1 (simplest), or colocate
   (etcd/pgbouncer on db VM) like the old engine. `Instance.components[]` supports
   both; `expand` decides.
4. **Placeholder substitution layer.** A common `__VAR__` resolver over
   `deployed_topology` (old `BakeMachineOverride`/`__STROPPY_DB_HOST__` style), or
   schemapb `${var}` + a resolve pass. Pick one substitution mechanism.
5. **Teardown / idempotency / crash-recovery.** Temporal gives retries + workflow
   durability; the old `agent_commands` orphan-reaper is replaced by Temporal
   activity retries. Confirm teardown-always + `WORKFLOW_ID_REUSE_POLICY`.
6. **Suite = N TestWorkflows** (proto already). Per-cell topology built at suite
   finish (N bakes) honoring `max_parallel`.
