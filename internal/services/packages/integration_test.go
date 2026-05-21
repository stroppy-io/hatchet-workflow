//go:build integration

package packages_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
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

// fakeUploader is a no-op S3 presigner: it never touches the network and always
// returns a dummy URL.
type fakeUploader struct{}

func (fakeUploader) PresignPut(_ context.Context, objectKey string, _ time.Duration) (string, error) {
	return "https://s3.example.test/" + objectKey + "?signed=1", nil
}

// fixture holds a per-test service over an isolated database, plus the executor
// and tx manager so tests can seed prerequisite rows.
type fixture struct {
	svc      *packages.PackageService
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
	return &fixture{
		svc:      packages.NewPackageService(log, executor, trm, az, fakeUploader{}),
		executor: executor,
		trm:      trm,
	}
}

// seedTenantWithMember creates an account + tenant and grants the account the
// given role in that tenant. It returns the account and tenant ids. Built via the
// admin services + a direct TenantMember insert (no OWNER required to bootstrap).
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
		Account:  &models.Account{Email: fmt.Sprintf("u-%d@example.com", time.Now().UnixNano()), Nickname: "u"},
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
		f.addMember(t, ctx, accID, tenantID, role)
	}
	return accID, tenantID
}

func (f *fixture) addMember(
	t *testing.T,
	ctx context.Context,
	accID *models.AccountId,
	tenantID *models.TenantId,
	role models.TenantMember_Role,
) {
	t.Helper()
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
	_, err := members.Execute(ctx, models.TenantMembers.Insert().From(member.IntoPlain().AllSetters()...))
	require.NoError(t, err)
}

// accountCtx builds a request context carrying an account principal (non-admin).
func accountCtx(ctx context.Context, accID *models.AccountId) context.Context {
	return caller.NewContext(ctx, &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: accID,
	})
}

func TestPackageRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// ADMIN can upload; the package round-trips through list & a fresh get.
	accID, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	rctx := accountCtx(ctx, accID)

	up, err := f.svc.RequestPackageUpload(rctx, &uipb.RequestPackageUploadRequest{
		TenantId:    tenantID,
		Name:        "postgres-15",
		DbKind:      "postgresql",
		DbVersion:   "15.4",
		DebFilename: "postgresql-15.deb",
	})
	require.NoError(t, err)
	require.NotEmpty(t, up.GetUploadUrl())
	require.Contains(t, up.GetUploadUrl(), "s3.example.test")
	pkgID := up.GetPackage().GetEntity().GetId().GetValue()
	require.NotEmpty(t, pkgID)
	require.NotEmpty(t, up.GetPackage().GetDebObjectUri())

	// ListPackages (VIEWER min role; ADMIN satisfies it) returns the new row.
	list, err := f.svc.ListPackages(rctx, &uipb.ListPackagesRequest{TenantId: tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetPackages(), 1)
	got := list.GetPackages()[0]
	require.Equal(t, pkgID, got.GetEntity().GetId().GetValue())
	require.Equal(t, "postgres-15", got.GetName())
	require.Equal(t, "postgresql", got.GetDbKind())
	require.Equal(t, "15.4", got.GetDbVersion())
	require.Equal(t, tenantID.GetValue(), got.GetOwned().GetTenantId().GetValue())
	require.False(t, got.GetIsBuiltin())

	// DeletePackage soft-deletes; the row drops out of the list.
	_, err = f.svc.DeletePackage(rctx, &uipb.DeletePackageRequest{
		TenantId: tenantID,
		Id:       &models.Ulid{Value: pkgID},
	})
	require.NoError(t, err)

	afterDel, err := f.svc.ListPackages(rctx, &uipb.ListPackagesRequest{TenantId: tenantID})
	require.NoError(t, err)
	require.Empty(t, afterDel.GetPackages())
}

func TestListPackagesIsolatedPerTenant(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	accA, tenantA := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	_, tenantB := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)
	rctxA := accountCtx(ctx, accA)

	_, err := f.svc.RequestPackageUpload(rctxA, &uipb.RequestPackageUploadRequest{
		TenantId:    tenantA,
		Name:        "mysql-8",
		DbKind:      "mysql",
		DbVersion:   "8.0",
		DebFilename: "mysql-8.deb",
	})
	require.NoError(t, err)

	// Tenant A sees its package.
	listA, err := f.svc.ListPackages(rctxA, &uipb.ListPackagesRequest{TenantId: tenantA})
	require.NoError(t, err)
	require.Len(t, listA.GetPackages(), 1)

	// accA is not a member of tenant B -> denied (proves tenant-scoped RBAC).
	_, err = f.svc.ListPackages(rctxA, &uipb.ListPackagesRequest{TenantId: tenantB})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestRequestPackageUploadDeniedForViewer(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// VIEWER can list but cannot upload (upload requires ADMIN).
	accID, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_VIEWER)
	rctx := accountCtx(ctx, accID)

	_, err := f.svc.RequestPackageUpload(rctx, &uipb.RequestPackageUploadRequest{
		TenantId:    tenantID,
		Name:        "pg",
		DbKind:      "postgresql",
		DbVersion:   "16",
		DebFilename: "pg.deb",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// The denial happened before any insert: list is empty.
	list, err := f.svc.ListPackages(rctx, &uipb.ListPackagesRequest{TenantId: tenantID})
	require.NoError(t, err)
	require.Empty(t, list.GetPackages())
}

func TestListPackagesAnonymousUnauthenticated(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, tenantID := f.seedTenantWithMember(t, ctx, models.TenantMember_ROLE_ADMIN)

	// No caller in context -> authz rejects with Unauthenticated.
	_, err := f.svc.ListPackages(ctx, &uipb.ListPackagesRequest{TenantId: tenantID})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
