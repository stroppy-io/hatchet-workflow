package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/ide"
)

// TestIdeTicketHandler_RejectsOffOriginTargets is the regression guard for the
// ticket-exfiltration hole: the ticket is authorized against targetURL.Path
// alone, so an absolute or protocol-relative target used to pass that check and
// still hand the SPA a URL pointing at another origin — with a live, single-use
// ticket bound to this account and scope in its query string. That origin could
// then redeem the ticket itself.
//
// The handler must reject such a target BEFORE it ever mints one, which is why
// a bare TicketIssuer (no authorizer, no minter) suffices here: reaching Issue
// at all would panic, so a passing test proves rejection happened first.
func TestIdeTicketHandler_RejectsOffOriginTargets(t *testing.T) {
	h := ideTicketHandler(&ide.TicketIssuer{})

	for _, target := range []string{
		"https://evil.example/ide/instance/provider/yandex/",
		"http://evil.example/ide/instance/provider/yandex/",
		"//evil.example/ide/instance/provider/yandex/",
		"https://user:pw@evil.example/ide/instance/provider/yandex/",
		"/etc/passwd",
		"/grafana/",
		"ide/instance/provider/yandex/",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/ide/ticket?target="+target, nil)
		h(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("target %q: status = %d, want %d (body %q)",
				target, rec.Code, http.StatusBadRequest, strings.TrimSpace(rec.Body.String()))
		}
		if strings.Contains(rec.Body.String(), "ticket") {
			t.Errorf("target %q: response mentions a ticket for an off-origin target", target)
		}
	}
}
