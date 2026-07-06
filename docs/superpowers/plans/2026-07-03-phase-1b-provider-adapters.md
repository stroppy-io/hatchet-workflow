# Phase 1B: Provider Adapters + Device Path (closes I2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Checkbox steps.

**Goal:** Real adapters bridging the provider-runner's fake `terraformExec`/`dockerExec` interfaces to the actual `terraform.Actor`/`docker.Executor`, and thread a disk device path from provisioning through to `DiskView.Path` so `${{ machine.disks[0].path }}` resolves at runtime (closes whole-branch review finding I2).

**Architecture:** Thin adapter structs implementing the Task-17 fake interfaces over the real executors (build `*Terraform_Input`/`*Docker_Input` the way `renderTerraformInput`/`renderDockerInput` do). Device path carried as a `MachineState` endpoint label (`disk_device`), read by `dslrun.machineViewFor` into `DiskView.Path`. Real executor runs are deploy-gated; adapters are unit-tested on input construction + output parsing, not live infra.

**Tech Stack:** Go 1.25, internal/infrastructure/{terraform,docker} executors, internal/infrastructure/provider, internal/workflows/dslrun, internal/dsl/expr views.

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase, NEVER Co-Authored-By/AI attribution.
- Spec §4.2 of `docs/superpowers/specs/2026-07-03-flow-wiring-design.md`.
- No deploy: adapters tested on input-construction + output-parsing with fakes/stubs; NO live terraform/docker daemon runs. Live provision is a deploy-gated follow-up.
- Do NOT modify the existing `terraform.Actor`/`docker.Executor` — only adapt over them.
- golangci clean for new code. No bare `go mod tidy`.

---

### Task 1: Device path through the domain — MachineState label → DiskView.Path (I2 core)

**Files:**
- Modify: `internal/infrastructure/provider/terraform.go` (add `disk_device` to stroppy_machines parse + MachineState label), `internal/infrastructure/provider/docker.go` (device from bind path → label)
- Modify: `internal/workflows/dslrun.go` (`machineViewFor` fills `DiskView.Path` from the label)
- Modify: `examples/dsl/postgres-ha/providers/yandex/module/*.tf` + `outputs` (add `disk_device` to stroppy_machines output) — only if it affects the golden; check
- Test: `internal/workflows/dslrun_test.go` (device path binding), `internal/infrastructure/provider/*_test.go` (label set)

**Interfaces:**
- Consumes: `deploymentpb.MachineState` (endpoints + labels), `dslpb.MachineGroup`, `expr.DiskView{Path, SizeGb}`.
- Produces: convention — `MachineState`'s private endpoint carries label `disk_device` = device path (e.g. `/dev/vdb` terraform, host bind path docker). `dslrun.machineViewFor` reads it into `DiskView.Path` for machine index 0's first disk.

- [ ] **Step 1: Failing test in dslrun_test.go** — construct `ExecuteCompiledPlanInput` with a MachineState whose endpoint has label `disk_device: /dev/vdb`; a steps job with cmd `mkfs ${{ machine.disks[0].path }}`; assert the interpolated command (via a mock CallCmd stub capturing the script) contains `/dev/vdb`, not empty. RED because machineViewFor currently sets Path "".

- [ ] **Step 2: Run — FAIL** (`go test ./internal/workflows/ -run CompiledPlan -v`)

- [ ] **Step 3: Implement** — in `dslrun.machineViewFor`, when building `DiskView`, read the `disk_device` label off the machine's endpoint/labels (grep how machineViewFor reads MachineState — mirror the address lookup). Set `DiskView.Path`. In provider terraform.go: extend `tfMachineOutput` with `DiskDevice string json:"disk_device"`, set the label on the produced MachineState (default `/dev/vdb` if module omits it — document). In docker.go: set `disk_device` label to the container's data bind host path (or a conventional in-container device); document the docker convention.

- [ ] **Step 4: Run — PASS**. Also add a provider-level test asserting the produced MachineState carries the `disk_device` label.

- [ ] **Step 5: Golden check** — run `go test ./internal/dsl/ -run Golden`. If the postgres-ha module needs a `disk_device` output to stay consistent, add it and `-update` the golden; commit the golden change. If golden unaffected (module outputs aren't in the compiled plan), skip.

- [ ] **Step 6: Commit** `feat(provider): thread disk device path into machine view (closes I2)`

---

### Task 2: Real terraform adapter

**Files:**
- Create: `internal/infrastructure/provider/terraform_adapter.go` (real `terraformExec` impl over `terraform.Actor`)
- Test: `internal/infrastructure/provider/terraform_adapter_test.go`

**Interfaces:**
- Consumes: `terraform.{Actor, NewWorkdirWithParams, NewWdId, WithVarFile, WithTfFiles, WithEnv, TfOutput}`, `yandextf.EmbeddedTfFiles()` (embedded HCL), the `terraformExec` interface (Task 17).
- Produces:

```go
// terraformActorExec adapts terraform.Actor to the terraformExec interface.
type terraformActorExec struct {
	actor    *terraform.Actor
	tfFiles  []terraform.TfFile   // embedded module HCL (from EmbeddedTfFiles or moduleDir read)
	env      map[string]string    // provider env (e.g. YC_TOKEN...) — passed in
}
func NewTerraformActorExec(actor *terraform.Actor, tfFiles []terraform.TfFile, env map[string]string) *terraformActorExec
func (a *terraformActorExec) Apply(ctx, dir string, varsJSON []byte) (map[string][]byte, error) // build WorkdirWithParams(NewWdId(dir), WithTfFiles, WithVarFile(varsJSON), WithEnv) → ApplyTerraform → map[string][]byte(TfOutput)
func (a *terraformActorExec) Destroy(ctx, dir string, varsJSON []byte) error // DestroyExisting(NewWdId(dir), options)
```

- Mirror how `renderTerraformInput` (`internal/workflows/provider_render.go`) builds the Operation (VarFileName, embedded files, env) — but here via the Actor's Option chain, not the proto Terraform_Input. Study terraform/actor.go Option funcs + executor.go workdirOptions for the exact option set.

- [ ] **Step 1: Failing test** — this adapter can't run real terraform (no binary/deploy). Test the WorkdirWithParams CONSTRUCTION: inject a fake actor (interface wrapping ApplyTerraform/DestroyExisting) capturing the `*WorkdirWithParams`; assert Apply builds it with the varsJSON as var file, tfFiles attached, env set, correct WdId from dir. RED (adapter doesn't exist).

Refactor note: `terraform.Actor` is a concrete struct. Introduce a minimal local interface `type tfActor interface { ApplyTerraform(ctx, *terraform.WorkdirWithParams) (terraform.TfOutput, error); DestroyExisting(ctx, terraform.WdId, ...terraform.Option) error }` that `*terraform.Actor` satisfies, so the test can inject a fake. Put it in terraform_adapter.go.

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** the adapter + local tfActor interface.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(provider): terraform actor adapter for provider exec`

---

### Task 3: Real docker adapter

**Files:**
- Create: `internal/infrastructure/provider/docker_adapter.go` (real `dockerExec` impl over `docker.Executor`)
- Test: `internal/infrastructure/provider/docker_adapter_test.go`

**Interfaces:**
- Consumes: `docker.Executor.{Up,Down}(ctx, *deployment.Docker_Input)`, `deployment.{Docker_Input,Docker_Container,Docker_Network}`, the `dockerExec` interface + `ContainerSpec`/`ContainerState` (Task 17).
- Produces:

```go
// dockerExecutorExec adapts docker.Executor (batch Up/Down over Docker_Input)
// to the per-machine dockerExec interface. EnsureContainer builds a
// single-entry Docker_Input.Containers map (+ the run's network) and calls
// Up; RemoveContainers builds a Docker_Input with the network + accumulated
// container names and calls Down.
type dockerExecutorExec struct {
	exec    *docker.Executor
	// tracks container names per network so RemoveContainers can Down them
	mu      sync.Mutex
	byNet   map[string][]string
}
func NewDockerExecutorExec(exec *docker.Executor) *dockerExecutorExec
func (a *dockerExecutorExec) EnsureContainer(ctx, spec ContainerSpec) (ContainerState, error) // 1-entry Docker_Input → Up → parse Docker_ContainerOutput for IP
func (a *dockerExecutorExec) RemoveContainers(ctx, networkName string) error                 // Docker_Input{Network, Containers: tracked} → Down
```

- Study `renderDockerInput` (`provider_render.go`) + docker/executor.go Up/Down for the Docker_Input shape (network, container image/env/labels, how IP is read from Docker_ContainerOutput). Introduce a local `dockerRunner` interface (Up/Down over *Docker_Input) that *docker.Executor satisfies, so tests inject a fake capturing the Docker_Input.

- [ ] **Step 1: Failing test** — fake dockerRunner capturing Docker_Input; assert EnsureContainer builds a 1-container map with image/env/labels/network from ContainerSpec; fake returns a Docker_Output with an IP → ContainerState.InternalIP populated; two EnsureContainer calls then RemoveContainers → Down called with both container names + network. RED.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** adapter + local dockerRunner interface + name tracking.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(provider): docker executor adapter for provider exec`

---

### Task 4: Provider factory + real-executor wiring seam

**Files:**
- Create: `internal/infrastructure/provider/factory.go` (`NewProviderForRef` selecting docker builtin vs terraform module from a ProviderRef + available executors)
- Test: `internal/infrastructure/provider/factory_test.go`

**Interfaces:**
- Consumes: `NewDocker`, `NewTerraform`, the two adapters (Tasks 2/3), `dslpb.ProviderRef`.
- Produces:

```go
// Deps carries the real executors + module source a factory needs.
type Deps struct {
	DockerExec dockerExec          // real adapter (Task 3) or fake
	Actor      *terraform.Actor    // for terraform providers
	Env        map[string]string   // provider env
	ModuleDir  func(providerName string) (dir string, tfFiles []terraform.TfFile, ok bool) // resolves a provider name → module HCL (builtin "docker" → not a tf module)
}
// NewProviderForRef returns the Provider for ref.Name: "docker" → NewDocker(deps.DockerExec);
// otherwise → NewTerraform(moduleDir, NewTerraformActorExec(actor, tfFiles, env)).
func NewProviderForRef(ref *dslpb.ProviderRef, deps Deps) (Provider, error)
```

- This is the seam the RunRecipeWorkflow (phase 1D) calls to get a Provider. v1: `ModuleDir` resolves builtin "yandex" → `yandextf.EmbeddedTfFiles()`; unknown provider → error (recipe-supplied provider modules come later with recipe storage). Document.

- [ ] **Step 1: Failing test** — `NewProviderForRef({Name:"docker"}, deps)` returns a docker-backed Provider (non-nil, no error); `{Name:"yandex"}` with a stub ModuleDir returns a terraform-backed Provider; unknown name → error. RED.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(provider): provider factory selecting docker/terraform by ref`

---

## Self-Review
- Spec §4.2 coverage: real adapters (T2/T3), device path I2 (T1), factory seam (T4). Live provision deploy-gated (documented, not in scope).
- I2 closed: T1 makes `${{ machine.disks[0].path }}` resolve non-empty end-to-end (provider label → machineViewFor → DiskView.Path → interpolation), locked by a dslrun test.
- Deferred to 1D: factory called by RunRecipeWorkflow; real executors constructed in app.
- No placeholders: interfaces + test assertions concrete. Types consistent (terraformExec/dockerExec from Task 17 unchanged; adapters satisfy them).
