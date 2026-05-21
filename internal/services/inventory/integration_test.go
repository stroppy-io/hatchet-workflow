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
	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
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

// seedAllocation inserts a NetworkAllocation row into the DB mirror.
func (f *fixture) seedAllocation(t *testing.T, ctx context.Context, tenantID *models.TenantId, cidr string) string {
	t.Helper()
	repo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.NetworkAllocations.Table, f.executor),
		models.NetworkAllocationConverter,
	)
	id := ids.New()
	na := &models.NetworkAllocation{
		Id:         id,
		TenantId:   tenantID,
		Provider:   deployment.Provider_PROVIDER_YANDEX,
		Cidr:       &system.Cidr{Value: cidr},
		Timestamps: &models.Timestamps{CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		Tags:       &common.Tags{Tags: []string{"test"}},
	}
	_, err := repo.Execute(ctx, models.NetworkAllocations.Insert().From(na.IntoPlain().AllSetters()...))
	require.NoError(t, err)
	return id
}

func accountCtx(ctx context.Context, accID *models.AccountId) context.Context {
	return caller.NewContext(ctx, &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: accID,
	})
}

func TestListNetworkAllocationsRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// ADMIN can read the network mirror.
	accID, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	id1 := f.seedAllocation(t, ctx, tenantID, "10.42.0.0/24")
	id2 := f.seedAllocation(t, ctx, tenantID, "10.42.1.0/24")

	rctx := accountCtx(ctx, accID)
	list, err := f.svc.ListNetworkAllocations(rctx, &uipb.ListNetworkAllocationsRequest{TenantId: tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetNetworkAllocations(), 2)

	gotIDs := map[string]string{}
	for _, a := range list.GetNetworkAllocations() {
		require.Equal(t, tenantID.GetValue(), a.GetTenantId().GetValue())
		gotIDs[a.GetId()] = a.GetCidr().GetValue()
	}
	require.Equal(t, "10.42.0.0/24", gotIDs[id1])
	require.Equal(t, "10.42.1.0/24", gotIDs[id2])
}

func TestListNetworkAllocationsIsolatedPerTenant(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	accA, tenantA := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	_, tenantB := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	f.seedAllocation(t, ctx, tenantA, "10.10.0.0/24")
	f.seedAllocation(t, ctx, tenantB, "10.20.0.0/24")

	rctxA := accountCtx(ctx, accA)
	list, err := f.svc.ListNetworkAllocations(rctxA, &uipb.ListNetworkAllocationsRequest{TenantId: tenantA})
	require.NoError(t, err)
	require.Len(t, list.GetNetworkAllocations(), 1)
	require.Equal(t, "10.10.0.0/24", list.GetNetworkAllocations()[0].GetCidr().GetValue())

	// accA is not a member of tenant B -> denied.
	_, err = f.svc.ListNetworkAllocations(rctxA, &uipb.ListNetworkAllocationsRequest{TenantId: tenantB})
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
