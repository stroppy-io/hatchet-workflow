# Phase 1C: Nomad Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Checkbox steps.

**Goal:** Extend agent bootstrap so provisioned nodes install and run a Nomad agent — the gateway node as a single-node Nomad server, every other node as a Nomad client stamping `meta.stroppy_node_id` (which `nomad.BuildJob`'s per-node constraint targets). Both the VM (cloud-init) and docker paths.

**Architecture:** `agent.Bootstrap` gains Nomad fields; `CloudInit` template gains conditional Nomad blocks (install binary, write `/etc/nomad.d/{server|client}.hcl`, systemd unit). Docker path renders a Nomad sidecar container (docker driver + docker.sock). A role-assignment helper marks the gateway node server / rest clients — consumed by the RunRecipeWorkflow (phase 1D). Real Nomad install/run is deploy-gated; this phase unit-tests the rendered config only.

**Tech Stack:** Go 1.25, internal/domain/agent bootstrap, internal/workflows/provider_render (docker), internal/dsl/nomad jobspec (the constraint consumer).

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase, NEVER Co-Authored-By/AI attribution.
- Spec §4.4 of `docs/superpowers/specs/2026-07-03-flow-wiring-design.md`.
- No deploy: unit-test the RENDERED cloud-init/docker-file/nomad-hcl strings (contain the right stanzas, meta key = machineID). NO real nomad install/run.
- `nomad.BuildJob` (internal/dsl/nomad/jobspec.go) pins each task group with `${meta.stroppy_node_id} == <NodeID>` and hardcodes datacenter `dc1` — the client config MUST set `meta.stroppy_node_id = <machineID>` and `datacenter = "dc1"` to match. Read jobspec.go to confirm the exact meta key + dc before writing.
- golangci clean. No bare `go mod tidy`.

---

### Task 1: Bootstrap Nomad fields + VM cloud-init Nomad blocks

**Files:**
- Modify: `internal/domain/agent/bootstrap.go` (Bootstrap struct + CloudInit template + a NomadConfig renderer)
- Test: `internal/domain/agent/bootstrap_test.go` (or existing test file — grep)

**Interfaces:**
- Produces:

```go
type NomadRole string
const (
	NomadRoleNone   NomadRole = ""
	NomadRoleServer NomadRole = "server" // single-node: server + client enabled
	NomadRoleClient NomadRole = "client"
)
// Bootstrap gains:
//   NomadRole       NomadRole
//   NomadServerAddr string // for clients: "<gateway-private-ip>:4647"; empty for server (self)
//   NomadVersion    string // install version, default a pinned constant
// CloudInit renders, when NomadRole != NomadRoleNone, additional write_files
// (/etc/nomad.d/server.hcl or client.hcl) + a nomad systemd unit + a runcmd
// that installs the nomad binary (curl the release zip — deploy-gated URL) and
// `systemctl enable --now nomad`.
```

- Nomad HCL content:
  - server: `datacenter = "dc1"`, `data_dir`, `server { enabled = true; bootstrap_expect = 1 }`, `client { enabled = true }` (single-node dev: server+client colocated), docker driver plugin stanza.
  - client: `datacenter = "dc1"`, `data_dir`, `client { enabled = true; servers = ["<NomadServerAddr>"]; meta { stroppy_node_id = "<machineID>" } }`, docker driver plugin stanza.
- Install: `runcmd` curls `https://releases.hashicorp.com/nomad/<version>/nomad_<version>_linux_amd64.zip`, unzips to /usr/local/bin/nomad, writes the systemd unit, enables it. Pin `DefaultNomadVersion` constant. Document the release URL is deploy-gated (node needs egress or a mirror — a follow-up may route it through the gateway like the agent binary).

- [ ] **Step 1: Read jobspec.go** — confirm exact meta key (`stroppy_node_id`) + datacenter (`dc1`) the constraint uses. Read bootstrap.go's existing CloudInit template + test file.

- [ ] **Step 2: Failing tests** — `CloudInit(machineID, Bootstrap{NomadRole: NomadRoleClient, NomadServerAddr: "10.0.0.1:4647"}, opts)` → output contains `/etc/nomad.d/client.hcl`, `meta {`, `stroppy_node_id = "<machineID>"`, `servers = ["10.0.0.1:4647"]`, `datacenter = "dc1"`, a nomad systemd unit, and the install runcmd. `NomadRoleServer` → `server.hcl`, `bootstrap_expect = 1`, `client { enabled = true }`. `NomadRoleNone` (default) → NO nomad stanzas (existing behavior unchanged — assert the pre-nomad template output is byte-identical for the none case). RED.

- [ ] **Step 3: Run — FAIL** (`go test ./internal/domain/agent/ -run Nomad -v`)

- [ ] **Step 4: Implement** — add fields, a `renderNomadHCL(role, serverAddr, machineID)` helper, extend the template with `{{- if .NomadRole}}` blocks (write_files for the hcl + systemd unit, runcmd for install+enable). Keep the none-path output identical.

- [ ] **Step 5: Run — PASS** (all existing agent tests still green — the none-path byte-identical assertion guards this)

- [ ] **Step 6: Commit** `feat(agent): nomad server/client bootstrap in cloud-init`

---

### Task 2: Docker path Nomad sidecar

**Files:**
- Modify: `internal/infrastructure/provider/docker.go` (or `internal/workflows/provider_render.go applyDockerAgentBootstrap` — pick where docker bootstrap files/containers are assembled; grep both)
- Test: alongside

**Interfaces:**
- For the docker provider (dev), the gateway node's container additionally runs Nomad (single-node server+client, docker driver via mounted `/var/run/docker.sock`). Simplest v1: the gateway container gets Nomad installed + a nomad config file + starts nomad alongside the agent (or a dedicated nomad sidecar container in the same network). Decide (document): (a) nomad inside the agent container (extra file + the agent's entrypoint starts nomad), or (b) a separate `nomad` container per run added to the Docker_Input. Recommendation: (b) a dedicated nomad container (image `hashicorp/nomad`, privileged, `/var/run/docker.sock` bind, host network, config with server+client+docker-driver) — cleaner, mirrors how services become sibling containers. Non-gateway docker nodes are plain agent containers (no nomad).
- Produces: when the docker provider provisions the gateway node, the Docker_Input gains a nomad container (or the gateway container gains nomad files) with `meta.stroppy_node_id` NOT needed on the single-node server (jobspec constraint targets clients — but in single-node docker, all service jobs land on the one nomad; document how BuildJob's per-node constraint is satisfied when there's one docker nomad — likely the single node stamps every requested node's meta, OR docker service-jobs use a relaxed constraint. Resolve this: for docker dev, BuildJob's node constraint may not match a single-node nomad — either the docker nomad node stamps a wildcard/all meta, or the docker path sets NodeRef to the single nomad node. DOCUMENT the docker-single-node constraint story explicitly; if it needs a jobspec tweak for the docker case, note it as a 1D concern rather than changing jobspec here).

- [ ] **Step 1: Failing test** — docker provider provisioning with a gateway flag → produced Docker_Input contains a nomad container (image, privileged, docker.sock bind, config with server+client+docker driver). RED.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** the nomad container rendering for the gateway docker node; document the single-node constraint story.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(provider): nomad sidecar container for docker gateway node`

---

### Task 3: Role assignment + server-addr helper

**Files:**
- Create: `internal/domain/agent/nomad_roles.go` (helper)
- Test: `internal/domain/agent/nomad_roles_test.go`

**Interfaces:**
- Produces:

```go
// AssignNomadRoles marks the gateway machine as the nomad server and every
// other machine as a client pointing at the gateway's private address, given
// the provisioned machines and which node is the gateway.
// Returns a per-machineID Bootstrap-field patch the caller merges when building
// each node's Bootstrap.
type NomadAssignment struct {
	Role       NomadRole
	ServerAddr string // "" for the server; "<gatewayPrivateIP>:4647" for clients
}
func AssignNomadRoles(machineIDs []string, gatewayID, gatewayPrivateIP string) map[string]NomadAssignment
```

- Gateway → {Server, ""}; others → {Client, "<gatewayPrivateIP>:4647"}. Deterministic. This is what the RunRecipeWorkflow (1D) calls after provisioning to fill each node's Bootstrap.NomadRole/NomadServerAddr before rendering cloud-init/docker.

- [ ] **Step 1: Failing test** — 3 machines, gateway=one of them, ip=10.0.0.5 → gateway gets Server/"", others get Client/"10.0.0.5:4647". Unknown gatewayID not in list → gateway assignment absent (or error — decide, document). RED.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(agent): nomad role assignment helper`

---

## Self-Review
- Spec §4.4 coverage: VM bootstrap (T1), docker bootstrap (T2), role assignment (T3). Real nomad install/run deploy-gated (documented).
- The `meta.stroppy_node_id` = machineID contract with `nomad.BuildJob`'s constraint is explicitly verified against jobspec.go (T1 Step 1) and locked by the client-render test (T1 Step 2).
- Docker single-node constraint story: flagged as a possible 1D concern if BuildJob's per-node constraint doesn't fit one docker nomad — documented, not silently broken.
- Deferred to 1D: RunRecipeWorkflow calls AssignNomadRoles + fills Bootstrap + routes NomadSubmitJobActivity to the gateway task queue.
- No placeholders: HCL content, template stanzas, helper signatures concrete.
