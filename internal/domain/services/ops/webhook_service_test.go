package ops_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

func setupWebhookSvc(t *testing.T) (*ops.WebhookService, *iampb.TenantId, *iampb.UserId) {
	t.Helper()
	f := fixture.NewIAM(t)
	exec := f.F.Executor.(*sqlexec.TxExecutor)
	svc := ops.NewWebhookService(exec, f.F.TxMgr, f.F.Events)

	ctx := context.Background()
	u, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "wh@test.com", Nickname: "wh"}, "P@ss1234!")
	require.NoError(t, err)
	tn, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "WHTest"}}, u.GetId())
	require.NoError(t, err)
	return svc, tn.GetId(), u.GetId()
}

func TestWebhookServiceCRUD(t *testing.T) {
	svc, tenantID, userID := setupWebhookSvc(t)
	ctx := context.Background()

	// Create
	w := &opspb.Webhook{
		Url:     "https://example.com/hook",
		Enabled: true,
		Secret:  "test-secret",
		Identity: &commonpb.Identity{Name: "my-webhook"},
	}
	created, err := svc.CreateWebhook(ctx, tenantID, userID, w)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())
	require.Equal(t, "https://example.com/hook", created.GetUrl())

	// Get
	got, err := svc.GetWebhook(ctx, created.GetId())
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), got.GetId().GetValue())
	require.Equal(t, "https://example.com/hook", got.GetUrl())

	// List
	list, err := svc.ListWebhooks(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Update
	got.Url = "https://example.com/hook2"
	updated, err := svc.UpdateWebhook(ctx, got)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/hook2", updated.GetUrl())

	// Delete
	deleted, err := svc.DeleteWebhook(ctx, created.GetId())
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), deleted.GetId().GetValue())

	// After delete: not found
	_, err = svc.GetWebhook(ctx, created.GetId())
	require.Error(t, err)

	// List should be empty
	list, err = svc.ListWebhooks(ctx, tenantID)
	require.NoError(t, err)
	require.Empty(t, list)
}
