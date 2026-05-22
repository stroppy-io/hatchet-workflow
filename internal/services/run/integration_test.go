//go:build integration

package run_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/dagstore"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	rtlogs "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/logs"
	rtmetrics "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenancy"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil"
)

var testContainer *testutil.PostgresContainer

func TestMain(m *testing.M) {
	ctx := context.Background()
	c, err := testutil.NewPostgresContainer(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres: %v\n", err)
		os.Exit(1)
	}
	defer c.Close(ctx)
	if err := c.CreateTemplateDB(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create template db: %v\n", err)
		os.Exit(1)
	}
	testContainer = c
	os.Exit(m.Run())
}

// --- Fakes for the cheap-to-stub injected deps. ProviderResolver returns a minimal
// docker deployment; run.SubmitTestRun builds a valid Dag from it via BuildTestDag.
// Logs/Metrics return empty data; ShareStore mints a canned token. No real cloud/
// VictoriaLogs/VictoriaMetrics/S3 is touched. The DagStore is REAL.

type fakeProvider struct{}

func (fakeProvider) Resolve(_ context.Context, _ string, _ *domain.TestPreset) (*deployment.Deployment, error) {
	return &deployment.Deployment{
		Provider:   deployment.Provider_PROVIDER_DOCKER,
		Deployment: &deployment.Deployment_Docker{Docker: &deployment.Docker{Input: &deployment.Docker_Input{}}},
	}, nil
}

type fakeLogs struct{}

func (fakeLogs) QueryRunLogs(_ context.Context, _ string, _ *uipb.QueryRunLogsRequest) (*rtlogs.LogPage, error) {
	return &rtlogs.LogPage{}, nil
}

func (fakeLogs) StreamRunLogs(_ context.Context, _, _, _ string, _ grpc.ServerStreamingServer[rtlogs.LogLine]) error {
	return nil
}

func (fakeLogs) BuildLogLink(_ context.Context, _ string, _ *uipb.BuildLogLinkRequest) (string, error) {
	return "https://logs.example.com/link", nil
}

type fakeMetrics struct{}

func (fakeMetrics) GetRunMetrics(_ context.Context, runID string) (*rtmetrics.RunMetrics, error) {
	return &rtmetrics.RunMetrics{}, nil
}

func (fakeMetrics) CompareRuns(_ context.Context, _, _ string) (*rtmetrics.Comparison, error) {
	return &rtmetrics.Comparison{}, nil
}

type fakeShare struct{}

func (fakeShare) CreateShareLink(_ context.Context, _ *models.TestRun) (string, string, error) {
	return "share-token-123", "https://share.example.com/share-token-123", nil
}

func (fakeShare) GetSharedRun(_ context.Context, token string) (*uipb.GetSharedRunResponse, error) {
	return &uipb.GetSharedRunResponse{}, nil
}

// fixture bundles the RunService over a fresh cloned DB plus the seeded FK chain
// (account -> tenant -> tenant_member) so authz.Require passes. The real planner +
// dagstore let SubmitTestRun persist a Dag row alongside the TestRun.
type fixture struct {
	svc       *run.RunService
	executor  exec.DB
	store     *dagstore.Store
	tenantID  *models.TenantId
	accountID *models.AccountId
	ctx       context.Context // carries the seeded account caller
}

func newFixture(t *testing.T, role models.TenantMember_Role) *fixture {
	t.Helper()
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()

	az := authz.New(log, executor)
	store := dagstore.New(log, executor)
	svc := run.New(log, executor, trm, az,
		fakeProvider{},
		store, // real dagstore
		fakeLogs{}, fakeMetrics{}, fakeShare{})

	ctx := context.Background()
	acctSvc := tenancy.NewAccountAdminService(log, executor, trm)
	tenantSvc := tenancy.NewTenantAdminService(log, executor, trm)

	acc, err := acctSvc.CreateAccount(ctx, &adminpb.CreateAccountRequest{
		Account:  &models.Account{Email: "owner@example.com", Nickname: "owner"},
		Password: "s3cret-pass",
	})
	require.NoError(t, err)
	accountID := &models.AccountId{Value: acc.GetEntity().GetId().GetValue()}

	tenant, err := tenantSvc.CreateTenant(ctx, &adminpb.CreateTenantRequest{
		Tenant: &models.Tenant{OwnerAccountId: accountID},
	})
	require.NoError(t, err)
	tenantID := &models.TenantId{Value: tenant.GetEntity().GetId().GetValue()}

	// Seed the membership row directly (AddMemberToTenant requires OWNER — circular
	// for the first member).
	memberRepo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.TenantMembers.Table, executor),
		models.TenantMemberConverter,
	)
	member := &models.TenantMember{
		Entity:    ids.NewEntity(),
		TenantId:  tenantID,
		AccountId: accountID,
		Role:      role,
	}
	_, err = memberRepo.Execute(ctx, models.TenantMembers.Insert().From(member.IntoPlain().AllSetters()...))
	require.NoError(t, err)

	callerCtx := caller.NewContext(ctx, &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: accountID,
	})

	return &fixture{
		svc:       svc,
		executor:  executor,
		store:     store,
		tenantID:  tenantID,
		accountID: accountID,
		ctx:       callerCtx,
	}
}

// singlePreset is a minimal valid single-node postgres topology — the planner
// compiles it into a valid Dag (mirrors planner_test.singlePreset).
func singlePreset() *domain.TestPreset {
	return &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{
					Id: "db-1", Cores: 4, MemoryGb: 16, DiskGb: 100, DataDisksGb: []uint64{200},
					Components: []*domain.Topology_Component{{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE}},
				},
			},
		},
	}
}

// dagRepo reads the dags table directly to assert the planner's Dag persisted.
func (f *fixture) dagRepo() *repository.ProtoRepository[
	models.DagAlias, models.DagColumnAlias, *models.DagScanner, *models.Dag,
] {
	return repository.NewProtoRepository(
		repository.NewScannerRepository(models.Dags.Table, f.executor),
		models.DagConverter,
	)
}

// TestSubmitTestRunPersistsRunAndDag is the core flow: SubmitTestRun compiles the
// preset (real planner), persists the Dag (real dagstore) + the TestRun in one tx,
// and the row round-trips through Get/List with the Dag reachable from the dags table.
func TestSubmitTestRunPersistsRunAndDag(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	name := "nightly-pg"
	desc := "single-node postgres smoke"
	created, err := f.svc.SubmitTestRun(f.ctx, &uipb.SubmitTestRunRequest{
		TenantId:    f.tenantID,
		TestPreset:  singlePreset(),
		Name:        &name,
		Description: &desc,
	})
	require.NoError(t, err)
	runID := created.GetEntity().GetId().GetValue()
	require.NotEmpty(t, runID)
	require.Equal(t, f.accountID.GetValue(), created.GetOwned().GetOwnerAccountId().GetValue())
	require.Equal(t, f.tenantID.GetValue(), created.GetOwned().GetTenantId().GetValue())
	require.NotEmpty(t, created.GetDag().GetValue(), "run must reference a compiled dag")

	// The Dag row was persisted by the real dagstore in the same tx.
	dagRow, err := f.dagRepo().QueryRow(context.Background(),
		models.Dags.SelectAll().Where(models.Dags.Id.Eq(created.GetDag().GetValue())))
	require.NoError(t, err)
	require.Equal(t, f.tenantID.GetValue(), dagRow.GetTenantId().GetValue())
	require.Equal(t, primitive.Status_STATUS_PENDING, dagRow.GetStatus())
	// The serialized payload is the compiled Dag with the canonical lifecycle nodes.
	nodeIDs := map[string]bool{}
	for _, n := range dagRow.GetPayload().GetNodes() {
		nodeIDs[n.GetId()] = true
	}
	for _, want := range []string{"render_config", "terraform_apply", "install_and_run", "collect_results", "terraform_destroy"} {
		require.True(t, nodeIDs[want], "compiled dag missing node %q", want)
	}
	// Tenant scoping travels in dag.Metadata.
	require.Equal(t, f.tenantID.GetValue(), dagRow.GetPayload().GetMetadata()["tenant_id"])

	// GetTestRun round-trips.
	got, err := f.svc.GetTestRun(f.ctx, &uipb.GetTestRunRequest{
		TenantId: f.tenantID,
		Id:       &models.TestRunId{Value: runID},
	})
	require.NoError(t, err)
	require.Equal(t, runID, got.GetEntity().GetId().GetValue())
	require.Equal(t, name, got.GetName())
	require.Equal(t, created.GetDag().GetValue(), got.GetDag().GetValue())

	// ListTestRuns returns it.
	list, err := f.svc.ListTestRuns(f.ctx, &uipb.ListTestRunsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetTestRuns(), 1)
	require.Equal(t, runID, list.GetTestRuns()[0].GetEntity().GetId().GetValue())
}

// TestCancelTestRunMovesDagToCancelling: submit then cancel; the run's Dag flips to
// CANCELLING in the store (it started PENDING, so isTerminal is false).
func TestCancelTestRunMovesDagToCancelling(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	created, err := f.svc.SubmitTestRun(f.ctx, &uipb.SubmitTestRunRequest{
		TenantId:   f.tenantID,
		TestPreset: singlePreset(),
	})
	require.NoError(t, err)

	_, err = f.svc.CancelTestRun(f.ctx, &uipb.CancelTestRunRequest{
		TenantId: f.tenantID,
		Id:       &models.TestRunId{Value: created.GetEntity().GetId().GetValue()},
	})
	require.NoError(t, err)

	dag, err := f.store.GetDag(context.Background(), created.GetDag().GetValue())
	require.NoError(t, err)
	require.NotNil(t, dag)
	require.Equal(t, primitive.Status_STATUS_CANCELLING, dag.GetStatus())
}

// TestGetMetricsAndShareDelegateToClients exercises the thin delegating RPCs over a
// real persisted run: the fakes return canned data, so we mostly assert no error +
// the run lookup (loadRun) succeeds. CreateShareLink returns the fake token.
func TestGetMetricsAndShareDelegateToClients(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	created, err := f.svc.SubmitTestRun(f.ctx, &uipb.SubmitTestRunRequest{
		TenantId:   f.tenantID,
		TestPreset: singlePreset(),
	})
	require.NoError(t, err)
	runID := &models.TestRunId{Value: created.GetEntity().GetId().GetValue()}

	mtr, err := f.svc.GetRunMetrics(f.ctx, &uipb.GetRunMetricsRequest{TenantId: f.tenantID, RunId: runID})
	require.NoError(t, err)
	require.NotNil(t, mtr)

	link, err := f.svc.BuildLogLink(f.ctx, &uipb.BuildLogLinkRequest{TenantId: f.tenantID, RunId: runID})
	require.NoError(t, err)
	require.Equal(t, "https://logs.example.com/link", link.GetUrl())

	share, err := f.svc.CreateShareLink(f.ctx, &uipb.CreateShareLinkRequest{TenantId: f.tenantID, RunId: runID})
	require.NoError(t, err)
	require.Equal(t, "share-token-123", share.GetToken())
}

// --- RBAC negatives ---

// VIEWER cannot SubmitTestRun (requires ADMIN).
func TestSubmitTestRunRBACDeniesViewer(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_VIEWER)

	_, err := f.svc.SubmitTestRun(f.ctx, &uipb.SubmitTestRunRequest{
		TenantId:   f.tenantID,
		TestPreset: singlePreset(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// VIEWER can still read (ListTestRuns requires VIEWER).
func TestListTestRunsAllowsViewer(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_VIEWER)

	list, err := f.svc.ListTestRuns(f.ctx, &uipb.ListTestRunsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Empty(t, list.GetTestRuns())
}

// An anonymous caller (no principal) is unauthenticated.
func TestSubmitTestRunRBACDeniesAnonymous(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	_, err := f.svc.ListTestRuns(context.Background(), &uipb.ListTestRunsRequest{TenantId: f.tenantID})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// GetTestRun for a non-existent id returns NotFound.
func TestGetTestRunNotFound(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	_, err := f.svc.GetTestRun(f.ctx, &uipb.GetTestRunRequest{
		TenantId: f.tenantID,
		Id:       &models.TestRunId{Value: ids.New()},
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
