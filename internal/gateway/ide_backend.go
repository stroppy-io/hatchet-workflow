package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// IdeBackendResolver resolves the code-server backend an /ide/* request
// should be proxied to, keyed by the request itself (in practice, by the
// scope encoded in r.URL.Path — see internal/ide.Scope/ParseScope). This is
// the per-org multiplexing seam T1-T3's report flagged as needed once
// code-server ships as one process per org rather than a single static
// backend (spec §9.2's decided "shared per-tenant code-server" — many
// backends, not one). Config.IdeBackends is optional: when unset, Config.
// IdeBackend's single static-backend behavior is unchanged (see New()).
type IdeBackendResolver interface {
	// Backend returns the base URL ("http://host:port") to proxy r to. An
	// error means the target backend could not be resolved/started — surfaced
	// as 503, never silently proxied to some default backend.
	Backend(r *http.Request) (string, error)
}

// newDynamicIdeProxy builds an http.Handler that resolves a fresh backend
// per request via resolver, then reverse-proxies to it. Unlike
// newMonitorProxy (one fixed backend, one long-lived *httputil.
// ReverseProxy), a new ReverseProxy is constructed per request here because
// the target genuinely varies per request — resolver.Backend is expected to
// be cheap for the already-running case (Manager.EnsureRunning's docker
// ContainerInspect fast path) and only slow the first time a given org's
// container needs to start.
func newDynamicIdeProxy(resolver IdeBackendResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target, err := resolver.Backend(r)
		if err != nil {
			http.Error(w, "ide backend unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		u, err := url.Parse(target)
		if err != nil {
			http.Error(w, "ide backend misconfigured", http.StatusInternalServerError)
			return
		}
		httputil.NewSingleHostReverseProxy(u).ServeHTTP(w, r)
	})
}
