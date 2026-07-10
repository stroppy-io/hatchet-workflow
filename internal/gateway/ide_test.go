package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubIdeAuthorizer struct{ allow bool }

func (s stubIdeAuthorizer) CanAuthor(*http.Request) bool { return s.allow }

func TestGatewayRoutesIdeTrafficToProxyWhenAuthorized(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	g, err := New(Config{TemporalHostPort: "127.0.0.1:0", IdeBackend: backend.URL, IdeAuthorizer: stubIdeAuthorizer{allow: true}})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := gotPath, "/ide/org/acme/"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestGatewayRejectsIdeTrafficWhenUnauthorized(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("backend must not be called when unauthorized")
	}))
	defer backend.Close()

	g, err := New(Config{TemporalHostPort: "127.0.0.1:0", IdeBackend: backend.URL, IdeAuthorizer: stubIdeAuthorizer{allow: false}})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if got, want := rec.Code, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestGatewayIdeRouteNotFoundWhenBackendUnconfigured(t *testing.T) {
	g, err := New(Config{TemporalHostPort: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestGatewayRefusesIdeBackendWithoutAuthorizer(t *testing.T) {
	// /ide/* proxies into a code-server holding a live worktree of a catalog
	// repo. Serving it with no authorizer would let any caller author any
	// org's (or the instance's) bundles, so New must refuse to start rather
	// than fall open. The previous test here asserted the opposite and
	// enshrined the hole as a "dev/test convention".
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("backend must never be reached without an authorizer")
	}))
	defer backend.Close()

	_, err := New(Config{TemporalHostPort: "127.0.0.1:0", IdeBackend: backend.URL})
	if err == nil {
		t.Fatal("expected New to fail closed when IdeBackend is set without IdeAuthorizer")
	}
	if !strings.Contains(err.Error(), "without IdeAuthorizer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type stubIdeBackendResolver struct {
	target string
	err    error
}

func (s stubIdeBackendResolver) Backend(*http.Request) (string, error) { return s.target, s.err }

// TestGatewayRoutesIdeTrafficViaBackendResolver_WhenSet proves IdeBackends
// (Task 4's per-org multiplexing) takes priority over the static IdeBackend
// field and that different requests can be routed to different backends —
// exactly the seam T1-T3's report said Task 4 would need to add.
func TestGatewayRoutesIdeTrafficViaBackendResolver_WhenSet(t *testing.T) {
	var gotOrgAPath, gotOrgBPath string
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrgAPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backendA.Close()
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrgBPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backendB.Close()

	resolver := ideBackendByPrefix{
		"/ide/org/org-a/": backendA.URL,
		"/ide/org/org-b/": backendB.URL,
	}

	g, err := New(Config{
		TemporalHostPort: "127.0.0.1:0",
		IdeBackend:       "http://this-must-be-ignored.invalid", // IdeBackends takes priority
		IdeBackends:      resolver,
		IdeAuthorizer:    stubIdeAuthorizer{allow: true},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/org-a/file.yaml", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("org-a status = %d", rec.Code)
	}
	if gotOrgAPath != "/ide/org/org-a/file.yaml" || gotOrgBPath != "" {
		t.Fatalf("org-a request reached the wrong backend: A=%q B=%q", gotOrgAPath, gotOrgBPath)
	}

	rec2 := httptest.NewRecorder()
	g.serveHTTP(rec2, httptest.NewRequest(http.MethodGet, "/ide/org/org-b/file.yaml", nil))
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("org-b status = %d", rec2.Code)
	}
	if gotOrgBPath != "/ide/org/org-b/file.yaml" {
		t.Fatalf("org-b request never reached its own backend: got %q", gotOrgBPath)
	}
}

// TestGatewayIdeBackendResolver_ErrorIsServiceUnavailable proves a resolver
// failure (e.g. the org's code-server container could not be started) never
// falls through to any default backend — it 503s.
func TestGatewayIdeBackendResolver_ErrorIsServiceUnavailable(t *testing.T) {
	g, err := New(Config{
		TemporalHostPort: "127.0.0.1:0",
		IdeBackends:      stubIdeBackendResolver{err: errBackendUnavailable},
		IdeAuthorizer:    stubIdeAuthorizer{allow: true},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}
	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

var errBackendUnavailable = errors.New("backend unavailable")

// stubIdeTicketExchanger is a scriptable IdeTicketExchanger for the gateway
// tests below — it does not parse real tickets, only asserts the gateway
// wires Exchange's return values through to the HTTP response correctly
// (the actual ticket/scope validation is covered by internal/ide's own
// tests against the real identity.IdeTicketService).
type stubIdeTicketExchanger struct {
	cookie      *http.Cookie
	redirectURL string
	ok          bool
	err         error
	called      int
}

func (s *stubIdeTicketExchanger) Exchange(*http.Request) (*http.Cookie, string, bool, error) {
	s.called++
	return s.cookie, s.redirectURL, s.ok, s.err
}

// TestGatewayIdeTicket_ValidExchange_SetsCookieAndRedirects proves the
// happy path: a request with ok=true from the exchanger gets the cookie
// set on the response and a 302 to the clean URL — the backend is never
// hit on this same request (the browser makes a SECOND request, without
// the ticket, that then proxies normally).
func TestGatewayIdeTicket_ValidExchange_SetsCookieAndRedirects(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("backend must not be reached on the ticket-exchange request itself")
	}))
	defer backend.Close()

	exch := &stubIdeTicketExchanger{
		cookie:      &http.Cookie{Name: "stroppy_ide_session", Value: "sess-token", Path: "/ide/instance/provider/docker", HttpOnly: true},
		redirectURL: "/ide/instance/provider/docker/",
		ok:          true,
	}
	g, err := New(Config{
		TemporalHostPort:   "127.0.0.1:0",
		IdeBackend:         backend.URL,
		IdeAuthorizer:      stubIdeAuthorizer{allow: false}, // must not even be consulted on this request
		IdeTicketExchanger: exch,
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/?ticket=abc", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "/ide/instance/provider/docker/" {
		t.Fatalf("Location = %q", got)
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "stroppy_ide_session" && c.Value == "sess-token" && c.HttpOnly {
			found = true
		}
	}
	if !found {
		t.Fatal("session cookie was not set on the redirect response")
	}
	if exch.called != 1 {
		t.Fatalf("exchanger called %d times, want 1", exch.called)
	}
}

// TestGatewayIdeTicket_InvalidTicket_FailsClosedNeverRedirects proves an
// expired/replayed/cross-scope ticket (Exchange returning an error) is a
// 403 — never a redirect, and never a cookie, closing off the possibility
// of a redirect loop or a forged-scope cookie reaching the browser.
func TestGatewayIdeTicket_InvalidTicket_FailsClosedNeverRedirects(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("backend must not be reached with an invalid ticket")
	}))
	defer backend.Close()

	exch := &stubIdeTicketExchanger{err: errors.New("ticket scope mismatch")}
	g, err := New(Config{
		TemporalHostPort:   "127.0.0.1:0",
		IdeBackend:         backend.URL,
		IdeAuthorizer:      stubIdeAuthorizer{allow: true},
		IdeTicketExchanger: exch,
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/provider/pg/?ticket=bad", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("must not set a Location header on a rejected ticket, got %q", loc)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("must not set any cookie on a rejected ticket")
	}
}

// TestGatewayIdeTicket_NoTicketParam_FallsThroughToAuthorizer proves a
// request with no ticket parameter (the normal case once a session cookie
// or Authorization header is already established) is unaffected by the
// exchanger being configured — it falls straight through to IdeAuthorizer
// exactly as before this handshake existed.
func TestGatewayIdeTicket_NoTicketParam_FallsThroughToAuthorizer(t *testing.T) {
	var reached bool
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	exch := &stubIdeTicketExchanger{ok: false} // "no ticket present"
	g, err := New(Config{
		TemporalHostPort:   "127.0.0.1:0",
		IdeBackend:         backend.URL,
		IdeAuthorizer:      stubIdeAuthorizer{allow: true},
		IdeTicketExchanger: exch,
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/", nil))
	if rec.Code != http.StatusNoContent || !reached {
		t.Fatalf("expected the request to reach the backend via IdeAuthorizer, status=%d reached=%v", rec.Code, reached)
	}
}

// TestGatewayIdeWebsocketUpgrade_CarriesCookieToAuthorizer proves a
// websocket-upgrade shaped request (the shape code-server's live editing
// socket arrives as) that carries a Cookie header reaches IdeAuthorizer
// with that cookie intact — the gateway does no special-casing for
// Upgrade requests before the authorizer check, so the browser's
// automatically-attached session cookie authenticates the socket exactly
// like any other proxied request.
func TestGatewayIdeWebsocketUpgrade_CarriesCookieToAuthorizer(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer backend.Close()

	seenCookie := &recordingAuthorizer{}
	g, err := New(Config{
		TemporalHostPort: "127.0.0.1:0",
		IdeBackend:       backend.URL,
		IdeAuthorizer:    seenCookie,
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/socket", nil)
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "websocket")
	r.AddCookie(&http.Cookie{Name: "stroppy_ide_session", Value: "sess-token"})

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, r)
	if seenCookie.cookieValue != "sess-token" {
		t.Fatalf("IdeAuthorizer did not see the session cookie on the upgrade request, got %q", seenCookie.cookieValue)
	}
}

// recordingAuthorizer records the stroppy_ide_session cookie value it saw
// and always allows, so the proxy step actually runs.
type recordingAuthorizer struct{ cookieValue string }

func (r *recordingAuthorizer) CanAuthor(req *http.Request) bool {
	if c, err := req.Cookie("stroppy_ide_session"); err == nil {
		r.cookieValue = c.Value
	}
	return true
}

// ideBackendByPrefix is a tiny path-prefix-keyed IdeBackendResolver used
// only by this test file, standing in for internal/ide's real resolver
// (which keys by internal/ide.Scope rather than a raw path prefix).
type ideBackendByPrefix map[string]string

func (r ideBackendByPrefix) Backend(req *http.Request) (string, error) {
	for prefix, target := range r {
		if strings.HasPrefix(req.URL.Path, prefix) {
			return target, nil
		}
	}
	return "", errBackendUnavailable
}
