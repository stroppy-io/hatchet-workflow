//go:build integration

package application

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// iamEvent posts a Standard-Webhooks signed IAM event.
func (e *e2e) iamEvent(id, typ string, data map[string]any, secret, project string) int {
	e.t.Helper()
	raw, _ := json.Marshal(data)
	body, _ := json.Marshal(map[string]any{"id": id, "type": typ, "version": 1, "occurred_at": time.Now().UTC(), "project_id": project, "environment": "test", "data": json.RawMessage(raw)})
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s.%d.", id, ts)
	mac.Write(body)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, e.ts.URL+"/webhooks/iam", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Webhook-Id", id)
	req.Header.Set("Webhook-Timestamp", fmt.Sprint(ts))
	req.Header.Set("Webhook-Signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func TestE2EIAMWebhook(t *testing.T) {
	e := e2eServer(t)
	ctx := e.ctx
	person := e.person(slug("alice")+"@example.com", "Alice")

	t.Run("signature and project are enforced", func(t *testing.T) {
		if st := e.iamEvent("evt-bad-sig", "session.revoked", map[string]any{"session_id": "s1", "user_id": person.UserID.String()}, "wrong", "proj-test"); st != http.StatusUnauthorized {
			t.Fatalf("bad signature: %d", st)
		}
		if st := e.iamEvent("evt-bad-proj", "session.revoked", map[string]any{"session_id": "s1", "user_id": person.UserID.String()}, "whsec-test", "other"); st != http.StatusForbidden {
			t.Fatalf("wrong project: %d", st)
		}
		if r := e.req(http.MethodGet, "/webhooks/iam", nil, ""); r.Status != http.StatusMethodNotAllowed {
			t.Fatalf("GET: %d", r.Status)
		}
	})

	t.Run("session.revoked lands on the denylist once", func(t *testing.T) {
		if st := e.iamEvent("evt-1", "session.revoked", map[string]any{"session_id": "sess-1", "user_id": person.UserID.String(), "project_id": "proj-test"}, "whsec-test", "proj-test"); st != http.StatusNoContent {
			t.Fatalf("revoked: %d", st)
		}
		denied, err := e.app.services.IAM.SessionDenied(ctx, "sess-1")
		if err != nil || !denied {
			t.Fatalf("denied=%v err=%v", denied, err)
		}
		// Replay of the same event id is a no-op.
		if st := e.iamEvent("evt-1", "session.revoked", map[string]any{"session_id": "sess-2", "user_id": person.UserID.String(), "project_id": "proj-test"}, "whsec-test", "proj-test"); st != http.StatusNoContent {
			t.Fatalf("replay: %d", st)
		}
		if denied, _ := e.app.services.IAM.SessionDenied(ctx, "sess-2"); denied {
			t.Fatal("replayed event was applied")
		}
	})

	t.Run("email.changed and user.deleted reach the profile", func(t *testing.T) {
		if st := e.iamEvent("evt-2", "email.changed", map[string]any{"user_id": person.UserID.String(), "email": "Alice.New@Example.com"}, "whsec-test", "proj-test"); st != http.StatusNoContent {
			t.Fatalf("email: %d", st)
		}
		p, err := e.app.services.Profiles.Get(ctx, person.UserID)
		if err != nil || p.Email != "alice.new@example.com" {
			t.Fatalf("profile %+v err=%v", p, err)
		}
		if st := e.iamEvent("evt-3", "user.deleted", map[string]any{"user_id": person.UserID.String()}, "whsec-test", "proj-test"); st != http.StatusNoContent {
			t.Fatalf("deleted: %d", st)
		}
		if _, err := e.app.services.Profiles.Get(ctx, person.UserID); err == nil {
			t.Fatal("profile survived user.deleted")
		}
		if st := e.iamEvent("evt-4", "something.else", map[string]any{}, "whsec-test", "proj-test"); st != http.StatusNoContent {
			t.Fatalf("unknown event: %d", st)
		}
		if st := e.iamEvent("evt-5", "session.revoked", map[string]any{"session_id": "x"}, "whsec-test", "proj-test"); st != http.StatusBadRequest {
			t.Fatalf("incomplete data: %d", st)
		}
	})
	_ = uuid.Nil
}
