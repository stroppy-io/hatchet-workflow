package ide

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ideSessionCookieName is the httpOnly cookie the browser-auth handshake
// (TicketExchanger) sets and Authorizer.verifiedSession reads. One name is
// shared across every scope: the cookie's Path attribute (Scope.Prefix)
// keeps the browser from ever sending an org-acme cookie on an org-beta (or
// instance) request, and Authorizer independently re-checks the cookie's
// own scope claim against the request's parsed scope — see CanAuthor's doc
// for why both layers matter.
const ideSessionCookieName = "stroppy_ide_session"

// TicketMinter mints a short-lived, single-use ticket binding (accountID,
// scope) — the identity.IdeTicketService shape, narrowed so tests can fake
// it without pulling in identity's full dependency surface (same pattern
// as TokenVerifier/PermissionResolver/TenantResolver in authorizer.go).
type TicketMinter interface {
	MintTicket(accountID, scope string) (string, error)
}

// TicketConsumer validates+consumes a ticket (single-use) and mints the
// longer-lived session token the exchanged cookie carries.
type TicketConsumer interface {
	ConsumeTicket(ctx context.Context, ticket string) (accountID, scope string, err error)
	MintSession(accountID, scope string) (string, error)
}

// SessionVerifier validates a session cookie token minted by
// TicketConsumer.MintSession. Authorizer holds one (optional: nil disables
// cookie-based auth, leaving only the Authorization-header path — e.g. in
// tests that don't exercise the browser-auth handshake).
type SessionVerifier interface {
	VerifySession(ctx context.Context, token string) (accountID, scope string, err error)
}

// TicketIssuer mints an ide_ticket for a browser navigation. It is the
// authenticated half of the handshake: called from a normal XHR/fetch that
// CAN carry the SPA's Bearer access token (unlike the subsequent browser
// navigation to /ide/<scope>/..., which cannot set custom headers). It
// reuses Authorizer.CanAuthor byte-for-byte — a ticket is mintable if and
// only if the identical RBAC check gateway.serveHTTP already enforces on
// every direct Authorization-header request would pass, so there is no
// second, parallel authorization decision anywhere in this handshake.
type TicketIssuer struct {
	Authorizer *Authorizer
	Tickets    TicketMinter
}

// Issue authorizes r's Bearer token against targetPath (an "/ide/..."
// path — the SAME path form ParseScope/Authorizer.CanAuthor already parse)
// and, only on success, mints a ticket bound to (caller's accountID,
// targetPath's scope key). r itself is never proxied anywhere; it supplies
// only the Authorization header and request context — a shallow probe
// request pointed at targetPath is what CanAuthor actually evaluates, so
// the mint endpoint's own URL is irrelevant to the authorization decision.
func (i *TicketIssuer) Issue(r *http.Request, targetPath string) (ticket string, scope Scope, err error) {
	if i == nil || i.Authorizer == nil || i.Tickets == nil {
		return "", Scope{}, fmt.Errorf("ide: ticket issuer not configured")
	}
	scope, err = ParseScope(targetPath)
	if err != nil {
		return "", Scope{}, err
	}
	probe := r.Clone(r.Context())
	probe.URL = &url.URL{Path: targetPath}
	claims, ok := i.Authorizer.verifiedClaims(probe)
	if !ok {
		return "", Scope{}, fmt.Errorf("ide: missing or invalid bearer token")
	}
	if !i.Authorizer.CanAuthor(probe) {
		return "", Scope{}, fmt.Errorf("ide: not authorized to author %q", targetPath)
	}
	ticket, err = i.Tickets.MintTicket(claims.GetAccountId(), scope.Key())
	if err != nil {
		return "", Scope{}, err
	}
	return ticket, scope, nil
}

// TicketExchanger implements gateway.IdeTicketExchanger: it turns a
// single-use ticket query parameter into an httpOnly, scope-bound session
// cookie plus a clean redirect URL.
type TicketExchanger struct {
	Tickets TicketConsumer
}

// Exchange implements gateway.IdeTicketExchanger. ok=false, err=nil means
// "r carries no ticket parameter" (nothing to do — the caller falls
// through to normal Authorization-header/cookie authorization). Any error
// (bad signature, expired, already used, wrong scope) is the caller's
// signal to fail closed with 403 — never a redirect, so a replayed or
// cross-scope ticket can never coax a redirect loop or a cookie out of the
// gateway.
func (e *TicketExchanger) Exchange(r *http.Request) (cookie *http.Cookie, redirectURL string, ok bool, err error) {
	if e == nil || e.Tickets == nil {
		return nil, "", false, nil
	}
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		return nil, "", false, nil
	}
	scope, err := ParseScope(r.URL.Path)
	if err != nil {
		return nil, "", false, err
	}
	accountID, ticketScope, err := e.Tickets.ConsumeTicket(r.Context(), ticket)
	if err != nil {
		return nil, "", false, err
	}
	if accountID == "" || ticketScope != scope.Key() {
		// A ticket minted for a DIFFERENT scope (a different org, a
		// different entry, instance vs. org) must never open THIS scope —
		// this is the one check standing between "ticket leaked/replayed
		// against a different URL" and an actual cross-tenant IDE session.
		// The ticket is already consumed above (ConsumeTicket is
		// single-use) even on this rejection path — a wrong-scope ticket
		// does not get a second chance either.
		return nil, "", false, fmt.Errorf("ide: ticket scope %q does not match request scope %q", ticketScope, scope.Key())
	}
	session, err := e.Tickets.MintSession(accountID, scope.Key())
	if err != nil {
		return nil, "", false, err
	}
	// Secure is set whenever the request itself arrived over TLS, OR a
	// trusted reverse proxy (Caddy, the deployment's TLS terminator) marked
	// it as such via X-Forwarded-Proto. The dev/stage stands run the
	// gateway as plain HTTP behind Caddy — Secure MUST NOT be hard-coded
	// true there, or the browser silently drops the cookie and every
	// request after the redirect 403s. It also MUST NOT be hard-coded
	// false: shipping that as the default would silently downgrade a real
	// TLS deployment's cookie to non-Secure. Deriving it per-request from
	// the actual scheme is the only choice that is correct in both cases
	// without a new config flag.
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	cookie = &http.Cookie{
		Name:     ideSessionCookieName,
		Value:    session,
		Path:     scope.Prefix(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	q := r.URL.Query()
	q.Del("ticket")
	redirectURL = scope.Prefix() + scope.Rest
	if encoded := q.Encode(); encoded != "" {
		redirectURL += "?" + encoded
	}
	return cookie, redirectURL, true, nil
}
