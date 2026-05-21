//go:build integration

package suite_test

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/dagstore"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite"
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

// fakeProvider returns minimal (empty) DeploymentParams; the real planner compiles
// a valid Dag from them. No real cloud is touched. Planner + DagStore are REAL.
type fakeProvider struct{}

func (fakeProvider) Resolve(_ context.Context, _ string, _ *domain.Topology) (*planner.DeploymentParams, error) {
	return &planner.DeploymentParams{}, nil
}

// fixture bundles the SuiteService over a fresh cloned DB plus the seeded FK chain
// (account -> tenant -> tenant_member) so authz.Require passes.
type fixture struct {
	svc       *suite.SuiteService
	executor  exec.DB
	store     *dagstore.Store
	tenantID  *models.TenantId
	accountID *models.AccountId
	ctx       context.Context
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
	svc := suite.New(log, executor, trm, az, planner.New(), fakeProvider{}, store)

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

// singlePreset is a minimal valid single-node postgres topology.
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

// twoTestSuitePreset has two tests so we can assert per-test runs + dags.
func twoTestSuitePreset() *domain.SuitePreset {
	return &domain.SuitePreset{
		Tests: []*domain.TestPreset{singlePreset(), singlePreset()},
		Scheduling: &domain.SuitePreset_Scheduling{
			OnNodeFailure: primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP,
		},
	}
}

func (f *fixture) testRunRepo() *repository.ProtoRepository[
	models.TestRunAlias, models.TestRunColumnAlias, *models.TestRunScanner, *models.TestRun,
] {
	return repository.NewProtoRepository(
		repository.NewScannerRepository(models.TestRuns.Table, f.executor),
		models.TestRunConverter,
	)
}

func (f *fixture) dagRepo() *repository.ProtoRepository[
	models.DagAlias, models.DagColumnAlias, *models.DagScanner, *models.Dag,
] {
	return repository.NewProtoRepository(
		repository.NewScannerRepository(models.Dags.Table, f.executor),
		models.DagConverter,
	)
}

// createSuite is a helper: persists a suite with the given preset and returns its id.
func (f *fixture) createSuite(t *testing.T, preset *domain.SuitePreset) *models.SuiteId {
	t.Helper()
	created, err := f.svc.CreateSuite(f.ctx, &uipb.CreateSuiteRequest{
		TenantId: f.tenantID,
		Preset:   preset,
	})
	require.NoError(t, err)
	return &models.SuiteId{Value: created.GetEntity().GetId().GetValue()}
}

// TestCreateAndGetSuiteRoundTrip: CreateSuite persists, GetSuite reads it back with
// the preset intact.
func TestCreateAndGetSuiteRoundTrip(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	suiteID := f.createSuite(t, twoTestSuitePreset())

	got, err := f.svc.GetSuite(f.ctx, &uipb.GetSuiteRequest{TenantId: f.tenantID, SuiteId: suiteID})
	require.NoError(t, err)
	require.Equal(t, suiteID.GetValue(), got.GetEntity().GetId().GetValue())
	require.Len(t, got.GetPreset().GetTests(), 2)
	require.Equal(t, f.accountID.GetValue(), got.GetOwned().GetOwnerAccountId().GetValue())
}

// TestLaunchSuiteRunPersistsRunsAndDags is the core flow: LaunchSuiteRun compiles
// every test into its own Dag (real planner), builds an orchestration Dag of
// dag_ref nodes, and persists the lot (per-test Dags + orchestration Dag +
// SuiteRun + per-test TestRuns) in one tx. We assert each piece via the tables.
func TestLaunchSuiteRunPersistsRunsAndDags(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	suiteID := f.createSuite(t, twoTestSuitePreset())

	suiteRun, err := f.svc.LaunchSuiteRun(f.ctx, &uipb.LaunchSuiteRunRequest{
		TenantId: f.tenantID,
		SuiteId:  suiteID,
	})
	require.NoError(t, err)
	suiteRunID := suiteRun.GetEntity().GetId().GetValue()
	require.NotEmpty(t, suiteRunID)
	require.Equal(t, suiteID.GetValue(), suiteRun.GetSuiteId().GetValue())
	require.Equal(t, f.accountID.GetValue(), suiteRun.GetOwned().GetOwnerAccountId().GetValue())
	orchDagID := suiteRun.GetDag().GetValue()
	require.NotEmpty(t, orchDagID, "suite run must reference an orchestration dag")

	// Two per-test TestRuns persisted, both linked back to the SuiteRun.
	testRuns, err := f.testRunRepo().Query(context.Background(),
		models.TestRuns.SelectAll().Where(models.TestRuns.SuiteRunId.Eq(&suiteRunID)))
	require.NoError(t, err)
	require.Len(t, testRuns, 2)

	// Each TestRun references a persisted per-test Dag with the canonical nodes.
	for _, tr := range testRuns {
		require.Equal(t, f.tenantID.GetValue(), tr.GetOwned().GetTenantId().GetValue())
		dagRow, derr := f.dagRepo().QueryRow(context.Background(),
			models.Dags.SelectAll().Where(models.Dags.Id.Eq(tr.GetDag().GetValue())))
		require.NoError(t, derr)
		require.Equal(t, primitive.Status_STATUS_PENDING, dagRow.GetStatus())
		nodeIDs := map[string]bool{}
		for _, n := range dagRow.GetPayload().GetNodes() {
			nodeIDs[n.GetId()] = true
		}
		require.True(t, nodeIDs["install_and_run"], "per-test dag missing install_and_run")
	}

	// The orchestration Dag persisted and references the per-test Dags via dag_ref nodes.
	orchRow, err := f.dagRepo().QueryRow(context.Background(),
		models.Dags.SelectAll().Where(models.Dags.Id.Eq(orchDagID)))
	require.NoError(t, err)
	require.Equal(t, primitive.Status_STATUS_PENDING, orchRow.GetStatus())
	require.Len(t, orchRow.GetPayload().GetNodes(), 2, "orchestration dag has one dag_ref node per test")
	refDagIDs := map[string]bool{}
	for _, n := range orchRow.GetPayload().GetNodes() {
		require.NotEmpty(t, n.GetDagRef().GetDagId(), "orchestration node must be a dag_ref")
		refDagIDs[n.GetDagRef().GetDagId()] = true
	}
	for _, tr := range testRuns {
		require.True(t, refDagIDs[tr.GetDag().GetValue()],
			"orchestration dag must dag_ref test run's dag %s", tr.GetDag().GetValue())
	}

	// GetSuiteRun + ListSuiteRuns round-trip.
	got, err := f.svc.GetSuiteRun(f.ctx, &uipb.GetSuiteRunRequest{
		TenantId:   f.tenantID,
		SuiteRunId: &models.SuiteRunId{Value: suiteRunID},
	})
	require.NoError(t, err)
	require.Equal(t, suiteRunID, got.GetEntity().GetId().GetValue())

	list, err := f.svc.ListSuiteRuns(f.ctx, &uipb.ListSuiteRunsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetSuiteRuns(), 1)
	require.Equal(t, suiteRunID, list.GetSuiteRuns()[0].GetEntity().GetId().GetValue())
}

// TestCancelSuiteRunMovesOrchDagToCancelling: launch then cancel; orchestration Dag
// flips to CANCELLING.
func TestCancelSuiteRunMovesOrchDagToCancelling(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	suiteID := f.createSuite(t, twoTestSuitePreset())
	suiteRun, err := f.svc.LaunchSuiteRun(f.ctx, &uipb.LaunchSuiteRunRequest{TenantId: f.tenantID, SuiteId: suiteID})
	require.NoError(t, err)

	_, err = f.svc.CancelSuiteRun(f.ctx, &uipb.CancelSuiteRunRequest{
		TenantId:   f.tenantID,
		SuiteRunId: &models.SuiteRunId{Value: suiteRun.GetEntity().GetId().GetValue()},
	})
	require.NoError(t, err)

	dag, err := f.store.GetDag(context.Background(), suiteRun.GetDag().GetValue())
	require.NoError(t, err)
	require.NotNil(t, dag)
	require.Equal(t, primitive.Status_STATUS_CANCELLING, dag.GetStatus())
}

// --- RBAC negatives ---

// VIEWER cannot CreateSuite (requires ADMIN).
func TestCreateSuiteRBACDeniesViewer(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_VIEWER)

	_, err := f.svc.CreateSuite(f.ctx, &uipb.CreateSuiteRequest{
		TenantId: f.tenantID,
		Preset:   twoTestSuitePreset(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// VIEWER cannot LaunchSuiteRun (requires ADMIN). Seed the suite directly so the
// authz failure (not a missing-suite NotFound) is what's exercised.
func TestLaunchSuiteRunRBACDeniesViewer(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_VIEWER)

	suiteRepo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Suites.Table, f.executor),
		models.SuiteConverter,
	)
	s := &models.Suite{
		Entity: ids.NewEntity(),
		Owned:  &models.Own{OwnerAccountId: f.accountID, TenantId: f.tenantID},
		Preset: twoTestSuitePreset(),
	}
	_, err := suiteRepo.Execute(context.Background(), models.Suites.Insert().From(s.IntoPlain().AllSetters()...))
	require.NoError(t, err)

	_, err = f.svc.LaunchSuiteRun(f.ctx, &uipb.LaunchSuiteRunRequest{
		TenantId: f.tenantID,
		SuiteId:  &models.SuiteId{Value: s.GetEntity().GetId().GetValue()},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// An anonymous caller (no principal) is unauthenticated.
func TestListSuiteRunsRBACDeniesAnonymous(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	_, err := f.svc.ListSuiteRuns(context.Background(), &uipb.ListSuiteRunsRequest{TenantId: f.tenantID})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// LaunchSuiteRun for a non-existent suite returns NotFound.
func TestLaunchSuiteRunSuiteNotFound(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	_, err := f.svc.LaunchSuiteRun(f.ctx, &uipb.LaunchSuiteRunRequest{
		TenantId: f.tenantID,
		SuiteId:  &models.SuiteId{Value: ids.New()},
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
