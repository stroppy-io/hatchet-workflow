package webhookworker_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
	"go.uber.org/zap"

	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/webhookworker"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func testHmacSign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookWorker_DeliverAndVerifySignature(t *testing.T) {
	f := fixture.NewIAM(t)
	exec := f.F.Executor.(*sqlexec.TxExecutor)
	svc := opssvc.NewWebhookService(exec, f.F.TxMgr, f.F.Events)
	ctx := context.Background()

	uid := fmt.Sprintf("ww-%s", t.Name())
	u, err := f.IAM.CreateUser(ctx, &iampb.User{
		Email:    uid + "@test.com",
		Nickname: uid,
	}, "P@ss1234!")
	require.NoError(t, err)
	tn, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{
		Identity: &commonpb.Identity{Name: "WWTest-" + uid},
	}, u.GetId())
	require.NoError(t, err)

	var receivedSig string
	var receivedBody bytes.Buffer
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Stroppy-Signature")
		receivedBody.ReadFrom(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(recv.Close)

	secret := "test-webhook-secret"
	wh, err := svc.CreateWebhook(ctx, tn.GetId(), u.GetId(), &opspb.Webhook{
		Url:        recv.URL,
		Enabled:    true,
		Secret:     secret,
		MaxRetries: 3,
		Identity:   &commonpb.Identity{Name: "ww-hook"},
	})
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]string{"event": "test.run.done"})
	err = svc.EnqueueDelivery(ctx, wh.GetId().GetValue(), opspb.WebhookEvent_WEBHOOK_EVENT_RUN_COMPLETED, payload)
	require.NoError(t, err)

	worker := webhookworker.New(svc, zap.NewNop(), 0)
	err = worker.TickOnce(ctx)
	require.NoError(t, err)

	require.NotEmpty(t, receivedBody.String())
	expectedSig := "sha256=" + testHmacSign([]byte(secret), payload)
	require.Equal(t, expectedSig, receivedSig)
}
