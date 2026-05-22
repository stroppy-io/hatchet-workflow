//go:build integration

package webhook_test

import (
	"context"
	"fmt"
	"os"
	"sync"
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
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenancy"
	"github.com/stroppy-io/stroppy-cloud/internal/services/webhook"
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

// recordSender is a fake Sender that records deliveries instead of performing real
// HTTP in integration tests.
type recordSender struct {
	mu   sync.Mutex
	sent []sentRecord
}

type sentRecord struct {
	url, secret string
	payload     []byte
}

func (r *recordSender) Send(_ context.Context, url, secret string, payload []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, sentRecord{url, secret, payload})
	return nil
}

func (r *recordSender) urls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.sent))
	for i, s := range r.sent {
		out[i] = s.url
	}
	return out
}

// fixture bundles the system-under-test and the prerequisites a tenant-scoped
// RBAC request needs: a tenant, an account, and the account's TenantMember role.
type fixture struct {
	svc       *webhook.WebhookService
	executor  exec.DB
	sender    *recordSender
	tenantID  *models.TenantId
	accountID *models.AccountId
	ctx       context.Context // carries an ADMIN account caller
}

// newFixture builds the WebhookService over a fresh cloned DB and seeds the FK
// chain (account -> tenant -> tenant_member) so authz.Require passes.
func newFixture(t *testing.T, role models.TenantMember_Role) *fixture {
	t.Helper()
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()

	az := authz.New(log, executor)
	sender := &recordSender{}
	svc := webhook.NewWebhookService(log, executor, trm, az, sender)

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

	// Seed the membership row directly: AddMemberToTenant itself requires OWNER,
	// which is circular for the first member.
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
		sender:    sender,
		tenantID:  tenantID,
		accountID: accountID,
		ctx:       callerCtx,
	}
}

// TestWebhookDeliverRunEvent verifies run-lifecycle delivery fires only the enabled
// webhooks subscribed to the event.
func TestWebhookDeliverRunEvent(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	mk := func(url string, enabled bool, events ...models.Webhook_Event) {
		_, err := f.svc.CreateWebhook(f.ctx, &uipb.CreateWebhookRequest{
			TenantId: f.tenantID,
			Webhook:  &models.Webhook{Url: url, Events: events, Enabled: enabled},
			Secret:   "sec-" + url,
		})
		require.NoError(t, err)
	}
	mk("https://h/completed", true, models.Webhook_EVENT_RUN_COMPLETED)                             // fires
	mk("https://h/failed", true, models.Webhook_EVENT_RUN_FAILED)                                   // wrong event
	mk("https://h/disabled", false, models.Webhook_EVENT_RUN_COMPLETED)                             // disabled
	mk("https://h/both", true, models.Webhook_EVENT_RUN_COMPLETED, models.Webhook_EVENT_RUN_FAILED) // fires

	err := f.svc.DeliverRunEvent(f.ctx, f.tenantID.GetValue(), models.Webhook_EVENT_RUN_COMPLETED, "run-1", "nightly")
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"https://h/completed", "https://h/both"}, f.sender.urls(),
		"only enabled webhooks subscribed to RUN_COMPLETED must fire")

	// The payload carries the event + run id, signed with the webhook's secret.
	f.sender.mu.Lock()
	defer f.sender.mu.Unlock()
	require.Contains(t, string(f.sender.sent[0].payload), `"event":"run.completed"`)
	require.Contains(t, string(f.sender.sent[0].payload), `"id":"run-1"`)
	require.NotEmpty(t, f.sender.sent[0].secret, "secret resolved for signing")
}

func TestWebhookCRUDRoundTrip(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	// Create with a write-only secret.
	created, err := f.svc.CreateWebhook(f.ctx, &uipb.CreateWebhookRequest{
		TenantId: f.tenantID,
		Webhook: &models.Webhook{
			Url:     "https://hooks.example.com/run",
			Events:  []models.Webhook_Event{models.Webhook_EVENT_RUN_COMPLETED},
			Enabled: true,
		},
		Secret: "shhh-write-only",
	})
	require.NoError(t, err)
	id := created.GetEntity().GetId().GetValue()
	require.NotEmpty(t, id)
	require.Equal(t, f.accountID.GetValue(), created.GetOwned().GetOwnerAccountId().GetValue())
	require.Equal(t, f.tenantID.GetValue(), created.GetOwned().GetTenantId().GetValue())

	// List returns the new webhook.
	list, err := f.svc.ListWebhooks(f.ctx, &uipb.ListWebhooksRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetWebhooks(), 1)
	require.Equal(t, id, list.GetWebhooks()[0].GetEntity().GetId().GetValue())
	require.Equal(t, "https://hooks.example.com/run", list.GetWebhooks()[0].GetUrl())

	// The secret is write-only (never on the proto). Verify it persisted by reading
	// it back through the scanner, exactly as WebhookService.TestWebhook does.
	secretRepo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Webhooks.Table, f.executor),
		models.WebhookConverter,
	)
	row, err := secretRepo.Scanner().QueryRow(context.Background(),
		models.Webhooks.Select(models.WebhookColumnSecret).Where(models.Webhooks.Id.Eq(id)))
	require.NoError(t, err)
	require.Equal(t, "shhh-write-only", row.Secret)

	// Update mutable fields (full replace; update_mask not honored by the service).
	updated, err := f.svc.UpdateWebhook(f.ctx, &uipb.UpdateWebhookRequest{
		TenantId: f.tenantID,
		Webhook: &models.Webhook{
			Entity:  &models.Entity{Id: &models.Ulid{Value: id}},
			Url:     "https://hooks.example.com/changed",
			Events:  []models.Webhook_Event{models.Webhook_EVENT_RUN_FAILED},
			Enabled: false,
		},
	})
	require.NoError(t, err)
	require.Equal(t, id, updated.GetEntity().GetId().GetValue())
	require.Equal(t, "https://hooks.example.com/changed", updated.GetUrl())
	require.False(t, updated.GetEnabled())

	// TestWebhook resolves the secret + delivers via the no-op sender.
	_, err = f.svc.TestWebhook(f.ctx, &uipb.TestWebhookRequest{
		TenantId: f.tenantID,
		Id:       &models.Ulid{Value: id},
	})
	require.NoError(t, err)

	// Delete soft-deletes; List no longer returns it.
	_, err = f.svc.DeleteWebhook(f.ctx, &uipb.DeleteWebhookRequest{
		TenantId: f.tenantID,
		Id:       &models.Ulid{Value: id},
	})
	require.NoError(t, err)

	list, err = f.svc.ListWebhooks(f.ctx, &uipb.ListWebhooksRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Empty(t, list.GetWebhooks())
}

// Negative: a VIEWER (below the required ADMIN role) is denied.
func TestWebhookRBACDeniesViewer(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_VIEWER)

	_, err := f.svc.CreateWebhook(f.ctx, &uipb.CreateWebhookRequest{
		TenantId: f.tenantID,
		Webhook:  &models.Webhook{Url: "https://hooks.example.com/run"},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// Negative: an anonymous caller (no principal in ctx) is unauthenticated.
func TestWebhookRBACDeniesAnonymous(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	_, err := f.svc.ListWebhooks(context.Background(), &uipb.ListWebhooksRequest{TenantId: f.tenantID})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// Negative: updating a non-existent webhook returns NotFound.
func TestWebhookUpdateNotFound(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	_, err := f.svc.UpdateWebhook(f.ctx, &uipb.UpdateWebhookRequest{
		TenantId: f.tenantID,
		Webhook: &models.Webhook{
			Entity: &models.Entity{Id: &models.Ulid{Value: ids.New()}},
			Url:    "https://hooks.example.com/ghost",
		},
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
