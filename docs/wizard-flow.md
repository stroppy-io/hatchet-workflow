# Wizard flow (test + suite)

Canonical workflow for the **test wizard** and **suite wizard** under the
schema-split architecture: a pure, provider-agnostic **database** schema, a
separate **provider** schema, a **workload** schema, and machines **derived**
from the DB config (no cross-validation web). This is the contract the UI and
the wizard services follow.

## Layers (recap)

| Layer | What | Where |
|---|---|---|
| **database** | pure, provider-agnostic DB config (topology=logical roles/counts, HA, replication, tuning, auth, …). Owns its DB-internal cross-rules. | `internal/schemas/databases/<kind>` (one schema per kind) |
| **workload** | stroppy config (script/vus/duration/…), provider- and db-agnostic | `internal/schemas/workload` |
| **provider** | provider settings (creds/network/disk catalog/limits/zones). Comes from `TenantSettings.providers`, **not** the form | `internal/schemas/providers/{yandex,docker}` |
| **expand** | role→VM derivation + provider overlay + capacity sanity (bake-time, Go) | `internal/schemas/expand` |

Key principle: **the user never enters machines.** The DB config determines the
logical roles (1 primary + N replicas + DCS quorum + proxy + …); `expand` turns
those into VMs with default shapes; a provider overlay fills zone/disk/platform;
3 capacity checks run at bake. Correctness is **by construction** (count) +
**by provider option lists** (disk/zone) + **DB-internal rules** (sync≤replica…),
not a cross-schema rule web.

---

## The machine model — ABSTRACT + emitted overrides

A **machine is abstract**: `{cores, memory_gb, disk_gb}` — pure capacity, NO
provider data (`expand.VM`). The database (logical roles) determines how many
machines of each role are needed; `expand` derives them with default shapes.

The **provider override params** are SEPARATE and **server-emitted**: given the
chosen `provider_type` and the DB config, the backend decides WHICH override
params to show per role (yandex: `disk_type` from the tenant catalog, `zone`,
`platform_id`, `network_acceleration`; docker: none). Options are narrowed to the
**tenant** provider settings; the user edits only these. Built by
`expand.OverrideSchema(provider, dbKind, role, tenantProviderSettings)`; the
override vocabulary lives in `providers/override.go`.

At bake: abstract machine → `topology.Instance.machine_info`; override values →
`Instance.provider_parms`. (This is exactly the proto's split.)

## The wizard `form` (composite, server-emitted)

`form` is a single `schemapb.Filled` the server builds and the UI renders. There
is **no separate "provider" step** — the provider machine overrides sit on the
step that owns those machines (DB machines on the DATABASE step).

```
form (root)
├─ provider_type                 ← selector, default from TenantSettings; NOT provider settings
├─ database  (STEP "database")
│   ├─ config                    ← pure db schema (provider-agnostic), prefix "database."
│   └─ machines[ per role ]      ← derived by expand (role + count + abstract shape, editable)
│        + override params       ← server-emitted from (provider × db × role), options from tenant
└─ workload  (STEP "workload")
    ├─ config                    ← workload schema, prefix "workload."
    └─ machines[ runner ]        ← stroppy runner(s) + their overrides
```

- **provider SETTINGS** (creds/network/disk catalog/limits/zones) are NEVER in the
  form — they are tenant-fixed (`TenantSettings.providers`) and only feed the
  override option lists + capacity limits.
- **machines per role** (not per individual VM): one editable form per role with
  its count shown.
- Switching DB **kind** (or `provider_type`) re-derives machines + re-emits the
  override schema → server **re-emits** `form.schema`; UI re-renders.

`draft.topology` is a server-derived OUTPUT, recomputed each patch via `expand`
(+ the user's override values); never free-form user input.

---

## Test wizard — lifecycle

```
StartTestWizard(tenant_id, name?, test_preset_id?)
  → draft{ form(schema+seed values), topology=[], errors, ready=false }

loop  PatchTestWizard(draft_id, form: Filled)        ← UI sends edited values
  server:
    1. validate form (db schema rules + workload rules)         → errors[]
    2. resolve tenant provider settings: TenantSettings[provider_type]
    3. machines = expand.Expand(kind, form.database) + expand.WithWorkload(.,runners)  ← abstract, per role
    4. if provider_type/kind/roles changed → re-emit per-role override schema:
         expand.OverrideSchema(provider_type, kind, role, tenantProv)   ← options from tenant
       → re-emit form.schema (UI re-renders)
    5. errors += expand.Sanity(kind, db, provider_type, tenantProv, machines)  ← capacity/zones
    6. ready = (len(errors)==0)
  → draft{ form, topology, errors, ready }

FinishTestWizard(draft_id, start?, save_as_preset?, ratings?)   ← rejected unless ready
  → bake: domain.TestRun{
        database  = baked form.database,
        workload  = baked form.workload,
        provider  = ProviderSettings(provider_type, tenant settings),
        topology  = mapped from expand VMs → topology.Topology (Instances+Components),
     }
  optionally start (TestRunAPI.StartTestRun → TestWorkflow) and/or save TestPreset
```

`errors` carry BOTH schema-validation failures (path-grouped: `database.*`,
`workload.*`) and bake-time capacity/sanity failures (quota/RAM/zones), so the
UI shows everything before Finish.

---

## Suite wizard — lifecycle

A suite is a **matrix**: `db_preset_ids` × `workload_preset_ids` (+ whole
`test_preset_ids`), run under **ONE** provider. Presets already hold baked
db/workload params — the suite wizard only **composes** them; it does NOT re-edit
params and has NO per-topology provider settings.

```
form (root)
├─ provider_type        enum {yandex, docker}   ← ONE provider for the whole suite
├─ db_preset_ids        [string]
├─ workload_preset_ids  [string]
├─ test_preset_ids      [string]
└─ max_parallel         uint32
```

```
StartSuiteWizard(tenant_id, name?, suite_id?)
  → draft{ form, preview=[], errors, ready=false }

loop  PatchSuiteWizard(draft_id, form)
  server:
    1. validate form
    2. expand matrix: every (db_preset × workload_preset) + each test_preset → Cell
    3. per Cell:
         compatible = ScriptSupported(db_kind, protocol, script)   ← cross-schema check (here, not in a schema)
         errors     = expand.Sanity(kind, preset.db, provider_type, settings, expand(preset.db))  ← per-cell capacity
         ready      = compatible && len(errors)==0
       (no topology baked per cell here — only lightweight summaries)
    4. draft.ready = (>=1 compatible cell) && form valid
  → draft{ form, preview:[Cell{db_preset_id, workload_preset_id, test_preset_id, name, compatible, ready, errors}], errors, ready }

FinishSuiteWizard(draft_id)                                       ← rejected unless ready
  → bake: domain.SuiteRun{
        test_runs = [ per compatible cell:
            domain.TestRun{ database=preset.db, workload=preset.workload,
                            provider=ProviderSettings(provider_type),
                            topology=expand(preset.db) + overlay } ],
        max_parallel,
     }
```

`Cell.errors` (per-cell capacity/sanity) is the channel for the N-cell case — a
cell over quota is surfaced in preview, not silently dropped at finish.

---

## Where each rule lives (no duplication)

| Rule kind | Lives in | Example |
|---|---|---|
| DB-internal cross-field | the **DB schema** (standalone, `root`=db form) | `sync_count ≤ replica_count`, `wal_compression→PG15` |
| Workload value ranges | the **workload schema** | duration suffix s/m/h, vus≥0 |
| (db,workload) compatibility | **wizard server** (cross-schema) | `ScriptSupported(kind, proto, script)` |
| Machine count | **nobody** — correct by construction (`expand`) | 1 + replicas + dcs quorum |
| Disk class / zone validity | **provider option lists** (overlay) | disk ∈ provider catalog |
| Capacity (RAM/quota/zones) | **`expand.Sanity`** at bake/patch | cores ≤ limit, mirror-3-dc ⇒ ≥3 zones |

The old two-sources-of-truth drift (FE + BE hardcoding rules) is gone: each rule
exists once, in the schema or the server, and the UI renders from the schema.

---

## Proto mapping (the remaining wiring)

- `expand.VM{Role, Name, Shape{cores,mem,disk}}` (ABSTRACT) → one
  `topology.Instance{machine_info}` per machine + one `Component{kind=Role,
  allocated_on_instance_id}`; `Connection`s by role.
- the user's **override values** (from the emitted override schema) →
  `Instance.provider_parms`. The override OPTIONS come from
  `TenantSettings.providers[provider_type]`.
- `provider_type` + `TenantSettings.providers[type]` → `deployment.ProviderSettings`.
- `models.{Test,Suite}WizardDraftRecord.form` = the composite `schemapb.Filled`
  (db config + workload config + provider_type + per-role abstract machines +
  emitted overrides); `.topology` / `.preview[].errors` = derived outputs.

Open items: the `expand.VM → topology.Instance/Component` mapper, DB-specific
override constraints (io-m3 %93, mirror-3-dc zones) in `OverrideSchema`, and the
concrete `WizardEngine.Compute` that runs this pipeline + fills `Cell.errors`.
