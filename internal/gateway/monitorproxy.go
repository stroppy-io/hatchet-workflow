package gateway

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
)

// newMonitorProxy builds a reverse proxy that relays agent metrics/log traffic
// (the /insert/* ingest paths — vmagent remote_write + vector jsonline — and the
// /select/* query paths) to the internal monitoring backend (vmauth). Cloud VMs
// can only reach the public gateway, not the in-cluster vmauth, so the gateway
// forwards their writes/reads — keeping "agents know ONLY the server address"
// true for monitoring too. The original path (/insert/<accountID>/...) is
// preserved; vmauth routes it.
//
// Agent/collector requests are authenticated with per-agent JWTs. backendToken
// is injected only on the forwarded backend request so vmauth can authenticate
// the server-side relay; it is never accepted as an agent credential.
func newMonitorProxy(backend, backendToken string, agentTokens AgentTokenVerifier) (http.Handler, error) {
	u, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	backendToken = strings.TrimSpace(backendToken)
	if backendToken != "" {
		base := rp.Director
		rp.Director = func(r *http.Request) {
			base(r)
			r.Header.Set("Authorization", "Bearer "+backendToken)
		}
	}
	if agentTokens == nil {
		return rp, nil
	}
	return requireAgentBearer(agentTokens, rp), nil
}

func requireAgentBearer(agentTokens AgentTokenVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentClaimsFromBearer(r.Header.Get("Authorization"), agentTokens)
		if !ok {
			http.Error(w, "invalid agent token", http.StatusUnauthorized)
			return
		}
		if err := bindMonitorLabels(r, claims); err != nil {
			http.Error(w, fmt.Sprintf("invalid monitoring payload: %v", err), http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bindMonitorLabels(r *http.Request, claims *agentdomain.TokenClaims) error {
	if r == nil || r.Body == nil || !strings.HasPrefix(r.URL.Path, "/insert/") {
		return nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	_ = r.Body.Close()
	if len(body) == 0 {
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = 0
		return nil
	}
	rewritten, err := rewriteMonitorPayload(r.URL.Path, body, claims)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewReader(rewritten))
	r.ContentLength = int64(len(rewritten))
	r.Header.Set("Content-Length", fmt.Sprintf("%d", len(rewritten)))
	return nil
}
