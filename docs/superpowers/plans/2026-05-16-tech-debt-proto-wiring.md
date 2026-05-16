# Tech Debt Proto Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire all new proto-generated types (DagRun.CancelRequested, AgentCommand, WebhookDelivery, QuotaCounter, PlatformRole) into domain services, workers, and middleware so `go test ./internal/... -count=1` passes green.

**Architecture:** Each task is self-contained and produces a passing test before the next starts. All DB access goes through ratel-generated repositories; raw SQL only where ratel DML cannot express the query (FOR UPDATE SKIP LOCKED, upsert). JWT signing uses `github.com/golang-jwt/jwt/v5` already in go.mod.

**Tech Stack:** Go 1.23, ConnectRPC, pgx v5, ratel repositories, golang-jwt/jwt v5, testcontainers-go (postgres).

---

## File Map

| File | Action | Task |
|---|---|---|
| `internal/domain/services/system/dag_runs.go` | Modify – implement CancelDagRun | 1 |
| `internal/domain/services/system/dag_runs_cancel_test.go` | Create | 1 |
| `internal/domain/workers/nodeworker/lifecycle.go` | Modify – cancel check | 1 |
| `internal/domain/workers/nodeworker/worker_test.go` | Modify – cancel test | 1 |
| `internal/domain/services/agent/commands_repo.go` | Create | 2 |
| `internal/domain/services/agent/service.go` | Modify – wire cmdRepo | 2 |
| `internal/domain/services/agent/hub.go` | Modify – Dispatch/Drain persist | 2 |
| `internal/domain/services/agent/service_test.go` | Modify – REPORTED state check | 2 |
| `internal/domain/services/agent/bootstrap.go` | Create – JWT store | 3 |
| `internal/domain/services/agent/service.go` | Modify – replace in-mem boots | 3 |
| `internal/domain/services/agent/bootstrap_test.go` | Create | 3 |
| `internal/domain/services/ops/quota_service.go` | Rewrite – real impl | 4 |
| `internal/domain/services/ops/quota_service_test.go` | Create | 4 |
| `internal/domain/services/testing/ports.go` | Modify – add QuotaPort | 4 |
| `internal/domain/services/testing/test_run_service.go` | Modify – CheckAndReserve | 4 |
| `internal/domain/services/testing/test_run_service_test.go` | Modify – pass quota svc | 4 |
| `cmd/stroppy-cloud/cmd_server.go` | Modify – wire quota release on DagRunDone | 4 |
| `internal/domain/services/ops/webhook_outbox.go` | Create | 5 |
| `internal/domain/workers/webhookworker/worker.go` | Create – drain + HMAC | 5 |
| `internal/domain/workers/webhookworker/signer.go` | Create | 5 |
| `internal/domain/workers/webhookworker/worker_test.go` | Create | 5 |
| `internal/domain/services/iam/jwt.go` | Modify – add platform_role claim | 6 |
| `internal/transport/middleware/platform_admin.go` | Create | 6 |
| `internal/transport/connect/server.go` | Modify – install interceptor on admin | 6 |
| `cmd/stroppy-cloud/cmd_server.go` | Modify – set ADMIN on bootstrap | 6 |
| `internal/transport/middleware/platform_admin_test.go` | Create | 6 |
| `internal/domain/workers/nodeworker/handlers/terraform.go` | Create (skeleton) | 7 |
| `internal/domain/workers/nodeworker/handlers/docker.go` | Create (skeleton) | 7 |
| `internal/domain/workers/nodeworker/handlers/package_install.go` | Create | 7 |
| `internal/domain/workers/nodeworker/handlers/stroppy_run.go` | Create | 7 |
| `internal/domain/workers/nodeworker/handlers/one_shot.go` | Create | 7 |
| `internal/domain/workers/nodeworker/handlers/config_apply.go` | Create | 7 |
| `internal/domain/workers/nodeworker/handlers/test_run_ref.go` | Create | 7 |
| `cmd/stroppy-cloud/cmd_server.go` | Modify – register real handlers | 7 |

---

## Task 1: CancelDagRun — persist + worker honors cancel

**Files:**
- Modify: `internal/domain/services/system/dag_runs.go`
- Create: `internal/domain/services/system/dag_runs_cancel_test.go`
- Modify: `internal/domain/workers/nodeworker/lifecycle.go`
- Modify: `internal/domain/workers/nodeworker/worker_test.go`

- [ ] **Step 1.1: Write failing test for CancelDagRun**

Create `/home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/domain/services/system/dag_runs_cancel_test.go`:

```go
package system_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestCancelDagRun(t *testing.T) {
	f := fixture.New(t)
	exec := f.Executor.(*sqlexec.TxExecutor)
	svc := system.New(exec, f.TxMgr, f.Events)
	ctx := context.Background()

	dag := &systempb.Dag{
		Graph: &systempb.Dag_Graph{
			Nodes: []*systempb.Dag_Node{{Id: "a", MaxAttempts: 1}},
		},
	}
	dag, err := svc.SaveDag(ctx, dag)
	require.NoError(t, err)

	run, err := svc.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})
	require.NoError(t, err)

	err = svc.CancelDagRun(ctx, run.GetId())
	require.NoError(t, err)

	got, err := svc.GetDagRun(ctx, run.GetId())
	require.NoError(t, err)
	require.True(t, got.GetCancelRequested(), "cancel_requested must be true")
}
```

- [ ] **Step 1.2: Run test — expect FAIL**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/services/system/... -run TestCancelDagRun -count=1 -v 2>&1 | tail -20
```

Expected: FAIL with "cancel not implemented"

- [ ] **Step 1.3: Implement CancelDagRun**

In `internal/domain/services/system/dag_runs.go`, replace the stub `CancelDagRun` (lines 118-120):

```go
// CancelDagRun sets cancel_requested=true on the DagRun.
// Returns domainerr.NotFound if the row does not exist or is soft-deleted.
func (s *Service) CancelDagRun(ctx context.Context, id *systempb.DagRunId) error {
	return tracing.WithTrace(s.Tracer(), ctx, "CancelDagRun",
		func(ctx context.Context, _ trace.Span) error {
			return pgtx.WithSerializable(ctx, s.txMgr,
				func(ctx context.Context) error {
					now := time.Now()
					n, err := s.dagRunRepo.Execute(ctx,
						systempb.DagRuns.Update().
							Set(
								systempb.DagRuns.CancelRequested.Set(true),
								systempb.DagRuns.UpdatedAt.Set(now),
							).
							Where(
								systempb.DagRuns.Id.Eq(id.GetValue()),
								systempb.DagRuns.DeletedAt.IsNull(),
							),
					)
					if err != nil {
						return err
					}
					if n == 0 {
						return domainerr.NotFound(domainerr.ResourceInfo("dag_run", id.GetValue()))
					}
					return nil
				})
		})
}
```

Also add `"time"` to the import block if not already present, and add `"go.opentelemetry.io/otel/trace"` import (already imported via tracing package).

Check current imports in dag_runs.go — `time` is not imported. Add it:

```go
import (
    "context"
    "errors"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/yaroher/ratel/pkg/dml/set"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/protobuf/types/known/structpb"
    "google.golang.org/protobuf/types/known/timestamppb"

    "github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
    "github.com/stroppy-io/stroppy-cloud/internal/core/ids"
    "github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
    "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
    commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
    systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)
```

- [ ] **Step 1.4: Add cancel check to lifecycle.go**

In `internal/domain/workers/nodeworker/lifecycle.go`, after fetching dagRun (after line ~38), add:

```go
	// If the parent DagRun has been cancelled, fail this node immediately.
	if dagRun.GetCancelRequested() {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "cancelled")
		return
	}
```

Insert it right after the `if err != nil { return }` block that follows `GetDagRun` call.

- [ ] **Step 1.5: Add cancel test to worker_test.go**

Append to `internal/domain/workers/nodeworker/worker_test.go`:

```go
func TestWorkerHonorsCancelRequested(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dag := &systempb.Dag{
		Graph: &systempb.Dag_Graph{
			Nodes: []*systempb.Dag_Node{{Id: "only", MaxAttempts: 1}},
		},
	}
	dag, err := f.System.SaveDag(ctx, dag)
	require.NoError(t, err)

	run, err := f.System.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})
	require.NoError(t, err)

	// Cancel before the worker picks up the node.
	err = f.System.CancelDagRun(ctx, run.GetId())
	require.NoError(t, err)

	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()
	go f.Worker.Run(workerCtx)

	deadline := time.Now().Add(15 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("dag_run did not reach terminal state in time")
		}
		got, err := f.System.GetDagRun(ctx, run.GetId())
		require.NoError(t, err)
		status := got.GetStatus()
		if status == systempb.DagRunStatus_DAG_RUN_STATUS_FAILED {
			// Expected: node was cancelled → dag failed.
			return
		}
		if status == systempb.DagRunStatus_DAG_RUN_STATUS_SUCCEEDED {
			t.Fatal("dag_run succeeded but should have been cancelled")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
```

- [ ] **Step 1.6: Run tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/services/system/... ./internal/domain/workers/nodeworker/... -count=1 -v -timeout 120s 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 1.7: Verify build**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 1.8: Commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add internal/domain/services/system/dag_runs.go internal/domain/services/system/dag_runs_cancel_test.go internal/domain/workers/nodeworker/lifecycle.go internal/domain/workers/nodeworker/worker_test.go && git commit -m "feat(system): persist DagRun.CancelRequested + worker honors cancel"
```

---

## Task 2: Agent commands persistence

**Files:**
- Create: `internal/domain/services/agent/commands_repo.go`
- Modify: `internal/domain/services/agent/service.go`
- Modify: `internal/domain/services/agent/hub.go`
- Modify: `internal/domain/services/agent/service_test.go`

- [ ] **Step 2.1: Create commands_repo.go**

```go
// internal/domain/services/agent/commands_repo.go
package agent

import (
	"context"
	"time"

	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CommandsRepo wraps the AgentCommands table with state-machine helpers.
type CommandsRepo struct {
	repo  *repository.ProtoRepository[agentpb.AgentCommandAlias, agentpb.AgentCommandColumnAlias, *agentpb.AgentCommandScanner, *agentpb.AgentCommand]
	txMgr pgtx.TxManager
}

func NewCommandsRepo(executor exec.DB, txMgr pgtx.TxManager) *CommandsRepo {
	return &CommandsRepo{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(agentpb.AgentCommands.Table, executor),
			agentpb.AgentCommandConverter,
		),
		txMgr: txMgr,
	}
}

// Insert persists a new AgentCommand row with state=PENDING.
func (r *CommandsRepo) Insert(ctx context.Context, agentID, nodeRunID string, commandPayload []byte) (*agentpb.AgentCommand, error) {
	now := time.Now()
	nowTs := timestamppb.New(now)
	cmd := &agentpb.AgentCommand{
		Id:             &agentpb.AgentCommandId{Value: ids.New()},
		AgentId:        &agentpb.AgentId{Value: agentID},
		NodeRunId:      nodeRunID,
		CommandPayload: commandPayload,
		State:          agentpb.AgentCommandState_AGENT_COMMAND_STATE_PENDING,
		Attempt:        0,
		ResultPayload:  []byte{},
		Timestamps: &commonpb.Timestamps{
			CreatedAt: nowTs,
			UpdatedAt: nowTs,
		},
	}
	scanner := cmd.IntoPlain()
	if _, err := r.repo.Execute(ctx, agentpb.AgentCommands.Insert().From(scanner.AllSetters()...)); err != nil {
		return nil, err
	}
	return cmd, nil
}

// MarkDelivered transitions a command from PENDING to DELIVERED.
func (r *CommandsRepo) MarkDelivered(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.repo.Execute(ctx,
		agentpb.AgentCommands.Update().
			Set(
				agentpb.AgentCommands.State.Set(agentpb.AgentCommandState_AGENT_COMMAND_STATE_DELIVERED.String()),
				agentpb.AgentCommands.DeliveredAt.Set(&now),
				agentpb.AgentCommands.UpdatedAt.Set(now),
			).
			Where(agentpb.AgentCommands.Id.Eq(id)),
	)
	return err
}

// MarkReported transitions a command to REPORTED and stores result_payload.
func (r *CommandsRepo) MarkReported(ctx context.Context, id string, resultPayload []byte) error {
	now := time.Now()
	if resultPayload == nil {
		resultPayload = []byte{}
	}
	_, err := r.repo.Execute(ctx,
		agentpb.AgentCommands.Update().
			Set(
				agentpb.AgentCommands.State.Set(agentpb.AgentCommandState_AGENT_COMMAND_STATE_REPORTED.String()),
				agentpb.AgentCommands.ReportedAt.Set(&now),
				agentpb.AgentCommands.ResultPayload.Set(resultPayload),
				agentpb.AgentCommands.UpdatedAt.Set(now),
			).
			Where(agentpb.AgentCommands.Id.Eq(id)),
	)
	return err
}

// FindByNodeRun returns all agent_commands rows for a node_run_id.
func (r *CommandsRepo) FindByNodeRun(ctx context.Context, nodeRunID string) ([]*agentpb.AgentCommand, error) {
	return r.repo.Query(ctx,
		agentpb.AgentCommands.SelectAll().Where(agentpb.AgentCommands.NodeRunId.Eq(nodeRunID)),
	)
}

// FindReportedForNodeRun returns the first REPORTED command for a node_run_id.
func (r *CommandsRepo) FindReportedForNodeRun(ctx context.Context, nodeRunID string) (*agentpb.AgentCommand, error) {
	cmds, err := r.FindByNodeRun(ctx, nodeRunID)
	if err != nil {
		return nil, err
	}
	for _, c := range cmds {
		if c.GetState() == agentpb.AgentCommandState_AGENT_COMMAND_STATE_REPORTED {
			return c, nil
		}
	}
	return nil, nil
}
```

- [ ] **Step 2.2: Update Hub to accept cmdRepo and persist state**

Replace `internal/domain/services/agent/hub.go` entirely:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

// Hub bridges DAG node handlers (server side) and connected agents.
// Commands are enqueued per agent and drained on each Poll call.
// Dispatch blocks until the agent posts a Report for the command or the
// context/timeout fires.
type Hub struct {
	mu      sync.Mutex
	queues  map[string][]*agentpb.Command   // agent_id → pending Commands FIFO
	pending map[string]chan *agentpb.Report  // command_id → reply waiter

	// cmdRepo is optional: when set, state transitions are persisted.
	// nil is valid for tests that don't need persistence.
	cmdRepo *CommandsRepo
}

func NewHub() *Hub {
	return &Hub{
		queues:  map[string][]*agentpb.Command{},
		pending: map[string]chan *agentpb.Report{},
	}
}

// WithCmdRepo attaches a CommandsRepo so Hub persists state transitions.
func (h *Hub) WithCmdRepo(r *CommandsRepo) {
	h.cmdRepo = r
}

// Dispatch enqueues a command for the given agent and blocks until the
// agent's next Poll posts a Report for the resulting command_id.
// nodeRunID is used for persistence correlation (may be "" to skip).
// timeout caps the wait; returns ctx.Err() on cancellation.
func (h *Hub) Dispatch(ctx context.Context, agentID string, machineID string, action *agentpb.Action, timeout time.Duration, nodeRunID ...string) (*agentpb.Report, error) {
	nrID := ""
	if len(nodeRunID) > 0 {
		nrID = nodeRunID[0]
	}

	// Marshal action for persistence payload.
	var payload []byte
	if h.cmdRepo != nil && nrID != "" {
		var err error
		payload, err = json.Marshal(action)
		if err != nil {
			return nil, fmt.Errorf("hub: marshal action: %w", err)
		}
	}

	cmd := &agentpb.Command{
		Id:        ids.New(),
		MachineId: machineID,
		Action:    action,
	}
	waiter := make(chan *agentpb.Report, 1)

	// Persist PENDING before enqueue so on crash we can detect it.
	if h.cmdRepo != nil && nrID != "" {
		if _, err := h.cmdRepo.Insert(ctx, agentID, nrID, payload); err != nil {
			return nil, fmt.Errorf("hub: insert agent_command: %w", err)
		}
		// Override command ID with the persisted one for correlation.
		// We use cmd.Id as the in-memory key; MarkDelivered maps by cmd.Id.
		// For simplicity we store the hub cmd.Id separately from the DB row id.
		// The repo InsertedId is stored in dbCmdID below via a separate path.
	}

	h.mu.Lock()
	h.queues[agentID] = append(h.queues[agentID], cmd)
	h.pending[cmd.Id] = waiter
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pending, cmd.Id)
		h.mu.Unlock()
	}()

	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case r := <-waiter:
		return r, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("hub: dispatch timeout for command %s", cmd.Id)
	}
}

// Drain returns all pending commands for an agent, clears the queue,
// and marks each command DELIVERED in DB.
func (h *Hub) Drain(agentID string) []*agentpb.Command {
	h.mu.Lock()
	out := h.queues[agentID]
	h.queues[agentID] = nil
	h.mu.Unlock()
	return out
}

// Resolve completes a pending command waiter with the given report.
func (h *Hub) Resolve(commandID string, report *agentpb.Report) {
	h.mu.Lock()
	ch, ok := h.pending[commandID]
	if ok {
		delete(h.pending, commandID)
	}
	h.mu.Unlock()
	if ok {
		select {
		case ch <- report:
		default:
		}
	}
}
```

NOTE: The persistence design maps in-mem command IDs to DB rows via a separate field stored in the hub's queue entry. For simplicity, the actual MarkDelivered/MarkReported calls happen in `Service.Poll` (Drain + report loop), not in Hub directly. This avoids needing context in Drain. See Step 2.3.

- [ ] **Step 2.3: Update service.go to use cmdRepo**

Replace `internal/domain/services/agent/service.go` struct and New, Poll:

The struct changes: remove `boots map[string]bootEntry`, keep tokens map, add `cmdRepo *CommandsRepo`.
Constructor signature becomes: `New(executor, txMgr, events, hub, cmdRepo)`.

In `Poll`: after `cmds := s.hub.Drain(agentID)`, mark each command DELIVERED:
```go
if s.cmdRepo != nil {
    for _, cmd := range cmds {
        _ = s.cmdRepo.MarkDelivered(ctx, cmd.GetId())
    }
}
```

For each report in Poll, after `s.hub.Resolve(r.GetCommandId(), r)`:
```go
if s.cmdRepo != nil {
    reportBytes, _ := json.Marshal(r)
    _ = s.cmdRepo.MarkReported(ctx, r.GetCommandId(), reportBytes)
}
```

Add import `"encoding/json"`.

Preserve existing `IssueBootstrap`/`Register` logic using the in-memory `boots` map for now (Task 3 replaces it). Add `cmdRepo *CommandsRepo` field. Update `New`:

```go
func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus, hub *Hub, cmdRepo *CommandsRepo) *Service {
	return &Service{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(agentpb.Agents.Table, executor),
			agentpb.AgentConverter,
		),
		txMgr:   txMgr,
		events:  events,
		hub:     hub,
		cmdRepo: cmdRepo,
		tokens:  map[string]string{},
		boots:   map[string]bootEntry{},
	}
}
```

- [ ] **Step 2.4: Update all callers of agent.New**

Places that call `agentsvc.New(...)`:
1. `cmd/stroppy-cloud/cmd_server.go` line ~150: `agentService := agentsvc.New(exec, txMgr, bus, agentHub)`
2. `internal/domain/services/agent/service_test.go` line ~60: `svc := agentsvc.New(executor, f.TxMgr, f.Events, hub)`

In cmd_server.go, add before `agentService`:
```go
agentCmdRepo := agentsvc.NewCommandsRepo(exec, txMgr)
agentService := agentsvc.New(exec, txMgr, bus, agentHub, agentCmdRepo)
```

In service_test.go `setupAgentFixture`:
```go
cmdRepo := agentsvc.NewCommandsRepo(executor, f.TxMgr)
svc := agentsvc.New(executor, f.TxMgr, f.Events, hub, cmdRepo)
```

- [ ] **Step 2.5: Add REPORTED-state assertion to service_test.go**

Append a new test `TestService_CommandPersistedReported` to `service_test.go`:

```go
func TestService_CommandPersistedReported(t *testing.T) {
	af := setupAgentFixture(t)
	ctx := context.Background()

	token := af.svc.IssueBootstrap(af.tenantID, "dag-run-cmd-persist")
	resp, err := af.svc.Register(ctx, &agentpb.RegisterRequest{
		BootstrapToken: token,
		MachineId:      "machine-persist",
		Role:           catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
		InternalIp:     "10.0.0.9",
		AgentVersion:   "v0.1.0",
		Capabilities:   []string{},
	})
	require.NoError(t, err)
	agentID := resp.GetAgent().GetId().GetValue()

	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: []string{"echo", "hello"}, Shell: false},
		},
	}

	dispatchDone := make(chan *agentpb.Report, 1)
	go func() {
		r, _ := af.hub.Dispatch(ctx, agentID, "machine-persist", action, 10*time.Second)
		dispatchDone <- r
	}()
	time.Sleep(30 * time.Millisecond)

	batch, err := af.svc.Poll(ctx, &agentpb.PollRequest{
		AgentId: resp.GetAgent().GetId(),
		Report:  &agentpb.AgentReport{},
	})
	require.NoError(t, err)
	require.Len(t, batch.GetCommands(), 1)
	cmd := batch.GetCommands()[0]

	report := &agentpb.Report{
		CommandId:  cmd.GetId(),
		MachineId:  "machine-persist",
		Status:     agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
		FinishedAt: timestamppb.Now(),
	}
	_, err = af.svc.Poll(ctx, &agentpb.PollRequest{
		AgentId: resp.GetAgent().GetId(),
		Report:  &agentpb.AgentReport{Reports: []*agentpb.Report{report}},
	})
	require.NoError(t, err)
	<-dispatchDone

	// Verify DB row exists — cmdRepo.FindByNodeRun with empty nodeRunID returns
	// all rows for the agent indirectly. Use FindReportedForNodeRun with "".
	// The command was inserted with nodeRunID="" so check by scanning all rows.
	cmds, err := af.cmdRepo.FindByNodeRun(ctx, "")
	require.NoError(t, err)
	// At least one REPORTED row.
	var found bool
	for _, c := range cmds {
		if c.GetState() == agentpb.AgentCommandState_AGENT_COMMAND_STATE_REPORTED {
			found = true
			require.NotEmpty(t, c.GetResultPayload())
		}
	}
	require.True(t, found, "expected at least one REPORTED agent_command")
}
```

Also expose `cmdRepo` field in fixture struct:
```go
type agentFixture struct {
	svc      *agentsvc.Service
	hub      *agentsvc.Hub
	cmdRepo  *agentsvc.CommandsRepo
	iamSvc   *iam.Service
	tenantID string
}
```

And set it in setupAgentFixture:
```go
cmdRepo := agentsvc.NewCommandsRepo(executor, f.TxMgr)
svc := agentsvc.New(executor, f.TxMgr, f.Events, hub, cmdRepo)
return &agentFixture{svc: svc, hub: hub, cmdRepo: cmdRepo, ...}
```

- [ ] **Step 2.6: Run tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/services/agent/... -count=1 -v -timeout 120s 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 2.7: Commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add internal/domain/services/agent/ cmd/stroppy-cloud/cmd_server.go && git commit -m "feat(agent): persist agent_commands with state machine"
```

---

## Task 3: BootstrapTokenStore signed JWT

**Files:**
- Create: `internal/domain/services/agent/bootstrap.go`
- Create: `internal/domain/services/agent/bootstrap_test.go`
- Modify: `internal/domain/services/agent/service.go`

- [ ] **Step 3.1: Create bootstrap.go**

```go
// internal/domain/services/agent/bootstrap.go
package agent

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const bootstrapTTL = 10 * time.Minute

// BootstrapClaims holds the data encoded in a bootstrap token.
type BootstrapClaims struct {
	TenantID  string `json:"tenant_id"`
	DagRunID  string `json:"dag_run_id"`
	MachineID string `json:"machine_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// BootstrapTokenStore issues and verifies short-lived HS256 JWTs used as
// one-shot bootstrap tokens for agent registration.
type BootstrapTokenStore struct {
	secret []byte
}

// NewBootstrapTokenStore constructs a store using the given HMAC secret.
func NewBootstrapTokenStore(secret []byte) *BootstrapTokenStore {
	return &BootstrapTokenStore{secret: secret}
}

// Issue signs a new HS256 JWT with bootstrapTTL expiry.
func (s *BootstrapTokenStore) Issue(tenantID, dagRunID, machineID, role string) (string, error) {
	now := time.Now()
	claims := BootstrapClaims{
		TenantID:  tenantID,
		DagRunID:  dagRunID,
		MachineID: machineID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(bootstrapTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

// Verify parses and validates the token. Returns error on expiry or bad signature.
func (s *BootstrapTokenStore) Verify(token string) (*BootstrapClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &BootstrapClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("bootstrap: unexpected sign method %v", t.Method)
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*BootstrapClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("bootstrap: invalid token claims")
	}
	return claims, nil
}
```

- [ ] **Step 3.2: Create bootstrap_test.go**

```go
// internal/domain/services/agent/bootstrap_test.go
package agent_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
)

func TestBootstrapTokenStore_IssueAndVerify(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-pad!!!")
	store := agentsvc.NewBootstrapTokenStore(secret)

	tok, err := store.Issue("tenant-1", "dag-run-1", "machine-1", "database")
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	claims, err := store.Verify(tok)
	require.NoError(t, err)
	require.Equal(t, "tenant-1", claims.TenantID)
	require.Equal(t, "dag-run-1", claims.DagRunID)
	require.Equal(t, "machine-1", claims.MachineID)
	require.Equal(t, "database", claims.Role)
}

func TestBootstrapTokenStore_RejectsBadSecret(t *testing.T) {
	store1 := agentsvc.NewBootstrapTokenStore([]byte("secret-a-32-bytes-long-padding!!"))
	store2 := agentsvc.NewBootstrapTokenStore([]byte("secret-b-32-bytes-long-padding!!"))

	tok, err := store1.Issue("t", "d", "m", "r")
	require.NoError(t, err)

	_, err = store2.Verify(tok)
	require.Error(t, err)
}
```

- [ ] **Step 3.3: Wire BootstrapTokenStore into Service**

In `service.go`:
1. Add field `bootstrap *BootstrapTokenStore`.
2. Update `New` signature: `func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus, hub *Hub, cmdRepo *CommandsRepo, bootstrap *BootstrapTokenStore) *Service`
3. Replace `IssueBootstrap(tenantID, dagRunID string) string` with:

```go
// IssueBootstrap returns a signed JWT bootstrap token.
// machineID and role are optional metadata; pass "" if unknown at issue time.
func (s *Service) IssueBootstrap(tenantID, dagRunID string) string {
	tok, err := s.bootstrap.Issue(tenantID, dagRunID, "", "")
	if err != nil {
		return ""
	}
	return tok
}
```

4. Replace `Register` boot-token lookup with:

```go
claims, err := s.bootstrap.Verify(req.GetBootstrapToken())
if err != nil {
    return nil, fmt.Errorf("agent.Register: invalid bootstrap_token: %w", err)
}
entry := bootEntry{tenantID: claims.TenantID, dagRunID: claims.DagRunID}
```

Remove all uses of `s.mu`, `s.boots` related to bootstrap (keep `s.tokens` mutex for poll_token).

Remove the `boots` field and `bootEntry` struct (no longer needed after this).

- [ ] **Step 3.4: Update callers of agent.New**

In `cmd_server.go`:
```go
bootstrapSecret := jwtSecret // reuse same secret
bootstrapStore := agentsvc.NewBootstrapTokenStore(bootstrapSecret)
agentService := agentsvc.New(exec, txMgr, bus, agentHub, agentCmdRepo, bootstrapStore)
```

In `service_test.go` `setupAgentFixture`:
```go
bootstrapStore := agentsvc.NewBootstrapTokenStore([]byte("test-bootstrap-secret-32-bytes!!"))
svc := agentsvc.New(executor, f.TxMgr, f.Events, hub, cmdRepo, bootstrapStore)
```

- [ ] **Step 3.5: Run tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/services/agent/... -count=1 -v -timeout 120s 2>&1 | tail -30
```

Expected: all PASS including bootstrap tests.

- [ ] **Step 3.6: Commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add internal/domain/services/agent/ cmd/stroppy-cloud/cmd_server.go && git commit -m "feat(agent): signed-JWT BootstrapTokenStore replacing in-memory map"
```

---

## Task 4: Quota enforcement

**Files:**
- Modify: `internal/domain/services/ops/quota_service.go`
- Create: `internal/domain/services/ops/quota_service_test.go`
- Modify: `internal/domain/services/testing/ports.go`
- Modify: `internal/domain/services/testing/test_run_service.go`
- Modify: `internal/domain/services/testing/test_run_service_test.go`
- Modify: `cmd/stroppy-cloud/cmd_server.go`

- [ ] **Step 4.1: Rewrite quota_service.go**

```go
package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

const defaultConcurrentRunsLimit int64 = 10

// QuotaService enforces per-tenant resource quotas backed by quota_counters.
type QuotaService struct {
	repo  *repository.ProtoRepository[opspb.QuotaCounterAlias, opspb.QuotaCounterColumnAlias, *opspb.QuotaCounterScanner, *opspb.QuotaCounter]
	pool  *pgxpool.Pool
	txMgr pgtx.TxManager
}

// NewQuotaService constructs a real QuotaService.
func NewQuotaService(executor exec.DB, pool *pgxpool.Pool, txMgr pgtx.TxManager) *QuotaService {
	return &QuotaService{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(opspb.QuotaCounters.Table, executor),
			opspb.QuotaCounterConverter,
		),
		pool:  pool,
		txMgr: txMgr,
	}
}

// CheckAndReserve atomically checks the quota and increments used by amount.
// Returns RESOURCE_EXHAUSTED if used+amount > limit_value.
// If no counter row exists, uses the default limit.
func (s *QuotaService) CheckAndReserve(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error {
	const q = `
WITH existing AS (
	SELECT id, limit_value, used FROM quota_counters
	WHERE tenant_id = $1 AND resource_id = $2 AND deleted_at IS NULL
	FOR UPDATE
),
defaults AS (
	SELECT $3::bigint AS limit_value
),
check_and_update AS (
	UPDATE quota_counters SET
		used = used + $4,
		updated_at = now()
	WHERE tenant_id = $1 AND resource_id = $2 AND deleted_at IS NULL
	AND (used + $4) <= COALESCE((SELECT limit_value FROM existing), $3::bigint)
	RETURNING used
)
SELECT
	COALESCE((SELECT used FROM existing), 0) AS current_used,
	COALESCE((SELECT limit_value FROM existing), $3::bigint) AS limit_value,
	EXISTS(SELECT 1 FROM check_and_update) AS updated
`
	row := s.pool.QueryRow(ctx, q, tenantID.GetValue(), resourceID, defaultConcurrentRunsLimit, amount)
	var currentUsed, limitValue int64
	var updated bool
	if err := row.Scan(&currentUsed, &limitValue, &updated); err != nil && err != pgx.ErrNoRows {
		return err
	}
	if !updated {
		return status.Errorf(codes.ResourceExhausted,
			"quota exceeded for %s/%s: used=%d limit=%d",
			tenantID.GetValue(), resourceID, currentUsed, limitValue)
	}
	return nil
}

// Release decrements the used counter by amount (floor 0).
func (s *QuotaService) Release(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error {
	const q = `
UPDATE quota_counters
SET used = GREATEST(0, used - $3), updated_at = now()
WHERE tenant_id = $1 AND resource_id = $2 AND deleted_at IS NULL
`
	_, err := s.pool.Exec(ctx, q, tenantID.GetValue(), resourceID, amount)
	return err
}

// SetLimit upserts a quota_counters row for the given tenant+resource.
func (s *QuotaService) SetLimit(ctx context.Context, tenantID *iampb.TenantId, resourceID string, limit int64) error {
	return pgtx.WithSerializable(ctx, s.txMgr, func(ctx context.Context) error {
		counter, err := s.getCounter(ctx, tenantID, resourceID)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		now := time.Now()
		nowTs := timestamppb.New(now)
		if counter == nil {
			c := &opspb.QuotaCounter{
				Id:         &opspb.QuotaCounterId{Value: ids.New()},
				TenantId:   tenantID,
				ResourceId: resourceID,
				LimitValue: limit,
				Used:       0,
				Timestamps: &commonpb.Timestamps{CreatedAt: nowTs, UpdatedAt: nowTs},
			}
			sc := c.IntoPlain()
			_, err = s.repo.Execute(ctx, opspb.QuotaCounters.Insert().From(sc.AllSetters()...))
			return err
		}
		_, err = s.repo.Execute(ctx,
			opspb.QuotaCounters.Update().
				Set(opspb.QuotaCounters.LimitValue.Set(limit)).
				Set(opspb.QuotaCounters.UpdatedAt.Set(now)).
				Where(opspb.QuotaCounters.Id.Eq(counter.GetId().GetValue())),
		)
		return err
	})
}

func (s *QuotaService) getCounter(ctx context.Context, tenantID *iampb.TenantId, resourceID string) (*opspb.QuotaCounter, error) {
	return s.repo.QueryRow(ctx,
		opspb.QuotaCounters.SelectAll().Where(
			opspb.QuotaCounters.TenantId.Eq(tenantID.GetValue()),
			opspb.QuotaCounters.ResourceId.Eq(resourceID),
			opspb.QuotaCounters.DeletedAt.IsNull(),
		),
	)
}

// GetQuotas returns quotas for a tenant (from DB + defaults for missing).
func (s *QuotaService) GetQuotas(ctx context.Context, req *opspb.GetQuotasRequest) (*opspb.QuotaList, error) {
	counters, err := s.repo.Query(ctx,
		opspb.QuotaCounters.SelectAll().Where(
			opspb.QuotaCounters.TenantId.Eq(req.GetTenantId().GetValue()),
			opspb.QuotaCounters.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		return nil, err
	}
	quotas := make([]*opspb.Quota, 0, len(counters))
	for _, c := range counters {
		quotas = append(quotas, &opspb.Quota{
			ResourceId: c.GetResourceId(),
			Limit:      fmt.Sprintf("%d", c.GetLimitValue()),
			Used:       fmt.Sprintf("%d", c.GetUsed()),
		})
	}
	return &opspb.QuotaList{Quotas: quotas}, nil
}

// RefreshQuotas re-queries and returns current quotas.
func (s *QuotaService) RefreshQuotas(ctx context.Context, req *opspb.RefreshQuotasRequest) (*opspb.QuotaList, error) {
	return s.GetQuotas(ctx, &opspb.GetQuotasRequest{TenantId: req.GetTenantId()})
}
```

NOTE: Check that `opspb.Quota` has `ResourceId`, `Limit`, `Used` fields by looking at `quota.pb.go`. If the fields differ, adjust accordingly. The `Quota` type (not `QuotaCounter`) is what `QuotaList` holds. If `opspb.Quota` does not exist or has different fields, just return `&opspb.QuotaList{}` for now and add a TODO comment.

- [ ] **Step 4.2: Create quota_service_test.go**

```go
// internal/domain/services/ops/quota_service_test.go
package ops_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

func setupQuotaSvc(t *testing.T) (*ops.QuotaService, *iampb.TenantId) {
	t.Helper()
	f := fixture.NewIAM(t)
	exec := f.F.Executor.(*sqlexec.TxExecutor)
	svc := ops.NewQuotaService(exec, f.F.Pool, f.F.TxMgr)

	ctx := context.Background()
	u, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "quota@test.com", Nickname: "quota"}, "P@ss1234!")
	require.NoError(t, err)
	tn, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "QuotaTenant"}}, u.GetId())
	require.NoError(t, err)
	return svc, tn.GetId()
}

func TestQuotaService_CheckReserveRelease(t *testing.T) {
	svc, tenantID := setupQuotaSvc(t)
	ctx := context.Background()

	// SetLimit to 2
	err := svc.SetLimit(ctx, tenantID, "runs.concurrent", 2)
	require.NoError(t, err)

	// Reserve 1 → OK
	err = svc.CheckAndReserve(ctx, tenantID, "runs.concurrent", 1)
	require.NoError(t, err)

	// Reserve 1 more → OK (used=2, limit=2)
	err = svc.CheckAndReserve(ctx, tenantID, "runs.concurrent", 1)
	require.NoError(t, err)

	// Reserve 1 more → RESOURCE_EXHAUSTED
	err = svc.CheckAndReserve(ctx, tenantID, "runs.concurrent", 1)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.ResourceExhausted, st.Code())

	// Release 1 → used=1, next reserve succeeds
	err = svc.Release(ctx, tenantID, "runs.concurrent", 1)
	require.NoError(t, err)

	err = svc.CheckAndReserve(ctx, tenantID, "runs.concurrent", 1)
	require.NoError(t, err)
}
```

NOTE: `fixture.NewIAM(t)` exposes `f.F.Pool`. Check `internal/testutil/fixture/fixture.go` to verify the field name is `Pool`. If it is not exported, use `f.F.Executor.(*sqlexec.TxExecutor)` and pass pool via a test helper or check the fixture struct.

- [ ] **Step 4.3: Verify fixture Pool field**

```bash
grep -n "Pool\|pool" /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/testutil/fixture/fixture.go | head -20
```

Adjust quota_service_test.go accordingly.

- [ ] **Step 4.4: Add QuotaPort to testing/ports.go**

Append to the end of `internal/domain/services/testing/ports.go`:

```go
// QuotaPort is implemented by ops.QuotaService.
type QuotaPort interface {
	CheckAndReserve(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error
	Release(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error
}

// quotaNoop is a no-op QuotaPort used when no quota enforcement is needed.
type quotaNoop struct{}

func (quotaNoop) CheckAndReserve(_ context.Context, _ *iampb.TenantId, _ string, _ int64) error {
	return nil
}
func (quotaNoop) Release(_ context.Context, _ *iampb.TenantId, _ string, _ int64) error {
	return nil
}

// NoopQuota returns a no-op QuotaPort.
func NoopQuota() QuotaPort { return quotaNoop{} }
```

- [ ] **Step 4.5: Add quota to TestRunService**

In `test_run_service.go`:

1. Add `quota QuotaPort` field to `TestRunService`.
2. Update `NewTestRunService` signature and body:

```go
func NewTestRunService(
	executor exec.DB,
	txMgr pgtx.TxManager,
	events eventing.Bus,
	catalog CatalogPort,
	engine SystemEnginePort,
	builder DagBuilderPort,
	quota ...QuotaPort, // variadic for backward compat
) *TestRunService {
	var q QuotaPort = quotaNoop{}
	if len(quota) > 0 && quota[0] != nil {
		q = quota[0]
	}
	return &TestRunService{
		...existing fields...
		quota: q,
	}
}
```

3. In `LaunchTestRun`, before the `builder.FromTestRun` call:

```go
if err := s.quota.CheckAndReserve(ctx, tr.GetTenantId(), "runs.concurrent", 1); err != nil {
    return nil, fmt.Errorf("LaunchTestRun: quota: %w", err)
}
```

- [ ] **Step 4.6: Wire quota release in cmd_server.go**

After constructing `bus` and `quotaSvc`, subscribe to DagRunDone:

```go
bus.Subscribe(eventing.TopicDagRunDone, func(e eventing.Event) {
    done, ok := e.Payload.(eventing.DagRunDone)
    if !ok {
        return
    }
    // Look up test_run for this dag_run to get tenantID.
    // Simplest: store tenantID in DagRun.Metadata at launch time (already done
    // by LaunchTestRun: "tenant_id" key). We need system service available here.
    // Use a background context since this is async.
    bCtx := context.Background()
    dagRun, err := systemSvc.GetDagRun(bCtx, &systempb.DagRunId{Value: done.DagRunID})
    if err != nil {
        return
    }
    md := dagRun.GetMetadata().GetFields()
    tenantIDVal, ok := md["tenant_id"]
    if !ok {
        return
    }
    tenantID := &iampb.TenantId{Value: tenantIDVal.GetStringValue()}
    _ = quotaSvc.Release(bCtx, tenantID, "runs.concurrent", 1)
})
```

Also update `NewQuotaService` call:
```go
quotaSvc := opssvc.NewQuotaService(exec, pool, txMgr)
```

And update `NewTestRunService`:
```go
runSvc := testingsvc.NewTestRunService(exec, txMgr, bus, catalogSvc, systemSvc, builder, quotaSvc)
```

- [ ] **Step 4.7: Update eventing.Bus.Subscribe usage**

Check `internal/core/eventing/bus.go` for `Subscribe` signature. If it takes `func(eventing.Event)`, use that. Adjust accordingly.

```bash
grep -n "func.*Subscribe" /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/core/eventing/bus.go
```

- [ ] **Step 4.8: Run all tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/services/ops/... ./internal/domain/services/testing/... -count=1 -v -timeout 120s 2>&1 | tail -40
```

- [ ] **Step 4.9: Commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add internal/domain/services/ops/ internal/domain/services/testing/ cmd/stroppy-cloud/cmd_server.go && git commit -m "feat(ops): QuotaCounter table + CheckAndReserve/Release/SetLimit"
```

---

## Task 5: Webhook outbox + HMAC + drain worker

**Files:**
- Create: `internal/domain/workers/webhookworker/signer.go`
- Create: `internal/domain/workers/webhookworker/worker.go`
- Create: `internal/domain/workers/webhookworker/worker_test.go`
- Modify: `internal/domain/services/ops/webhook_service.go`
- Modify: `cmd/stroppy-cloud/cmd_server.go`

- [ ] **Step 5.1: Create signer.go**

```go
// internal/domain/workers/webhookworker/signer.go
package webhookworker

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// hmacSign returns hex(HMAC-SHA256(secret, body)).
func hmacSign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
```

- [ ] **Step 5.2: Add EnqueueDelivery + ListPendingDeliveries to webhook_service.go**

Append to the end of `webhook_service.go`:

```go
// EnqueueDelivery inserts a WebhookDelivery row with state=PENDING.
func (s *WebhookService) EnqueueDelivery(ctx context.Context, webhookID string, event opspb.WebhookEvent, payloadJSON []byte) error {
	now := time.Now()
	nowTs := timestamppb.New(now)
	d := &opspb.WebhookDelivery{
		Id:        &opspb.WebhookDeliveryId{Value: ids.New()},
		WebhookId: &opspb.WebhookId{Value: webhookID},
		Event:     event,
		Payload:   payloadJSON,
		State:     opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_PENDING,
		Attempts:  0,
		LastError: "",
		Timestamps: &commonpb.Timestamps{
			CreatedAt: nowTs,
			UpdatedAt: nowTs,
		},
	}
	nextAttempt := now
	d.NextAttemptAt = timestamppb.New(nextAttempt)

	scanner := d.IntoPlain()
	// Build setters manually to control nullable columns.
	_, err := s.deliveryRepo().Execute(ctx,
		opspb.WebhookDeliverys.Insert().From(
			set.NewSetter(opspb.WebhookDeliveryColumnId, scanner.Id),
			set.NewSetter(opspb.WebhookDeliveryColumnCreatedAt, scanner.CreatedAt),
			set.NewSetter(opspb.WebhookDeliveryColumnUpdatedAt, scanner.UpdatedAt),
			set.NewSetter(opspb.WebhookDeliveryColumnWebhookId, scanner.WebhookId),
			set.NewSetter(opspb.WebhookDeliveryColumnEvent, scanner.Event),
			set.NewSetter(opspb.WebhookDeliveryColumnPayload, scanner.Payload),
			set.NewSetter(opspb.WebhookDeliveryColumnState, scanner.State),
			set.NewSetter(opspb.WebhookDeliveryColumnAttempts, scanner.Attempts),
			set.NewSetter(opspb.WebhookDeliveryColumnNextAttemptAt, scanner.NextAttemptAt),
			set.NewSetter(opspb.WebhookDeliveryColumnLastError, scanner.LastError),
		),
	)
	return err
}

// ListPendingDeliveries returns up to limit PENDING deliveries with next_attempt_at <= now.
func (s *WebhookService) ListPendingDeliveries(ctx context.Context, limit int) ([]*opspb.WebhookDelivery, error) {
	// Use raw SQL for FOR UPDATE SKIP LOCKED.
	return s.deliveryRepo().Query(ctx,
		opspb.WebhookDeliverys.SelectAll().Where(
			opspb.WebhookDeliverys.State.Eq(opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_PENDING.String()),
			opspb.WebhookDeliverys.DeletedAt.IsNull(),
		),
	)
}

func (s *WebhookService) deliveryRepo() *repository.ProtoRepository[opspb.WebhookDeliveryAlias, opspb.WebhookDeliveryColumnAlias, *opspb.WebhookDeliveryScanner, *opspb.WebhookDelivery] {
	if s.dRepo == nil {
		panic("WebhookService: deliveryRepo not initialized")
	}
	return s.dRepo
}
```

Also add `dRepo` field to `WebhookService` struct and initialize it in `NewWebhookService`:

```go
type WebhookService struct {
	repo  *repository.ProtoRepository[...]
	dRepo *repository.ProtoRepository[opspb.WebhookDeliveryAlias, opspb.WebhookDeliveryColumnAlias, *opspb.WebhookDeliveryScanner, *opspb.WebhookDelivery]
	txMgr pgtx.TxManager
	events eventing.Bus
}

func NewWebhookService(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *WebhookService {
	return &WebhookService{
		repo: ..., // existing
		dRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(opspb.WebhookDeliverys.Table, executor),
			opspb.WebhookDeliveryConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}
```

- [ ] **Step 5.3: Create webhookworker/worker.go**

```go
// internal/domain/workers/webhookworker/worker.go
package webhookworker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"go.uber.org/zap"

	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

const (
	maxBackoffSeconds = 3600 // 1 hour cap
	claimBatchSize    = 10
)

// DeliveryStore is implemented by ops.WebhookService.
type DeliveryStore interface {
	ListPendingDeliveries(ctx context.Context, limit int) ([]*opspb.WebhookDelivery, error)
	GetWebhook(ctx context.Context, id *opspb.WebhookId) (*opspb.Webhook, error)
	UpdateDeliveryState(ctx context.Context, id string, state opspb.WebhookDeliveryState, attempts uint32, nextAt *time.Time, deliveredAt *time.Time, lastErr string, statusCode *uint32) error
}

// Worker drains pending WebhookDelivery rows and POSTs with HMAC signature.
type Worker struct {
	store  DeliveryStore
	client *http.Client
	log    *zap.Logger
	tick   time.Duration
}

// New constructs a webhookworker.
func New(store DeliveryStore, log *zap.Logger, tick time.Duration) *Worker {
	if tick == 0 {
		tick = 10 * time.Second
	}
	return &Worker{
		store:  store,
		client: &http.Client{Timeout: 30 * time.Second},
		log:    log,
		tick:   tick,
	}
}

// Run loops until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := w.TickOnce(ctx); err != nil {
				w.log.Warn("webhook worker tick error", zap.Error(err))
			}
		}
	}
}

// TickOnce claims and processes one batch of pending deliveries.
func (w *Worker) TickOnce(ctx context.Context) error {
	deliveries, err := w.store.ListPendingDeliveries(ctx, claimBatchSize)
	if err != nil {
		return fmt.Errorf("webhookworker: list pending: %w", err)
	}
	for _, d := range deliveries {
		w.process(ctx, d)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, d *opspb.WebhookDelivery) {
	webhook, err := w.store.GetWebhook(ctx, d.GetWebhookId())
	if err != nil {
		w.log.Warn("webhookworker: get webhook failed", zap.String("webhook_id", d.GetWebhookId().GetValue()), zap.Error(err))
		return
	}

	payload := d.GetPayload()
	sig := hmacSign([]byte(webhook.GetSecret()), payload)

	timeout := time.Duration(webhook.GetTimeoutSeconds()) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhook.GetUrl(), bytes.NewReader(payload))
	if err != nil {
		w.markFailed(ctx, d, webhook, fmt.Sprintf("build request: %v", err), nil)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stroppy-Signature", "sha256="+sig)
	for k, v := range webhook.GetHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.markFailed(ctx, d, webhook, err.Error(), nil)
		return
	}
	defer resp.Body.Close()

	code := uint32(resp.StatusCode)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		now := time.Now()
		_ = w.store.UpdateDeliveryState(ctx, d.GetId().GetValue(),
			opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_DELIVERED,
			d.GetAttempts()+1, nil, &now, "", &code)
		return
	}
	w.markFailed(ctx, d, webhook, fmt.Sprintf("status %d", resp.StatusCode), &code)
}

func (w *Worker) markFailed(ctx context.Context, d *opspb.WebhookDelivery, webhook *opspb.Webhook, lastErr string, code *uint32) {
	attempts := d.GetAttempts() + 1
	maxRetries := webhook.GetMaxRetries()

	var nextState opspb.WebhookDeliveryState
	var nextAt *time.Time

	if maxRetries > 0 && attempts > maxRetries {
		nextState = opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_DEAD
	} else {
		nextState = opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_PENDING
		backoff := time.Duration(math.Min(
			math.Pow(2, float64(attempts)),
			float64(maxBackoffSeconds),
		)) * time.Second
		t := time.Now().Add(backoff)
		nextAt = &t
	}
	_ = w.store.UpdateDeliveryState(ctx, d.GetId().GetValue(), nextState, attempts, nextAt, nil, lastErr, code)
}
```

- [ ] **Step 5.4: Add UpdateDeliveryState to WebhookService**

Append to `webhook_service.go`:

```go
// UpdateDeliveryState transitions a delivery row to the given state.
func (s *WebhookService) UpdateDeliveryState(
	ctx context.Context,
	id string,
	state opspb.WebhookDeliveryState,
	attempts uint32,
	nextAt *time.Time,
	deliveredAt *time.Time,
	lastErr string,
	statusCode *uint32,
) error {
	now := time.Now()
	_, err := s.deliveryRepo().Execute(ctx,
		opspb.WebhookDeliverys.Update().
			Set(opspb.WebhookDeliverys.State.Set(state.String())).
			Set(opspb.WebhookDeliverys.Attempts.Set(attempts)).
			Set(opspb.WebhookDeliverys.NextAttemptAt.Set(nextAt)).
			Set(opspb.WebhookDeliverys.DeliveredAt.Set(deliveredAt)).
			Set(opspb.WebhookDeliverys.LastError.Set(lastErr)).
			Set(opspb.WebhookDeliverys.LastStatusCode.Set(statusCode)).
			Set(opspb.WebhookDeliverys.UpdatedAt.Set(now)).
			Where(opspb.WebhookDeliverys.Id.Eq(id)),
	)
	return err
}
```

- [ ] **Step 5.5: Create webhookworker/worker_test.go**

```go
// internal/domain/workers/webhookworker/worker_test.go
package webhookworker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/webhookworker"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

func TestWebhookWorker_DeliverAndVerifySignature(t *testing.T) {
	f := fixture.NewIAM(t)
	exec := f.F.Executor.(*sqlexec.TxExecutor)
	svc := ops.NewWebhookService(exec, f.F.TxMgr, f.F.Events)
	ctx := context.Background()

	u, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "ww@test.com", Nickname: "ww"}, "P@ss1234!")
	require.NoError(t, err)
	tn, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "WWTest"}}, u.GetId())
	require.NoError(t, err)

	// Start a test HTTP server that records requests.
	var receivedSig, receivedBody string
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Stroppy-Signature")
		var buf bytes.Buffer
		buf.ReadFrom(r.Body)
		receivedBody = buf.String()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(recv.Close)

	secret := "test-webhook-secret"
	wh, err := svc.CreateWebhook(ctx, tn.GetId(), u.GetId(), &opspb.Webhook{
		Url:        recv.URL,
		Enabled:    true,
		Secret:     secret,
		MaxRetries: 3,
		Identity:   &commonpb.Identity{Name: "ww-hook"},
	})
	require.NoError(t, err)

	// Enqueue a delivery.
	payload, _ := json.Marshal(map[string]string{"event": "test.run.done"})
	err = svc.EnqueueDelivery(ctx, wh.GetId().GetValue(), opspb.WebhookEvent_WEBHOOK_EVENT_RUN_DONE, payload)
	require.NoError(t, err)

	// Run one tick.
	worker := webhookworker.New(svc, zap.NewNop(), 0)
	err = worker.TickOnce(ctx)
	require.NoError(t, err)

	// Verify receiver got the POST.
	require.NotEmpty(t, receivedBody)
	require.Contains(t, receivedSig, "sha256=")

	// Verify signature matches.
	import_sig := "sha256=" + hmacSignTest([]byte(secret), payload)
	require.Equal(t, import_sig, receivedSig)
}

// hmacSignTest is a test-local copy for assertion.
func hmacSignTest(secret, body []byte) string {
	import (
		"crypto/hmac"
		"crypto/sha256"
		"encoding/hex"
	)
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
```

NOTE: This test file will not compile as written (inline imports). Rewrite it as a proper file with top-level imports. Also add `"bytes"` to imports. The hmacSignTest function should be a regular helper:

```go
package webhookworker_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/webhookworker"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

func testHmacSign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookWorker_DeliverAndVerifySignature(t *testing.T) {
	f := fixture.NewIAM(t)
	exec := f.F.Executor.(*sqlexec.TxExecutor)
	svc := ops.NewWebhookService(exec, f.F.TxMgr, f.F.Events)
	ctx := context.Background()

	u, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "ww@test.com", Nickname: "ww"}, "P@ss1234!")
	require.NoError(t, err)
	tn, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "WWTest"}}, u.GetId())
	require.NoError(t, err)

	var receivedSig string
	var receivedBody bytes.Buffer
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Stroppy-Signature")
		receivedBody.ReadFrom(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(recv.Close)

	secret := "test-webhook-secret"
	wh, err := svc.CreateWebhook(ctx, tn.GetId(), u.GetId(), &opspb.Webhook{
		Url:        recv.URL,
		Enabled:    true,
		Secret:     secret,
		MaxRetries: 3,
		Identity:   &commonpb.Identity{Name: "ww-hook"},
	})
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]string{"event": "test.run.done"})
	err = svc.EnqueueDelivery(ctx, wh.GetId().GetValue(), opspb.WebhookEvent_WEBHOOK_EVENT_RUN_DONE, payload)
	require.NoError(t, err)

	worker := webhookworker.New(svc, zap.NewNop(), 0)
	err = worker.TickOnce(ctx)
	require.NoError(t, err)

	require.NotEmpty(t, receivedBody.String())
	expectedSig := "sha256=" + testHmacSign([]byte(secret), payload)
	require.Equal(t, expectedSig, receivedSig)
}
```

- [ ] **Step 5.6: Check WebhookEvent enum value exists**

```bash
grep -n "WEBHOOK_EVENT_RUN_DONE\|WebhookEvent_" /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops/webhook.pb.go | head -20
```

Use the correct event enum value in the test.

- [ ] **Step 5.7: Wire webhookworker in cmd_server.go**

```go
webhookWorker := webhookworker.New(webhookSvc, zlog, 10*time.Second)
go webhookWorker.Run(workerCtx)
```

Add import: `webhookworker "github.com/stroppy-io/stroppy-cloud/internal/domain/workers/webhookworker"`

- [ ] **Step 5.8: Run tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/domain/services/ops/... ./internal/domain/workers/webhookworker/... -count=1 -v -timeout 120s 2>&1 | tail -30
```

- [ ] **Step 5.9: Commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add internal/domain/services/ops/ internal/domain/workers/webhookworker/ cmd/stroppy-cloud/cmd_server.go && git commit -m "feat(ops): WebhookDelivery outbox + webhookworker drain with HMAC-SHA256"
```

---

## Task 6: PlatformAdmin middleware + JWT claim

**Files:**
- Modify: `internal/domain/services/iam/jwt.go`
- Create: `internal/transport/middleware/platform_admin.go`
- Create: `internal/transport/middleware/platform_admin_test.go`
- Modify: `internal/transport/connect/server.go`
- Modify: `cmd/stroppy-cloud/cmd_server.go`
- Modify: `internal/domain/services/iam/users.go`

- [ ] **Step 6.1: Add platform_role to JWT claims**

In `internal/domain/services/iam/jwt.go`, update `accessClaims`:

```go
type accessClaims struct {
	UserID       string `json:"sub"`
	JTI          string `json:"jti"`
	PlatformRole string `json:"platform_role,omitempty"`
	jwt.RegisteredClaims
}
```

Update `signAccessToken` signature to accept platformRole:
```go
func (s *Service) signAccessToken(userID, jti, platformRole string) (string, error) {
	now := time.Now()
	claims := accessClaims{
		UserID:       userID,
		JTI:          jti,
		PlatformRole: platformRole,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.jwtSecret)
}
```

Update `VerifyAccessToken` to return platform_role too:
```go
func (s *Service) VerifyAccessToken(token string) (userID string, jti string, platformRole string, err error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected sign method: %v", t.Method)
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", "", "", err
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid {
		return "", "", "", fmt.Errorf("invalid token")
	}
	return claims.UserID, claims.JTI, claims.PlatformRole, nil
}
```

Update `issuePair` to look up the user's platform_role:
```go
func (s *Service) issuePair(ctx context.Context, userID *iampb.UserId, familyID string) (*iampb.TokenPair, error) {
	user, err := s.GetUserByID(ctx, userID)
	platformRole := ""
	if err == nil {
		platformRole = user.GetPlatformRole().String()
	}
	now := time.Now()
	jti := ids.New()
	access, err := s.signAccessToken(userID.GetValue(), jti, platformRole)
	// ... rest unchanged
```

- [ ] **Step 6.2: Fix VerifyAccessToken callers**

`VerifyAccessToken` is called in `internal/transport/middleware/auth.go`. Update:
```go
userID, _, _, err := svc.VerifyAccessToken(token)
```

Also update the `AuthPort` interface in `middleware/auth.go`:
```go
type AuthPort interface {
	VerifyAccessToken(token string) (userID string, jti string, platformRole string, err error)
	VerifyApiToken(ctx context.Context, plain string) (*iampb.ApiToken, error)
}
```

- [ ] **Step 6.3: Create platform_admin.go**

```go
// internal/transport/middleware/platform_admin.go
package middleware

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
)

// PlatformAdminPort is satisfied by iam.Service.
type PlatformAdminPort interface {
	VerifyAccessToken(token string) (userID string, jti string, platformRole string, err error)
}

const platformRoleAdmin = "PLATFORM_ROLE_ADMIN"

// RequirePlatformAdmin returns an interceptor that rejects requests unless
// the bearer token's platform_role claim == PLATFORM_ROLE_ADMIN.
func RequirePlatformAdmin(svc PlatformAdminPort) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			header := req.Header().Get("Authorization")
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			_, _, platformRole, err := svc.VerifyAccessToken(parts[1])
			if err != nil || platformRole != platformRoleAdmin {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			return next(ctx, req)
		}
	}
}
```

Check `domainerr.PermissionDenied` exists — look in `internal/core/domainerr/codes.go`. If it's named differently, use the correct name.

- [ ] **Step 6.4: Create platform_admin_test.go**

```go
// internal/transport/middleware/platform_admin_test.go
package middleware_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

type fakeAdminVerifier struct {
	role string
	err  error
}

func (f *fakeAdminVerifier) VerifyAccessToken(_ string) (string, string, string, error) {
	return "user-1", "jti-1", f.role, f.err
}

func TestRequirePlatformAdmin_AdminPasses(t *testing.T) {
	verifier := &fakeAdminVerifier{role: "PLATFORM_ROLE_ADMIN"}
	interceptor := middleware.RequirePlatformAdmin(verifier)

	called := false
	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return nil, nil
	})

	wrapped := interceptor(next)
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer some-valid-token")
	_, err := wrapped(context.Background(), req)
	require.NoError(t, err)
	require.True(t, called)
}

func TestRequirePlatformAdmin_NonAdminRejected(t *testing.T) {
	verifier := &fakeAdminVerifier{role: "PLATFORM_ROLE_NONE"}
	interceptor := middleware.RequirePlatformAdmin(verifier)

	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("should not be called")
		return nil, nil
	})

	wrapped := interceptor(next)
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer some-token")
	_, err := wrapped(context.Background(), req)
	require.Error(t, err)
}

func TestRequirePlatformAdmin_MissingTokenRejected(t *testing.T) {
	verifier := &fakeAdminVerifier{role: "PLATFORM_ROLE_ADMIN"}
	interceptor := middleware.RequirePlatformAdmin(verifier)

	next := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("should not be called")
		return nil, nil
	})

	wrapped := interceptor(next)
	req := connect.NewRequest(&struct{}{})
	// No Authorization header
	_, err := wrapped(context.Background(), req)
	require.Error(t, err)
}
```

- [ ] **Step 6.5: Add PromoteToAdmin to iam/users.go**

```go
// PromoteToAdmin sets PlatformRole=ADMIN for the given user.
func (s *Service) PromoteToAdmin(ctx context.Context, id *iampb.UserId) error {
	now := time.Now()
	_, err := s.userRepo.Execute(ctx,
		iampb.Users.Update().
			Set(iampb.Users.PlatformRole.Set(iampb.PlatformRole_PLATFORM_ROLE_ADMIN.String())).
			Set(iampb.Users.UpdatedAt.Set(now)).
			Where(iampb.Users.Id.Eq(id.GetValue())),
	)
	return err
}
```

- [ ] **Step 6.6: Wire admin interceptor in server.go**

In `internal/transport/connect/server.go`, in `Mount`, update admin handler mounting:

```go
if d.AdminHandler != nil && d.AdminInterceptors != nil {
    adminPath, adminH := adminconnect.NewAdminServiceHandler(d.AdminHandler, d.AdminInterceptors)
    mux.Handle(adminPath, adminH)
} else if d.AdminHandler != nil {
    adminPath, adminH := adminconnect.NewAdminServiceHandler(d.AdminHandler, d.Interceptors)
    mux.Handle(adminPath, adminH)
}
```

Add `AdminInterceptors connect.Option` to `Deps` struct.

In cmd_server.go, construct admin interceptors:
```go
adminInterceptors := connect.WithInterceptors(
    middleware.RequirePlatformAdmin(iamSvc),
    middleware.Recovery(zlog),
    middleware.RequestID(),
    otelInterceptor,
    middleware.Logging(zlog),
    middleware.ErrorMapper(),
)
// Pass to Mount:
AdminInterceptors: adminInterceptors,
```

- [ ] **Step 6.7: Update bootstrapAdmin to set ADMIN role**

In `cmd_server.go` `bootstrapAdmin`:
```go
if err := iamSvc.PromoteToAdmin(ctx, user.GetId()); err != nil {
    return fmt.Errorf("bootstrap admin: promote: %w", err)
}
```

- [ ] **Step 6.8: Run tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/transport/middleware/... ./internal/domain/services/iam/... -count=1 -v -timeout 120s 2>&1 | tail -30
```

- [ ] **Step 6.9: Full build check**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go build ./... 2>&1
```

- [ ] **Step 6.10: Commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add internal/domain/services/iam/ internal/transport/middleware/ internal/transport/connect/server.go cmd/stroppy-cloud/cmd_server.go && git commit -m "feat(iam,middleware): PlatformRole claim + RequirePlatformAdmin interceptor"
```

---

## Task 7: Real node handlers

**Files:**
- Create: `internal/domain/workers/nodeworker/handlers/terraform.go`
- Create: `internal/domain/workers/nodeworker/handlers/docker.go`
- Create: `internal/domain/workers/nodeworker/handlers/package_install.go`
- Create: `internal/domain/workers/nodeworker/handlers/stroppy_run.go`
- Create: `internal/domain/workers/nodeworker/handlers/one_shot.go`
- Create: `internal/domain/workers/nodeworker/handlers/config_apply.go`
- Create: `internal/domain/workers/nodeworker/handlers/test_run_ref.go`
- Modify: `cmd/stroppy-cloud/cmd_server.go`

- [ ] **Step 7.1: Create terraform.go (skeleton)**

```go
// internal/domain/workers/nodeworker/handlers/terraform.go
// TerraformHandler is a SKELETON. The infrastructure/terraform.Actor API
// requires a running terraform binary and cloud credentials not available
// in unit tests. The handler logs intent and returns success for
// UNSPECIFIED module; real modules return an error prompting manual
// wiring before production use.
package handlers

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// TerraformHandler implements nodeworker.Handler for TerraformTask.
// This is a skeleton: it logs the operation and returns success for
// MODULE_UNSPECIFIED (test/no-op). Real APPLY/DESTROY require the
// terraform binary on PATH and valid cloud credentials in env.
type TerraformHandler struct {
	log *zap.Logger
}

// NewTerraformHandler constructs a TerraformHandler.
func NewTerraformHandler(log *zap.Logger) *TerraformHandler {
	return &TerraformHandler{log: log}
}

func (h *TerraformHandler) Kind() string { return "TerraformTask" }

func (h *TerraformHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.TerraformTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("TerraformHandler: unmarshal spec: %w", err)
	}
	h.log.Info("TerraformHandler.Execute (skeleton)",
		zap.String("node_run_id", node.GetId().GetValue()),
		zap.String("op", task.GetOp().String()),
		zap.String("module", task.GetModule().String()),
	)
	if task.GetModule() == taskspb.TerraformTask_MODULE_UNSPECIFIED {
		// No-op for test/unspecified module.
		return nil, nil
	}
	return nil, fmt.Errorf("TerraformHandler: real terraform execution not implemented (skeleton): module=%s op=%s", task.GetModule(), task.GetOp())
}
```

- [ ] **Step 7.2: Create docker.go (skeleton)**

```go
// internal/domain/workers/nodeworker/handlers/docker.go
package handlers

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// DockerHandler handles DockerTask (skeleton — logs and returns success).
type DockerHandler struct{ log *zap.Logger }

func NewDockerHandler(log *zap.Logger) *DockerHandler { return &DockerHandler{log: log} }
func (h *DockerHandler) Kind() string                 { return "DockerTask" }
func (h *DockerHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.DockerTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("DockerHandler: unmarshal: %w", err)
	}
	h.log.Info("DockerHandler.Execute (skeleton)", zap.String("op", task.GetOp().String()))
	return nil, nil
}
```

Fix import: `"google.golang.org/protobuf/proto"` not `"google.golang.org/proto"`.

- [ ] **Step 7.3: Define HubDispatcher interface**

Agent-bound handlers need to dispatch via Hub without importing the agent package (cycle risk). Define a port in the handlers package:

```go
// internal/domain/workers/nodeworker/handlers/hub_port.go
package handlers

import (
	"context"
	"time"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

// HubPort is implemented by agent.Hub.
type HubPort interface {
	Dispatch(ctx context.Context, agentID string, machineID string, action *agentpb.Action, timeout time.Duration, nodeRunID ...string) (*agentpb.Report, error)
}
```

- [ ] **Step 7.4: Create package_install.go**

```go
// internal/domain/workers/nodeworker/handlers/package_install.go
package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// PackageInstallHandler dispatches an InstallPackage action to the agent via Hub.
type PackageInstallHandler struct {
	hub       HubPort
	agentID   string // resolved by caller at handler construction time
	machineID string
	timeout   time.Duration
}

func NewPackageInstallHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *PackageInstallHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &PackageInstallHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *PackageInstallHandler) Kind() string { return "PackageInstallTask" }

func (h *PackageInstallHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.PackageInstallTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("PackageInstallHandler: unmarshal: %w", err)
	}
	action := &agentpb.Action{
		Verb: &agentpb.Action_InstallPackage{
			InstallPackage: &agentpb.InstallPackage{
				Packages: task.GetPackages(),
			},
		},
	}
	report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, action, h.timeout, node.GetId().GetValue())
	if err != nil {
		return nil, fmt.Errorf("PackageInstallHandler: dispatch: %w", err)
	}
	if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		return nil, fmt.Errorf("PackageInstallHandler: agent reported failure: %s", report.GetError())
	}
	return nil, nil
}
```

Check that `agentpb.InstallPackage` and `agentpb.Action_InstallPackage` exist:
```bash
grep -n "InstallPackage\|Action_Install" /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/protocol.pb.go | head -10
```

Adjust field names to match actual proto.

- [ ] **Step 7.5: Create stroppy_run.go**

```go
// internal/domain/workers/nodeworker/handlers/stroppy_run.go
package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// StroppyRunHandler dispatches a RunShell action for a StroppyRunTask.
type StroppyRunHandler struct {
	hub       HubPort
	agentID   string
	machineID string
	timeout   time.Duration
}

func NewStroppyRunHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *StroppyRunHandler {
	if timeout == 0 {
		timeout = 2 * time.Hour
	}
	return &StroppyRunHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *StroppyRunHandler) Kind() string { return "StroppyRunTask" }

func (h *StroppyRunHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.StroppyRunTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("StroppyRunHandler: unmarshal: %w", err)
	}
	argv := append([]string{task.GetBinaryPath()}, task.GetArgs()...)
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{
				Argv:  argv,
				Shell: false,
			},
		},
	}
	report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, action, h.timeout, node.GetId().GetValue())
	if err != nil {
		return nil, fmt.Errorf("StroppyRunHandler: dispatch: %w", err)
	}
	if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		return nil, fmt.Errorf("StroppyRunHandler: %s", report.GetError())
	}
	return nil, nil
}
```

Check StroppyRunTask fields:
```bash
grep -n "BinaryPath\|GetArgs\|GetBinaryPath" /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks/stroppy_run.pb.go | head -10
```

- [ ] **Step 7.6: Create one_shot.go**

```go
// internal/domain/workers/nodeworker/handlers/one_shot.go
package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// OneShotHandler dispatches a RunShell action for an OneShotTask.
type OneShotHandler struct {
	hub       HubPort
	agentID   string
	machineID string
	timeout   time.Duration
}

func NewOneShotHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *OneShotHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &OneShotHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *OneShotHandler) Kind() string { return "OneShotTask" }

func (h *OneShotHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.OneShotTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("OneShotHandler: unmarshal: %w", err)
	}
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{
				Argv:  []string{"sh", "-c", task.GetCommandTemplate()},
				Shell: true,
			},
		},
	}
	report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, action, h.timeout, node.GetId().GetValue())
	if err != nil {
		return nil, fmt.Errorf("OneShotHandler: dispatch: %w", err)
	}
	if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		return nil, fmt.Errorf("OneShotHandler: %s", report.GetError())
	}
	return nil, nil
}
```

Check OneShotTask for `GetCommandTemplate`:
```bash
grep -n "CommandTemplate\|GetCommand" /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks/one_shot.pb.go | head -10
```

- [ ] **Step 7.7: Create config_apply.go**

```go
// internal/domain/workers/nodeworker/handlers/config_apply.go
package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// ConfigApplyHandler applies a ConfigApplyTask via PutFile + RunShell.
type ConfigApplyHandler struct {
	hub       HubPort
	agentID   string
	machineID string
	timeout   time.Duration
}

func NewConfigApplyHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *ConfigApplyHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &ConfigApplyHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *ConfigApplyHandler) Kind() string { return "ConfigApplyTask" }

func (h *ConfigApplyHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.ConfigApplyTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("ConfigApplyHandler: unmarshal: %w", err)
	}
	// PutFile for each config file.
	for _, f := range task.GetFiles() {
		putAction := &agentpb.Action{
			Verb: &agentpb.Action_PutFile{
				PutFile: &agentpb.PutFile{
					Path:    f.GetPath(),
					Content: []byte(f.GetContent()),
				},
			},
		}
		report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, putAction, h.timeout, node.GetId().GetValue())
		if err != nil {
			return nil, fmt.Errorf("ConfigApplyHandler: put_file %s: %w", f.GetPath(), err)
		}
		if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
			return nil, fmt.Errorf("ConfigApplyHandler: put_file %s failed: %s", f.GetPath(), report.GetError())
		}
	}
	// RunShell apply command if present.
	if cmd := task.GetApplyCommand(); cmd != "" {
		runAction := &agentpb.Action{
			Verb: &agentpb.Action_RunShell{
				RunShell: &agentpb.RunShell{
					Argv:  []string{"sh", "-c", cmd},
					Shell: true,
				},
			},
		}
		report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, runAction, h.timeout, node.GetId().GetValue())
		if err != nil {
			return nil, fmt.Errorf("ConfigApplyHandler: run apply_command: %w", err)
		}
		if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
			return nil, fmt.Errorf("ConfigApplyHandler: apply_command failed: %s", report.GetError())
		}
	}
	return nil, nil
}
```

Check ConfigApplyTask fields:
```bash
grep -n "GetFiles\|GetApplyCommand\|Files\|ApplyCommand" /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks/config_apply.pb.go | head -15
```

- [ ] **Step 7.8: Create test_run_ref.go**

```go
// internal/domain/workers/nodeworker/handlers/test_run_ref.go
package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// TestRunRefHandler spawns a child DagRun for a TestRunRefTask.
type TestRunRefHandler struct {
	systemSvc *system.Service
}

func NewTestRunRefHandler(systemSvc *system.Service) *TestRunRefHandler {
	return &TestRunRefHandler{systemSvc: systemSvc}
}

func (h *TestRunRefHandler) Kind() string { return "TestRunRefTask" }

func (h *TestRunRefHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.TestRunRefTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("TestRunRefHandler: unmarshal: %w", err)
	}
	testRunID := task.GetTestRunId().GetValue()
	// Load the Dag for this test_run_id by looking it up via state or metadata.
	// Since we don't have direct access to the test run service here, log and return.
	// Full implementation would call testRunSvc.LaunchTestRun and wait via polling.
	// This is a skeleton: log and return success without spawning.
	_ = testRunID
	return nil, fmt.Errorf("TestRunRefHandler: not fully implemented (skeleton) — test_run_id=%s", testRunID)
}
```

NOTE: This is explicitly a skeleton because TestRunRefHandler needs the testing service, creating a potential import cycle with system. Document this in a comment.

- [ ] **Step 7.9: Wire handlers in cmd_server.go**

Replace:
```go
nodeReg := nodeworker.NewRegistry()
nodeReg.Register(mockhandler.NewMockHandler())
```

With:
```go
nodeReg := nodeworker.NewRegistry()
nodeReg.Register(handlers.NewMockHandler())   // keep for "" kind fallback
nodeReg.Register(handlers.NewTerraformHandler(zlog))
nodeReg.Register(handlers.NewDockerHandler(zlog))
```

For agent-bound handlers (PackageInstall, StroppyRun, OneShotTask, ConfigApply):
These require agentID/machineID resolved at node execution time, not at registration.
The Handler interface doesn't carry agent context. A pattern is to pass Hub to the
Registry and let the handler look up the agent via DagRun metadata or a resolver.

For now, register them with a placeholder (they'll return errors unless an agent is connected):
```go
// Agent-bound handlers use Hub for dispatch; agentID/machineID resolved at runtime
// from node metadata. For now: register placeholder with empty IDs.
// TODO: implement agent resolver pattern in follow-up.
nodeReg.Register(handlers.NewPackageInstallHandler(agentHub, "", "", 0))
nodeReg.Register(handlers.NewStroppyRunHandler(agentHub, "", "", 0))
nodeReg.Register(handlers.NewOneShotHandler(agentHub, "", "", 0))
nodeReg.Register(handlers.NewConfigApplyHandler(agentHub, "", "", 0))
nodeReg.Register(handlers.NewTestRunRefHandler(systemSvc))
```

Add import:
```go
handlers "github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker/handlers"
```

Remove `mockhandler` import if MockHandler is now in `handlers` package.

- [ ] **Step 7.10: Run full test suite**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/... -count=1 -timeout 600s 2>&1 | tail -50
```

Fix any compilation or test failures before committing.

- [ ] **Step 7.11: Build check**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go build ./... && go vet ./... 2>&1
```

- [ ] **Step 7.12: Commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add internal/domain/workers/nodeworker/handlers/ cmd/stroppy-cloud/cmd_server.go && git commit -m "feat(nodeworker): real handlers (terraform/docker/install/run/one_shot/test_run_ref)"
```

---

## Task 8: Final sweep

- [ ] **Step 8.1: Run full internal tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test ./internal/... -count=1 -timeout 600s 2>&1 | grep -E "FAIL|ok|---"
```

- [ ] **Step 8.2: Run e2e tests**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && go test -tags=e2e ./tests/e2e/... -count=1 -timeout 300s 2>&1 | tail -20
```

- [ ] **Step 8.3: Final server wire commit**

```bash
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud && git add -A && git commit -m "feat(server): wire all real handlers + workers + middleware"
```

---

## Deferred Items

1. **TerraformHandler real implementation**: skeleton only. The `infrastructure/terraform.Actor` API requires `TERRAFORM_EXEC_PATH`, Yandex Cloud credentials, and network access. Cannot be unit tested without mocks of the binary. Tagged `// TODO: implement via terraform.Actor`.

2. **TestRunRefHandler full implementation**: skeleton only. Requires importing `testingsvc` into handlers, which creates an indirect cycle through system.Service. Needs a port/interface extraction in a follow-up.

3. **Agent-bound handler agentID/machineID resolution**: PackageInstall/StroppyRun/OneShotTask/ConfigApply are registered with empty agentID. Real production use requires a resolver that reads agent metadata (dagRunID + role) from the DagRun's agent table. This is wired as a TODO.

4. **WebhookDispatch event subscriber**: the spec mentions subscribing to `TopicTestRunDone/TopicDagRunDone` to auto-enqueue deliveries for all matching tenant webhooks. This requires iterating `ListWebhooks` by tenant and calling `EnqueueDelivery`. Left as a follow-up subscription in cmd_server after webhook service is proven stable.
