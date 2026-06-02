package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// newMonitorProxy builds a reverse proxy that relays agent metrics/log traffic
// (the /insert/* ingest paths — vmagent remote_write + vector jsonline — and the
// /select/* query paths) to the internal monitoring backend (vmauth). Cloud VMs
// can only reach the public gateway, not the in-cluster vmauth, so the gateway
// forwards their writes/reads — keeping "agents know ONLY the server address"
// true for monitoring too. The original path (/insert/<accountID>/...) is
// preserved; vmauth routes it.
//
// When token is non-empty it is injected as an Authorization: Bearer header on
// every forwarded request so the gateway authenticates to vmauth on the agent's
// behalf (agents do not hold the monitoring bearer).
func newMonitorProxy(backend, token string) (http.Handler, error) {
	u, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	if token != "" {
		base := rp.Director
		rp.Director = func(r *http.Request) {
			base(r)
			r.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return rp, nil
}
