package gateway

import "net/http"

// IdeAuthorizer gates /ide/* before the request reaches code-server — spec
// docs/superpowers/specs/2026-07-08-sp-c-internal-git-ide.md §3 C4's
// "RBAC-проверка на границе гейтвея, не внутри code-server". SP-C cuts this
// interface; SP-B supplies the real RBAC-backed implementation
// (instance-admin/org-admin roles resolved from the request's authenticated
// principal + the repo scope encoded in the request path). nil disables the
// check (dev/test only), mirroring AgentTokenVerifier's existing nil-disables
// convention in this same package (see auth.go).
type IdeAuthorizer interface {
	// CanAuthor reports whether the request's authenticated principal may
	// open an IDE session against the target repo scope encoded in the
	// request path (r.URL.Path, e.g. "/ide/org/acme/..."). Returning false is
	// a 403, never a panic/error — an authorizer implementation error must
	// fail closed, decided by the implementation, not this seam.
	CanAuthor(r *http.Request) bool
}

// IdeTicketExchanger performs the browser-auth handshake's second step: a
// browser navigating to /ide/<scope>/...?ticket=... cannot carry the SPA's
// Bearer token (a plain navigation cannot set custom headers), so a
// short-lived single-use ticket stands in for it — minted by an
// authenticated API call before the navigation happens (see
// internal/ide.TicketIssuer), then exchanged here for an httpOnly session
// cookie the browser attaches to every subsequent /ide/<scope>/... request
// (assets, the websocket code-server needs) automatically. nil disables
// ticket-based auth entirely; /ide/* then falls back to IdeAuthorizer's
// Authorization-header check only, unaffected (curl/API callers, and every
// test that predates this handshake).
type IdeTicketExchanger interface {
	// Exchange inspects r for a ticket query parameter targeting r's own
	// scope-encoded path.
	//   - ok=false, err=nil: r carries no ticket — nothing to exchange,
	//     caller falls through to IdeAuthorizer.
	//   - ok=true, err=nil: the ticket was valid for r's exact scope; cookie
	//     is the session cookie to set on the response and redirectURL is
	//     the clean post-exchange URL (ticket parameter stripped) to
	//     redirect the browser to.
	//   - err != nil: the ticket was present but invalid, expired, already
	//     used, or scoped to a DIFFERENT scope than r's path. The caller
	//     MUST fail closed (403), never redirect — a bad ticket must never
	//     produce a cookie or a redirect loop.
	Exchange(r *http.Request) (cookie *http.Cookie, redirectURL string, ok bool, err error)
}
