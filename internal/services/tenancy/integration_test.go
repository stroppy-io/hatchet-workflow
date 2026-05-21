//go:build integration

package tenancy_test

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

	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
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

func newAdminServices(t *testing.T) (*tenancy.AccountAdminService, *tenancy.TenantAdminService, exec.DB) {
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()
	return tenancy.NewAccountAdminService(log, executor, trm), tenancy.NewTenantAdminService(log, executor, trm), executor
}

func TestCreateAccountAndTenant(t *testing.T) {
	acctSvc, tenantSvc, executor := newAdminServices(t)
	ctx := context.Background()

	acc, err := acctSvc.CreateAccount(ctx, &adminpb.CreateAccountRequest{
		Account:  &models.Account{Email: "alice@example.com", Nickname: "alice"},
		Password: "s3cret-pass",
	})
	require.NoError(t, err)
	require.NotEmpty(t, acc.GetEntity().GetId().GetValue())

	tenant, err := tenantSvc.CreateTenant(ctx, &adminpb.CreateTenantRequest{
		Tenant: &models.Tenant{OwnerAccountId: &models.AccountId{Value: acc.GetEntity().GetId().GetValue()}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, tenant.GetEntity().GetId().GetValue())

	// Read the tenant back; the owner FK must round-trip.
	repo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Tenants.Table, executor), models.TenantConverter)
	got, err := repo.QueryRow(ctx, models.Tenants.SelectAll().Where(
		models.Tenants.Id.Eq(tenant.GetEntity().GetId().GetValue())))
	require.NoError(t, err)
	require.Equal(t, acc.GetEntity().GetId().GetValue(), got.GetOwnerAccountId().GetValue())
}

func TestCreateTenantRejectsUnknownOwner(t *testing.T) {
	_, tenantSvc, _ := newAdminServices(t)
	_, err := tenantSvc.CreateTenant(context.Background(), &adminpb.CreateTenantRequest{
		Tenant: &models.Tenant{OwnerAccountId: &models.AccountId{Value: "01HZZZNONEXISTENT00000000"}},
	})
	require.Error(t, err) // owner_account_id FK violation
}
