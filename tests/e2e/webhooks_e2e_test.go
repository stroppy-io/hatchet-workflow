//go:build e2e

package e2e_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

func TestE2E_Webhook_CRUD(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "wh-crud")
	cli := env.WithTenant(tnt.GetValue())

	created, err := cli.Webhook.CreateWebhook(ctx, connect.NewRequest(&opspb.CreateWebhookRequest{
		Webhook: &opspb.Webhook{
			Identity: &commonpb.Identity{Name: "wh-1"},
			Url:      "http://example.invalid/hook",
			Secret:   "shared-secret",
			Enabled:  true,
		},
	}))
	require.NoError(t, err)
	id := created.Msg.GetId()

	got, err := cli.Webhook.GetWebhook(ctx, connect.NewRequest(id))
	require.NoError(t, err)
	require.Equal(t, "wh-1", got.Msg.GetIdentity().GetName())

	upd, err := cli.Webhook.UpdateWebhook(ctx, connect.NewRequest(&opspb.UpdateWebhookRequest{
		Webhook: &opspb.Webhook{
			Id: id, Identity: &commonpb.Identity{Name: "wh-1-renamed"},
			Url: "http://example.invalid/hook", Secret: "shared-secret", Enabled: false,
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "wh-1-renamed", upd.Msg.GetIdentity().GetName())

	lst, err := cli.Webhook.ListWebhooks(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.NotEmpty(t, lst.Msg.GetWebhooks())

	_, err = cli.Webhook.DeleteWebhook(ctx, connect.NewRequest(id))
	require.NoError(t, err)
}

// TestE2E_Webhook_TestWebhook_PostsSigned spins up a sink that captures the
// POST body + headers and asserts the body parses as the test-ping JSON
// envelope AND carries a valid X-Stroppy-Signature HMAC-SHA256 header keyed
// by the webhook's secret.
func TestE2E_Webhook_TestWebhook_PostsSigned(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "wh-sink")
	cli := env.WithTenant(tnt.GetValue())

	var mu sync.Mutex
	captured := struct {
		headers http.Header
		body    []byte
	}{}
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		captured.headers = r.Header.Clone()
		captured.body = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer sink.Close()

	created, err := cli.Webhook.CreateWebhook(ctx, connect.NewRequest(&opspb.CreateWebhookRequest{
		Webhook: &opspb.Webhook{
			Identity: &commonpb.Identity{Name: "sink"},
			Url:      sink.URL,
			Secret:   "sec",
			Enabled:  true,
		},
	}))
	require.NoError(t, err)
	resp, err := cli.Webhook.TestWebhook(ctx, connect.NewRequest(&opspb.TestWebhookRequest{
		Id: created.Msg.GetId(),
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.GetDelivered())

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, "application/json", captured.headers.Get("Content-Type"))
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(captured.body, &envelope))
	require.Equal(t, true, envelope["test"])
	// Signature must be present and verify against the webhook secret.
	sig := captured.headers.Get("X-Stroppy-Signature")
	require.NotEmpty(t, sig, "X-Stroppy-Signature header must be set")
	require.True(t, strings.HasPrefix(sig, "sha256="), "signature must be sha256= prefixed")
	mac := hmac.New(sha256.New, []byte("sec"))
	mac.Write(captured.body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	require.Equal(t, want, sig, "signature must match HMAC-SHA256(secret, body)")
}
