package ops_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

type quotaFixture struct {
	svc      *opssvc.QuotaService
	tenantID *iampb.TenantId
}

func newQuotaFixture(t *testing.T) *quotaFixture {
	t.Helper()
	iamF := fixture.NewIAM(t)
	ctx := context.Background()
	executor := iamF.F.Executor.(*sqlexec.TxExecutor)

	// Create a user + tenant to satisfy FK.
	user, err := iamF.IAM.CreateUser(ctx, &iampb.User{
		Email:    fmt.Sprintf("quota-owner-%s@test.com", t.Name()),
		Nickname: fmt.Sprintf("quotaowner-%s", t.Name()),
	}, "P@ssw0rd!")
	if err != nil {
		t.Fatalf("newQuotaFixture: create user: %v", err)
	}
	tenant, err := iamF.IAM.CreateTenant(ctx, &iampb.Tenant{
		Identity: &commonpb.Identity{Name: fmt.Sprintf("QuotaTenant-%s", t.Name())},
	}, user.GetId())
	if err != nil {
		t.Fatalf("newQuotaFixture: create tenant: %v", err)
	}

	svc := opssvc.NewQuotaService(executor, iamF.F.Pool, iamF.F.TxMgr)
	return &quotaFixture{
		svc:      svc,
		tenantID: tenant.GetId(),
	}
}

func TestQuotaService_SetLimitAndCheck(t *testing.T) {
	f := newQuotaFixture(t)
	ctx := context.Background()

	// SetLimit should create a new row.
	err := f.svc.SetLimit(ctx, f.tenantID, "runs.concurrent", 5)
	require.NoError(t, err)

	// CheckAndReserve within limit should succeed.
	err = f.svc.CheckAndReserve(ctx, f.tenantID, "runs.concurrent", 3)
	require.NoError(t, err)

	// Release should bring it back.
	err = f.svc.Release(ctx, f.tenantID, "runs.concurrent", 3)
	require.NoError(t, err)
}

func TestQuotaService_ExceedsLimit(t *testing.T) {
	f := newQuotaFixture(t)
	ctx := context.Background()

	err := f.svc.SetLimit(ctx, f.tenantID, "runs.concurrent", 2)
	require.NoError(t, err)

	// Reserve 2 — should succeed.
	err = f.svc.CheckAndReserve(ctx, f.tenantID, "runs.concurrent", 2)
	require.NoError(t, err)

	// Reserve 1 more — should fail with RESOURCE_EXHAUSTED.
	err = f.svc.CheckAndReserve(ctx, f.tenantID, "runs.concurrent", 1)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestQuotaService_GetQuotas(t *testing.T) {
	f := newQuotaFixture(t)
	ctx := context.Background()

	err := f.svc.SetLimit(ctx, f.tenantID, "runs.concurrent", 10)
	require.NoError(t, err)

	list, err := f.svc.GetQuotas(ctx, &opspb.GetQuotasRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.NotEmpty(t, list.GetQuotas())
	require.Equal(t, "runs.concurrent", list.GetQuotas()[0].GetResourceId())
	require.Equal(t, int64(10), list.GetQuotas()[0].GetLimit())
}
