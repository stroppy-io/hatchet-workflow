# Testing Services Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the user-facing testing domain — TestRunTemplate, TestRun, TestSuite, TestSuiteRun, SharedTestRun, SharedSuiteRun, Comparison — plus the `DagRunBuilder` that translates a TestRun into a `system.Dag` template the engine from Plan 03 can execute.

**Architecture:** Six ConnectRPC services in `internal/domain/services/testing/`, each backed by a ratel `ProtoRepository` and an `IamPort` for permission checks. Launch flows (`LaunchTestRun`, `LaunchTestSuite`, `InstantiateTestRun`) call into `DagRunBuilder` which materializes nodes for `prepare → seed → workload → teardown` and `INSERT`s a `DagRun` + `node_runs`. The `system.NodeWorker` from Plan 03 picks them up. `WatchTestRun` / `StreamTestRunLogs` subscribe to the in-memory bus from Plan 01 with DB backfill via `since` cursor. Comparison reads metrics from VictoriaMetrics through `infrastructure/victoria`.

**Tech Stack:** ConnectRPC, ratel-generated repositories, avito-tech tx manager (`pgtx`), `internal/core/eventing`, `internal/infrastructure/victoria`, testcontainers Postgres.

**Prerequisites:** Plans 01 (foundation+IAM), 02 (catalog+stroppy), 03 (system+workers) merged. The system service exposes a Go-level `Engine.LaunchDag(ctx, template *systempb.Dag, metadata) (*systempb.DagRun, error)` port.

---

## File manifest

**Create:**
- `internal/domain/services/testing/ports.go` — `IamPort`, `CatalogPort`, `SystemEnginePort`, `MetricsPort` interfaces
- `internal/domain/services/testing/template_service.go` — `TestRunTemplateService`
- `internal/domain/services/testing/template_service_test.go`
- `internal/domain/services/testing/test_run_service.go` — `TestRunService` (CRUD + Launch/Cancel/Instantiate + Get/Watch/Logs/Metrics)
- `internal/domain/services/testing/test_run_service_test.go`
- `internal/domain/services/testing/test_suite_service.go` — `TestSuiteService`
- `internal/domain/services/testing/test_suite_service_test.go`
- `internal/domain/services/testing/test_suite_run_service.go` — `TestSuiteRunService` (Launch/Cancel/Get/Watch/Logs/Metrics/List*)
- `internal/domain/services/testing/test_suite_run_service_test.go`
- `internal/domain/services/testing/shared_test_run_service.go`
- `internal/domain/services/testing/shared_test_run_service_test.go`
- `internal/domain/services/testing/shared_suite_run_service.go`
- `internal/domain/services/testing/shared_suite_run_service_test.go`
- `internal/domain/services/testing/comparison_service.go`
- `internal/domain/services/testing/comparison_service_test.go`
- `internal/domain/services/testing/dagbuilder/builder.go` — `Builder.FromTestRun(tr) (*systempb.Dag, error)`
- `internal/domain/services/testing/dagbuilder/builder_test.go`
- `internal/domain/services/testing/dagbuilder/suite_builder.go` — `Builder.FromTestSuite(s) (*systempb.Dag, error)` meta-DAG of child `test_run_ref` nodes
- `internal/domain/services/testing/dagbuilder/suite_builder_test.go`
- `internal/transport/connect/testing_handlers.go` — six Connect handler structs (one per service)
- `internal/transport/connect/testing_handlers_test.go`
- `internal/sdk/client/testing.go` — typed wrapper exposing all six clients
- `cmd/stroppy-cloud/cmd_cli/run.go` — `stroppy-cloud run {start, list, get, cancel, logs, metrics, share}`
- `cmd/stroppy-cloud/cmd_cli/suite.go` — `stroppy-cloud suite {list, create, launch, cancel, get}`
- `cmd/stroppy-cloud/cmd_cli/template.go` — `stroppy-cloud template {list, create, delete, instantiate}`
- `cmd/stroppy-cloud/cmd_cli/compare.go` — `stroppy-cloud compare runs <a> <b>`

**Modify:**
- `cmd/stroppy-cloud/cmd_server.go` — register six testing handlers, wire `dagbuilder` and `MetricsPort`
- `cmd/stroppy-cloud/cmd_cli/root.go` — register new subcommands
- `internal/domain/services/system/scheduler/scheduler.go:fire()` — replace stub with call into `TestRunService.InstantiateAndLaunch(ctx, scheduleID, templateID)` (closes the loop deferred from Plan 03)

---

## Task 1: Ports

**Files:**
- Create: `internal/domain/services/testing/ports.go`

- [ ] **Step 1: Write ports.go**

```go
package testing

import (
	"context"

	agentpb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	catalogpb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	testingpb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

type IamPort interface {
	HasTenantRole(ctx context.Context, u *iampb.UserId, t *iampb.TenantId, min iampb.TenantRole) (bool, error)
	UserFromCtx(ctx context.Context) (*iampb.UserId, error)
	TenantFromCtx(ctx context.Context) (*iampb.TenantId, error)
}

type CatalogPort interface {
	GetDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error)
	GetWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error)
	GetPackage(ctx context.Context, id *catalogpb.PackageId) (*catalogpb.Package, error)
}

type SystemEnginePort interface {
	LaunchDag(ctx context.Context, template *systempb.Dag, link systempb.DagRunLink) (*systempb.DagRun, error)
	CancelDagRun(ctx context.Context, id *systempb.DagRunId) error
	GetDagRun(ctx context.Context, id *systempb.DagRunId) (*systempb.DagRun, error)
	SubscribeProgress(ctx context.Context, dagRunID *systempb.DagRunId, since string) (<-chan *systempb.NodeRunUpdate, func(), error)
	StreamLogs(ctx context.Context, dagRunID *systempb.DagRunId, stepID string, since string) (<-chan *agentpb.LogLine, func(), error)
}

type MetricsPort interface {
	Query(ctx context.Context, tenant *iampb.TenantId, testRunID *testingpb.TestRunId, query string) (*testingpb.MetricSeriesList, error)
}
```

- [ ] **Step 2: Build to verify symbols resolve**

Run: `go build ./internal/domain/services/testing/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/services/testing/ports.go
git commit -m "feat(testing): add cross-domain ports"
```

---

## Task 2: TestRunTemplateService

**Files:**
- Create: `internal/domain/services/testing/template_service.go`
- Create: `internal/domain/services/testing/template_service_test.go`

- [ ] **Step 1: Write failing test**

```go
package testing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	testingsvc "github.com/arenadata/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	testingpb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestTemplateService_CreateGet(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	svc := testingsvc.NewTemplateService(f.Executor(), f.TxManager(), f.Events(), f.IAM())
	tenant := f.Tenant()
	user := f.User(tenant)
	ctx := f.AuthCtx(user, tenant)

	tpl, err := svc.CreateTestRunTemplate(ctx, &testingpb.CreateTestRunTemplateRequest{
		TenantId: tenant.Id,
		Name:     "tpc-c basic",
		Spec:     fixture.MinimalTestRunSpec(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, tpl.Id.GetValue())

	got, err := svc.GetTestRunTemplate(ctx, tpl.Id)
	require.NoError(t, err)
	require.Equal(t, tpl.Id.GetValue(), got.Id.GetValue())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/services/testing/ -run TestTemplateService -v`
Expected: FAIL with "NewTemplateService undefined".

- [ ] **Step 3: Implement template_service.go**

```go
package testing

import (
	"context"

	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/tracing"
	errorspb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	iampb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"github.com/naukograd-software/ratel/pkg/exec"
	"github.com/naukograd-software/ratel/pkg/repository"
)

type TemplateService struct {
	tracing.Entity
	repo      *repository.ProtoRepository[testingpb.TestRunTemplateAlias, testingpb.TestRunTemplateColumnAlias, *testingpb.TestRunTemplateScanner, *testingpb.TestRunTemplate]
	txManager pgtx.TxManager
	events    eventing.Publisher
	iam       IamPort
}

func NewTemplateService(executor exec.DB, txManager pgtx.TxManager, events eventing.Publisher, iam IamPort) *TemplateService {
	return &TemplateService{
		Entity: tracing.NewEntity("testing.TemplateService"),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestRunTemplates.Table, executor),
			testingpb.TestRunTemplateConverter,
		),
		txManager: txManager,
		events:    events,
		iam:       iam,
	}
}

func (s *TemplateService) CreateTestRunTemplate(ctx context.Context, req *testingpb.CreateTestRunTemplateRequest) (*testingpb.TestRunTemplate, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTestRunTemplate",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestRunTemplate, error) {
			user, err := s.iam.UserFromCtx(ctx)
			if err != nil {
				return nil, err
			}
			ok, err := s.iam.HasTenantRole(ctx, user, req.TenantId, iampb.TenantRole_TENANT_ROLE_EDITOR)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, domainerr.E(errorspb.Code_PERMISSION_DENIED)
			}
			return pgtx.WithSerializableRet(ctx, s.txManager, func(ctx context.Context) (*testingpb.TestRunTemplate, error) {
				tpl := &testingpb.TestRunTemplate{
					Id:       &testingpb.TestRunTemplateId{Value: ids.NewULID()},
					TenantId: req.TenantId,
					Name:     req.Name,
					Spec:     req.Spec,
					CreatedBy: user,
				}
				if err := s.repo.Insert(ctx, tpl); err != nil {
					return nil, domainerr.FromPg(err)
				}
				return tpl, nil
			})
		})
}

func (s *TemplateService) GetTestRunTemplate(ctx context.Context, id *testingpb.TestRunTemplateId) (*testingpb.TestRunTemplate, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "GetTestRunTemplate",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestRunTemplate, error) {
			tpl, err := s.repo.FindByID(ctx, id.GetValue())
			if err != nil {
				return nil, domainerr.FromPg(err)
			}
			if _, err := s.checkRead(ctx, tpl.TenantId); err != nil {
				return nil, err
			}
			return tpl, nil
		})
}

func (s *TemplateService) UpdateTestRunTemplate(ctx context.Context, req *testingpb.UpdateTestRunTemplateRequest) (*testingpb.TestRunTemplate, error) { /* same pattern with editor role */ }
func (s *TemplateService) DeleteTestRunTemplate(ctx context.Context, id *testingpb.TestRunTemplateId) (*testingpb.TestRunTemplate, error)            { /* editor role + soft delete */ }
func (s *TemplateService) ListTestRunTemplates(ctx context.Context, t *iampb.TenantId) (*testingpb.TestRunTemplate_List, error)                     { /* viewer role */ }

func (s *TemplateService) checkRead(ctx context.Context, tenant *iampb.TenantId) (*iampb.UserId, error) {
	user, err := s.iam.UserFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	ok, err := s.iam.HasTenantRole(ctx, user, tenant, iampb.TenantRole_TENANT_ROLE_VIEWER)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domainerr.E(errorspb.Code_PERMISSION_DENIED)
	}
	return user, nil
}
```

- [ ] **Step 4: Run test to verify pass**

Run: `go test ./internal/domain/services/testing/ -run TestTemplateService -v`
Expected: PASS.

- [ ] **Step 5: Fill in Update/Delete/List with same role check, write tests for each**

Add tests `TestTemplateService_Update`, `TestTemplateService_Delete`, `TestTemplateService_List` paralleling create test, then implement. Each: viewer cannot mutate; editor can; cross-tenant returns `PERMISSION_DENIED`.

Run: `go test ./internal/domain/services/testing/ -run TestTemplateService -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/services/testing/template_service.go internal/domain/services/testing/template_service_test.go
git commit -m "feat(testing): TestRunTemplateService CRUD"
```

---

## Task 3: DagBuilder — TestRun → Dag

**Files:**
- Create: `internal/domain/services/testing/dagbuilder/builder.go`
- Create: `internal/domain/services/testing/dagbuilder/builder_test.go`

- [ ] **Step 1: Write failing test**

```go
package dagbuilder_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestBuilder_FromTestRun_BasicShape(t *testing.T) {
	t.Parallel()
	tr := fixture.SampleTestRun(t) // database_preset=cockroach-3node, workload_preset=tpcc, package=cockroach-23
	b := dagbuilder.New(fixture.NewCatalogPortStub(t))
	dag, err := b.FromTestRun(context.Background(), tr)
	require.NoError(t, err)
	require.Len(t, dag.Nodes, 5) // provision, install, init, workload, teardown

	byID := map[string]*systempb.DagNode{}
	for _, n := range dag.Nodes {
		byID[n.Id] = n
	}
	require.Contains(t, byID, "provision")
	require.Equal(t, []string{}, byID["provision"].DependsOn)
	require.Equal(t, []string{"provision"}, byID["install"].DependsOn)
	require.Equal(t, []string{"install"}, byID["init"].DependsOn)
	require.Equal(t, []string{"init"}, byID["workload"].DependsOn)
	require.Equal(t, []string{"workload"}, byID["teardown"].DependsOn)
}

func TestBuilder_FromTestRun_NoTeardownWhenKeepCluster(t *testing.T) {
	tr := fixture.SampleTestRun(t)
	tr.Spec.KeepClusterAfter = true
	b := dagbuilder.New(fixture.NewCatalogPortStub(t))
	dag, err := b.FromTestRun(context.Background(), tr)
	require.NoError(t, err)
	require.Len(t, dag.Nodes, 4)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/services/testing/dagbuilder/ -v`
Expected: FAIL with "dagbuilder.New undefined".

- [ ] **Step 3: Implement builder.go**

```go
package dagbuilder

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	tasksp "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	systempb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	testingpb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	testingsvc "github.com/arenadata/stroppy-io/stroppy-cloud/internal/domain/services/testing"
)

type Builder struct {
	catalog testingsvc.CatalogPort
}

func New(catalog testingsvc.CatalogPort) *Builder { return &Builder{catalog: catalog} }

func (b *Builder) FromTestRun(ctx context.Context, tr *testingpb.TestRun) (*systempb.Dag, error) {
	dbPreset, err := b.catalog.GetDatabasePreset(ctx, tr.Spec.DatabasePresetId)
	if err != nil { return nil, err }
	wlPreset, err := b.catalog.GetWorkloadPreset(ctx, tr.Spec.WorkloadPresetId)
	if err != nil { return nil, err }
	pkg, err := b.catalog.GetPackage(ctx, tr.Spec.PackageId)
	if err != nil { return nil, err }

	nodes := []*systempb.DagNode{}
	prov, err := nodeAny("provision", nil, &tasksp.TerraformTask{
		Operation: tasksp.TerraformOp_TERRAFORM_OP_APPLY,
		ModuleSource: dbPreset.TerraformModule,
		Vars: dbPreset.TerraformVars,
	})
	if err != nil { return nil, err }
	nodes = append(nodes, prov)

	inst, err := nodeAny("install", []string{"provision"}, &tasksp.InstallPackageTask{
		PackageId: pkg.Id, TargetHosts: tasksp.TargetHosts_TARGET_HOSTS_ALL,
	})
	if err != nil { return nil, err }
	nodes = append(nodes, inst)

	init, err := nodeAny("init", []string{"install"}, &tasksp.OneShotTask{
		Script: dbPreset.InitScript, TargetHosts: tasksp.TargetHosts_TARGET_HOSTS_PRIMARY,
	})
	if err != nil { return nil, err }
	nodes = append(nodes, init)

	wl, err := nodeAny("workload", []string{"init"}, &tasksp.StroppyRunTask{
		WorkloadPresetId: wlPreset.Id,
		DatabasePresetId: dbPreset.Id,
		StroppyVersion: tr.Spec.StroppyVersion,
	})
	if err != nil { return nil, err }
	nodes = append(nodes, wl)

	if !tr.Spec.KeepClusterAfter {
		td, err := nodeAny("teardown", []string{"workload"}, &tasksp.TerraformTask{
			Operation: tasksp.TerraformOp_TERRAFORM_OP_DESTROY,
			ModuleSource: dbPreset.TerraformModule,
			Vars: dbPreset.TerraformVars,
		})
		if err != nil { return nil, err }
		nodes = append(nodes, td)
	}

	return &systempb.Dag{Nodes: nodes}, nil
}

func nodeAny(id string, deps []string, spec interface{ proto.Message }) (*systempb.DagNode, error) {
	any, err := anypb.New(spec)
	if err != nil { return nil, err }
	return &systempb.DagNode{Id: id, DependsOn: deps, Spec: any}, nil
}
```

- [ ] **Step 4: Run test to verify pass**

Run: `go test ./internal/domain/services/testing/dagbuilder/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/testing/dagbuilder/builder.go internal/domain/services/testing/dagbuilder/builder_test.go
git commit -m "feat(testing): DagBuilder.FromTestRun"
```

---

## Task 4: TestRunService — CRUD + Launch/Cancel

**Files:**
- Create: `internal/domain/services/testing/test_run_service.go`
- Create: `internal/domain/services/testing/test_run_service_test.go`

- [ ] **Step 1: Write failing test for CRUD + Launch**

```go
func TestTestRunService_CreateLaunchCancel(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	engine := f.MockSystemEngine() // records LaunchDag/CancelDagRun calls
	svc := testingsvc.NewTestRunService(f.Executor(), f.TxManager(), f.Events(), f.IAM(), f.Catalog(), engine,
		dagbuilder.New(f.Catalog()))

	tenant := f.Tenant()
	user := f.User(tenant)
	ctx := f.AuthCtx(user, tenant)

	tr, err := svc.CreateTestRun(ctx, &testingpb.CreateTestRunRequest{
		TenantId: tenant.Id,
		Spec: fixture.MinimalTestRunSpec(),
	})
	require.NoError(t, err)
	require.Equal(t, testingpb.TestRunStatus_TEST_RUN_STATUS_PENDING, tr.Status)

	launched, err := svc.LaunchTestRun(ctx, tr.Id)
	require.NoError(t, err)
	require.NotNil(t, launched.DagRunId)
	require.Equal(t, testingpb.TestRunStatus_TEST_RUN_STATUS_RUNNING, launched.Status)
	require.Len(t, engine.LaunchCalls, 1)

	cancelled, err := svc.CancelTestRun(ctx, tr.Id)
	require.NoError(t, err)
	require.Equal(t, testingpb.TestRunStatus_TEST_RUN_STATUS_CANCELING, cancelled.Status)
	require.Len(t, engine.CancelCalls, 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/services/testing/ -run TestTestRunService_CreateLaunchCancel -v`
Expected: FAIL "NewTestRunService undefined".

- [ ] **Step 3: Implement test_run_service.go**

```go
package testing

import (
	"context"

	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	"github.com/arenadata/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	errorspb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	iampb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	testingpb "github.com/arenadata/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/naukograd-software/ratel/pkg/exec"
	"github.com/naukograd-software/ratel/pkg/repository"
)

type TestRunService struct {
	tracing.Entity
	runs      *repository.ProtoRepository[testingpb.TestRunAlias, testingpb.TestRunColumnAlias, *testingpb.TestRunScanner, *testingpb.TestRun]
	templates *repository.ProtoRepository[testingpb.TestRunTemplateAlias, testingpb.TestRunTemplateColumnAlias, *testingpb.TestRunTemplateScanner, *testingpb.TestRunTemplate]
	txManager pgtx.TxManager
	events    eventing.Publisher
	iam       IamPort
	catalog   CatalogPort
	engine    SystemEnginePort
	metrics   MetricsPort
	builder   *dagbuilder.Builder
}

func NewTestRunService(
	executor exec.DB, txm pgtx.TxManager, events eventing.Publisher,
	iam IamPort, catalog CatalogPort, engine SystemEnginePort, metrics MetricsPort,
	builder *dagbuilder.Builder,
) *TestRunService {
	return &TestRunService{
		Entity: tracing.NewEntity("testing.TestRunService"),
		runs: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestRuns.Table, executor),
			testingpb.TestRunConverter,
		),
		templates: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestRunTemplates.Table, executor),
			testingpb.TestRunTemplateConverter,
		),
		txManager: txm, events: events, iam: iam, catalog: catalog,
		engine: engine, metrics: metrics, builder: builder,
	}
}

func (s *TestRunService) CreateTestRun(ctx context.Context, req *testingpb.CreateTestRunRequest) (*testingpb.TestRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTestRun", func(ctx context.Context, _ trace.Span) (*testingpb.TestRun, error) {
		user, err := s.requireRole(ctx, req.TenantId, iampb.TenantRole_TENANT_ROLE_EDITOR)
		if err != nil { return nil, err }
		return pgtx.WithSerializableRet(ctx, s.txManager, func(ctx context.Context) (*testingpb.TestRun, error) {
			tr := &testingpb.TestRun{
				Id: &testingpb.TestRunId{Value: ids.NewULID()},
				TenantId: req.TenantId,
				Spec:     req.Spec,
				Status:   testingpb.TestRunStatus_TEST_RUN_STATUS_PENDING,
				CreatedBy: user,
			}
			if err := s.runs.Insert(ctx, tr); err != nil {
				return nil, domainerr.FromPg(err)
			}
			s.events.Publish(ctx, eventing.TestRunCreated{TenantID: req.TenantId, TestRunID: tr.Id})
			return tr, nil
		})
	})
}

func (s *TestRunService) LaunchTestRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.TestRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "LaunchTestRun", func(ctx context.Context, _ trace.Span) (*testingpb.TestRun, error) {
		return pgtx.WithSerializableRet(ctx, s.txManager, func(ctx context.Context) (*testingpb.TestRun, error) {
			tr, err := s.runs.FindByID(ctx, id.GetValue())
			if err != nil { return nil, domainerr.FromPg(err) }
			if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_EDITOR); err != nil {
				return nil, err
			}
			if tr.Status != testingpb.TestRunStatus_TEST_RUN_STATUS_PENDING {
				return nil, domainerr.E(errorspb.Code_FAILED_PRECONDITION,
					&errorspb.Detail{Kind: &errorspb.Detail_PreconditionFailure{
						PreconditionFailure: &errorspb.PreconditionFailure{Type: "test_run.status", Subject: tr.Status.String()},
					}})
			}
			dag, err := s.builder.FromTestRun(ctx, tr)
			if err != nil { return nil, err }
			dr, err := s.engine.LaunchDag(ctx, dag, systempb.DagRunLink{
				Kind: systempb.DagRunLink_LINK_KIND_TEST_RUN,
				Id:   tr.Id.GetValue(),
			})
			if err != nil { return nil, err }
			tr.DagRunId = dr.Id
			tr.Status = testingpb.TestRunStatus_TEST_RUN_STATUS_RUNNING
			tr.StartedAt = timestamppb.Now()
			if err := s.runs.Update(ctx, tr); err != nil { return nil, domainerr.FromPg(err) }
			s.events.Publish(ctx, eventing.TestRunLaunched{TenantID: tr.TenantId, TestRunID: tr.Id, DagRunID: dr.Id})
			return tr, nil
		})
	})
}

func (s *TestRunService) CancelTestRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.TestRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CancelTestRun", func(ctx context.Context, _ trace.Span) (*testingpb.TestRun, error) {
		return pgtx.WithSerializableRet(ctx, s.txManager, func(ctx context.Context) (*testingpb.TestRun, error) {
			tr, err := s.runs.FindByID(ctx, id.GetValue())
			if err != nil { return nil, domainerr.FromPg(err) }
			if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_EDITOR); err != nil { return nil, err }
			if tr.Status != testingpb.TestRunStatus_TEST_RUN_STATUS_RUNNING {
				return nil, domainerr.E(errorspb.Code_FAILED_PRECONDITION)
			}
			if err := s.engine.CancelDagRun(ctx, tr.DagRunId); err != nil { return nil, err }
			tr.Status = testingpb.TestRunStatus_TEST_RUN_STATUS_CANCELING
			if err := s.runs.Update(ctx, tr); err != nil { return nil, domainerr.FromPg(err) }
			return tr, nil
		})
	})
}

func (s *TestRunService) requireRole(ctx context.Context, t *iampb.TenantId, r iampb.TenantRole) (*iampb.UserId, error) {
	u, err := s.iam.UserFromCtx(ctx)
	if err != nil { return nil, err }
	ok, err := s.iam.HasTenantRole(ctx, u, t, r)
	if err != nil { return nil, err }
	if !ok { return nil, domainerr.E(errorspb.Code_PERMISSION_DENIED) }
	return u, nil
}
```

- [ ] **Step 4: Run test pass**

Run: `go test ./internal/domain/services/testing/ -run TestTestRunService_CreateLaunchCancel -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/testing/test_run_service.go internal/domain/services/testing/test_run_service_test.go
git commit -m "feat(testing): TestRunService create/launch/cancel"
```

---

## Task 5: TestRunService — Get/List/Update/Delete/Instantiate

**Files:**
- Modify: `internal/domain/services/testing/test_run_service.go`
- Modify: `internal/domain/services/testing/test_run_service_test.go`

- [ ] **Step 1: Write failing tests for each verb**

Tests `TestTestRunService_GetIncludesSteps`, `TestTestRunService_List_ScopedToTenant`, `TestTestRunService_Update_OnlyMetadata`, `TestTestRunService_Delete_OnlyTerminal`, `TestTestRunService_InstantiateFromTemplate`. Each follows the same fixture pattern.

`TestTestRunService_GetIncludesSteps`:
```go
got, err := svc.GetTestRun(ctx, tr.Id)
require.NoError(t, err)
require.Equal(t, tr.Id.Value, got.TestRun.Id.Value)
require.NotNil(t, got.Steps) // hydrated from system.NodeRunUpdate query
```

`TestTestRunService_Delete_OnlyTerminal`:
```go
_, err := svc.DeleteTestRun(ctx, runningTR.Id)
require.ErrorContains(t, err, "FAILED_PRECONDITION")
```

`TestTestRunService_InstantiateFromTemplate`:
```go
tpl := f.TestRunTemplate(tenant)
inst, err := svc.InstantiateTestRun(ctx, tpl.Id)
require.NoError(t, err)
require.Equal(t, tpl.Spec, inst.Spec)
require.Equal(t, testingpb.TestRunStatus_TEST_RUN_STATUS_PENDING, inst.Status)
```

- [ ] **Step 2: Run; expect FAIL on each**

Run: `go test ./internal/domain/services/testing/ -run TestTestRunService -v`
Expected: FAIL.

- [ ] **Step 3: Implement remaining methods**

```go
func (s *TestRunService) GetTestRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.GetTestRunResponse, error) {
	tr, err := s.runs.FindByID(ctx, id.GetValue())
	if err != nil { return nil, domainerr.FromPg(err) }
	if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_VIEWER); err != nil { return nil, err }
	resp := &testingpb.GetTestRunResponse{TestRun: tr}
	if tr.DagRunId != nil {
		dr, err := s.engine.GetDagRun(ctx, tr.DagRunId)
		if err != nil { return nil, err }
		resp.Steps = stepsFromNodes(dr)
		resp.Progress = progressFromDagRun(dr)
	}
	return resp, nil
}

func (s *TestRunService) ListTestRuns(ctx context.Context, t *iampb.TenantId) (*testingpb.TestRun_List, error) {
	if _, err := s.requireRole(ctx, t, iampb.TenantRole_TENANT_ROLE_VIEWER); err != nil { return nil, err }
	rows, err := s.runs.FindAll(ctx, testingpb.TestRunColumnAlias.TenantId.EqQ(t.GetValue()))
	if err != nil { return nil, domainerr.FromPg(err) }
	return &testingpb.TestRun_List{Items: rows}, nil
}

func (s *TestRunService) UpdateTestRun(ctx context.Context, req *testingpb.UpdateTestRunRequest) (*testingpb.TestRun, error) {
	tr, err := s.runs.FindByID(ctx, req.Id.GetValue())
	if err != nil { return nil, domainerr.FromPg(err) }
	if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_EDITOR); err != nil { return nil, err }
	// only name + description mutable, never spec
	tr.Name = req.Name
	tr.Description = req.Description
	if err := s.runs.Update(ctx, tr); err != nil { return nil, domainerr.FromPg(err) }
	return tr, nil
}

func (s *TestRunService) DeleteTestRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.TestRun, error) {
	tr, err := s.runs.FindByID(ctx, id.GetValue())
	if err != nil { return nil, domainerr.FromPg(err) }
	if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_EDITOR); err != nil { return nil, err }
	if !isTerminal(tr.Status) {
		return nil, domainerr.E(errorspb.Code_FAILED_PRECONDITION)
	}
	if err := s.runs.SoftDelete(ctx, id.GetValue()); err != nil { return nil, domainerr.FromPg(err) }
	return tr, nil
}

func (s *TestRunService) InstantiateTestRun(ctx context.Context, tplID *testingpb.TestRunTemplateId) (*testingpb.TestRun, error) {
	tpl, err := s.templates.FindByID(ctx, tplID.GetValue())
	if err != nil { return nil, domainerr.FromPg(err) }
	return s.CreateTestRun(ctx, &testingpb.CreateTestRunRequest{
		TenantId: tpl.TenantId,
		Spec:     tpl.Spec,
		Name:     tpl.Name + " (instance)",
	})
}

func isTerminal(s testingpb.TestRunStatus) bool {
	return s == testingpb.TestRunStatus_TEST_RUN_STATUS_SUCCEEDED ||
		s == testingpb.TestRunStatus_TEST_RUN_STATUS_FAILED ||
		s == testingpb.TestRunStatus_TEST_RUN_STATUS_CANCELED ||
		s == testingpb.TestRunStatus_TEST_RUN_STATUS_PENDING
}
```

- [ ] **Step 4: Run; PASS**

Run: `go test ./internal/domain/services/testing/ -run TestTestRunService -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/testing/test_run_service.go internal/domain/services/testing/test_run_service_test.go
git commit -m "feat(testing): TestRunService get/list/update/delete/instantiate"
```

---

## Task 6: TestRunService — Watch/StreamLogs/Metrics

**Files:**
- Modify: `internal/domain/services/testing/test_run_service.go`
- Modify: `internal/domain/services/testing/test_run_service_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestTestRunService_WatchTestRun_ReceivesProgress(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	engine := f.MockSystemEngine()
	svc := f.TestRunService(engine)
	tr := f.RunningTestRun()
	ctx, cancel := context.WithCancel(f.AuthCtx(tr.OwnerUser, tr.Tenant))
	defer cancel()

	ch, err := svc.WatchTestRun(ctx, &testingpb.WatchTestRunRequest{Id: tr.Id})
	require.NoError(t, err)
	engine.EmitProgress(tr.DagRunId, "install", systempb.NodeRunStatus_NODE_RUN_STATUS_DONE)
	select {
	case p := <-ch:
		require.Equal(t, "install", p.StepId)
		require.Equal(t, testingpb.StepStatus_STEP_STATUS_DONE, p.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("no progress event received")
	}
}

func TestTestRunService_StreamTestRunLogs_BackfillsThenLive(t *testing.T) { /* similar */ }
func TestTestRunService_GetTestRunMetrics_DelegatesToMetricsPort(t *testing.T) { /* similar */ }
```

- [ ] **Step 2: FAIL** — `go test ... -run TestTestRunService_Watch`.

- [ ] **Step 3: Implement**

```go
func (s *TestRunService) WatchTestRun(ctx context.Context, req *testingpb.WatchTestRunRequest) (<-chan *testingpb.TestRunProgress, error) {
	tr, err := s.runs.FindByID(ctx, req.Id.GetValue())
	if err != nil { return nil, domainerr.FromPg(err) }
	if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_VIEWER); err != nil { return nil, err }
	upstream, release, err := s.engine.SubscribeProgress(ctx, tr.DagRunId, req.Since)
	if err != nil { return nil, err }
	out := make(chan *testingpb.TestRunProgress, 16)
	go func() {
		defer close(out); defer release()
		for u := range upstream {
			select { case out <- mapNodeUpdateToStep(tr.Id, u): case <-ctx.Done(): return }
		}
	}()
	return out, nil
}

func (s *TestRunService) StreamTestRunLogs(ctx context.Context, req *testingpb.StreamTestRunLogsRequest) (<-chan *agentpb.LogLine, error) {
	tr, err := s.runs.FindByID(ctx, req.Id.GetValue())
	if err != nil { return nil, domainerr.FromPg(err) }
	if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_VIEWER); err != nil { return nil, err }
	return s.engine.StreamLogs(ctx, tr.DagRunId, req.StepId, req.Since)
}

func (s *TestRunService) GetTestRunMetrics(ctx context.Context, req *testingpb.GetTestRunMetricsRequest) (*testingpb.MetricSeriesList, error) {
	tr, err := s.runs.FindByID(ctx, req.Id.GetValue())
	if err != nil { return nil, domainerr.FromPg(err) }
	if _, err := s.requireRole(ctx, tr.TenantId, iampb.TenantRole_TENANT_ROLE_VIEWER); err != nil { return nil, err }
	return s.metrics.Query(ctx, tr.TenantId, tr.Id, req.PromqlQuery)
}
```

- [ ] **Step 4: PASS** — `go test ... -run TestTestRunService_Watch`.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/testing/test_run_service.go internal/domain/services/testing/test_run_service_test.go
git commit -m "feat(testing): TestRunService Watch/StreamLogs/Metrics"
```

---

## Task 7: TestSuiteService

**Files:**
- Create: `internal/domain/services/testing/test_suite_service.go`
- Create: `internal/domain/services/testing/test_suite_service_test.go`

- [ ] **Step 1: Write failing test for CRUD + Clone**

```go
func TestTestSuiteService_CreateCloneList(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	svc := testingsvc.NewTestSuiteService(f.Executor(), f.TxManager(), f.Events(), f.IAM())
	tenant := f.Tenant()
	ctx := f.AuthCtx(f.User(tenant), tenant)

	s, err := svc.CreateTestSuite(ctx, &testingpb.CreateTestSuiteRequest{
		TenantId: tenant.Id, Name: "stress",
		Templates: []*testingpb.TestRunTemplateId{f.TestRunTemplate(tenant).Id},
	})
	require.NoError(t, err)

	clone, err := svc.CloneTestSuite(ctx, s.Id)
	require.NoError(t, err)
	require.NotEqual(t, s.Id.Value, clone.Id.Value)
	require.Equal(t, s.Templates, clone.Templates)

	list, err := svc.ListTestSuites(ctx, tenant.Id)
	require.NoError(t, err)
	require.Len(t, list.Items, 2)
}
```

- [ ] **Step 2: FAIL.**
- [ ] **Step 3: Implement** mirroring `TemplateService` shape: repo on `testingpb.TestSuites`, role checks (editor for mutate, viewer for read), `CloneTestSuite` deep-copies template list with new ULID.

- [ ] **Step 4: PASS.**
- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/testing/test_suite_service.go internal/domain/services/testing/test_suite_service_test.go
git commit -m "feat(testing): TestSuiteService CRUD+Clone"
```

---

## Task 8: SuiteDagBuilder + TestSuiteRunService — Launch/Cancel

**Files:**
- Create: `internal/domain/services/testing/dagbuilder/suite_builder.go`
- Create: `internal/domain/services/testing/dagbuilder/suite_builder_test.go`
- Create: `internal/domain/services/testing/test_suite_run_service.go`
- Create: `internal/domain/services/testing/test_suite_run_service_test.go`

- [ ] **Step 1: Failing test for suite builder**

```go
func TestBuilder_FromTestSuite_OneNodePerChild(t *testing.T) {
	suite := fixture.TestSuiteWithNTemplates(t, 3)
	b := dagbuilder.New(fixture.NewCatalogPortStub(t))
	dag, err := b.FromTestSuite(context.Background(), suite, fixture.MaterializedChildTestRunIDs(3))
	require.NoError(t, err)
	require.Len(t, dag.Nodes, 3)
	for _, n := range dag.Nodes {
		require.Empty(t, n.DependsOn) // parallel by default
	}
}
```

- [ ] **Step 2: FAIL.**
- [ ] **Step 3: Implement `Builder.FromTestSuite`** — one `tasksp.TestRunRefTask{TestRunId: child}` per child id. Honor `suite.Spec.Sequential bool`: if true, chain `DependsOn = [previous]`.

- [ ] **Step 4: PASS.**
- [ ] **Step 5: Failing test for `LaunchTestSuite`**

```go
func TestTestSuiteRunService_Launch(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	engine := f.MockSystemEngine()
	svc := f.TestSuiteRunService(engine)
	tenant := f.Tenant()
	ctx := f.AuthCtx(f.User(tenant), tenant)
	suite := f.TestSuiteWithTemplates(tenant, 2)

	run, err := svc.LaunchTestSuite(ctx, suite.Id)
	require.NoError(t, err)
	require.Equal(t, testingpb.TestSuiteRunStatus_TEST_SUITE_RUN_STATUS_RUNNING, run.Status)
	require.Len(t, run.Children, 2)
	require.Len(t, engine.LaunchCalls, 1) // one meta-DAG
	// each child has its own TestRun row
	for _, c := range run.Children {
		_, err := f.TestRunService(nil).GetTestRun(ctx, c.TestRunId)
		require.NoError(t, err)
	}
}
```

- [ ] **Step 6: FAIL.**
- [ ] **Step 7: Implement TestSuiteRunService LaunchTestSuite + CancelTestSuiteRun + Get/Watch/Logs/Metrics/ListBySuite/ListByTenant**

For `LaunchTestSuite`: in a SERIALIZABLE tx, materialize one `TestRun` per template (status=PENDING, spec=template.Spec), build meta-DAG via `Builder.FromTestSuite(suite, childIDs)`, call `engine.LaunchDag(ctx, metaDag, DagRunLink{Kind: SUITE_RUN, Id: suiteRunID})`. Persist `TestSuiteRun` with `Children = [(TestRunId, NodeId)]` mapping. Set status to RUNNING.

Watch/Logs/Metrics route through the engine the same way as `TestRunService`.

`CancelTestSuiteRun` calls `engine.CancelDagRun(metaDagRunID)`; the engine cascades via the `test_run_ref` node handler's ctx propagation (handler watches child DagRun, propagates cancel down).

- [ ] **Step 8: PASS.**
- [ ] **Step 9: Commit**

```bash
git add internal/domain/services/testing/dagbuilder/suite_builder.go internal/domain/services/testing/dagbuilder/suite_builder_test.go internal/domain/services/testing/test_suite_run_service.go internal/domain/services/testing/test_suite_run_service_test.go
git commit -m "feat(testing): TestSuiteRunService + suite DagBuilder"
```

---

## Task 9: SharedTestRunService

**Files:**
- Create: `internal/domain/services/testing/shared_test_run_service.go`
- Create: `internal/domain/services/testing/shared_test_run_service_test.go`

- [ ] **Step 1: Failing test**

```go
func TestSharedTestRunService_CreateRevokeGetByToken(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	svc := f.SharedTestRunService()
	tenant := f.Tenant()
	owner := f.User(tenant)
	ctx := f.AuthCtx(owner, tenant)
	tr := f.CompletedTestRun(tenant)

	share, err := svc.CreateSharedTestRun(ctx, &testingpb.CreateSharedTestRunRequest{
		TestRunId: tr.Id, ExpiresInSeconds: 3600,
	})
	require.NoError(t, err)
	require.NotEmpty(t, share.Token)

	// Anonymous (no auth) access by token
	got, err := svc.GetByToken(context.Background(), &testingpb.GetSharedTestRunByTokenRequest{Token: share.Token})
	require.NoError(t, err)
	require.Equal(t, tr.Id.Value, got.TestRunSnapshot.Id.Value)

	_, err = svc.RevokeSharedTestRun(ctx, share.Id)
	require.NoError(t, err)
	_, err = svc.GetByToken(context.Background(), &testingpb.GetSharedTestRunByTokenRequest{Token: share.Token})
	require.ErrorContains(t, err, "NOT_FOUND")
}
```

- [ ] **Step 2: FAIL.**
- [ ] **Step 3: Implement** — repo on `testingpb.SharedTestRuns`. `CreateSharedTestRun`: editor role on owning tenant, token = `ids.NewRandomB64(32)`, expires_at computed, store snapshot pointer (id only — `GetByToken` hydrates via `TestRunService`). `RevokeSharedTestRun`: editor role, set `revoked_at`. `GetByToken`: NO auth — public; reject if `revoked_at IS NOT NULL OR expires_at < now()` with `NOT_FOUND` (don't leak existence). Add to auth middleware bypass list.

- [ ] **Step 4: PASS.**
- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/testing/shared_test_run_service.go internal/domain/services/testing/shared_test_run_service_test.go
git commit -m "feat(testing): SharedTestRunService"
```

---

## Task 10: SharedSuiteRunService

**Files:**
- Create: `internal/domain/services/testing/shared_suite_run_service.go`
- Create: `internal/domain/services/testing/shared_suite_run_service_test.go`

- [ ] **Step 1-5:** Mirror Task 9 verbatim against `SharedSuiteRuns` repo and `TestSuiteRun` snapshot. Commit message: `feat(testing): SharedSuiteRunService`.

---

## Task 11: ComparisonService

**Files:**
- Create: `internal/domain/services/testing/comparison_service.go`
- Create: `internal/domain/services/testing/comparison_service_test.go`

- [ ] **Step 1: Failing test**

```go
func TestComparisonService_CompareRuns(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	metrics := f.MockMetricsPort()
	svc := testingsvc.NewComparisonService(f.IAM(), metrics)
	tenant := f.Tenant()
	a := f.CompletedTestRun(tenant); b := f.CompletedTestRun(tenant)
	metrics.SetSeries(a.Id, "p99_ms", []float64{12, 14, 13})
	metrics.SetSeries(b.Id, "p99_ms", []float64{8, 9, 10})

	resp, err := svc.CompareRuns(f.AuthCtx(f.User(tenant), tenant), &testingpb.CompareRunsRequest{
		BaselineId: a.Id, CandidateId: b.Id, Metrics: []string{"p99_ms"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Diffs, 1)
	require.InDelta(t, -33.3, resp.Diffs[0].PercentChange, 1)
}
```

- [ ] **Step 2: FAIL.**
- [ ] **Step 3: Implement** — `CompareRuns`: viewer role on baseline & candidate tenants (must match), query metrics via `MetricsPort.Query(... "avg_over_time(metric)")` for each id, compute `(candidate - baseline) / baseline * 100`, return `[]MetricDiff`. `CrossCompareBatch`: loop over pairs.

- [ ] **Step 4: PASS.**
- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/testing/comparison_service.go internal/domain/services/testing/comparison_service_test.go
git commit -m "feat(testing): ComparisonService"
```

---

## Task 12: Connect handlers

**Files:**
- Create: `internal/transport/connect/testing_handlers.go`
- Create: `internal/transport/connect/testing_handlers_test.go`

- [ ] **Step 1: Failing handler test using in-process Connect client**

```go
func TestTestRunHandler_LaunchHonorsIdempotency(t *testing.T) {
	t.Parallel()
	srv := harness.NewConnectServer(t) // wires full middleware chain + svc
	tenant, user := srv.SeedTenantUser()
	cli := harness.NewTestRunClient(srv)

	tr, err := cli.CreateTestRun(srv.UserCtx(user, tenant), &testingpb.CreateTestRunRequest{...})
	require.NoError(t, err)

	hdr := http.Header{"X-Idempotency-Key": []string{"k1"}}
	r1, err := cli.LaunchTestRunWithHeaders(srv.UserCtx(user, tenant), tr.Id, hdr)
	require.NoError(t, err)
	r2, err := cli.LaunchTestRunWithHeaders(srv.UserCtx(user, tenant), tr.Id, hdr)
	require.NoError(t, err)
	require.Equal(t, r1.DagRunId.Value, r2.DagRunId.Value) // cached replay
}
```

- [ ] **Step 2: FAIL.**
- [ ] **Step 3: Implement six handler structs**

Each handler:
- `Handle<Verb>` constructs a request, defers to service method, wraps response in `connect.Response`.
- Streaming verbs (`WatchTestRun`, `StreamTestRunLogs`, `WatchTestSuiteRun`, `StreamTestSuiteRunLogs`) implement `Handle<Verb>(ctx, *connect.Request[Req], *connect.ServerStream[Resp]) error` and forward channel to stream.
- No business logic, no role checks (delegate to service).

```go
type TestRunHandler struct {
	svc *testingsvc.TestRunService
}

func (h *TestRunHandler) LaunchTestRun(ctx context.Context, req *connect.Request[testingpb.TestRunId]) (*connect.Response[testingpb.TestRun], error) {
	out, err := h.svc.LaunchTestRun(ctx, req.Msg)
	if err != nil { return nil, err }
	return connect.NewResponse(out), nil
}

func (h *TestRunHandler) WatchTestRun(ctx context.Context, req *connect.Request[testingpb.WatchTestRunRequest], stream *connect.ServerStream[testingpb.TestRunProgress]) error {
	ch, err := h.svc.WatchTestRun(ctx, req.Msg)
	if err != nil { return err }
	for p := range ch {
		if err := stream.Send(p); err != nil { return err }
	}
	return nil
}
```

- [ ] **Step 4: PASS** all handler tests.
- [ ] **Step 5: Commit**

```bash
git add internal/transport/connect/testing_handlers.go internal/transport/connect/testing_handlers_test.go
git commit -m "feat(transport): testing connect handlers"
```

---

## Task 13: Scheduler `fire()` integration

**Files:**
- Modify: `internal/domain/services/system/scheduler/scheduler.go`
- Modify: `internal/domain/services/system/scheduler/scheduler_test.go`

- [ ] **Step 1: Failing integration test**

```go
func TestScheduler_FireInstantiatesAndLaunchesTestRun(t *testing.T) {
	t.Parallel()
	f := fixture.New(t)
	engine := f.MockSystemEngine()
	trsvc := f.TestRunService(engine)
	sched := scheduler.New(f.Executor(), f.TxManager(), trsvc, f.Clock())
	tenant := f.Tenant()
	tpl := f.TestRunTemplate(tenant)
	f.Schedule(tenant, tpl, "*/5 * * * *", time.Now().Add(-time.Second))

	require.NoError(t, sched.Tick(context.Background()))

	runs, err := trsvc.ListTestRuns(f.AuthCtx(f.AdminUser(), tenant), tenant.Id)
	require.NoError(t, err)
	require.Len(t, runs.Items, 1)
	require.Equal(t, testingpb.TestRunStatus_TEST_RUN_STATUS_RUNNING, runs.Items[0].Status)
}
```

- [ ] **Step 2: FAIL.**
- [ ] **Step 3: Replace `fire()` stub**

```go
type TestRunLauncher interface {
	InstantiateTestRun(ctx context.Context, id *testingpb.TestRunTemplateId) (*testingpb.TestRun, error)
	LaunchTestRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.TestRun, error)
}

func (s *Scheduler) fire(ctx context.Context, claimed *systempb.Schedule) error {
	tr, err := s.launcher.InstantiateTestRun(ctx, claimed.TemplateId)
	if err != nil { return err }
	_, err = s.launcher.LaunchTestRun(ctx, tr.Id)
	return err
}
```

Constructor now takes `launcher TestRunLauncher`.

- [ ] **Step 4: PASS.**
- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/system/scheduler/scheduler.go internal/domain/services/system/scheduler/scheduler_test.go
git commit -m "feat(scheduler): wire fire() to TestRunService"
```

---

## Task 14: SDK client + CLI subcommands

**Files:**
- Create: `internal/sdk/client/testing.go`
- Create: `cmd/stroppy-cloud/cmd_cli/run.go`
- Create: `cmd/stroppy-cloud/cmd_cli/suite.go`
- Create: `cmd/stroppy-cloud/cmd_cli/template.go`
- Create: `cmd/stroppy-cloud/cmd_cli/compare.go`
- Modify: `cmd/stroppy-cloud/cmd_cli/root.go`

- [ ] **Step 1: sdk/client/testing.go**

```go
type TestingClients struct {
	Templates    testingv1connect.TestRunTemplateServiceClient
	Runs         testingv1connect.TestRunServiceClient
	Suites       testingv1connect.TestSuiteServiceClient
	SuiteRuns    testingv1connect.TestSuiteRunServiceClient
	SharedRuns   testingv1connect.SharedTestRunServiceClient
	SharedSuites testingv1connect.SharedSuiteRunServiceClient
	Comparison   testingv1connect.ComparisonServiceClient
}

func NewTestingClients(httpc *http.Client, base string, opts ...connect.ClientOption) *TestingClients {
	return &TestingClients{
		Templates:  testingv1connect.NewTestRunTemplateServiceClient(httpc, base, opts...),
		Runs:       testingv1connect.NewTestRunServiceClient(httpc, base, opts...),
		// ...
	}
}
```

- [ ] **Step 2: CLI run.go subcommands**

```
stroppy-cloud run start --template <id> [--watch]
stroppy-cloud run list
stroppy-cloud run get <id>
stroppy-cloud run cancel <id>
stroppy-cloud run logs <id> [--step <id>] [--since <cursor>] [--follow]
stroppy-cloud run metrics <id> --query <promql>
stroppy-cloud run share <id> [--ttl 3600]
```

`run start --watch`: calls `InstantiateTestRun` then `LaunchTestRun` then `WatchTestRun` and renders a live progress table via `tea`/text-only fallback.

`run logs --follow`: opens `StreamTestRunLogs`, prints each line; on Ctrl-C closes stream.

- [ ] **Step 3: suite.go**

```
stroppy-cloud suite list
stroppy-cloud suite create --name X --templates <ids,...>
stroppy-cloud suite launch <id>
stroppy-cloud suite cancel <id>
stroppy-cloud suite get <id>
```

- [ ] **Step 4: template.go**

```
stroppy-cloud template list
stroppy-cloud template create --name X --spec <file.yaml>
stroppy-cloud template delete <id>
stroppy-cloud template instantiate <id>  # creates TestRun, prints id
```

- [ ] **Step 5: compare.go**

```
stroppy-cloud compare runs <baseline-id> <candidate-id> [--metric p99_ms,tps,...]
```

Renders a table of `MetricDiff` rows.

- [ ] **Step 6: Wire into root.go**

```go
root.AddCommand(NewRunCmd(sdk), NewSuiteCmd(sdk), NewTemplateCmd(sdk), NewCompareCmd(sdk))
```

- [ ] **Step 7: Smoke test**

Run `go build ./cmd/stroppy-cloud/...`. Expected: PASS.

Run `./stroppy-cloud --help` shows all new subcommands.

- [ ] **Step 8: Commit**

```bash
git add internal/sdk/client/testing.go cmd/stroppy-cloud/cmd_cli/
git commit -m "feat(cli): testing subcommands run/suite/template/compare"
```

---

## Task 15: cmd_server wiring + e2e smoke

**Files:**
- Modify: `cmd/stroppy-cloud/cmd_server.go`
- Create: `tests/e2e/testing_smoke_test.go`

- [ ] **Step 1: Wire services in cmd_server.go**

```go
catalogPort := catalogsvc.AsPort(catalogService)
enginePort := systemsvc.AsEnginePort(systemEngine)
metricsPort := victoria.AsMetricsPort(victoriaClient)
builder := dagbuilder.New(catalogPort)

templateSvc := testingsvc.NewTemplateService(executor, txMgr, eventBus, iamPort)
testRunSvc := testingsvc.NewTestRunService(executor, txMgr, eventBus, iamPort, catalogPort, enginePort, metricsPort, builder)
suiteSvc := testingsvc.NewTestSuiteService(executor, txMgr, eventBus, iamPort)
suiteRunSvc := testingsvc.NewTestSuiteRunService(executor, txMgr, eventBus, iamPort, enginePort, builder, testRunSvc)
sharedRunSvc := testingsvc.NewSharedTestRunService(executor, txMgr, iamPort, testRunSvc)
sharedSuiteSvc := testingsvc.NewSharedSuiteRunService(executor, txMgr, iamPort, suiteRunSvc)
comparisonSvc := testingsvc.NewComparisonService(iamPort, metricsPort)

scheduler := schedsvc.New(executor, txMgr, testRunSvc, clock)

mux.Handle(testingv1connect.NewTestRunTemplateServiceHandler(&connecthandlers.TemplateHandler{Svc: templateSvc}, interceptors))
mux.Handle(testingv1connect.NewTestRunServiceHandler(&connecthandlers.TestRunHandler{Svc: testRunSvc}, interceptors))
// ... six more
```

Add `GetByToken` paths to auth-bypass list.

- [ ] **Step 2: e2e test**

```go
//go:build e2e

func TestE2E_TemplateCreate_TestRunLaunch_WaitDone(t *testing.T) {
	srv := e2e.StartServer(t, e2e.WithMockHandlers()) // mock terraform/install/stroppy_run all return DONE immediately
	defer srv.Close()
	cli := srv.LoginAsAdmin()

	tpl, err := cli.Templates.CreateTestRunTemplate(srv.Ctx(), &testingpb.CreateTestRunTemplateRequest{...})
	require.NoError(t, err)
	tr, err := cli.Runs.InstantiateTestRun(srv.Ctx(), tpl.Id)
	require.NoError(t, err)
	tr2, err := cli.Runs.LaunchTestRun(srv.Ctx(), tr.Id)
	require.NoError(t, err)
	require.Equal(t, testingpb.TestRunStatus_TEST_RUN_STATUS_RUNNING, tr2.Status)

	require.Eventually(t, func() bool {
		got, err := cli.Runs.GetTestRun(srv.Ctx(), tr.Id)
		return err == nil && got.TestRun.Status == testingpb.TestRunStatus_TEST_RUN_STATUS_SUCCEEDED
	}, 30*time.Second, 200*time.Millisecond)
}
```

- [ ] **Step 3: Run**

```
go test -tags=e2e ./tests/e2e/ -run TestE2E_TemplateCreate_TestRunLaunch_WaitDone -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/stroppy-cloud/cmd_server.go tests/e2e/testing_smoke_test.go
git commit -m "feat(server): wire testing services + e2e smoke"
```

---

## Acceptance criteria

- All six testing services compile and `go test ./internal/domain/services/testing/...` is green (~3 min wall time, real Postgres testcontainer).
- `DagBuilder.FromTestRun` produces 4-or-5-node DAG matching `KeepClusterAfter` flag; `FromTestSuite` produces N-node meta-DAG honoring `Sequential` flag.
- `LaunchTestRun` mutates `TestRun.Status` to `RUNNING`, persists `DagRunId`, publishes `TestRunLaunched` event, calls `Engine.LaunchDag` exactly once.
- `WatchTestRun` and `StreamTestRunLogs` produce live events sourced from the in-memory bus with DB backfill.
- `SharedTestRunService.GetByToken` is reachable without auth and rejects expired/revoked tokens with `NOT_FOUND`.
- `ComparisonService.CompareRuns` returns per-metric `PercentChange` against `MetricsPort` stub.
- Scheduler `fire()` calls `Instantiate → Launch` and produces a RUNNING TestRun.
- All six handler structs registered in `cmd_server.go`; all CLI subcommands present in `--help`.
- e2e: template→instantiate→launch→eventually(SUCCEEDED) green in <30 s with mocked node handlers.

---

## Out of scope (next plan: 05-agent)

- Real terraform/install/stroppy node handlers — Plan 03 ships mock; Plan 05 ships real agent + 5-verb dispatch and real `agent.Hub` integration so node handlers actually drive remote work.
- Webhook delivery on `TestRunCompleted` — Plan 07 (ops).
- Admin cross-tenant TestRun list — Plan 07 (admin).
- Frontend wiring — Plan 08.
