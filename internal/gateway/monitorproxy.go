package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// newMonitorProxy builds a reverse proxy that relays agent metrics/log ingest
// (the /insert/* paths: vmagent remote_write + vector jsonline) to the internal
// monitoring backend (vmauth). Cloud VMs can only reach the public gateway, not
// the in-cluster vmauth, so the gateway forwards their writes — keeping "agents
// know ONLY the server address" true for monitoring too. The original path
// (/insert/<accountID>/...) and auth headers are preserved; vmauth routes them.
func newMonitorProxy(backend string) (http.Handler, error) {
	u, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(u), nil
}
