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
