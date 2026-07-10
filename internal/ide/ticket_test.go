package ide

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/identity"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// realTickets builds a real identity.IdeTicketService — used instead of a
// fake so these tests exercise the actual JWT signing/verification the
// production wiring uses, not a stub that could silently diverge from it.
func realTickets(t *testing.T) *identity.IdeTicketService {
	t.Helper()
	svc, err := identity.NewIdeTicketService(identity.Config{SigningSecret: "test-signing-secret-abc123"})
	if err != nil {
		t.Fatalf("ticket service: %v", err)
	}
	return svc
}

func adminAuthorizer(tokens TokenVerifier, sessions SessionVerifier) *Authorizer {
	return &Authorizer{Tokens: tokens, Sessions: sessions}
}

func TestTicketIssuer_MintsOnlyWhenCanAuthorWouldPass(t *testing.T) {
	tickets := realTickets(t)
	tv := fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{
		"admin-tok": {AccountId: "acc-1", IsAdmin: true},
		"user-tok":  {AccountId: "acc-2", IsAdmin: false},
	}}
	issuer := &TicketIssuer{Authorizer: adminAuthorizer(tv, nil), Tickets: tickets}

	if _, _, err := issuer.Issue(req("/", "admin-tok"), "/ide/instance/provider/docker/"); err != nil {
		t.Fatalf("platform admin ticket mint must succeed: %v", err)
	}
	if _, _, err := issuer.Issue(req("/", "user-tok"), "/ide/instance/provider/docker/"); err == nil {
		t.Fatal("a caller CanAuthor would reject must not receive a ticket")
	}
	if _, _, err := issuer.Issue(req("/", ""), "/ide/instance/provider/docker/"); err == nil {
		t.Fatal("a request with no bearer token must not receive a ticket")
	}
}

func TestTicketExchanger_ExchangesValidTicketForScopedCookie(t *testing.T) {
	tickets := realTickets(t)
	ticket, err := tickets.MintTicket("acc-1", "instance:provider:docker")
	if err != nil {
		t.Fatalf("MintTicket: %v", err)
	}
	ex := &TicketExchanger{Tickets: tickets}
	r := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/?ticket="+ticket, nil)
	cookie, redirectURL, ok, err := ex.Exchange(r)
	if err != nil || !ok {
		t.Fatalf("Exchange: ok=%v err=%v", ok, err)
	}
	if cookie.Name != ideSessionCookieName {
		t.Fatalf("cookie name = %q", cookie.Name)
	}
	if cookie.Path != "/ide/instance/provider/docker" {
		t.Fatalf("cookie path = %q", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie must be HttpOnly")
	}
	if redirectURL != "/ide/instance/provider/docker/" {
		t.Fatalf("redirect url = %q", redirectURL)
	}

	// The cookie now authorizes exactly this scope via Authorizer.
	a := adminAuthorizer(nil, tickets)
	assertReq := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/", nil)
	assertReq.AddCookie(cookie)
	if !a.CanAuthor(assertReq) {
		t.Fatal("a freshly exchanged session cookie must authorize its own scope")
	}
}

func TestTicketExchanger_NoTicketParam_IsNoop(t *testing.T) {
	ex := &TicketExchanger{Tickets: realTickets(t)}
	r := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/", nil)
	_, _, ok, err := ex.Exchange(r)
	if ok || err != nil {
		t.Fatalf("no ticket param must be a no-op: ok=%v err=%v", ok, err)
	}
}

func TestTicketExchanger_RejectsTicketReplay(t *testing.T) {
	tickets := realTickets(t)
	ticket, _ := tickets.MintTicket("acc-1", "instance:provider:docker")
	ex := &TicketExchanger{Tickets: tickets}
	r1 := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/?ticket="+ticket, nil)
	if _, _, ok, err := ex.Exchange(r1); !ok || err != nil {
		t.Fatalf("first exchange must succeed: ok=%v err=%v", ok, err)
	}
	r2 := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/?ticket="+ticket, nil)
	if _, _, ok, err := ex.Exchange(r2); ok || err == nil {
		t.Fatal("replaying the same ticket must be rejected (single-use)")
	}
}

// TestTicketExchanger_RejectsCrossScopeTicket proves a ticket minted for
// one scope cannot open a DIFFERENT scope: an instance-scoped ticket
// against an org URL, an org-A ticket against org-B, and an org-A
// provider ticket against org-A's workflow entry.
func TestTicketExchanger_RejectsCrossScopeTicket(t *testing.T) {
	cases := []struct {
		name        string
		ticketScope string
		requestPath string
	}{
		{"instance ticket against org scope", "instance:provider:docker", "/ide/org/acme/provider/docker/"},
		{"org-A ticket against org-B", "org:acme:provider:pg", "/ide/org/beta/provider/pg/"},
		{"provider ticket against workflow entry, same org", "org:acme:provider:pg", "/ide/org/acme/workflow/pg/"},
		{"org ticket against instance scope", "org:acme:provider:pg", "/ide/instance/provider/pg/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tickets := realTickets(t)
			ticket, err := tickets.MintTicket("acc-1", tc.ticketScope)
			if err != nil {
				t.Fatalf("MintTicket: %v", err)
			}
			ex := &TicketExchanger{Tickets: tickets}
			r := httptest.NewRequest(http.MethodGet, tc.requestPath+"?ticket="+ticket, nil)
			_, _, ok, err := ex.Exchange(r)
			if ok || err == nil {
				t.Fatalf("a ticket for scope %q must not exchange for a session on %q", tc.ticketScope, tc.requestPath)
			}
		})
	}
}

func TestAuthorizer_CanAuthor_CookieScopedToDifferentEntry_IsRejected(t *testing.T) {
	tickets := realTickets(t)
	session, err := tickets.MintSession("acc-1", "org:acme:provider:pg")
	if err != nil {
		t.Fatalf("MintSession: %v", err)
	}
	a := adminAuthorizer(nil, tickets)

	r := httptest.NewRequest(http.MethodGet, "/ide/org/acme/provider/OTHER/", nil)
	r.AddCookie(&http.Cookie{Name: ideSessionCookieName, Value: session})
	if a.CanAuthor(r) {
		t.Fatal("a session cookie scoped to one entry must not authorize a different entry")
	}

	r2 := httptest.NewRequest(http.MethodGet, "/ide/org/beta/provider/pg/", nil)
	r2.AddCookie(&http.Cookie{Name: ideSessionCookieName, Value: session})
	if a.CanAuthor(r2) {
		t.Fatal("a session cookie scoped to org acme must not authorize org beta")
	}
}

func TestAuthorizer_CanAuthor_ExpiredOrGarbageCookie_FailsClosed(t *testing.T) {
	a := adminAuthorizer(nil, realTickets(t))
	r := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/", nil)
	r.AddCookie(&http.Cookie{Name: ideSessionCookieName, Value: "not-a-jwt"})
	if a.CanAuthor(r) {
		t.Fatal("a garbage cookie must not authorize")
	}
}

// TestAuthorizer_CanAuthor_WebsocketUpgradeRequest proves the cookie-based
// authorization path is not header/content-negotiation specific: a
// websocket upgrade request (the shape code-server's live editing socket
// arrives as) carrying the same cookie a normal GET would is authorized
// identically. The gateway performs no special-casing for Upgrade
// requests (see gateway.go's serveHTTP), so this is really testing that
// nothing about CanAuthor implicitly assumes a non-upgrade request.
func TestAuthorizer_CanAuthor_WebsocketUpgradeRequest(t *testing.T) {
	tickets := realTickets(t)
	session, err := tickets.MintSession("acc-1", "instance:provider:docker")
	if err != nil {
		t.Fatalf("MintSession: %v", err)
	}
	a := adminAuthorizer(nil, tickets)
	r := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/", nil)
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "websocket")
	r.AddCookie(&http.Cookie{Name: ideSessionCookieName, Value: session})
	if !a.CanAuthor(r) {
		t.Fatal("a websocket upgrade request carrying a valid session cookie must be authorized")
	}
}

func TestAuthorizer_CanAuthor_HeaderPathStillWorksWhenSessionsConfigured(t *testing.T) {
	tv := fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"admin-tok": {AccountId: "acc-1", IsAdmin: true}}}
	a := adminAuthorizer(tv, realTickets(t))
	if !a.CanAuthor(req("/ide/instance/provider/docker/", "admin-tok")) {
		t.Fatal("the Authorization-header path must keep working once Sessions is configured")
	}
}
