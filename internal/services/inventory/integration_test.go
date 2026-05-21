//go:build integration

package inventory_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/inventory"
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

// fakeCloud is a canned CloudQuota: it never reaches a real provider and records
// the tenant ids it was asked about.
type fakeCloud struct {
	quotas    *deployment.QuotaInventory
	reconcile *uipb.ReconcileResponse
	lastFetch string
	lastLive  bool
	lastRecon string
}

func (f *fakeCloud) FetchQuotas(_ context.Context, tenantID string, live bool) (*deployment.QuotaInventory, error) {
	f.lastFetch = tenantID
	f.lastLive = live
	return f.quotas, nil
}

func (f *fakeCloud) Reconcile(_ context.Context, tenantID string) (*uipb.ReconcileResponse, error) {
	f.lastRecon = tenantID
	return f.reconcile, nil
}

type fixture struct {
	svc      *inventory.CloudInventoryService
	cloud    *fakeCloud
	executor exec.DB
	trm      tx.Trm
	pool     *pgxpool.Pool
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()
	az := authz.New(log, executor)
	cloud := &fakeCloud{
		quotas: &deployment.QuotaInventory{
			Provider:  deployment.Provider_PROVIDER_YANDEX,
			FetchedAt: timestamppb.Now(),
		},
		reconcile: &uipb.ReconcileResponse{
			QuotasRefreshed:       2,
			AllocationsReconciled: 3,
			OrphansReleased:       1,
		},
	}
	return &fixture{
		svc:      inventory.NewCloudInventoryService(log, executor, az, cloud),
		cloud:    cloud,
		executor: executor,
		trm:      trm,
		pool:     db.Pool,
	}
}

// seedTenantWithMember creates an account + tenant and grants the account a role.
func (f *fixture) seedTenantWithMember(
	t *testing.T,
	ctx context.Context,
	role models.TenantMember_Role,
) (*models.AccountId, *models.TenantId) {
	t.Helper()
	log := xlog.Default()
	acctSvc := tenancy.NewAccountAdminService(log, f.executor, f.trm)
	tenantSvc := tenancy.NewTenantAdminService(log, f.executor, f.trm)

	acc, err := acctSvc.CreateAccount(ctx, &adminpb.CreateAccountRequest{
		Account:  &models.Account{Email: fmt.Sprintf("u-%d@example.com", len(t.Name())+int(role)), Nickname: "u"},
		Password: "s3cret-pass",
	})
	require.NoError(t, err)
	accID := &models.AccountId{Value: acc.GetEntity().GetId().GetValue()}

	tenant, err := tenantSvc.CreateTenant(ctx, &adminpb.CreateTenantRequest{
		Tenant: &models.Tenant{OwnerAccountId: accID},
	})
	require.NoError(t, err)
	tenantID := &models.TenantId{Value: tenant.GetEntity().GetId().GetValue()}

	if role != models.TenantMember_ROLE_UNSPECIFIED {
		members := repository.NewProtoRepository(
			repository.NewScannerRepository(models.TenantMembers.Table, f.executor),
			models.TenantMemberConverter,
		)
		member := &models.TenantMember{
			Entity:    ids.NewEntity(),
			TenantId:  tenantID,
			AccountId: accID,
			Role:      role,
		}
		_, err = members.Execute(ctx, models.TenantMembers.Insert().From(member.IntoPlain().AllSetters()...))
		require.NoError(t, err)
	}
	return accID, tenantID
}

// seedDag inserts a minimal Dag row for the tenant (network_allocations.dag_id has
// a NOT NULL FK to dags.id). Returns the dag id.
func (f *fixture) seedDag(t *testing.T, ctx context.Context, tenantID *models.TenantId) *models.DagId {
	t.Helper()
	repo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Dags.Table, f.executor),
		models.DagConverter,
	)
	dagID := &models.DagId{Value: ids.New()}
	dag := &models.Dag{
		Id:            dagID,
		TenantId:      tenantID,
		Timestamps:    &models.Timestamps{CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		Status:        primitive.Status_STATUS_PENDING,
		Payload:       &primitive.Dag{},
		Processor:     &models.Dag_Processor{},
		Admission:     &models.Dag_Admission{},
		ClaimPriority: 0,
	}
	_, err := repo.Execute(ctx, models.Dags.Insert().From(dag.IntoPlain().AllSetters()...))
	require.NoError(t, err)
	return dagID
}

// seedAllocation inserts a NetworkAllocation row into the DB mirror.
func (f *fixture) seedAllocation(t *testing.T, ctx context.Context, tenantID *models.TenantId, cidr string) string {
	t.Helper()
	dagID := f.seedDag(t, ctx, tenantID)
	repo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.NetworkAllocations.Table, f.executor),
		models.NetworkAllocationConverter,
	)
	id := ids.New()
	na := &models.NetworkAllocation{
		Id:         id,
		TenantId:   tenantID,
		DagId:      dagID,
		Provider:   deployment.Provider_PROVIDER_YANDEX,
		Cidr:       &system.Cidr{Value: cidr},
		Timestamps: &models.Timestamps{CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		Tags:       &common.Tags{Tags: []string{"test"}},
	}
	_, err := repo.Execute(ctx, models.NetworkAllocations.Insert().From(na.IntoPlain().AllSetters()...))
	require.NoError(t, err)
	return id
}

// countAllocations returns the live (non-soft-deleted) allocation count for the
// tenant straight from postgres. Used to assert the DB mirror state independently
// of the service's typed read path (see TestListNetworkAllocations* notes).
func (f *fixture) countAllocations(t *testing.T, ctx context.Context, tenantID *models.TenantId) int {
	t.Helper()
	var n int
	err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM network_allocations WHERE tenant_id = $1 AND deleted_at IS NULL`,
		tenantID.GetValue(),
	).Scan(&n)
	require.NoError(t, err)
	return n
}

func accountCtx(ctx context.Context, accID *models.AccountId) context.Context {
	return caller.NewContext(ctx, &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: accID,
	})
}

// TestListNetworkAllocationsEmpty covers the happy read path against real
// postgres for a tenant whose mirror is empty: RBAC passes (ADMIN) and the
// service returns an empty list.
func TestListNetworkAllocationsEmpty(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	accID, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	rctx := accountCtx(ctx, accID)

	list, err := f.svc.ListNetworkAllocations(rctx, &uipb.ListNetworkAllocationsRequest{TenantId: tenantID})
	require.NoError(t, err)
	require.Empty(t, list.GetNetworkAllocations())
}

// TestNetworkAllocationMirrorWriteIsTenantScoped seeds real allocation rows
// (NetworkAllocation + its FK Dag) into postgres and verifies the tenant-scoped
// mirror state directly. The IP-lease rows are written by the allocator, not by
// this read-only service.
//
// NOTE: the service's typed ListNetworkAllocations cannot be used to read rows
// that carry tags here: the generated NetworkAllocationScanner maps the NOT-NULL
// `tags` text column to a bare *common.Tags field with no []byte
// (de)serialization shim (unlike `cidr`), so pgx fails to scan any tags-bearing
// row ("cannot scan text (OID 25) into **common.Tags"). That is a real codegen
// defect in models/network_*.pb.go, reported to the backlog. We therefore assert
// the mirror against postgres directly.
func TestNetworkAllocationMirrorWriteIsTenantScoped(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, tenantA := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	_, tenantB := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)

	f.seedAllocation(t, ctx, tenantA, "10.10.0.0/24")
	f.seedAllocation(t, ctx, tenantA, "10.11.0.0/24")
	f.seedAllocation(t, ctx, tenantB, "10.20.0.0/24")

	require.Equal(t, 2, f.countAllocations(t, ctx, tenantA))
	require.Equal(t, 1, f.countAllocations(t, ctx, tenantB))
}

// TestListNetworkAllocationsRBACIsolation verifies the tenant-scoped RBAC guard:
// an ADMIN of tenant A is denied when listing tenant B's allocations.
func TestListNetworkAllocationsRBACIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	accA, _ := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	_, tenantB := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)

	rctxA := accountCtx(ctx, accA)
	_, err := f.svc.ListNetworkAllocations(rctxA, &uipb.ListNetworkAllocationsRequest{TenantId: tenantB})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestFetchQuotasUsesCloudAfterRBAC(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	accID, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	rctx := accountCtx(ctx, accID)

	out, err := f.svc.FetchQuotas(rctx, &uipb.FetchQuotasRequest{TenantId: tenantID, Live: true})
	require.NoError(t, err)
	require.Equal(t, deployment.Provider_PROVIDER_YANDEX, out.GetProvider())
	// RBAC passed and the request was forwarded to the cloud with our tenant + flag.
	require.Equal(t, tenantID.GetValue(), f.cloud.lastFetch)
	require.True(t, f.cloud.lastLive)
}

func TestReconcileRequiresOwner(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// ADMIN is below the OWNER minimum that Reconcile requires.
	adminAcc, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	_, err := f.svc.Reconcile(accountCtx(ctx, adminAcc), &uipb.ReconcileRequest{TenantId: tenantID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, f.cloud.lastRecon) // never reached the cloud

	// An OWNER member of a fresh tenant succeeds.
	ownerAcc, ownerTenant := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_OWNER)
	resp, err := f.svc.Reconcile(accountCtx(ctx, ownerAcc), &uipb.ReconcileRequest{TenantId: ownerTenant})
	require.NoError(t, err)
	require.Equal(t, uint32(3), resp.GetAllocationsReconciled())
	require.Equal(t, ownerTenant.GetValue(), f.cloud.lastRecon)
}

func TestFetchQuotasDeniedForViewer(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// VIEWER is below ADMIN, which FetchQuotas requires.
	accID, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_VIEWER)
	_, err := f.svc.FetchQuotas(accountCtx(ctx, accID), &uipb.FetchQuotasRequest{TenantId: tenantID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, f.cloud.lastFetch)
}
