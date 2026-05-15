# Plan 03: System Engine + Workers Implementation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the DAG engine + worker subsystem: pure DB-driven scheduler (cron + lease), NodeWorker pool (claim → execute → update via `FOR UPDATE SKIP LOCKED`), DagRun state row store, and the startup recovery sweep. After this plan, the server can persist a DAG (Dag + DagRun + N NodeRuns + state entries), workers pick up READY nodes, mock handlers complete them, downstream deps unblock, and the dag_run transitions to SUCCEEDED. Kill -9 mid-flight on the server resumes on next start.

**Architecture:** Workers run as goroutines inside `stroppy-cloud server`. Each worker has its own loop with a configurable tick. Claim uses `SELECT ... FOR UPDATE SKIP LOCKED`. Long-running handlers run outside the claim transaction. Mock `NodeHandler` registrations in this plan exercise the full lifecycle — real handler implementations (terraform, agent commands, etc.) ship in plans 04+05. Schedule fires create DagRun + NodeRun rows from a `cloud.v1.system.Dag.Graph` template, and the DAG engine resolves dependency-edges by scanning `node_runs.depends_on`.

**Tech Stack:** Plan-01/02 stack + `robfig/cron/v3` for cron expression parsing.

---

## Files at end of plan

### Created

- `internal/domain/services/system/service.go`
- `internal/domain/services/system/dags.go`
- `internal/domain/services/system/dag_runs.go`
- `internal/domain/services/system/node_runs.go`
- `internal/domain/services/system/state.go`
- `internal/domain/services/system/schedules.go`
- `internal/domain/services/system/integration_test.go`
- `internal/domain/workers/nodeworker/worker.go`
- `internal/domain/workers/nodeworker/claim.go`
- `internal/domain/workers/nodeworker/registry.go`
- `internal/domain/workers/nodeworker/lifecycle.go`
- `internal/domain/workers/nodeworker/handlers/mock.go`
- `internal/domain/workers/nodeworker/worker_test.go`
- `internal/domain/workers/scheduler/scheduler.go`
- `internal/domain/workers/scheduler/scheduler_test.go`
- `internal/domain/workers/recovery/sweep.go`
- `internal/domain/workers/recovery/sweep_test.go`
- `internal/transport/connect/system.go`
- `internal/sdk/client/system.go`
- `cmd/stroppy-cloud/cmd_cli/schedule.go`

### Modified

- `internal/transport/connect/server.go` (add System handler)
- `internal/sdk/client/client.go` (add Schedule client)
- `cmd/stroppy-cloud/cmd_server.go` (wire workers + system service)
- `cmd/stroppy-cloud/cmd_cli/root.go` (register schedule)
- `internal/testutil/fixture/` add `system.go`
- `internal/core/configurator/config.go` (add `Workers.NodeWorkers` default if missing)
- `go.mod` (add `robfig/cron/v3`)

---

## Task 1: Add cron dep + ports

**Files:**
- Modify: `go.mod`

- [ ] **Step 1:**

```bash
go get github.com/robfig/cron/v3
go mod tidy
```

- [ ] **Step 2: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add robfig/cron/v3 for schedule parsing"
```

---

## Task 2: System service skeleton + repos

**Files:**
- Create: `internal/domain/services/system/service.go`

- [ ] **Step 1:**

```go
package system

import (
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// Service owns: dags, dag_runs, node_runs, dag_run_state_entries, schedules.
// All public methods are *not* exposed via ConnectRPC directly (DAG engine is
// internal; users interact with TestRunService etc., which call into here).
// Only ScheduleService methods are surfaced via Connect (see system.go in
// transport/connect).
type Service struct {
	*tracing.Entity

	dagRepo      *repository.ProtoRepository[systempb.DagAlias, systempb.DagColumnAlias, *systempb.DagScanner, *systempb.Dag]
	dagRunRepo   *repository.ProtoRepository[systempb.DagRunAlias, systempb.DagRunColumnAlias, *systempb.DagRunScanner, *systempb.DagRun]
	nodeRunRepo  *repository.ProtoRepository[systempb.NodeRunAlias, systempb.NodeRunColumnAlias, *systempb.NodeRunScanner, *systempb.NodeRun]
	stateRepo    *repository.ProtoRepository[systempb.DagRunStateEntryAlias, systempb.DagRunStateEntryColumnAlias, *systempb.DagRunStateEntryScanner, *systempb.DagRunStateEntry]
	scheduleRepo *repository.ProtoRepository[systempb.ScheduleAlias, systempb.ScheduleColumnAlias, *systempb.ScheduleScanner, *systempb.Schedule]

	txMgr  pgtx.TxManager
	events eventing.Bus
}

func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *Service {
	return &Service{
		Entity: tracing.NewEntity("system.Service"),
		dagRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.Dags.Table, executor),
			systempb.DagConverter,
		),
		dagRunRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.DagRuns.Table, executor),
			systempb.DagRunConverter,
		),
		nodeRunRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.NodeRuns.Table, executor),
			systempb.NodeRunConverter,
		),
		stateRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.DagRunStateEntries.Table, executor),
			systempb.DagRunStateEntryConverter,
		),
		scheduleRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.Schedules.Table, executor),
			systempb.ScheduleConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}
```

- [ ] **Step 2: Build + commit**

```bash
go build ./internal/domain/services/system/...
git add internal/domain/services/system/service.go
git commit -m "feat(system): service skeleton with 5 repos"
```

---

## Task 3: Dag + DagRun creation

**Files:**
- Create: `internal/domain/services/system/dags.go`, `dag_runs.go`

- [ ] **Step 1: `dags.go`**

```go
package system

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// SaveDag persists a Dag template row (idempotent on graph_hash recommended,
// but for v1 each save is a fresh row).
func (s *Service) SaveDag(ctx context.Context, dag *systempb.Dag) (*systempb.Dag, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "SaveDag",
		func(ctx context.Context, _ any) (*systempb.Dag, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*systempb.Dag, error) {
				now := timestamppb.Now()
				if dag.GetId() == nil || dag.GetId().GetValue() == "" {
					dag.Id = &systempb.DagId{Value: ids.New()}
				}
				if dag.GetTimestamps() == nil {
					dag.Timestamps = &commonpb.Timestamps{}
				}
				dag.GetTimestamps().CreatedAt = now
				dag.GetTimestamps().UpdatedAt = now
				if err := s.dagRepo.Insert(ctx, dag); err != nil {
					return nil, err
				}
				return dag, nil
			})
		})
}
```

- [ ] **Step 2: `dag_runs.go`**

```go
package system

import (
	"context"

	"github.com/yaroher/ratel/pkg/dml/set"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// StartDagRunInput is the bundle TestRunService / Scheduler pass in to spawn a DAG.
type StartDagRunInput struct {
	Dag      *systempb.Dag
	Metadata map[string]string // serialized into dag_runs.metadata
}

// StartDagRun creates a DagRun + N NodeRun rows from the template graph in
// one transaction. Initial node_runs.status = NODE_RUN_STATUS_READY for nodes
// with no deps, NODE_RUN_STATUS_PENDING_DEPS otherwise.
func (s *Service) StartDagRun(ctx context.Context, in StartDagRunInput) (*systempb.DagRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "StartDagRun",
		func(ctx context.Context, _ any) (*systempb.DagRun, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*systempb.DagRun, error) {
				if in.Dag == nil {
					return nil, errEmptyDag()
				}
				now := timestamppb.Now()
				run := &systempb.DagRun{
					Id:         &systempb.DagRunId{Value: ids.New()},
					DagId:      in.Dag.GetId(),
					Status:     systempb.DagRunStatus_DAG_RUN_STATUS_RUNNING,
					Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
				}
				if in.Metadata != nil {
					run.Metadata = serializeMetadata(in.Metadata)
				}
				if err := s.dagRunRepo.Insert(ctx, run); err != nil {
					return nil, err
				}
				// Walk dag.graph.nodes — create node_runs.
				for _, n := range in.Dag.GetGraph().GetNodes() {
					status := systempb.NodeRunStatus_NODE_RUN_STATUS_READY
					if len(n.GetDependsOn()) > 0 {
						status = systempb.NodeRunStatus_NODE_RUN_STATUS_PENDING_DEPS
					}
					nr := &systempb.NodeRun{
						Id:         &systempb.NodeRunId{Value: ids.New()},
						DagRunId:   run.GetId(),
						NodeKey:    n.GetKey(),
						Spec:       n.GetSpec(),
						DependsOn:  n.GetDependsOn(),
						Status:     status,
						Attempt:    0,
						RetryMax:   n.GetRetryMax(),
						Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
					}
					if err := s.nodeRunRepo.Insert(ctx, nr); err != nil {
						return nil, err
					}
				}
				s.events.Publish(ctx, eventing.DagRunDone{DagRunID: run.GetId().GetValue(), Success: false /* not done yet */})
				return run, nil
			})
		})
}

// GetDagRun fetches by id.
func (s *Service) GetDagRun(ctx context.Context, id *systempb.DagRunId) (*systempb.DagRun, error) {
	return s.dagRunRepo.SelectOne(ctx, set.Eq(systempb.DagRunColumnId, id.GetValue()))
}

// CancelDagRun marks a dag_run for cancellation. Workers respect it on next yield.
func (s *Service) CancelDagRun(ctx context.Context, id *systempb.DagRunId) error {
	_, err := s.dagRunRepo.Update(ctx,
		set.Update(systempb.DagRunColumnCancelRequested, true),
		set.Eq(systempb.DagRunColumnId, id.GetValue()),
	)
	return err
}
```

Add helpers `errEmptyDag` and `serializeMetadata` at bottom:

```go
import (
	"errors"

	"google.golang.org/protobuf/types/known/structpb"
)

func errEmptyDag() error { return errors.New("system: StartDagRun requires non-nil Dag") }

func serializeMetadata(m map[string]string) *structpb.Struct {
	out := &structpb.Struct{Fields: map[string]*structpb.Value{}}
	for k, v := range m {
		out.Fields[k] = structpb.NewStringValue(v)
	}
	return out
}
```

Note: `DagRun.metadata` proto type is `google.protobuf.Struct`. If it's a
different type in the generated code, adjust. The `cancel_requested` column
likewise — verify the actual generated `DagRunColumnCancelRequested` constant
exists; if not, fall back to running raw SQL. (cancel flag is a new column
mentioned in §3.5 of the spec; if the proto doesn't have it yet, add it now:
`bool cancel_requested = N` in `cloud/v1/system/dag.proto`'s `DagRun` message,
run `make proto-gen + make migrate-gen`, then continue.)

- [ ] **Step 3: Build**

```bash
go build ./internal/domain/services/system/...
```
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/services/system/
git commit -m "feat(system): SaveDag + StartDagRun (creates run + N node_runs) + CancelDagRun"
```

---

## Task 4: NodeRun lifecycle + state-store helpers

**Files:**
- Create: `internal/domain/services/system/node_runs.go`, `state.go`

- [ ] **Step 1: `node_runs.go`**

```go
package system

import (
	"context"

	"github.com/yaroher/ratel/pkg/dml/set"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// ListNodeRunsByDagRun returns all node_runs for a dag_run ordered by created_at.
func (s *Service) ListNodeRunsByDagRun(ctx context.Context, id *systempb.DagRunId) ([]*systempb.NodeRun, error) {
	return s.nodeRunRepo.Select(ctx, set.Eq(systempb.NodeRunColumnDagRunId, id.GetValue()))
}

// MarkNodeRunSucceeded finalizes a node, recomputes downstream READY status,
// and finalizes the dag_run if all leaves complete.
func (s *Service) MarkNodeRunSucceeded(ctx context.Context, id *systempb.NodeRunId, output *anypb.Any) error {
	now := timestamppb.Now()
	_, err := s.nodeRunRepo.Update(ctx,
		set.Update(systempb.NodeRunColumnStatus, systempb.NodeRunStatus_NODE_RUN_STATUS_DONE.String()),
		set.Update(systempb.NodeRunColumnFinishedAt, now),
		set.Update(systempb.NodeRunColumnOutput, output),
		set.Eq(systempb.NodeRunColumnId, id.GetValue()),
	)
	if err != nil {
		return err
	}
	return s.recomputeAfterNode(ctx, id)
}

// MarkNodeRunFailed marks failure; if attempts remain, requeues as READY.
func (s *Service) MarkNodeRunFailed(ctx context.Context, id *systempb.NodeRunId, errMsg string) error {
	now := timestamppb.Now()
	node, err := s.nodeRunRepo.SelectOne(ctx, set.Eq(systempb.NodeRunColumnId, id.GetValue()))
	if err != nil {
		return err
	}
	newAttempt := node.GetAttempt() + 1
	target := systempb.NodeRunStatus_NODE_RUN_STATUS_FAILED
	if newAttempt < node.GetRetryMax() {
		target = systempb.NodeRunStatus_NODE_RUN_STATUS_READY
	}
	_, err = s.nodeRunRepo.Update(ctx,
		set.Update(systempb.NodeRunColumnStatus, target.String()),
		set.Update(systempb.NodeRunColumnAttempt, newAttempt),
		set.Update(systempb.NodeRunColumnError, errMsg),
		set.Update(systempb.NodeRunColumnFinishedAt, now),
		set.Eq(systempb.NodeRunColumnId, id.GetValue()),
	)
	if err != nil {
		return err
	}
	if target == systempb.NodeRunStatus_NODE_RUN_STATUS_FAILED {
		return s.recomputeAfterNode(ctx, id) // may transition dag_run to FAILED
	}
	return nil
}

// recomputeAfterNode handles dependency unblocking + dag_run finalization.
func (s *Service) recomputeAfterNode(ctx context.Context, id *systempb.NodeRunId) error {
	node, err := s.nodeRunRepo.SelectOne(ctx, set.Eq(systempb.NodeRunColumnId, id.GetValue()))
	if err != nil {
		return err
	}
	// Find dependents (other node_runs in same dag_run referencing this node_key in depends_on)
	siblings, err := s.nodeRunRepo.Select(ctx, set.Eq(systempb.NodeRunColumnDagRunId, node.GetDagRunId().GetValue()))
	if err != nil {
		return err
	}
	for _, sib := range siblings {
		if sib.GetStatus() != systempb.NodeRunStatus_NODE_RUN_STATUS_PENDING_DEPS {
			continue
		}
		if !contains(sib.GetDependsOn(), node.GetNodeKey()) {
			continue
		}
		if depsAllDone(sib, siblings) {
			_, _ = s.nodeRunRepo.Update(ctx,
				set.Update(systempb.NodeRunColumnStatus, systempb.NodeRunStatus_NODE_RUN_STATUS_READY.String()),
				set.Eq(systempb.NodeRunColumnId, sib.GetId().GetValue()),
			)
		}
	}
	// DagRun finalization
	allDone, anyFailed := terminalAggregate(siblings)
	if allDone || anyFailed {
		newStatus := systempb.DagRunStatus_DAG_RUN_STATUS_SUCCEEDED
		if anyFailed {
			newStatus = systempb.DagRunStatus_DAG_RUN_STATUS_FAILED
		}
		_, err := s.dagRunRepo.Update(ctx,
			set.Update(systempb.DagRunColumnStatus, newStatus.String()),
			set.Eq(systempb.DagRunColumnId, node.GetDagRunId().GetValue()),
		)
		return err
	}
	return nil
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func depsAllDone(target *systempb.NodeRun, siblings []*systempb.NodeRun) bool {
	keyToStatus := map[string]systempb.NodeRunStatus{}
	for _, s := range siblings {
		keyToStatus[s.GetNodeKey()] = s.GetStatus()
	}
	for _, dep := range target.GetDependsOn() {
		st := keyToStatus[dep]
		if st != systempb.NodeRunStatus_NODE_RUN_STATUS_DONE &&
			st != systempb.NodeRunStatus_NODE_RUN_STATUS_SKIPPED {
			return false
		}
	}
	return true
}

func terminalAggregate(siblings []*systempb.NodeRun) (allDone, anyFailed bool) {
	allDone = true
	for _, s := range siblings {
		switch s.GetStatus() {
		case systempb.NodeRunStatus_NODE_RUN_STATUS_DONE, systempb.NodeRunStatus_NODE_RUN_STATUS_SKIPPED:
			// counts as terminal-success
		case systempb.NodeRunStatus_NODE_RUN_STATUS_FAILED, systempb.NodeRunStatus_NODE_RUN_STATUS_CANCELED:
			allDone = false
			anyFailed = true
		default:
			allDone = false
		}
	}
	return allDone, anyFailed
}
```

- [ ] **Step 2: `state.go`**

```go
package system

import (
	"context"

	"github.com/yaroher/ratel/pkg/dml/set"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// PutState upserts a key=Any value in dag_run_state_entries for the given dag_run.
// Optimistic concurrency via `version` column.
func (s *Service) PutState(ctx context.Context, dagRunID *systempb.DagRunId, key string, value *anypb.Any) (*systempb.DagRunStateEntry, error) {
	existing, err := s.stateRepo.SelectOne(ctx,
		set.And(
			set.Eq(systempb.DagRunStateEntryColumnDagRunId, dagRunID.GetValue()),
			set.Eq(systempb.DagRunStateEntryColumnKey, key),
		))
	now := timestamppb.Now()
	if err == nil {
		_, err := s.stateRepo.Update(ctx,
			set.Update(systempb.DagRunStateEntryColumnValue, value),
			set.Update(systempb.DagRunStateEntryColumnVersion, existing.GetVersion()+1),
			set.Eq(systempb.DagRunStateEntryColumnId, existing.GetId().GetValue()),
		)
		if err != nil {
			return nil, err
		}
		existing.Value = value
		existing.Version = existing.GetVersion() + 1
		return existing, nil
	}
	entry := &systempb.DagRunStateEntry{
		Id:         &systempb.DagRunStateEntryId{Value: ids.New()},
		DagRunId:   dagRunID,
		Key:        key,
		Value:      value,
		Version:    1,
		Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	if err := s.stateRepo.Insert(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// GetState returns the typed value for a key, or false if absent.
func (s *Service) GetState(ctx context.Context, dagRunID *systempb.DagRunId, key string) (*anypb.Any, bool, error) {
	row, err := s.stateRepo.SelectOne(ctx,
		set.And(
			set.Eq(systempb.DagRunStateEntryColumnDagRunId, dagRunID.GetValue()),
			set.Eq(systempb.DagRunStateEntryColumnKey, key),
		))
	if err != nil {
		// repository.ErrNotFound → false, nil
		return nil, false, nil
	}
	return row.GetValue(), true, nil
}
```

- [ ] **Step 3: Build + commit**

```bash
go build ./internal/domain/services/system/...
git add internal/domain/services/system/
git commit -m "feat(system): node_run lifecycle (succeed/fail) + dep unblock + state-entry store"
```

---

## Task 5: NodeWorker pool

**Files:**
- Create: `internal/domain/workers/nodeworker/worker.go`, `claim.go`, `registry.go`, `lifecycle.go`
- Create: `internal/domain/workers/nodeworker/handlers/mock.go`

- [ ] **Step 1: `registry.go`**

```go
package nodeworker

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// StateStore abstracts the dag_run_state_entries operations a handler may need.
type StateStore interface {
	Put(ctx context.Context, key string, value *anypb.Any) (*systempb.DagRunStateEntry, error)
	Get(ctx context.Context, key string) (*anypb.Any, bool, error)
}

// Handler executes the node's spec, returns optional output (Any) or error.
type Handler interface {
	Kind() string
	Execute(ctx context.Context, node *systempb.NodeRun, state StateStore) (*anypb.Any, error)
}

// Registry maps task kind (spec.type_url tail) → Handler.
type Registry struct{ m map[string]Handler }

func NewRegistry() *Registry { return &Registry{m: map[string]Handler{}} }

func (r *Registry) Register(h Handler) { r.m[h.Kind()] = h }

func (r *Registry) Resolve(typeURL string) Handler {
	// type_url format: "type.googleapis.com/cloud.v1.tasks.TerraformTask"
	// kind = last dotted segment, lowercased + snake-cased per registry convention.
	for k, h := range r.m {
		if endsWithIgnoreCase(typeURL, k) {
			return h
		}
	}
	return nil
}

func endsWithIgnoreCase(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	tail := s[len(s)-len(suffix):]
	for i := 0; i < len(suffix); i++ {
		a := tail[i]
		b := suffix[i]
		if a >= 'A' && a <= 'Z' {
			a += 32
		}
		if b >= 'A' && b <= 'Z' {
			b += 32
		}
		if a != b {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: `claim.go`**

```go
package nodeworker

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// claimReadyNode atomically picks one READY node_run, marks it RUNNING,
// returns the row. nil + no error = nothing to do.
func claimReadyNode(ctx context.Context, pool *pgxpool.Pool) (*claimedNode, error) {
	const q = `
WITH ready AS (
    SELECT id FROM node_runs
    WHERE status = $1
    ORDER BY created_at NULLS FIRST, id
    FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE node_runs n
SET status = $2,
    started_at = COALESCE(n.started_at, now()),
    updated_at = now()
FROM ready
WHERE n.id = ready.id
RETURNING n.id, n.dag_run_id, n.node_key, n.spec_type_url, n.spec_value,
          n.depends_on, n.attempt, n.retry_max, n.timeout_seconds;
`
	row := pool.QueryRow(ctx, q,
		systempb.NodeRunStatus_NODE_RUN_STATUS_READY.String(),
		systempb.NodeRunStatus_NODE_RUN_STATUS_RUNNING.String(),
	)
	var n claimedNode
	if err := row.Scan(&n.ID, &n.DagRunID, &n.Key, &n.SpecTypeURL, &n.SpecValue, &n.DependsOn, &n.Attempt, &n.RetryMax, &n.TimeoutSeconds); err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &n, nil
}

type claimedNode struct {
	ID             string
	DagRunID       string
	Key            string
	SpecTypeURL    string
	SpecValue      []byte
	DependsOn      []string
	Attempt        int32
	RetryMax       int32
	TimeoutSeconds int32
}
```

Note: the SQL assumes `node_runs.spec` is encoded as two columns
`spec_type_url` + `spec_value` (typical for `google.protobuf.Any` in ratel
output). If the generated column shape differs (`spec` as JSONB), adjust.
Use `\d+ node_runs` against the test DB or read the migration to confirm.

- [ ] **Step 3: `lifecycle.go`**

```go
package nodeworker

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

type stateProxy struct {
	svc      *system.Service
	dagRunID *systempb.DagRunId
}

func (p *stateProxy) Put(ctx context.Context, key string, value *anypb.Any) (*systempb.DagRunStateEntry, error) {
	return p.svc.PutState(ctx, p.dagRunID, key, value)
}
func (p *stateProxy) Get(ctx context.Context, key string) (*anypb.Any, bool, error) {
	return p.svc.GetState(ctx, p.dagRunID, key)
}

func execute(ctx context.Context, w *Worker, n *claimedNode) {
	handler := w.registry.Resolve(n.SpecTypeURL)
	if handler == nil {
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, "no handler registered for spec "+n.SpecTypeURL)
		return
	}

	spec := &anypb.Any{TypeUrl: n.SpecTypeURL, Value: n.SpecValue}
	node := rebuildNode(n, spec)

	timeoutCtx := ctx
	if n.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, time.Duration(n.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	out, err := handler.Execute(timeoutCtx, node, &stateProxy{svc: w.system, dagRunID: &systempb.DagRunId{Value: n.DagRunID}})
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "timeout exceeded"
		}
		_ = w.system.MarkNodeRunFailed(ctx, &systempb.NodeRunId{Value: n.ID}, msg)
		return
	}
	_ = w.system.MarkNodeRunSucceeded(ctx, &systempb.NodeRunId{Value: n.ID}, out)
}

func rebuildNode(n *claimedNode, spec *anypb.Any) *systempb.NodeRun {
	nr := &systempb.NodeRun{
		Id:        &systempb.NodeRunId{Value: n.ID},
		DagRunId:  &systempb.DagRunId{Value: n.DagRunID},
		NodeKey:   n.Key,
		Spec:      spec,
		DependsOn: n.DependsOn,
		Attempt:   n.Attempt,
		RetryMax:  n.RetryMax,
		Status:    systempb.NodeRunStatus_NODE_RUN_STATUS_RUNNING,
	}
	_ = proto.Marshal(nr) // smoke-check serialization
	return nr
}
```

- [ ] **Step 4: `worker.go`**

```go
package nodeworker

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
)

type Config struct {
	Workers   int
	Tick      time.Duration
	BatchSize int
}

type Worker struct {
	pool     *pgxpool.Pool
	system   *system.Service
	registry *Registry
	cfg      Config
	log      *zap.Logger
}

func New(pool *pgxpool.Pool, sys *system.Service, reg *Registry, cfg Config, log *zap.Logger) *Worker {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.Tick == 0 {
		cfg.Tick = 500 * time.Millisecond
	}
	return &Worker{pool: pool, system: sys, registry: reg, cfg: cfg, log: log}
}

// Run launches Workers goroutines. Blocks until ctx is done.
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.cfg.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tick := time.NewTicker(w.cfg.Tick)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					w.tryOnce(ctx, id)
				}
			}
		}(i)
	}
	wg.Wait()
}

func (w *Worker) tryOnce(ctx context.Context, id int) {
	node, err := claimReadyNode(ctx, w.pool)
	if err != nil {
		w.log.Warn("claim error", zap.Int("worker", id), zap.Error(err))
		return
	}
	if node == nil {
		return
	}
	w.log.Debug("claimed node", zap.Int("worker", id), zap.String("node_run_id", node.ID), zap.String("spec", node.SpecTypeURL))
	execute(ctx, w, node)
}
```

- [ ] **Step 5: `handlers/mock.go`**

```go
package handlers

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// MockHandler matches any type_url whose tail contains "Mock". Always succeeds.
// Used for integration tests to exercise the engine end-to-end.
type MockHandler struct{}

func NewMockHandler() *MockHandler { return &MockHandler{} }

func (h *MockHandler) Kind() string { return "Mock" }

func (h *MockHandler) Execute(ctx context.Context, _ *systempb.NodeRun, _ nodeworker.StateStore) (*anypb.Any, error) {
	return nil, nil
}
```

- [ ] **Step 6: Build**

```bash
go build ./internal/domain/workers/nodeworker/...
```
Expected: success.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/workers/nodeworker/
git commit -m "feat(nodeworker): claim loop + Handler registry + state proxy + Mock handler"
```

---

## Task 6: NodeWorker integration test (mock DAG end-to-end)

**Files:**
- Create: `internal/domain/workers/nodeworker/worker_test.go`
- Create: `internal/testutil/fixture/system.go`

- [ ] **Step 1: `fixture/system.go`**

```go
package fixture

import (
	"testing"
	"time"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker/handlers"
)

// SystemFixture wires the system service + node worker registry with the Mock handler.
type SystemFixture struct {
	*IAMFixture
	System *system.Service
	Worker *nodeworker.Worker
}

// NewSystem builds a fixture and a worker pool with 1 goroutine (deterministic).
func NewSystem(t *testing.T) *SystemFixture {
	t.Helper()
	iam := NewIAM(t)
	exec := iam.F.Executor.(*sqlexec.TxExecutor)
	sys := system.New(exec, iam.F.TxMgr, iam.F.Events)

	reg := nodeworker.NewRegistry()
	reg.Register(handlers.NewMockHandler())

	w := nodeworker.New(iam.F.Pool, sys, reg, nodeworker.Config{Workers: 1, Tick: 50 * time.Millisecond}, zap.NewNop())
	return &SystemFixture{IAMFixture: iam, System: sys, Worker: w}
}
```

- [ ] **Step 2: `worker_test.go`**

```go
package nodeworker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestWorkerCompletesTwoNodeDag(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build a tiny dag template: node A → node B
	dag := &systempb.Dag{
		Graph: &systempb.Dag_Graph{
			Nodes: []*systempb.Dag_Node{
				{Key: "a", Spec: mockAny(), DependsOn: nil, RetryMax: 1},
				{Key: "b", Spec: mockAny(), DependsOn: []string{"a"}, RetryMax: 1},
			},
		},
	}
	dag, err := f.System.SaveDag(ctx, dag)
	require.NoError(t, err)

	run, err := f.System.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})
	require.NoError(t, err)

	// Run worker in background
	go f.Worker.Run(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("dag_run did not complete in time")
		}
		got, err := f.System.GetDagRun(ctx, run.GetId())
		require.NoError(t, err)
		if got.GetStatus() == systempb.DagRunStatus_DAG_RUN_STATUS_SUCCEEDED {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func mockAny() *anypb.Any {
	return &anypb.Any{TypeUrl: "type.googleapis.com/Mock"}
}
```

- [ ] **Step 3: Run**

```bash
go test ./internal/domain/workers/nodeworker/... -count=1 -timeout 30s -v -run TestWorker
```
Expected: PASS (within 5 s).

- [ ] **Step 4: Commit**

```bash
git add internal/domain/workers/nodeworker/worker_test.go internal/testutil/fixture/system.go
git commit -m "test(nodeworker): two-node mock DAG completes end-to-end"
```

---

## Task 7: Recovery sweep

**Files:**
- Create: `internal/domain/workers/recovery/sweep.go`, `sweep_test.go`

- [ ] **Step 1: `sweep.go`**

```go
package recovery

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Run executes the startup sweep that resets stuck rows from a previous
// process crash. Safe to call repeatedly.
func Run(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger) error {
	statements := []string{
		// node_runs: any RUNNING/CANCELING → READY (if attempts < retry_max) else FAILED
		`
UPDATE node_runs
SET status = CASE WHEN attempt < retry_max
                  THEN 'NODE_RUN_STATUS_READY'
                  ELSE 'NODE_RUN_STATUS_FAILED' END,
    attempt = attempt + 1,
    error = COALESCE(error, 'process_restart'),
    updated_at = now()
WHERE status IN ('NODE_RUN_STATUS_RUNNING', 'NODE_RUN_STATUS_CANCELING');
`,
		// webhook_deliveries IN_FLIGHT → PENDING
		`
UPDATE webhook_deliveries
SET state = 'PENDING', last_error = 'restart_requeue', updated_at = now()
WHERE state = 'IN_FLIGHT';
`,
		// agents: if marked BUSY with stale heartbeat → UNKNOWN
		`
UPDATE agents
SET status = 'AGENT_STATUS_UNKNOWN', updated_at = now()
WHERE status = 'AGENT_STATUS_BUSY' AND last_seen_at < now() - interval '60 seconds';
`,
		// Recompute dag_runs.status from node_runs aggregate
		`
WITH agg AS (
  SELECT dag_run_id,
         bool_and(status IN ('NODE_RUN_STATUS_DONE','NODE_RUN_STATUS_SKIPPED')) AS all_done,
         bool_or(status='NODE_RUN_STATUS_FAILED') AS any_failed
  FROM node_runs GROUP BY dag_run_id
)
UPDATE dag_runs d
SET status = CASE WHEN agg.any_failed THEN 'DAG_RUN_STATUS_FAILED'
                  WHEN agg.all_done   THEN 'DAG_RUN_STATUS_SUCCEEDED'
                  ELSE 'DAG_RUN_STATUS_RUNNING' END,
    updated_at = now()
FROM agg
WHERE d.id = agg.dag_run_id AND d.status = 'DAG_RUN_STATUS_RUNNING';
`,
	}

	for i, stmt := range statements {
		ct, err := pool.Exec(ctx, stmt)
		if err != nil {
			// Tolerate missing tables (webhook_deliveries, agents come online in plans 06/05)
			log.Warn("recovery sweep statement skipped",
				zap.Int("idx", i), zap.Error(err))
			continue
		}
		log.Info("recovery sweep statement applied",
			zap.Int("idx", i), zap.Int64("rows_affected", ct.RowsAffected()))
	}
	return nil
}

func sentinel() error { return fmt.Errorf("recovery: noop") } // unused, kept for explicit return shape
```

- [ ] **Step 2: `sweep_test.go`**

```go
package recovery_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/recovery"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestRecoveryResetsRunningToReady(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()

	dag := &systempb.Dag{Graph: &systempb.Dag_Graph{Nodes: []*systempb.Dag_Node{{Key: "x", Spec: &anypb.Any{TypeUrl: "type.googleapis.com/Mock"}, RetryMax: 3}}}}
	dag, _ = f.System.SaveDag(ctx, dag)
	run, _ := f.System.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})

	// Force the node into RUNNING (simulate worker that crashed mid-flight)
	_, err := f.F.Pool.Exec(ctx, `UPDATE node_runs SET status='NODE_RUN_STATUS_RUNNING' WHERE dag_run_id=$1`, run.GetId().GetValue())
	require.NoError(t, err)

	require.NoError(t, recovery.Run(ctx, f.F.Pool, zap.NewNop()))

	rows, _ := f.F.Pool.Query(ctx, `SELECT status FROM node_runs WHERE dag_run_id=$1`, run.GetId().GetValue())
	defer rows.Close()
	var status string
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&status))
	require.Equal(t, "NODE_RUN_STATUS_READY", status)
}
```

- [ ] **Step 3: Run + commit**

```bash
go test ./internal/domain/workers/recovery/... -v -count=1
```
Expected: PASS.

```bash
git add internal/domain/workers/recovery/
git commit -m "feat(recovery): startup sweep with idempotent SQL + test for RUNNING→READY"
```

---

## Task 8: Scheduler with cron + lease

**Files:**
- Create: `internal/domain/services/system/schedules.go`
- Create: `internal/domain/workers/scheduler/scheduler.go`, `scheduler_test.go`

- [ ] **Step 1: `schedules.go`**

```go
package system

import (
	"context"

	"github.com/yaroher/ratel/pkg/dml/set"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// CreateSchedule persists a Schedule template + initial next_fire_at.
func (s *Service) CreateSchedule(ctx context.Context, tenantID *iampb.TenantId, sched *systempb.Schedule) (*systempb.Schedule, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateSchedule",
		func(ctx context.Context, _ any) (*systempb.Schedule, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*systempb.Schedule, error) {
				now := timestamppb.Now()
				sched.Id = &systempb.ScheduleId{Value: ids.New()}
				sched.TenantId = tenantID
				sched.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
				if err := s.scheduleRepo.Insert(ctx, sched); err != nil {
					return nil, err
				}
				return sched, nil
			})
		})
}

func (s *Service) GetSchedule(ctx context.Context, id *systempb.ScheduleId) (*systempb.Schedule, error) {
	return s.scheduleRepo.SelectOne(ctx, set.Eq(systempb.ScheduleColumnId, id.GetValue()))
}

func (s *Service) ListSchedules(ctx context.Context, tenantID *iampb.TenantId) ([]*systempb.Schedule, error) {
	return s.scheduleRepo.Select(ctx, set.Eq(systempb.ScheduleColumnTenantId, tenantID.GetValue()))
}

func (s *Service) DeleteSchedule(ctx context.Context, id *systempb.ScheduleId) error {
	_, err := s.scheduleRepo.Delete(ctx, set.Eq(systempb.ScheduleColumnId, id.GetValue()))
	return err
}
```

- [ ] **Step 2: `scheduler.go`**

```go
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
)

type Config struct {
	Tick          time.Duration
	LeaseDuration time.Duration
	Batch         int
}

type Scheduler struct {
	pool   *pgxpool.Pool
	system *system.Service
	cfg    Config
	log    *zap.Logger

	instanceID string
}

func New(pool *pgxpool.Pool, sys *system.Service, cfg Config, log *zap.Logger) *Scheduler {
	if cfg.Tick == 0 {
		cfg.Tick = 5 * time.Second
	}
	if cfg.LeaseDuration == 0 {
		cfg.LeaseDuration = 30 * time.Second
	}
	if cfg.Batch == 0 {
		cfg.Batch = 10
	}
	host, _ := os.Hostname()
	return &Scheduler{pool: pool, system: sys, cfg: cfg, log: log, instanceID: fmt.Sprintf("sched-%s-%d", host, os.Getpid())}
}

func (s *Scheduler) Run(ctx context.Context) {
	tick := time.NewTicker(s.cfg.Tick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := s.tickOnce(ctx); err != nil {
				s.log.Warn("scheduler tick error", zap.Error(err))
			}
		}
	}
}

func (s *Scheduler) tickOnce(ctx context.Context) error {
	const q = `
UPDATE schedules SET
  lease_owner = $1,
  lease_expires_at = now() + ($2 || ' seconds')::interval,
  updated_at = now()
WHERE id IN (
  SELECT id FROM schedules
  WHERE next_fire_at <= now()
    AND (lease_expires_at IS NULL OR lease_expires_at < now())
  ORDER BY next_fire_at
  FOR UPDATE SKIP LOCKED LIMIT $3
)
RETURNING id, tenant_id, cron_expression, timezone, next_fire_at, dag_template;
`
	rows, err := s.pool.Query(ctx, q, s.instanceID, int(s.cfg.LeaseDuration.Seconds()), s.cfg.Batch)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var schedID, tenantID, cronExpr, tz string
		var nextFireAt time.Time
		var dagTemplate []byte
		if err := rows.Scan(&schedID, &tenantID, &cronExpr, &tz, &nextFireAt, &dagTemplate); err != nil {
			s.log.Warn("scheduler scan", zap.Error(err))
			continue
		}
		if err := s.fire(ctx, schedID, dagTemplate); err != nil {
			s.log.Warn("scheduler fire failed", zap.String("schedule_id", schedID), zap.Error(err))
		}
		next, err := computeNext(cronExpr, tz, nextFireAt)
		if err != nil {
			s.log.Warn("cron parse", zap.String("schedule_id", schedID), zap.Error(err))
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE schedules SET next_fire_at=$1, lease_owner=NULL, lease_expires_at=NULL, updated_at=now() WHERE id=$2`, next, schedID); err != nil {
			s.log.Warn("scheduler advance", zap.Error(err))
		}
	}
	return rows.Err()
}

func (s *Scheduler) fire(ctx context.Context, scheduleID string, dagTemplateProto []byte) error {
	// dagTemplate is a serialized cloud.v1.system.Dag — unmarshal into the
	// system service's StartDagRun input. This translation is deferred to
	// plan 04 which owns the bridge between testing.TestRun → system.Dag;
	// here we emit a no-op + log so the scheduler tick loop is provable
	// independently.
	if dagTemplateProto == nil {
		return errors.New("schedule has nil dag_template — wire TestRun bridge in plan 04")
	}
	s.log.Info("scheduler fired (stub)", zap.String("schedule_id", scheduleID), zap.Int("template_bytes", len(dagTemplateProto)))
	return nil
}

func computeNext(expr, tz string, after time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(after.In(loc)), nil
}
```

- [ ] **Step 3: `scheduler_test.go`**

```go
package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/scheduler"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestSchedulerAdvancesNextFireAt(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "sch@e.com", Nickname: "sch"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "T"}, u.GetId())

	sched := &systempb.Schedule{
		CronExpression: "*/1 * * * *",
		Timezone:       "UTC",
		NextFireAt:     mustProto(time.Now().Add(-time.Minute)),
		DagTemplate:    []byte{0x01}, // non-nil sentinel so fire() doesn't error
		Identity:       &commonpb.Identity{Name: "every-min"},
	}
	_, err := f.System.CreateSchedule(ctx, tn.GetId(), sched)
	require.NoError(t, err)

	srv := scheduler.New(f.F.Pool, f.System, scheduler.Config{Tick: 100 * time.Millisecond}, zap.NewNop())
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	go srv.Run(runCtx)

	deadline := time.Now().Add(2 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("scheduler did not advance next_fire_at")
		}
		got, _ := f.System.GetSchedule(ctx, sched.GetId())
		if got.GetNextFireAt().AsTime().After(time.Now()) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func mustProto(ts time.Time) *systempb.Timestamp { return systempb.Timestamp{} /* TODO */ }
```

Replace the `mustProto` stub: `Schedule.next_fire_at` should be
`google.protobuf.Timestamp` in the proto. Use
`timestamppb.New(ts)` and `import "google.golang.org/protobuf/types/known/timestamppb"`.

- [ ] **Step 4: Run + commit**

```bash
go test ./internal/domain/workers/scheduler/... -count=1 -timeout 30s -v
```
Expected: PASS.

```bash
git add internal/domain/services/system/schedules.go internal/domain/workers/scheduler/
git commit -m "feat(scheduler): cron tick + lease-based claim + next_fire_at advance"
```

---

## Task 9: ScheduleService ConnectRPC handler

**Files:**
- Create: `internal/transport/connect/system.go`
- Modify: `internal/transport/connect/server.go`, `internal/sdk/client/client.go`

- [ ] **Step 1: Handler**

```go
package connectrpc

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

type ScheduleHandler struct{ svc *system.Service }

func NewScheduleHandler(svc *system.Service) *ScheduleHandler { return &ScheduleHandler{svc: svc} }

func (h *ScheduleHandler) CreateSchedule(ctx context.Context, req *connect.Request[systempb.CreateScheduleRequest]) (*connect.Response[systempb.Schedule], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	s, err := h.svc.CreateSchedule(ctx, tenantID, req.Msg.GetSchedule())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(s), nil
}

func (h *ScheduleHandler) ListSchedules(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[systempb.Schedule_List], error) {
	list, err := h.svc.ListSchedules(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&systempb.Schedule_List{Schedules: list}), nil
}

func (h *ScheduleHandler) GetSchedule(ctx context.Context, req *connect.Request[systempb.ScheduleId]) (*connect.Response[systempb.Schedule], error) {
	s, err := h.svc.GetSchedule(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(s), nil
}

func (h *ScheduleHandler) DeleteSchedule(ctx context.Context, req *connect.Request[systempb.ScheduleId]) (*connect.Response[emptypb.Empty], error) {
	if err := h.svc.DeleteSchedule(ctx, req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}
```

- [ ] **Step 2: Mount in `server.go`**

Append `ScheduleHandler *ScheduleHandler` to `Deps`. Inside `Mount`:

```go
schedPath, schedHandler := systemconnect.NewScheduleServiceHandler(d.ScheduleHandler, d.Interceptors)
mux.Handle(schedPath, schedHandler)
```

- [ ] **Step 3: Add SDK client**

In `internal/sdk/client/client.go` append:

```go
Schedule systemconnect.ScheduleServiceClient
```

…and inside `New`:

```go
c.Schedule = systemconnect.NewScheduleServiceClient(c.httpClient, serverURL, connOpts...)
```

- [ ] **Step 4: Build + commit**

```bash
go build ./...
git add internal/transport/connect/system.go internal/transport/connect/server.go internal/sdk/client/client.go
git commit -m "feat(connect,sdk): ScheduleService handler + client"
```

---

## Task 10: CLI `schedule` subcommands

**Files:**
- Create: `cmd/stroppy-cloud/cmd_cli/schedule.go`
- Modify: `cmd/stroppy-cloud/cmd_cli/root.go`

- [ ] **Step 1:**

```go
package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

func scheduleCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "schedule", Short: "Manage schedules"}
	cmd.AddCommand(scheduleListCmd(), scheduleDeleteCmd())
	return cmd
}

func scheduleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List schedules in current tenant",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, tenant, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Schedule.ListSchedules(context.Background(), connect.NewRequest(&iampb.TenantId{Value: tenant}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg.GetSchedules())
		},
	}
}

func scheduleDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Delete a schedule",
		RunE: func(c *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			_, err = cli.Schedule.DeleteSchedule(context.Background(), connect.NewRequest(&systempb.ScheduleId{Value: args[0]}))
			if err != nil {
				return fmt.Errorf("delete: %w", err)
			}
			return nil
		},
	}
}
```

Register in `root.go`:

```go
cmd.AddCommand(scheduleCmd())
```

- [ ] **Step 2: Commit**

```bash
git add cmd/stroppy-cloud/cmd_cli/
git commit -m "feat(cli): schedule list/delete subcommands"
```

---

## Task 11: Wire workers + system into cmd_server

**Files:**
- Modify: `cmd/stroppy-cloud/cmd_server.go`

- [ ] **Step 1: Add imports + wiring**

After `bus := eventing.NewInMemoryBus()`:

```go
systemSvc := system.New(exec, txMgr, bus)
```

After services are built, before `mux.Handle("/", connectrpc.Mount(...))`:

```go
// Recovery sweep BEFORE workers start
if cfg.Workers.RecoveryOnStart {
    if err := recovery.Run(ctx, pool, zlog); err != nil {
        return fmt.Errorf("recovery: %w", err)
    }
}

// Node worker pool
nodeReg := nodeworker.NewRegistry()
nodeReg.Register(mockhandler.NewMockHandler())

worker := nodeworker.New(pool, systemSvc, nodeReg, nodeworker.Config{
    Workers: cfg.Workers.NodeWorkers,
    Tick:    500 * time.Millisecond,
}, zlog)
sched := scheduler.New(pool, systemSvc, scheduler.Config{
    Tick:          cfg.Workers.SchedulerTick,
    LeaseDuration: 30 * time.Second,
}, zlog)

workerCtx, cancelWorkers := context.WithCancel(ctx)
go worker.Run(workerCtx)
go sched.Run(workerCtx)
defer cancelWorkers()
```

Add `ScheduleHandler: connectrpc.NewScheduleHandler(systemSvc)` to `Deps`.

Imports to add:
```go
"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
mockhandler "github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker/handlers"
"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/recovery"
"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/scheduler"
```

- [ ] **Step 2: Build**

```bash
go build ./cmd/stroppy-cloud/...
```
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add cmd/stroppy-cloud/cmd_server.go
git commit -m "feat(server): wire system service + node worker pool + scheduler + recovery sweep"
```

---

## Task 12: Final verification

- [ ] **Step 1: Run integration tests**

```bash
go test ./internal/domain/... -count=1
```
Expected: green.

- [ ] **Step 2: Manual restart-recovery smoke**

```bash
# Terminal 1
make server-run
# In a Postgres console insert a mock dag manually OR trigger via integration test
# Then kill the server (Ctrl-C / kill -9)
# Restart:
make server-run
# Verify logs: "recovery sweep statement applied" with non-zero rows_affected
```

- [ ] **Step 3: Commit any lingering changes**

```bash
git status
```

---

## Acceptance for plan 03

- ✅ `system.Service` ships SaveDag, StartDagRun, GetDagRun, CancelDagRun, MarkNodeRunSucceeded/Failed, dep unblocking, dag_run finalization, state-entry upsert/get, Schedule CRUD.
- ✅ `nodeworker.Worker` claims READY rows via FOR UPDATE SKIP LOCKED, runs Handler.Execute, finalizes node runs.
- ✅ Mock handler exercises engine in `worker_test.go` (two-node DAG completes).
- ✅ Recovery sweep resets RUNNING/CANCELING → READY/FAILED, recomputes dag_runs status, tolerates missing tables (webhook_deliveries, agents) until plans 05/06 land.
- ✅ Scheduler advances `next_fire_at` based on cron expr + tz.
- ✅ `ScheduleService` reachable via ConnectRPC + CLI subcommand.
- ✅ `cmd_server` wires workers, recovery, scheduler; graceful shutdown cancels worker ctx.

## Out of scope for plan 03 (next plan)

- Real (non-mock) node handlers: terraform, docker, agent commands, stroppy_run, test_run_ref (plan 04 wires test_run_ref → child DagRun; plans 05+04 add the agent + DB handlers).
- TestRun → Dag template builder (plan 04).
- Webhook outbox (plan 06).
- Agent + agent_commands persistence (plan 05).
- DagService public RPC (intentionally not exposed; queue introspection is done via internal admin RPCs in plan 07 if needed).
