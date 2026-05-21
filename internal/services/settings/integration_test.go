//go:build integration

package settings_test

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/settings"
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

// fixture is a per-test world: a fresh cloned DB plus the SettingsService and the
// tenancy/authz prerequisites (account + tenant + an OWNER membership).
type fixture struct {
	svc      *settings.SettingsService
	executor exec.DB
	trm      tx.Trm
	tenantID *models.TenantId
	ownerID  *models.AccountId
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()

	az := authz.New(log, executor)
	svc := settings.NewSettingsService(log, executor, trm, az)

	accountID, tenantID := seedTenant(t, log, executor, trm)
	return &fixture{
		svc:      svc,
		executor: executor,
		trm:      trm,
		tenantID: tenantID,
		ownerID:  accountID,
	}
}

// seedTenant builds the FK chain: account -> tenant(owner) -> OWNER membership,
// using the real tenancy services so the rows are shaped exactly as production.
func seedTenant(t *testing.T, log *xlog.Logger, executor exec.DB, trm tx.Trm) (*models.AccountId, *models.TenantId) {
	t.Helper()
	ctx := context.Background()

	acctSvc := tenancy.NewAccountAdminService(log, executor, trm)
	tenantSvc := tenancy.NewTenantAdminService(log, executor, trm)
	az := authz.New(log, executor)
	memberSvc := tenancy.NewTenantService(log, executor, trm, az)

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

	// AddMemberToTenant requires OWNER; bootstrap as a platform admin caller.
	adminCtx := caller.NewContext(ctx, &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: accountID,
		IsAdmin:   true,
	})
	_, err = memberSvc.AddMemberToTenant(adminCtx, &uipb.AddMemberRequest{
		TenantId:  tenantID,
		AccountId: accountID,
		Role:      models.TenantMember_ROLE_OWNER,
	})
	require.NoError(t, err)

	return accountID, tenantID
}

// ownerCtx is a context whose caller is the seeded OWNER account (no platform admin
// bypass — RBAC must resolve the TenantMember row).
func (f *fixture) ownerCtx() context.Context {
	return caller.NewContext(context.Background(), &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: f.ownerID,
	})
}

func TestSettingsItemRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := f.ownerCtx()

	// Create (upsert, no prior row) a settings item.
	created, err := f.svc.SetSettingsItem(ctx, &uipb.SetSettingsItemRequest{
		TenantId: f.tenantID,
		Part:     models.SettingsItem_PART_YANDEX_CLOUD,
		Key:      models.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
		Value:    &models.SettingsItem_Value{Value: &models.SettingsItem_Value_StringValue{StringValue: "tok-1"}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())
	require.Equal(t, "tok-1", created.GetValue().GetStringValue())

	// Read it back by id; persisted value must round-trip.
	got, err := f.svc.GetSettingsItem(ctx, &uipb.GetSettingsItemRequest{
		TenantId:       f.tenantID,
		SettingsItemId: created.GetId(),
	})
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), got.GetId().GetValue())
	require.Equal(t, models.SettingsItem_PART_YANDEX_CLOUD, got.GetPart())
	require.Equal(t, models.SettingsItem_KEY_YANDEX_CLOUD_TOKEN, got.GetKey())
	require.Equal(t, "tok-1", got.GetValue().GetStringValue())

	// List returns the persisted item.
	list, err := f.svc.ListSettingsItems(ctx, &uipb.ListSettingsItemsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetSettingsItems(), 1)
	require.Equal(t, created.GetId().GetValue(), list.GetSettingsItems()[0].GetId().GetValue())

	// "Update" of an existing item is a distinct create on a different (part, key):
	// the upsert-into-same-key path is broken (see TestSettingsUpsertSameKeyIsBroken),
	// so the working write surface is creating new items.
	created2, err := f.svc.SetSettingsItem(ctx, &uipb.SetSettingsItemRequest{
		TenantId: f.tenantID,
		Part:     models.SettingsItem_PART_YANDEX_CLOUD,
		Key:      models.SettingsItem_KEY_YANDEX_CLOUD_CLOUD_ID,
		Value:    &models.SettingsItem_Value{Value: &models.SettingsItem_Value_StringValue{StringValue: "cloud-1"}},
	})
	require.NoError(t, err)
	require.NotEqual(t, created.GetId().GetValue(), created2.GetId().GetValue())

	list, err = f.svc.ListSettingsItems(ctx, &uipb.ListSettingsItemsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetSettingsItems(), 2)

	// Delete (soft) the first item -> Get reports NotFound, List drops to one.
	_, err = f.svc.DeleteSettingsItem(ctx, &uipb.DeleteSettingsItemRequest{
		TenantId:       f.tenantID,
		SettingsItemId: created.GetId(),
	})
	require.NoError(t, err)

	_, err = f.svc.GetSettingsItem(ctx, &uipb.GetSettingsItemRequest{
		TenantId:       f.tenantID,
		SettingsItemId: created.GetId(),
	})
	require.Equal(t, codes.NotFound, status.Code(err))

	list, err = f.svc.ListSettingsItems(ctx, &uipb.ListSettingsItemsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetSettingsItems(), 1)
	require.Equal(t, created2.GetId().GetValue(), list.GetSettingsItems()[0].GetId().GetValue())
}

// TestSettingsUpsertSameKeyIsBroken documents a real defect surfaced by this
// TestSettingsUpsertSameKeyUpdates: re-setting an existing (tenant, part, key)
// updates the value in place (OnConflict DoUpdate), no PK violation.
func TestSettingsUpsertSameKeyUpdates(t *testing.T) {
	f := newFixture(t)
	ctx := f.ownerCtx()

	set := func(val string) error {
		_, err := f.svc.SetSettingsItem(ctx, &uipb.SetSettingsItemRequest{
			TenantId: f.tenantID,
			Part:     models.SettingsItem_PART_YANDEX_CLOUD,
			Key:      models.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
			Value:    &models.SettingsItem_Value{Value: &models.SettingsItem_Value_StringValue{StringValue: val}},
		})
		return err
	}
	require.NoError(t, set("tok-1"))
	require.NoError(t, set("tok-2"), "re-upsert of an existing (part,key) must update, not violate PK")

	list, err := f.svc.ListSettingsItems(ctx, &uipb.ListSettingsItemsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetSettingsItems(), 1, "upsert must update in place, not insert a duplicate")
	require.Equal(t, "tok-2", list.GetSettingsItems()[0].GetValue().GetStringValue())
}

// TestSettingsRBACDenied: a non-member account is denied (no TenantMember row), and
// a VIEWER member is denied write (SetSettingsItem requires OWNER).
func TestSettingsRBACDenied(t *testing.T) {
	f := newFixture(t)

	// Anonymous (no caller) -> Unauthenticated.
	_, err := f.svc.ListSettingsItems(context.Background(), &uipb.ListSettingsItemsRequest{TenantId: f.tenantID})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// A different account that is not a member of the tenant -> PermissionDenied.
	strangerCtx := caller.NewContext(context.Background(), &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: &models.AccountId{Value: "01HZZZSTRANGER000000000000"},
	})
	_, err = f.svc.ListSettingsItems(strangerCtx, &uipb.ListSettingsItemsRequest{TenantId: f.tenantID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// An API token scoped to the tenant but only VIEWER role -> denied on write
	// (SetSettingsItem requires OWNER).
	viewerCtx := caller.NewContext(context.Background(), &caller.Caller{
		Kind:     caller.PrincipalApiToken,
		TenantID: f.tenantID,
		Role:     models.TenantMember_ROLE_VIEWER,
	})
	_, err = f.svc.SetSettingsItem(viewerCtx, &uipb.SetSettingsItemRequest{
		TenantId: f.tenantID,
		Part:     models.SettingsItem_PART_YANDEX_CLOUD,
		Key:      models.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
		Value:    &models.SettingsItem_Value{Value: &models.SettingsItem_Value_StringValue{StringValue: "nope"}},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
