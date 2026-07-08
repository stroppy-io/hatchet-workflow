# SP-F: Execution Shape-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Shape up the already-live DSL execution engine (docker recipe-run is GREEN end-to-end on dev-stand) for the product-vision model: per-tenant provider credentials instead of one process-wide YC account (F1), per-run provisioning logs instead of one shared process stdout/stderr (F2), an observable/configurable Nomad service-health wait instead of a fixed 5-minute poll buried in one activity (F3), per-run terraform workdir/state isolation so two runs against the same provider stop colliding (F4), and two documented carryovers — Nomad multi-node role assignment and docker teardown surviving a control-plane restart (F5).

**Architecture:** `provider.NewProviderForRef` gains a `context.Context` and a new `provider.RunContext{TenantID, RunID}` parameter threaded in from `RecipeActivities.ProvisionActivity`/`TeardownActivity` (`internal/infrastructure/execution/recipe_activities.go`), which already receive both identifiers per-call. `provider.Deps` grows three new/changed fields — `EnvFn` (F1, replaces the static `Env` map), `LogSinkFn` (F2, new), `ModuleDir` (F4, signature grows tenantID+runID) — each resolved once per `NewProviderForRef` call, i.e. once per activity invocation, never inside the workflow. F3 needs no new activity: `NomadSubmitJobInput.WaitTimeout` already exists (`internal/agent/nomad_activities.go:45-49`) and is already honored by the polling loop — the gap is that `executeServiceJob` (`internal/workflows/dslrun.go`) never sets it. F5 is two independent, unrelated fixes bundled under one task per the spec's own grouping.

**Tech Stack:** Go, Temporal SDK (`go.temporal.io/sdk`), `github.com/hashicorp/terraform-exec/tfexec`, `github.com/hashicorp/nomad/api`, `github.com/docker/docker` client, existing AES-GCM `SecretStore` (`internal/infrastructure/identity`), `github.com/stretchr/testify/require`.

## Global Constraints

- No co-author footer in commits. Conventional Commits (`type(scope): summary`).
- This plan does NOT touch `internal/dsl/*` (compiler) or `ExecuteCompiledPlanWorkflow`'s DAG-walk loop (`internal/workflows/dslrun.go:117-197`) — every change is a point-wire into an existing extension seam, per spec §2's explicit "не переделывает движок" scope.
- Every `provider.Deps` field addition/change keeps the existing docker branch working unchanged: `EnvFn`/`LogSinkFn` are consulted only on the non-`"docker"` (terraform) branch of `NewProviderForRef`; the docker branch never calls them. `ModuleDir` (F4) is consulted only for the terraform branch, as today.
- No secret (resolved credential value) ever appears in a Temporal activity **input** struct — only identifiers (`TenantID`, `RunID`) cross the workflow→activity boundary; the activity resolves the secret itself, from `SecretStore`, before use. This is what makes F1 safe against Temporal's history persistence.
- Task order matters: **Task 1 (F1) must land before Task 2 (F2) and Task 4 (F4)** — both extend the same `NewProviderForRef`/`Deps` signature Task 1 establishes (`ctx context.Context, ref *dslpb.ProviderRef, run RunContext, deps Deps`). Task 3 (F3) and Task 5 (F5) are independent of the others and of each other.
- Every existing caller of a changed signature (`internal/infrastructure/provider/factory_test.go`, `internal/infrastructure/execution/recipe_activities.go`, `internal/infrastructure/provider/terraform_adapter_test.go`) is updated in the same task that changes the signature — a task is not "done" while the package fails to compile.
- **Blocking finding, not fixed by this plan (see Task 4's caveat):** the embedded terraform module this whole plan wires credentials/logs/workdirs into (`deployments/terraform/yandex/*.tf`) does not declare the DSL-pivot `stroppy_nodes`/`stroppy_machines` variable/output contract `internal/infrastructure/provider/terraform.go` assumes (`grep stroppy_nodes deployments/terraform/yandex/*.tf` — zero matches). Task 4's live acceptance test cannot pass until that HCL is updated; that HCL authoring is out of this plan's Go-code scope and is flagged in Self-Review as a prerequisite, not silently absorbed into F4's "done".

---

### Task 1: F1 — per-tenant provider deploy credentials (`EnvFn`, not process-wide `Env`)

Replace `provider.Deps.Env map[string]string` (built once in `internal/app/run.go` from process env vars, shared by every run regardless of tenant) with `Deps.EnvFn func(ctx, tenantID) (map[string]string, error)`, resolved inside `ProvisionActivity`/`TeardownActivity` from a new `identity_secrets` namespace. Confirms product-vision's org≡tenant_id decision (SP-B, `docs/superpowers/specs/2026-07-08-sp-b-catalog-tenancy.md:297`): there is no separate `OrgID` — `ProvisionActivityInput`/`TeardownActivityInput` already carry `TenantID`/gain it, respectively.

**Files:**
- Modify: `internal/infrastructure/identity/secrets.go` (new `ProviderDeployCreds` type + shared `sealValue`/`openValue`/`newAEAD` helpers extracted from `ProviderSecrets`)
- Create: `internal/infrastructure/identity/provider_deploy_creds_test.go`
- Modify: `internal/infrastructure/provider/factory.go` (`Deps.Env` → `Deps.EnvFn`; `NewProviderForRef` gains `ctx`/`RunContext`)
- Modify: `internal/infrastructure/provider/factory_test.go` (update 5 existing call sites to the new signature; add EnvFn coverage)
- Modify: `internal/infrastructure/execution/recipe_activities.go` (`ProvisionActivity`/`TeardownActivity` build `provider.RunContext` and pass `ctx`)
- Modify: `internal/workflows/runrecipe.go` (`TeardownActivityInput` gains `TenantID`; `teardown()` populates it)
- Modify: `internal/app/run.go` (drop `providerEnv`; wire `ProviderDeployCreds.Get` as `EnvFn`)

**Interfaces:**
```go
// internal/infrastructure/provider/factory.go
type RunContext struct {
    TenantID string
    RunID    string
}

type Deps struct {
    DockerExec  dockerExec
    Actor       tfActor
    EnvFn       func(ctx context.Context, tenantID string) (map[string]string, error)
    ModuleDir   func(providerName string) (dir string, tfFiles []terraform.TfFile, ok bool) // unchanged this task
    AgentTokens AgentTokenIssuer
}

func NewProviderForRef(ctx context.Context, ref *dslpb.ProviderRef, run RunContext, deps Deps) (Provider, error)
```
```go
// internal/infrastructure/identity/secrets.go
func NewProviderDeployCreds(store SecretStore, cfg Config) (*ProviderDeployCreds, error)
func (s *ProviderDeployCreds) Get(ctx context.Context, tenantID string) (map[string]string, error) // matches Deps.EnvFn's shape exactly
func (s *ProviderDeployCreds) Set(ctx context.Context, tenantID string, env map[string]string) error
```

- [ ] **Step 1: Write the failing tests**

`internal/infrastructure/identity/provider_deploy_creds_test.go`:
```go
package identity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
)

type fakeSecretStore struct {
	values map[string]map[string]string // namespace -> key -> value
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{values: map[string]map[string]string{}}
}

func (f *fakeSecretStore) Put(_ context.Context, namespace, key, value string) error {
	if f.values[namespace] == nil {
		f.values[namespace] = map[string]string{}
	}
	f.values[namespace][key] = value
	return nil
}

func (f *fakeSecretStore) Fetch(_ context.Context, namespace, key string) (string, error) {
	v, ok := f.values[namespace][key]
	if !ok {
		return "", derrors.ErrNotFound
	}
	return v, nil
}

func (f *fakeSecretStore) Remove(_ context.Context, namespace, key string) error {
	delete(f.values[namespace], key)
	return nil
}

func TestProviderDeployCreds_SetGet_RoundTripsPerTenant(t *testing.T) {
	store := newFakeSecretStore()
	creds, err := NewProviderDeployCreds(store, Config{SecretEncryptionKey: make([]byte, 32)})
	require.NoError(t, err)

	require.NoError(t, creds.Set(context.Background(), "tenant-a", map[string]string{"YC_TOKEN": "token-a"}))
	require.NoError(t, creds.Set(context.Background(), "tenant-b", map[string]string{"YC_TOKEN": "token-b"}))

	gotA, err := creds.Get(context.Background(), "tenant-a")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"YC_TOKEN": "token-a"}, gotA)

	gotB, err := creds.Get(context.Background(), "tenant-b")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"YC_TOKEN": "token-b"}, gotB, "tenant-b must never see tenant-a's token")

	// Sealed at rest: raw store value must not contain the plaintext token.
	raw, err := store.Fetch(context.Background(), nsProviderDeployCred, "tenant-a")
	require.NoError(t, err)
	require.NotContains(t, raw, "token-a")
}

func TestProviderDeployCreds_Get_UnknownTenant_ReturnsNotFound(t *testing.T) {
	store := newFakeSecretStore()
	creds, err := NewProviderDeployCreds(store, Config{SecretEncryptionKey: make([]byte, 32)})
	require.NoError(t, err)

	_, err = creds.Get(context.Background(), "no-such-tenant")
	require.ErrorIs(t, err, derrors.ErrNotFound,
		"no configured creds must be an explicit not-found error, never a silent fallback to process env")
}
```

`internal/infrastructure/provider/factory_test.go` — add:
```go
func TestNewProviderForRef_Yandex_ResolvesEnvViaEnvFnWithTenantID(t *testing.T) {
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	var gotTenant string
	deps := Deps{
		Actor: &fakeTfActor{},
		ModuleDir: func(name string) (string, []terraform.TfFile, bool) {
			return "yandex", tfFiles, true
		},
		EnvFn: func(_ context.Context, tenantID string) (map[string]string, error) {
			gotTenant = tenantID
			return map[string]string{"YC_TOKEN": "resolved-secret"}, nil
		},
	}

	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "yandex"},
		RunContext{TenantID: "tenant-1", RunID: "run-1"}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, "tenant-1", gotTenant)
}

func TestNewProviderForRef_Yandex_EnvFnError_Propagates(t *testing.T) {
	deps := Deps{
		Actor:     &fakeTfActor{},
		ModuleDir: func(string) (string, []terraform.TfFile, bool) { return "yandex", nil, true },
		EnvFn: func(context.Context, string) (map[string]string, error) {
			return nil, fmt.Errorf("no deploy credentials configured for tenant")
		},
	}
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "yandex"},
		RunContext{TenantID: "tenant-1"}, deps)
	require.Error(t, err)
	require.Nil(t, p)
}

func TestNewProviderForRef_Docker_NeverCallsEnvFn(t *testing.T) {
	called := false
	deps := Deps{
		DockerExec: &fakeDockerExec{},
		EnvFn: func(context.Context, string) (map[string]string, error) {
			called = true
			return nil, nil
		},
	}
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "docker"},
		RunContext{TenantID: "tenant-1"}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.False(t, called, "docker builtin needs no terraform credentials")
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./internal/infrastructure/identity/... ./internal/infrastructure/provider/...` → compile failure: `undefined: NewProviderDeployCreds`, `NewProviderForRef` called with wrong arg count, `Deps.EnvFn` undefined.

- [ ] **Step 3: Write minimal implementation**

`internal/infrastructure/identity/secrets.go` — extract shared seal/open, add the new adapter:
```go
const (
	nsApiTokenHash       = "api_token_hash"
	nsProviderSecret     = "provider_secret"
	nsProviderDeployCred = "provider_deploy_cred" // F1: distinct from nsProviderSecret (OIDC client secrets) —
	                                               // see docs/superpowers/specs/2026-07-08-sp-f-execution-shapeup.md §3 F1.
)

// newAEAD builds the AES-GCM cipher shared by every sealed secret adapter in
// this file, from the single configured SecretEncryptionKey.
func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) == 0 {
		return nil, derrors.FailedPrecondition("identity.secret_key_missing", "secret encryption key is not configured")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.secret_key_invalid", "secret encryption key must be 16, 24 or 32 bytes").Wrap(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.secret_key_invalid", "failed to initialise AES-GCM").Wrap(err)
	}
	return aead, nil
}

// sealValue/openValue: shared AES-GCM seal/open, used by both ProviderSecrets
// (OIDC client secrets) and ProviderDeployCreds (F1: terraform deploy creds)
// so the two adapters do not duplicate sealing logic.
func sealValue(aead cipher.AEAD, plaintext string) (string, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func openValue(aead cipher.AEAD, sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", derrors.Internal("stored secret is corrupt").Wrap(err)
	}
	ns := aead.NonceSize()
	if len(raw) < ns {
		return "", derrors.Internal("stored secret is truncated")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", derrors.Internal("failed to decrypt secret").Wrap(err)
	}
	return string(plaintext), nil
}
```
Rewrite `NewProviderSecrets` to call `newAEAD` (drop its inline `aes.NewCipher`/`cipher.NewGCM`), and its `seal`/`open` methods to call `sealValue`/`openValue` — same behavior, no duplicated crypto code.

Add the new adapter:
```go
// ProviderDeployCreds implements provider.Deps.EnvFn's exact shape (Get) for
// F1: it resolves the per-tenant terraform-provider deploy credentials
// (YC_TOKEN and friends) previously hardcoded as a single process-wide env
// map in internal/app/run.go. Sealed at rest (same key as ProviderSecrets,
// different namespace — nsProviderDeployCred is NOT nsProviderSecret, which
// is OIDC client secrets for SSO, an unrelated concept that happens to share
// the word "provider").
type ProviderDeployCreds struct {
	store SecretStore
	aead  cipher.AEAD
}

func NewProviderDeployCreds(store SecretStore, cfg Config) (*ProviderDeployCreds, error) {
	aead, err := newAEAD(cfg.SecretEncryptionKey)
	if err != nil {
		return nil, err
	}
	return &ProviderDeployCreds{store: store, aead: aead}, nil
}

func (s *ProviderDeployCreds) Set(ctx context.Context, tenantID string, env map[string]string) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal provider deploy creds: %w", err)
	}
	sealed, err := sealValue(s.aead, string(raw))
	if err != nil {
		return err
	}
	return s.store.Put(ctx, nsProviderDeployCred, tenantID, sealed)
}

// Get resolves tenantID's deploy credential env map. Its signature matches
// provider.Deps.EnvFn exactly, so internal/app/run.go assigns it directly
// (EnvFn: providerDeployCreds.Get) with no wrapper. Propagates
// derrors.ErrNotFound for a tenant with no configured credentials — the
// caller (NewProviderForRef) must surface that as an explicit provisioning
// error, never fall back to a process-wide credential set.
func (s *ProviderDeployCreds) Get(ctx context.Context, tenantID string) (map[string]string, error) {
	sealed, err := s.store.Fetch(ctx, nsProviderDeployCred, tenantID)
	if err != nil {
		return nil, err
	}
	plaintext, err := openValue(s.aead, sealed)
	if err != nil {
		return nil, err
	}
	var env map[string]string
	if err := json.Unmarshal([]byte(plaintext), &env); err != nil {
		return nil, derrors.Internal("stored provider deploy creds are corrupt").Wrap(err)
	}
	return env, nil
}

func (s *ProviderDeployCreds) Delete(ctx context.Context, tenantID string) error {
	return s.store.Remove(ctx, nsProviderDeployCred, tenantID)
}
```
Add `"encoding/json"` to the file's imports.

`internal/infrastructure/provider/factory.go`:
```go
// RunContext carries the per-run identifiers NewProviderForRef needs beyond
// the provider ref itself: which tenant owns the run (F1: credential scope,
// F2: log labeling) and which run this is (F2: log labeling, F4: workdir
// isolation). RecipeActivities (Task 4's ProvisionActivity/TeardownActivity)
// builds one per activity invocation from its own Input's TenantID/RunID.
type RunContext struct {
	TenantID string
	RunID    string
}

type Deps struct {
	DockerExec dockerExec
	Actor      tfActor
	// EnvFn resolves the terraform provider's deploy credentials (e.g.
	// YC_TOKEN) for tenantID, called once per NewProviderForRef invocation —
	// i.e. once per ProvisionActivity/TeardownActivity call, inside the
	// activity (never inside the workflow, so no secret value is ever
	// serialized into Temporal's workflow history). Replaces the old static
	// Env map[string]string (was resolved once, process-wide, from
	// os.Getenv — every tenant shared one YC account). nil is tolerated for
	// docker-only deployments (the docker branch never calls it); a nil
	// EnvFn used on the terraform branch is an EnvFn-caller error, not
	// silently treated as "no credentials" — see envFor below.
	EnvFn func(ctx context.Context, tenantID string) (map[string]string, error)
	ModuleDir func(providerName string) (dir string, tfFiles []terraform.TfFile, ok bool)
	AgentTokens AgentTokenIssuer
}

// NewProviderForRef returns the Provider that provisions/destroys machines
// for ref. ctx/run are used only by the terraform branch (EnvFn/ModuleDir
// resolution); the docker branch ignores both — see Deps.EnvFn's doc.
func NewProviderForRef(ctx context.Context, ref *dslpb.ProviderRef, run RunContext, deps Deps) (Provider, error) {
	name := ref.GetName()

	if name == "docker" {
		if deps.DockerExec == nil {
			return nil, fmt.Errorf("provider %q: no DockerExec configured", name)
		}
		return NewDocker(deps.DockerExec, deps.AgentTokens), nil
	}

	if deps.ModuleDir == nil {
		return nil, fmt.Errorf("provider %q: no ModuleDir resolver configured", name)
	}
	dir, tfFiles, ok := deps.ModuleDir(name)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}

	env, err := envFor(ctx, deps.EnvFn, run.TenantID)
	if err != nil {
		return nil, fmt.Errorf("provider %q: resolve deploy credentials for tenant %q: %w", name, run.TenantID, err)
	}

	return NewTerraform(dir, NewTerraformActorExec(deps.Actor, tfFiles, env)), nil
}

// envFor resolves deps.EnvFn against tenantID, tolerating a nil EnvFn as "no
// credentials configured" (empty env, not an error) — matches today's
// behavior for docker-only dev/test wiring that never sets EnvFn at all.
func envFor(ctx context.Context, fn func(context.Context, string) (map[string]string, error), tenantID string) (map[string]string, error) {
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, tenantID)
}
```

`internal/infrastructure/provider/factory_test.go` — update the 5 existing calls from `NewProviderForRef(ref, deps)` to `NewProviderForRef(context.Background(), ref, RunContext{}, deps)` (add `"context"` import).

`internal/infrastructure/execution/recipe_activities.go`:
```go
func (a *RecipeActivities) ProvisionActivity(
	ctx context.Context,
	in *workflows.ProvisionActivityInput,
) (*workflows.ProvisionActivityOutput, error) {
	if in == nil {
		return nil, errors.New("provision activity: input is required")
	}

	ref, err := enrichDockerRuntimeParams(in.ProviderRef, in.RunID, in.ServerAddr, in.GatewayGroup, in.TenantID)
	if err != nil {
		return nil, err
	}

	run := provider.RunContext{TenantID: in.TenantID, RunID: in.RunID}
	p, err := provider.NewProviderForRef(ctx, ref, run, a.deps)
	if err != nil {
		return nil, err
	}

	machines, err := p.Provision(ctx, ref, in.Groups)
	if err != nil {
		return nil, err
	}
	return &workflows.ProvisionActivityOutput{Machines: machines}, nil
}
```
Same substitution (`run := provider.RunContext{TenantID: in.TenantID, RunID: in.RunID}; p, err := provider.NewProviderForRef(ctx, ref, run, a.deps)`) in `TeardownActivity`.

`internal/workflows/runrecipe.go` — add `TenantID` to `TeardownActivityInput` and populate it:
```go
type TeardownActivityInput struct {
	ProviderRef *dslpb.ProviderRef
	RunID      string
	ServerAddr string
	TenantID   string // F1: EnvFn credential scope — Destroy resolves the same tenant's creds Provision used.
}
```
```go
return workflow.ExecuteActivity(actx, TeardownActivityName, &TeardownActivityInput{
	ProviderRef: ref,
	RunID:       w.in.RunID,
	ServerAddr:  w.in.Bootstrap.GetServerAddr(),
	TenantID:    w.in.TenantID,
}).Get(actx, nil)
```

`internal/app/run.go` — replace the `providerEnv`/`Env:` block:
```go
providerDeployCreds, err := identity.NewProviderDeployCreds(store.Secrets(), idCfg)
if err != nil {
	return fmt.Errorf("provider deploy creds: %w", err)
}
providerDeps := provider.Deps{
	DockerExec:  provider.NewDockerExecutorExec(dockerExecutor),
	Actor:       terraformActor,
	EnvFn:       providerDeployCreds.Get,
	AgentTokens: agentTokens,
	ModuleDir: func(name string) (string, []terraform.TfFile, bool) {
		if name != "yandex" {
			return "", nil, false
		}
		files, err := yandextf.EmbeddedTfFiles()
		if err != nil {
			return "", nil, false
		}
		return "yandex", files, true
	},
}
```
(`store.Secrets()`/`idCfg` are the same values `identity.NewProviderSecrets(store.Secrets(), idCfg)` already uses two lines above — no new construction inputs needed. Drop the `os.Getenv("YC_TOKEN")` etc. block entirely.)

- [ ] **Step 4: Run to verify pass**

`go test ./internal/infrastructure/identity/... ./internal/infrastructure/provider/... ./internal/infrastructure/execution/... ./internal/workflows/... ./internal/app/...` → PASS (the last one is a compile check — `internal/app` has no unit tests exercising this wiring directly).

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/identity/secrets.go internal/infrastructure/identity/provider_deploy_creds_test.go \
  internal/infrastructure/provider/factory.go internal/infrastructure/provider/factory_test.go \
  internal/infrastructure/execution/recipe_activities.go internal/workflows/runrecipe.go internal/app/run.go
git commit -m "feat(provider): resolve terraform deploy credentials per tenant, not process-wide"
```

---

### Task 2: F2 — per-run provisioning logs

`terraform.Actor.stdout/stderr` are actor-level (constructed once, `internal/app/run.go:422` calls `terraform.NewActor()` with no options → `os.Stdout`/`os.Stderr` for the whole process). `newTerraform` (`actor.go:345-361`) already calls `tf.SetStdout`/`tf.SetStderr` on every `Apply`/`Destroy` — the natural per-call injection point. Add a per-call writer that merges with the actor default via `io.MultiWriter`, threaded from a new `Deps.LogSinkFn(ctx, runID)`.

**Files:**
- Modify: `internal/infrastructure/terraform/actor.go` (`WithStdout`/`WithStderr` `Option`s + `Stdout()`/`Stderr()` accessors + `mergeWriter` in `newTerraform`)
- Create: `internal/infrastructure/terraform/actor_test.go`
- Modify: `internal/infrastructure/provider/terraform_adapter.go` (`terraformActorExec` gains stdout/stderr fields; `options()` appends them; update its stale "no accessor" doc comment)
- Modify: `internal/infrastructure/provider/terraform_adapter_test.go` (update the two `NewTerraformActorExec` calls; add stdout/stderr coverage)
- Modify: `internal/infrastructure/provider/factory.go` (`Deps.LogSinkFn`; `NewProviderForRef` resolves it)
- Modify: `internal/infrastructure/provider/factory_test.go` (LogSinkFn coverage)

**Interfaces:**
```go
// internal/infrastructure/terraform/actor.go
func WithStdout(w io.Writer) Option
func WithStderr(w io.Writer) Option
func (w *WorkdirWithParams) Stdout() io.Writer
func (w *WorkdirWithParams) Stderr() io.Writer

// internal/infrastructure/provider/factory.go
type Deps struct {
    // ...existing...
    LogSinkFn func(ctx context.Context, runID string) (stdout, stderr io.Writer) // F2
}
```

- [ ] **Step 1: Write the failing tests**

`internal/infrastructure/terraform/actor_test.go` (package `terraform`, whitebox — needed since `mergeWriter`/the `stdout`/`stderr` option-application are only observable from inside the package or via the new exported accessors):
```go
package terraform

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithStdoutStderr_SetFields(t *testing.T) {
	var out, errOut bytes.Buffer
	w := NewWorkdirWithParams(NewWdId("run-1"), WithStdout(&out), WithStderr(&errOut))
	require.Same(t, &out, w.Stdout())
	require.Same(t, &errOut, w.Stderr())
}

func TestWorkdirWithParams_NoStdoutOption_NilByDefault(t *testing.T) {
	w := NewWorkdirWithParams(NewWdId("run-1"))
	require.Nil(t, w.Stdout())
	require.Nil(t, w.Stderr())
}

func TestMergeWriter_NilPerRunFallsBackToActorDefault(t *testing.T) {
	var actorDefault bytes.Buffer
	got := mergeWriter(&actorDefault, nil)
	_, err := got.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, "hello", actorDefault.String())
}

func TestMergeWriter_BothWritersReceiveOutput(t *testing.T) {
	var actorDefault, perRun bytes.Buffer
	got := mergeWriter(&actorDefault, &perRun)
	_, err := got.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, "hello", actorDefault.String(), "actor-level default (dev/debug channel) still receives output")
	require.Equal(t, "hello", perRun.String(), "per-run writer (F2's log sink) receives the same output")
}
```

`internal/infrastructure/provider/terraform_adapter_test.go` — add:
```go
func TestTerraformActorExec_Apply_ThreadsStdoutStderrIntoWorkdir(t *testing.T) {
	fake := &fakeTfActor{applyOutput: terraform.TfOutput{"stroppy_machines": []byte(`[]`)}}
	var stdout, stderr bytes.Buffer
	adapter := NewTerraformActorExec(fake, nil, nil, &stdout, &stderr)

	_, err := adapter.Apply(context.Background(), "run-1", []byte(`{}`))
	require.NoError(t, err)
	require.NotNil(t, fake.applyWorkdir)
	require.Same(t, &stdout, fake.applyWorkdir.Stdout())
	require.Same(t, &stderr, fake.applyWorkdir.Stderr())
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./internal/infrastructure/terraform/... ./internal/infrastructure/provider/...` → FAIL: `undefined: mergeWriter`, `undefined: WithStdout`, `NewTerraformActorExec` called with 5 args against a 3-arg signature.

- [ ] **Step 3: Write minimal implementation**

`internal/infrastructure/terraform/actor.go`:
```go
type WorkdirWithParams struct {
	// ...existing fields...
	stdout io.Writer
	stderr io.Writer
}

// WithStdout/WithStderr set this apply/destroy call's per-run log sink (F2).
// nil (the default) means "no per-run sink" — newTerraform then writes only
// to the Actor's own default stdout/stderr (WithActorStdout/WithActorStderr),
// exactly as before this option existed.
func WithStdout(w io.Writer) Option { return func(wd *WorkdirWithParams) { wd.stdout = w } }
func WithStderr(w io.Writer) Option { return func(wd *WorkdirWithParams) { wd.stderr = w } }

// Stdout/Stderr expose the per-call writer set via WithStdout/WithStderr —
// needed by internal/infrastructure/provider's terraform_adapter_test.go,
// which (like WorkdirPath/StateFilePresent already did) can only observe
// WorkdirWithParams state through an exported accessor.
func (w *WorkdirWithParams) Stdout() io.Writer { return w.stdout }
func (w *WorkdirWithParams) Stderr() io.Writer { return w.stderr }
```
```go
// mergeWriter combines the Actor-level default (actorDefault, always
// non-nil — NewActor defaults it to os.Stdout/os.Stderr or io.Discard) with
// a per-run writer (perRun, nil unless WithStdout/WithStderr was used for
// this call). Both keep receiving output when perRun is set — the actor
// default remains a live dev/debug channel even once every run also gets
// its own per-run sink (F2).
func mergeWriter(actorDefault, perRun io.Writer) io.Writer {
	if perRun == nil {
		return actorDefault
	}
	return io.MultiWriter(actorDefault, perRun)
}
```
In `newTerraform`, replace:
```go
tf.SetStdout(a.stdout)
tf.SetStderr(a.stderr)
```
with:
```go
tf.SetStdout(mergeWriter(a.stdout, w.stdout))
tf.SetStderr(mergeWriter(a.stderr, w.stderr))
```

`internal/infrastructure/provider/terraform_adapter.go`:
```go
// terraformActorExec adapts terraform.Actor ... tfFiles is the module's
// embedded HCL (constant per module), env carries provider credentials
// (F1, resolved once per NewProviderForRef call), and stdout/stderr are this
// run's per-run log sink (F2, nil when no LogSinkFn is configured — every
// apply/destroy then falls back to the Actor's own default writer, unchanged
// pre-F2 behavior).
type terraformActorExec struct {
	actor         tfActor
	tfFiles       []terraform.TfFile
	env           map[string]string
	stdout, stderr io.Writer
}

func NewTerraformActorExec(actor tfActor, tfFiles []terraform.TfFile, env map[string]string, stdout, stderr io.Writer) *terraformActorExec {
	return &terraformActorExec{actor: actor, tfFiles: tfFiles, env: env, stdout: stdout, stderr: stderr}
}

func (a *terraformActorExec) options(varsJSON []byte) []terraform.Option {
	return []terraform.Option{
		terraform.WithTfFiles(a.tfFiles),
		terraform.WithVarFile(terraform.TfVarFile(varsJSON)),
		terraform.WithVarFileName(terraform.DefaultVarFileName),
		terraform.WithEnv(a.env),
		terraform.WithParallelism(10),
		terraform.WithPreserveExistingState(true),
		terraform.WithStdout(a.stdout),
		terraform.WithStderr(a.stderr),
	}
}
```
(Add `"io"` to imports.) Update the file's stale test-file comment ("tfFiles/varFile/env are unexported and the terraform package intentionally exposes no accessor for them") in `terraform_adapter_test.go` to note stdout/stderr now DO have accessors, so the comment does not read as contradicted by the new test added in Step 1.

`internal/infrastructure/provider/factory.go`:
```go
type Deps struct {
	// ...
	LogSinkFn func(ctx context.Context, runID string) (stdout, stderr io.Writer)
}

func NewProviderForRef(ctx context.Context, ref *dslpb.ProviderRef, run RunContext, deps Deps) (Provider, error) {
	// ...docker branch unchanged...
	// ...ModuleDir/env resolution as Task 1...
	stdout, stderr := logSinkFor(ctx, deps.LogSinkFn, run.RunID)
	return NewTerraform(dir, NewTerraformActorExec(deps.Actor, tfFiles, env, stdout, stderr)), nil
}

// logSinkFor resolves deps.LogSinkFn against runID, tolerating a nil
// LogSinkFn as "no per-run sink" (nil, nil) — every apply/destroy then falls
// back to the terraform.Actor's own default writer, matching pre-F2 behavior
// for callers (tests, docker-only dev wiring) that never set it.
func logSinkFor(ctx context.Context, fn func(context.Context, string) (io.Writer, io.Writer), runID string) (io.Writer, io.Writer) {
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, runID)
}
```
`NewTerraformActorExec(deps.Actor, tfFiles, env)` (Task 1's non-EnvFn tests / any other pre-Task-2 call site) becomes `NewTerraformActorExec(deps.Actor, tfFiles, env, nil, nil)` where Task 1 did not already pass stdout/stderr.

- [ ] **Step 4: Run to verify pass**

`go test ./internal/infrastructure/terraform/... ./internal/infrastructure/provider/...` → PASS.

- [ ] **Step 5: Wire a real LogSinkFn in `internal/app/run.go` and note the docker gap**

```go
providerDeps.LogSinkFn = func(ctx context.Context, runID string) (io.Writer, io.Writer) {
	// v1: label every line with run_id and forward to the same slog logger
	// the rest of the control-plane uses; a dedicated per-run log backend
	// (Vector/VictoriaMetrics Logs, matching agent-side logs — see
	// product-vision §6) is an open question (spec §9.5) this plan does not
	// resolve, since the control-plane process has no Vector sidecar of its
	// own the way agent nodes do.
	prefix := fmt.Sprintf("[run=%s][terraform] ", runID)
	return &prefixWriter{prefix: prefix, w: log.Writer()}, &prefixWriter{prefix: prefix, w: log.Writer()}
}
```
This plan intentionally does NOT wire `LogSinkFn` into the docker path (`internal/infrastructure/provider/docker_adapter.go`/`docker.go`): unlike `tfexec.Terraform`, `docker.Executor` drives containers through the Docker SDK (`ContainerCreate`/`ContainerStart`), which produces no live stdout/stderr stream to redirect the way a subprocess does — capturing a container's own logs would need a `ContainerLogs` fetch (a genuinely new, testable primitive, but a separate unit of work with its own interface/test surface) rather than a redirect of something that already exists. Flagged in Self-Review as deferred, not silently dropped.

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/terraform/actor.go internal/infrastructure/terraform/actor_test.go \
  internal/infrastructure/provider/terraform_adapter.go internal/infrastructure/provider/terraform_adapter_test.go \
  internal/infrastructure/provider/factory.go internal/infrastructure/provider/factory_test.go internal/app/run.go
git commit -m "feat(terraform): capture per-run apply/destroy output via LogSinkFn"
```

---

### Task 3: F3 — `executeServiceJob` wait-for-healthy (Variant A: expose the existing timeout)

**Correction versus the spec's own §4 interfaces sketch:** `NomadSubmitJobInput.WaitTimeout time.Duration` **already exists** (`internal/agent/nomad_activities.go:45-49`) and the poll loop (`nomad_activities.go:101-146`) already honors it (`waitTimeout := in.WaitTimeout; if waitTimeout <= 0 { waitTimeout = defaultNomadWaitTimeout }`). No backend/activity change is needed for Variant A. The actual gap is narrower: `executeServiceJob` (`internal/workflows/dslrun.go:666-668`) always constructs `NomadSubmitJobInput{JobJSON: jobJSON}` — `WaitTimeout` is left zero, so every service job silently gets the 5-minute default regardless of the DSL author's intent. `dslpb.ServiceSpec` already carries a `HealthCheck health = 8` field (`http`, `timeout`) that `internal/dsl/nomad/jobspec.go:147` lowers into the Nomad job's own native HTTP service check (Nomad-scheduler-level, unrelated to `NomadSubmitJobActivity`'s alloc-status poll) — `health.timeout` is exactly the DSL-author-facing knob this task threads into `WaitTimeout` too, so one `workflow.yaml` field now governs both "how long Nomad's own health check waits" and "how long the workflow waits for the alloc to come up."

A second, independent bug this task also fixes: `executeServiceJob`'s `workflow.ActivityOptions.StartToCloseTimeout` is a fixed `10 * time.Minute` — a `health.timeout` above ~9 minutes would have Temporal cancel the activity call (`StartToCloseTimeout` exceeded) before `NomadSubmitJobActivity`'s own internal `WaitTimeout` deadline is ever reached, silently defeating a longer configured timeout.

**Files:**
- Modify: `internal/workflows/dslrun.go` (`executeServiceJob`: parse `svc.GetHealth().GetTimeout()`, thread into `NomadSubmitJobInput.WaitTimeout` and into a widened `StartToCloseTimeout`)
- Modify: `internal/workflows/dslrun_test.go` (add a service with `Health` to `buildTestPlan`'s coverage; assert `WaitTimeout` reaches the activity input)

**Interfaces:**
```go
// internal/workflows/dslrun.go — no new exported type; a small internal helper.
func parseHealthTimeout(raw string) (time.Duration, error)
```

- [ ] **Step 1: Write the failing test**

`internal/workflows/dslrun_test.go` — add a plan variant and a targeted test (separate from `TestExecuteCompiledPlanWorkflow` so the existing fixture/assertions in that test stay untouched):
```go
func TestExecuteCompiledPlanWorkflow_ServiceHealthTimeout_ThreadsIntoNomadWaitTimeout(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerCompiledPlanActivityStubs(env)

	plan := buildTestPlan()
	plan.Services[0].Health = &dslpb.HealthCheck{Http: ":8080/health", Timeout: "45s"}

	var gotWaitTimeout time.Duration
	var gotWaitTimeoutSet bool
	env.OnActivity(NomadSubmitJobActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, in *stroppyagent.NomadSubmitJobInput) (*stroppyagent.NomadSubmitJobOutput, error) {
			gotWaitTimeout = in.WaitTimeout
			gotWaitTimeoutSet = true
			return &stroppyagent.NomadSubmitJobOutput{JobID: "echo", EvalID: "eval-1"}, nil
		},
	)
	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		&common.Cmd_Result{ExitCode: 0}, nil)
	env.OnActivity(workflowpb.EnsureAgentOnlineActivityActivityName, mock.Anything).Return(nil)

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, &ExecuteCompiledPlanInput{
		Plan:          plan,
		Machines:      buildTestInput().Machines,
		Bootstrap:     buildTestInput().Bootstrap,
		GatewayNodeID: buildTestInput().GatewayNodeID,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.True(t, gotWaitTimeoutSet, "NomadSubmitJobActivity was never called")
	require.Equal(t, 45*time.Second, gotWaitTimeout,
		"service.health.timeout must thread into NomadSubmitJobInput.WaitTimeout")
}

func TestExecuteCompiledPlanWorkflow_ServiceNoHealth_LeavesWaitTimeoutZero(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerCompiledPlanActivityStubs(env)

	plan := buildTestPlan() // plan.Services[0].Health is nil, as today

	var gotWaitTimeout time.Duration
	env.OnActivity(NomadSubmitJobActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, in *stroppyagent.NomadSubmitJobInput) (*stroppyagent.NomadSubmitJobOutput, error) {
			gotWaitTimeout = in.WaitTimeout
			return &stroppyagent.NomadSubmitJobOutput{JobID: "echo", EvalID: "eval-1"}, nil
		},
	)
	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		&common.Cmd_Result{ExitCode: 0}, nil)
	env.OnActivity(workflowpb.EnsureAgentOnlineActivityActivityName, mock.Anything).Return(nil)

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, buildTestInput())

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Zero(t, gotWaitTimeout, "no health block -> zero WaitTimeout -> activity's own 5m default applies, unchanged")
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./internal/workflows/ -run TestExecuteCompiledPlanWorkflow_ServiceHealthTimeout -v` → the first assertion (`45*time.Second`) FAILs against today's code (`gotWaitTimeout` is always zero).

- [ ] **Step 3: Write minimal implementation**

`internal/workflows/dslrun.go`, inside `executeServiceJob`:
```go
	nomadJob, err := dslnomad.BuildJob(evaluatedSvc, nodes)
	if err != nil {
		return fmt.Errorf("job %q: build nomad job for service %q: %w", job.GetId(), svcName, err)
	}
	jobJSON, err := json.Marshal(nomadJob)
	if err != nil {
		return fmt.Errorf("job %q: marshal nomad job for service %q: %w", job.GetId(), svcName, err)
	}

	// waitTimeout reuses the service's own health.timeout (already a
	// compile-validated time.ParseDuration string — see
	// internal/dsl/lower/lower.go's lowerHealth) as the budget
	// NomadSubmitJobActivity polls allocations for. A service with no health
	// block (waitTimeout == 0) leaves NomadSubmitJobActivity's own default
	// (5m, defaultNomadWaitTimeout) unchanged — this is additive, not a
	// behavior change for existing recipes (F3).
	waitTimeout, err := parseHealthTimeout(svc.GetHealth().GetTimeout())
	if err != nil {
		return fmt.Errorf("job %q: service %q: health.timeout: %w", job.GetId(), svcName, err)
	}

	taskQueue, err := agentTaskQueue(in.Bootstrap, in.GatewayNodeID)
	if err != nil {
		return fmt.Errorf("job %q: gateway task queue: %w", job.GetId(), err)
	}
	// activityTimeout must exceed waitTimeout (NomadSubmitJobActivity's own
	// internal poll deadline) or Temporal cancels the activity call before
	// that deadline is ever reached, silently truncating a longer configured
	// health.timeout. 5 minutes of headroom covers job registration +
	// scheduling overhead beyond the alloc-running wait itself.
	activityTimeout := 10 * time.Minute
	if waitTimeout > 0 {
		activityTimeout = waitTimeout + 5*time.Minute
	}
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           taskQueue,
		StartToCloseTimeout: activityTimeout,
		HeartbeatTimeout:    time.Minute,
	})
	var out stroppyagent.NomadSubmitJobOutput
	if err := workflow.ExecuteActivity(activityCtx, NomadSubmitJobActivityName, &stroppyagent.NomadSubmitJobInput{
		JobJSON:     jobJSON,
		WaitTimeout: waitTimeout,
	}).Get(activityCtx, &out); err != nil {
		return fmt.Errorf("job %q: nomad submit %q: %w", job.GetId(), svcName, err)
	}
	return nil
}

// parseHealthTimeout parses a ServiceSpec's health.timeout into the wait
// budget NomadSubmitJobActivity should poll for. Empty (no health block on
// this service) returns zero, which NomadSubmitJobActivity treats as "use
// its own default" (defaultNomadWaitTimeout, 5m).
func parseHealthTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", raw, err)
	}
	return d, nil
}
```

- [ ] **Step 4: Run to verify pass**

`go test ./internal/workflows/ -run TestExecuteCompiledPlanWorkflow -v` → PASS, including the pre-existing `TestExecuteCompiledPlanWorkflow` (unaffected — its service has no `Health`, so `waitTimeout` stays zero exactly as before).

- [ ] **Step 5: Note Variant B, deferred**

Add a doc comment above `executeServiceJob` recording the deferred option (per spec §3 F3): decoupling submit from wait into two separate DAG-visible steps (new `ast`/`dslpb` step type, built on the already-existing-but-unused `NomadJobStatusActivity`, `internal/agent/nomad_activities.go:162-189`) so a Run's per-job status (SP-E) can show "waiting for healthy" distinct from "submitted", and so an operator can wait longer without re-submitting the job. Deferred until live testing (Task 4's acceptance run, or a later HA-topology recipe) shows Variant A's single-activity-timeout budget is insufficient for a real slow-starting database.

- [ ] **Step 6: Commit**

```bash
git add internal/workflows/dslrun.go internal/workflows/dslrun_test.go
git commit -m "feat(dslrun): thread service.health.timeout into Nomad wait-for-healthy"
```

---

### Task 4: F4 — per-run terraform workdir/state isolation

`Deps.ModuleDir` resolves a provider NAME to BOTH the module's embedded HCL files AND (via `terraformProvider.moduleDir` → `terraform.NewWdId(dir)`, `terraform_adapter.go:56,69`) the terraform **workdir id**. `internal/app/run.go`'s current closure always returns the constant `"yandex"` for both — so every run against the yandex provider shares one workdir/`terraform.tfstate`, and `Actor.register` (`actor.go:313-321`) rejects a second concurrent run with `ErrWdAlreadyExists`. Fix: `ModuleDir` derives a per-run id from `RunID` (already threaded into `RunContext` by Task 1) while the embedded HCL file set (the actual module content) stays shared/constant, exactly as `terraform.NewWorkdirWithParams` already supports (it accepts any `WdId` string — no change needed in `terraform/actor.go` itself, confirming the spec's own note that this is "generation happens above, in the provider").

**Files:**
- Create: `internal/infrastructure/provider/moduledir.go` (`YandexModuleDirResolver`, extracted from the inline closure in `run.go` so it is unit-testable without `yandextf`'s embed I/O)
- Create: `internal/infrastructure/provider/moduledir_test.go`
- Modify: `internal/infrastructure/provider/factory.go` (`Deps.ModuleDir` signature grows `tenantID, runID`; `NewProviderForRef` passes `run.TenantID, run.RunID`)
- Modify: `internal/infrastructure/provider/factory_test.go` (update `ModuleDir` fakes to the new 3-arg signature)
- Modify: `internal/app/run.go` (`ModuleDir: provider.YandexModuleDirResolver(files)`)

**Interfaces:**
```go
// internal/infrastructure/provider/moduledir.go
func YandexModuleDirResolver(tfFiles []terraform.TfFile) func(tenantID, runID, name string) (dir string, files []terraform.TfFile, ok bool)

// internal/infrastructure/provider/factory.go
type Deps struct {
    // ...
    ModuleDir func(tenantID, runID, name string) (dir string, tfFiles []terraform.TfFile, ok bool)
}
```

- [ ] **Step 1: Write the failing test**

`internal/infrastructure/provider/moduledir_test.go`:
```go
package provider

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
)

func TestYandexModuleDirResolver_PerRunWdId(t *testing.T) {
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	resolve := YandexModuleDirResolver(tfFiles)

	dirA, filesA, okA := resolve("tenant-1", "run-a", "yandex")
	require.True(t, okA)
	require.Equal(t, tfFiles, filesA)

	dirB, _, okB := resolve("tenant-1", "run-b", "yandex")
	require.True(t, okB)

	require.NotEqual(t, dirA, dirB, "two runs against yandex must not collide on the same terraform WdId")
	require.Equal(t, "yandex-run-a", dirA)
	require.Equal(t, "yandex-run-b", dirB)
}

func TestYandexModuleDirResolver_EmptyRunID_FallsBackToConstant(t *testing.T) {
	resolve := YandexModuleDirResolver(nil)
	dir, _, ok := resolve("tenant-1", "", "yandex")
	require.True(t, ok)
	require.Equal(t, "yandex", dir, "ad-hoc callers without a run id (tests, direct use) keep the pre-F4 constant id")
}

func TestYandexModuleDirResolver_UnknownName_NotOK(t *testing.T) {
	resolve := YandexModuleDirResolver(nil)
	_, _, ok := resolve("tenant-1", "run-a", "nope")
	require.False(t, ok)
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./internal/infrastructure/provider/ -run TestYandexModuleDirResolver -v` → FAIL: `undefined: YandexModuleDirResolver`.

- [ ] **Step 3: Write minimal implementation**

`internal/infrastructure/provider/moduledir.go`:
```go
package provider

import "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"

// YandexModuleDirResolver builds a Deps.ModuleDir resolver for the builtin
// "yandex" terraform module. tfFiles is the module's embedded HCL, loaded
// once by the caller (yandextf.EmbeddedTfFiles() in internal/app/run.go) —
// this function does no I/O, so it is directly unit-testable with a fake
// file set.
//
// The returned dir doubles as the terraform.WdId (see terraform_adapter.go's
// Apply/Destroy: terraform.NewWdId(dir)) — F4: before this, every call
// returned the constant "yandex", so two concurrent runs against the yandex
// provider collided on the same terraform.Actor workdir/state
// (terraform.Actor.register's ErrWdAlreadyExists). Deriving the id from
// runID ("yandex-<runID>") gives each run its own workdir/tfstate; the
// module's HCL content (tfFiles) is unaffected — it is copied into whichever
// per-run workdir terraform.Actor.prepare creates.
//
// An empty runID (ad-hoc callers: tests, or any future direct construction
// that has no run context) falls back to the pre-F4 constant "yandex" so
// those callers are unaffected.
func YandexModuleDirResolver(tfFiles []terraform.TfFile) func(tenantID, runID, name string) (string, []terraform.TfFile, bool) {
	return func(_, runID, name string) (string, []terraform.TfFile, bool) {
		if name != "yandex" {
			return "", nil, false
		}
		wdID := "yandex"
		if runID != "" {
			wdID = "yandex-" + runID
		}
		return wdID, tfFiles, true
	}
}
```

`internal/infrastructure/provider/factory.go`:
```go
type Deps struct {
	// ...
	ModuleDir func(tenantID, runID, name string) (dir string, tfFiles []terraform.TfFile, ok bool)
}

func NewProviderForRef(ctx context.Context, ref *dslpb.ProviderRef, run RunContext, deps Deps) (Provider, error) {
	// ...docker branch unchanged...
	if deps.ModuleDir == nil {
		return nil, fmt.Errorf("provider %q: no ModuleDir resolver configured", name)
	}
	dir, tfFiles, ok := deps.ModuleDir(run.TenantID, run.RunID, name)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	// ...env/log-sink resolution as Tasks 1/2...
}
```

`internal/infrastructure/provider/factory_test.go` — update the `ModuleDir` fakes:
```go
ModuleDir: func(tenantID, runID, name string) (string, []terraform.TfFile, bool) {
	require.Equal(t, "yandex", name)
	return "yandex", tfFiles, true
},
```
(4 call sites: `TestNewProviderForRef_Yandex_ReturnsTerraformBackedProvider`, `TestNewProviderForRef_UnknownProvider_ErrorsWithName`, plus the two Task 1 tests added above.)

`internal/app/run.go`:
```go
files, err := yandextf.EmbeddedTfFiles()
if err != nil {
	return fmt.Errorf("load embedded yandex terraform files: %w", err)
}
providerDeps := provider.Deps{
	DockerExec:  provider.NewDockerExecutorExec(dockerExecutor),
	Actor:       terraformActor,
	EnvFn:       providerDeployCreds.Get,
	LogSinkFn:   /* Task 2 */,
	AgentTokens: agentTokens,
	ModuleDir:   provider.YandexModuleDirResolver(files),
}
```
(`yandextf.EmbeddedTfFiles()` is now called once, eagerly, at wiring time — moved out of the per-call closure it lived in before, since it no longer needs to run per-`ModuleDir`-call; `YandexModuleDirResolver` takes the already-loaded files.)

- [ ] **Step 4: Run to verify pass**

`go test ./internal/infrastructure/provider/...` → PASS.

- [ ] **Step 5: Live acceptance (dev-stand, yandex) — BLOCKED, documented caveat**

Per this plan's Global Constraints, the embedded module (`deployments/terraform/yandex/*.tf`) does not yet declare `stroppy_nodes`/`stroppy_machines` (confirmed: `grep -rn stroppy_nodes deployments/terraform/yandex/*.tf` → no matches; `variables.tf` instead declares `network`/`compute` variables from the pre-DSL-pivot flow). `terraformProvider.Provision` (`internal/infrastructure/provider/terraform.go:67-127`) will fail immediately on a live `terraform apply` (`Value for undeclared variable`) regardless of this task's WdId fix. **This task's Go-side fix (per-run WdId) is complete and unit-tested; the live acceptance criterion from spec §8 ("full terraform cycle on dev-stand") cannot be exercised until `deployments/terraform/yandex/*.tf` is updated to the `stroppy_nodes`/`stroppy_machines` contract — that HCL authoring is a separate, out-of-Go-scope prerequisite, tracked in Self-Review, not silently folded into this task's "done."**

Once that HCL exists, the acceptance test is: run two `RunRecipeWorkflow`s concurrently against the yandex provider, confirm neither hits `ErrWdAlreadyExists`; kill the control-plane process between provision and teardown, confirm a subsequent teardown still destroys via `Actor.DestroyExisting`'s on-disk fallback (`actor.go:288-311`, unchanged by this task, live-verified here for the first time).

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/provider/moduledir.go internal/infrastructure/provider/moduledir_test.go \
  internal/infrastructure/provider/factory.go internal/infrastructure/provider/factory_test.go internal/app/run.go
git commit -m "feat(provider): derive per-run terraform workdir id instead of a constant"
```

---

### Task 5: F5 — carryovers (Nomad multi-node placement decision + docker teardown-after-restart fix)

Two unrelated fixes bundled under one task per the spec's own F5 grouping. Part A is a documented deferral (a design-note commit, no behavior change — `AssignNomadRoles` genuinely cannot be safely wired into the current provisioning flow without a bigger change, explained below). Part B is a real, TDD, unit-tested fix.

**Files:**
- Modify: `internal/domain/agent/nomad_roles.go` (doc-comment-only: record the two-phase-apply blocker)
- Modify: `internal/infrastructure/docker/executor.go` (`Executor.ContainersByNetwork`)
- Modify: `internal/infrastructure/provider/docker_adapter.go` (`dockerRunner.ContainersByNetwork`; `RemoveContainers` falls back to it when `byNet` is empty)
- Modify: `internal/infrastructure/provider/docker_adapter_test.go` (fallback coverage)

#### Part A: `AssignNomadRoles` — explicit deferral, not wired

**Why not wired in this plan.** `AssignNomadRoles(machineIDs, gatewayID, gatewayPrivateIP)` needs the gateway's private IP to compute every OTHER machine's `ServerAddr` for cloud-init (`internal/domain/agent/bootstrap.go:112-114`'s `NomadServerAddr`, consumed by `renderNomadHCL` at first-boot time). But `terraformProvider.Provision` (`internal/infrastructure/provider/terraform.go:67-127`) issues exactly ONE `terraform apply` for every requested node across every group simultaneously (`p.exec.Apply(ctx, p.moduleDir, varsJSON)`, single call) — every machine's IP, including the gateway's, becomes known only when that ONE apply returns, which is also the moment every OTHER machine's first-boot cloud-init has already been baked and submitted as part of the same apply. There is no point in this flow, today, where the gateway's IP is known before the other machines' cloud-init is rendered. Wiring `AssignNomadRoles` for real needs either (a) a two-phase apply (create the gateway machine first, read its IP, then apply the remaining machines with that IP baked into their `ext`/cloud-init) or (b) a boot-time discovery mechanism (clients poll/retry-join a DNS name or tag-based lookup instead of a static baked IP) — both are provisioning-flow redesigns, explicitly out of this plan's scope (§2: "не проектирует новую архитектуру размещения"). `internal/domain/agent/nomad_roles_test.go` already unit-tests `AssignNomadRoles`'s pure assignment logic in isolation — no additional test is added here.

- [ ] **Step 1: Record the blocker in code, not just in this plan**

`internal/domain/agent/nomad_roles.go`, extend the doc comment on `AssignNomadRoles`:
```go
// AssignNomadRoles decides each provisioned machine's Nomad role...
// (existing doc unchanged) ...
//
// NOT YET WIRED (SP-F carryover, see
// docs/superpowers/specs/2026-07-08-sp-f-execution-shapeup.md §3 F5): the
// only caller that could use this — RunRecipeWorkflow's terraform
// provisioning path — issues a single terraform apply for every machine at
// once (internal/infrastructure/provider/terraform.go's
// terraformProvider.Provision), so the gateway's private IP this function
// needs is not known until every OTHER machine's cloud-init has already been
// baked into that same apply call. Wiring this for real needs either a
// two-phase apply (gateway first, IP known, then the rest) or boot-time
// server discovery instead of a statically baked ServerAddr — both are
// provisioning-flow redesigns, out of scope for a point-fix.
func AssignNomadRoles(machineIDs []string, gatewayID, gatewayPrivateIP string) map[string]NomadAssignment {
```

- [ ] **Step 2: Commit**

```bash
git add internal/domain/agent/nomad_roles.go
git commit -m "docs(agent): record why AssignNomadRoles is not wired into provisioning yet"
```

#### Part B: docker `byNet` survives a control-plane restart

`dockerExecutorExec.byNet` (`docker_adapter.go:33-34`) is populated only by `track()` at runtime and is empty after a process restart, so `RemoveContainers` (`docker_adapter.go:93-127`) removes nothing — `TeardownActivity` silently leaks every container for that run. Every container IS already labeled `stroppy.cloud/run_id` (`internal/infrastructure/provider/docker.go:175-179,327-329`), and Docker itself tracks each container's network membership independently of this process — `RemoveContainers` receives the network name directly (`dockerNetworkName(runID) = "stroppy-" + runID`), so falling back to "ask Docker which containers are on this network" needs no label parsing at all.

- [ ] **Step 1: Write the failing test**

`internal/infrastructure/provider/docker_adapter_test.go` — add to `fakeDockerRunner` and a new test:
```go
// containersByNetworkResult/-Err let tests configure ContainersByNetwork's
// canned response; containersByNetworkCalledWith records the argument.
type fakeDockerRunner struct {
	upInput *deploymentpb.Docker_Input
	upOut   *deploymentpb.Docker_Output
	upErr   error

	downInput *deploymentpb.Docker_Input
	downOut   *deploymentpb.Docker_Output
	downErr   error
	downFn    func()

	containersByNetworkResult   []string
	containersByNetworkErr      error
	containersByNetworkCalledWith string
}

func (f *fakeDockerRunner) ContainersByNetwork(_ context.Context, networkName string) ([]string, error) {
	f.containersByNetworkCalledWith = networkName
	return f.containersByNetworkResult, f.containersByNetworkErr
}

func TestDockerExecutorExec_RemoveContainers_FallsBackToNetworkDiscovery_WhenByNetIsEmpty(t *testing.T) {
	fake := &fakeDockerRunner{
		containersByNetworkResult: []string{"stroppy-run1-node-0", "stroppy-run1-node-1"},
	}
	// A fresh adapter (as after a control-plane restart) has never tracked
	// anything for "stroppy-run1" via EnsureContainer.
	adapter := NewDockerExecutorExec(fake)

	err := adapter.RemoveContainers(context.Background(), "stroppy-run1")
	require.NoError(t, err)

	require.Equal(t, "stroppy-run1", fake.containersByNetworkCalledWith,
		"empty in-memory byNet must fall back to asking Docker which containers are on this network")
	require.NotNil(t, fake.downInput)
	require.Len(t, fake.downInput.GetContainers(), 2)
	require.Contains(t, fake.downInput.GetContainers(), "stroppy-run1-node-0")
	require.Contains(t, fake.downInput.GetContainers(), "stroppy-run1-node-1")
}

func TestDockerExecutorExec_RemoveContainers_UsesTrackedNames_WithoutCallingNetworkDiscovery(t *testing.T) {
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{
			Containers: map[string]*deploymentpb.Docker_ContainerOutput{
				"stroppy-run1-node-0": {Id: "c1"},
			},
		},
	}
	adapter := NewDockerExecutorExec(fake)
	_, err := adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "stroppy-run1-node-0", Network: "stroppy-run1"})
	require.NoError(t, err)

	err = adapter.RemoveContainers(context.Background(), "stroppy-run1")
	require.NoError(t, err)

	require.Empty(t, fake.containersByNetworkCalledWith,
		"a live-tracked byNet entry must not trigger the discovery fallback")
	require.Contains(t, fake.downInput.GetContainers(), "stroppy-run1-node-0")
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./internal/infrastructure/provider/ -run TestDockerExecutorExec_RemoveContainers -v` → compile failure: `fakeDockerRunner` (in the adapter, via `dockerRunner`) does not satisfy a `ContainersByNetwork` method the test file references before it exists on the interface; once added to the interface, `NewDockerExecutorExec`'s current implementation never calls it, so the first new test's assertion `require.Equal(t, "stroppy-run1", fake.containersByNetworkCalledWith)` FAILs (empty string).

- [ ] **Step 3: Write minimal implementation**

`internal/infrastructure/docker/executor.go` — add a network-membership lookup next to the existing `Up`/`Down`:
```go
import (
	// ...existing...
	"github.com/docker/docker/api/types/filters"
)

// ContainersByNetwork lists the names of every container currently attached
// to networkName, per Docker's own network-membership tracking — used as
// RemoveContainers' fallback (F5) when the in-process byNet map has nothing
// for this network (e.g. after a control-plane restart), since Docker's
// network membership survives this process' own restarts even though the
// in-memory tracker does not.
func (e *Executor) ContainersByNetwork(ctx context.Context, networkName string) ([]string, error) {
	containers, err := e.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("network", networkName)),
	})
	if err != nil {
		return nil, fmt.Errorf("list containers on network %q: %w", networkName, err)
	}
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		for _, name := range c.Names {
			names = append(names, strings.TrimPrefix(name, "/")) // docker prefixes names with "/"
		}
	}
	return names, nil
}
```

`internal/infrastructure/provider/docker_adapter.go`:
```go
type dockerRunner interface {
	Up(ctx context.Context, input *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error)
	Down(ctx context.Context, input *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error)
	// ContainersByNetwork lists containers currently attached to networkName
	// (F5): RemoveContainers' fallback when this process' in-memory byNet
	// tracker has nothing for networkName, e.g. after a control-plane
	// restart lost the EnsureContainer-populated tracking.
	ContainersByNetwork(ctx context.Context, networkName string) ([]string, error)
}
```
```go
func (a *dockerExecutorExec) RemoveContainers(ctx context.Context, networkName string) error {
	a.mu.Lock()
	names := a.byNet[networkName]
	a.mu.Unlock()

	if len(names) == 0 {
		discovered, err := a.exec.ContainersByNetwork(ctx, networkName)
		if err != nil {
			return fmt.Errorf("discover containers on network %q: %w", networkName, err)
		}
		names = discovered
	}

	containers := make(map[string]*deploymentpb.Docker_Container, len(names))
	for _, name := range names {
		containers[name] = &deploymentpb.Docker_Container{}
	}

	input := &deploymentpb.Docker_Input{
		Network:    &deploymentpb.Docker_Network{Name: networkName},
		Containers: containers,
	}

	if _, err := a.exec.Down(ctx, input); err != nil {
		return fmt.Errorf("docker down %q: %w", networkName, err)
	}

	a.mu.Lock()
	remaining := make([]string, 0, len(a.byNet[networkName]))
	for _, name := range a.byNet[networkName] {
		if !slices.Contains(names, name) {
			remaining = append(remaining, name)
		}
	}
	if len(remaining) == 0 {
		delete(a.byNet, networkName)
	} else {
		a.byNet[networkName] = remaining
	}
	a.mu.Unlock()

	return nil
}
```
(`e.cli.ContainerList`'s `All: true` matters: a container that failed to start, or was stopped but not removed, is still a teardown target.)

- [ ] **Step 4: Run to verify pass**

`go test ./internal/infrastructure/provider/... ./internal/infrastructure/docker/...` → PASS. Note: `internal/infrastructure/docker` has no existing unit test harness for a real `*client.Client` (it talks to a real docker daemon) — `ContainersByNetwork` itself is exercised only transitively via the provider-package fake in this task; a live check (`docker network create tmp && docker run --network tmp ... && ContainersByNetwork(ctx, "tmp")` against a real daemon) is a manual/live verification, not part of this task's automated suite, consistent with the rest of `internal/infrastructure/docker` (no existing `executor_test.go`).

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/docker/executor.go internal/infrastructure/provider/docker_adapter.go internal/infrastructure/provider/docker_adapter_test.go
git commit -m "fix(provider): recover docker teardown after a control-plane restart"
```

---

## Self-Review

**Spec coverage (SP-F §3):**
- F1 (per-org/tenant creds) → Task 1: `EnvFn`, new `identity_secrets` namespace `provider_deploy_cred`, resolved inside the activity, never in workflow input. ✓ Confirms via research that "org" ≡ `tenant_id` (SP-B decision, `docs/superpowers/specs/2026-07-08-sp-b-catalog-tenancy.md:297`) — this plan threads the existing `TenantID` field rather than inventing a new `OrgID`, a deliberate, grounded deviation from spec §4's sketch (`OrgID string`).
- F2 (per-run logs) → Task 2: per-call `terraform.Option` writers merged via `io.MultiWriter`, `Deps.LogSinkFn`. Docker-side capture explicitly deferred with a concrete technical reason (no subprocess stdout to redirect via the Docker SDK — would need a separate `ContainerLogs`-based primitive), not silently dropped. ✓ (partial, documented)
- F3 (wait-for-healthy) → Task 3: **corrects** the spec's own premise that `NomadSubmitJobInput.WaitTimeout` needs to be added — it already exists; the actual fix is threading `ServiceSpec.Health.Timeout` into it from `executeServiceJob`, plus fixing the `StartToCloseTimeout` ceiling that would otherwise truncate a longer configured wait. Variant B recorded as deferred in a doc comment per spec's own recommendation. ✓
- F4 (terraform live) → Task 4: per-run `WdId` via `YandexModuleDirResolver`, unit-tested, no `terraform/actor.go` changes needed (confirms spec's own claim). **Discloses a blocking finding the spec did not anticipate:** the embedded `deployments/terraform/yandex/*.tf` module does not implement the `stroppy_nodes`/`stroppy_machines` contract at all (zero grep matches) — F4's own live-acceptance criterion is unreachable without separate HCL work, flagged explicitly rather than glossed over. ✓ (Go-side fix complete; live acceptance blocked on a disclosed external prerequisite)
- F5 (carryovers) → Task 5: Part A is a reasoned, code-documented deferral (not a no-op — records exactly why and what unblocks it: two-phase apply or boot-time discovery); Part B is a full TDD fix using Docker's own network-membership tracking (simpler and more robust than the spec's suggested label-based fallback, since it needs no label parsing and can't miss a mislabeled container). ✓

**Deviations from spec §4's interface sketch, and why:**
1. `OrgID` → reused existing `TenantID` (see F1 above) — grounded in an actual repo-wide search, not a guess.
2. `LogSinkFn` returns `(io.Writer, io.Writer)`, not `(io.WriteCloser, io.WriteCloser)` — avoids introducing a lifecycle-management question (who closes it, when) this plan has no concrete backend to answer yet (spec §9's own open question 5); a `Writer` composes trivially into `io.MultiWriter` with zero closing obligations, deferring the Closer decision to whichever task actually stands up the log backend.
3. `ModuleDir` signature is `(tenantID, runID, name)` returning the dir/WdId directly, not a separate `RunContext` parameter — matches the existing `dir string` return already being reused as `WdId` downstream (no new concept introduced, `terraform_adapter.go` is untouched).

**Placeholder scan:** no TBD/TODO left unresolved; every code block is either real, compiling, grounded-in-the-actual-file logic, or an explicit, reasoned deferral (Task 5 Part A, Task 2's docker gap) with a stated reason and a stated unblock condition — never a silent stub.

**Order dependency respected:** Task 1 introduces `ctx`/`RunContext`/`EnvFn` on `NewProviderForRef`; Task 2 and Task 4 both extend that same signature additively (`LogSinkFn`, then `ModuleDir`'s new params) rather than each re-deriving it — implementers should land Tasks 1→2→4 in order. Task 3 and Task 5 have no dependency on the others and can be done in any order, including in parallel by a different implementer.

**External dependencies (per spec §7), not owned by this plan:**
- SP-B must close *where* `identity_secrets`' `provider_deploy_cred` namespace's rows are administratively populated (this plan builds `Get`/`Set` on `ProviderDeployCreds`; no admin UI/RPC to call `Set` exists yet — out of scope, same as the spec's own §9.1 open question).
- SP-E is the consumer of Task 2's per-run writer and Task 3's per-job wait status — this plan produces the signal (a writer, a `WaitTimeout`), not the Run-model storage/UI for it.
- `deployments/terraform/yandex/*.tf`'s `stroppy_nodes`/`stroppy_machines` contract (Task 4's live-acceptance blocker) is HCL authoring, not Go — a separate, disclosed prerequisite this plan does not attempt.
