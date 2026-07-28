package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

/*
	Public (share-link) metrics access.

	A share link lets someone OUTSIDE the system look at ONE run. To show live
	Grafana panels to that person we must guarantee they can only ever read that
	one run's series — no other run, no other tenant, no arbitrary PromQL.

	The enforcement point is this proxy, NOT Grafana and NOT the dashboard URL
	variables (a viewer can edit those freely). Grafana's public organisation
	holds a single datasource whose URL points here; Grafana forwards the
	viewer's `stroppy_share` cookie to us (datasource `keepCookies`). On every
	request we:

	  1. resolve the cookie -> share -> run id (fail closed: no cookie, unknown,
	     revoked or expired token => 404, indistinguishable),
	  2. DROP any client-supplied extra_filters[] / extra_label,
	  3. force our own `extra_label=stroppy_run_id=<run>`,
	  4. clamp the time window to the run's own window,
	  5. allow only VictoriaMetrics read endpoints.

	Step 2 is load-bearing: VictoriaMetrics OR-s multiple `extra_filters[]`
	together, so a client-supplied one would widen the selection back out to
	every run. Multiple `extra_label`s are AND-ed (they can only narrow), but we
	strip those too rather than reason about it. Because the filter is applied
	inside VictoriaMetrics we never parse or rewrite PromQL: a query that pins a
	foreign stroppy_run_id simply yields no rows.
*/

// shareCookieName is the cookie the share page sets and Grafana forwards to the
// datasource (see the public datasource's jsonData.keepCookies).
const shareCookieName = "stroppy_share"

// scopeParams are the query/form parameters a client must never control on the
// public metrics path: they decide WHICH series the query may see.
var scopeParams = []string{"extra_label", "extra_filters", "extra_filters[]"}

// timeParams are clamped to the shared run's window.
var timeParams = []string{"start", "end", "time"}

// ShareScope is the run a public share token grants read access to.
type ShareScope struct {
	// RunID is the stroppy_run_id every query is narrowed to.
	RunID string
	// From/To bound the run. Zero From means unbounded; zero To means the run is
	// still going, so the window stays open at the top.
	From time.Time
	To   time.Time
}

// ShareResolver turns a public share token into the run it exposes. It MUST
// return an error for unknown, revoked and expired tokens alike, so the caller
// cannot tell them apart.
type ShareResolver interface {
	ResolveShare(ctx context.Context, token string) (ShareScope, error)
}

// publicMetricsPathAllowed reports whether rel (the path after
// /public/metrics/) is a VictoriaMetrics read endpoint we expose to share
// viewers. Everything else — writes, exports, admin — is refused.
func publicMetricsPathAllowed(rel string) bool {
	switch rel {
	case "api/v1/query",
		"api/v1/query_range",
		"api/v1/series",
		"api/v1/labels",
		// Grafana's Prometheus datasource probes buildinfo to pick its flavour.
		"api/v1/status/buildinfo":
		return true
	}
	// api/v1/label/<name>/values
	if rest, ok := strings.CutPrefix(rel, "api/v1/label/"); ok {
		name, ok := strings.CutSuffix(rest, "/values")
		return ok && name != "" && !strings.Contains(name, "/")
	}
	return false
}

// clampTime pins t into the run window. A zero bound means "unbounded".
func clampTime(t time.Time, scope ShareScope) time.Time {
	if !scope.From.IsZero() && t.Before(scope.From) {
		return scope.From
	}
	if !scope.To.IsZero() && t.After(scope.To) {
		return scope.To
	}
	return t
}

// parseVMTime accepts the two forms VictoriaMetrics takes: unix seconds (float)
// and RFC3339. Anything else is left untouched by the caller.
func parseVMTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	if secs, err := strconv.ParseFloat(raw, 64); err == nil {
		sec, frac := int64(secs), secs-float64(int64(secs))
		return time.Unix(sec, int64(frac*float64(time.Second))).UTC(), true
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC(), true
	}
	return time.Time{}, false
}

func formatVMTime(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

// scopeValues strips every client-controlled scoping parameter, forces our own
// run filter, and clamps the time window. It is the whole security contract of
// this file, applied identically to the query string and to a form body.
func scopeValues(v url.Values, scope ShareScope) {
	for key := range v {
		for _, banned := range scopeParams {
			if strings.EqualFold(key, banned) {
				delete(v, key)
			}
		}
	}
	v.Set("extra_label", promRunLabel+"="+scope.RunID)

	for _, key := range timeParams {
		raw := v.Get(key)
		if raw == "" {
			continue
		}
		if ts, ok := parseVMTime(raw); ok {
			v.Set(key, formatVMTime(clampTime(ts, scope)))
		}
	}
}

// maxPublicFormBody caps the form body we are willing to parse+rewrite.
const maxPublicFormBody = 1 << 20 // 1 MiB

// scopeRequest rewrites the request in place so it can only read scope.RunID.
// Grafana's Prometheus datasource sends /api/v1/query_range as a urlencoded
// POST, so the body has to be rewritten too — scoping only the query string
// would leave a hole.
func scopeRequest(r *http.Request, scope ShareScope) error {
	q := r.URL.Query()
	scopeValues(q, scope)
	r.URL.RawQuery = q.Encode()

	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		return nil
	}
	ctype := r.Header.Get("Content-Type")
	if ctype != "" && !strings.HasPrefix(ctype, "application/x-www-form-urlencoded") {
		return fmt.Errorf("unsupported content type %q", ctype)
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPublicFormBody+1))
	_ = r.Body.Close()
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if len(body) > maxPublicFormBody {
		return fmt.Errorf("body too large")
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return fmt.Errorf("parse form: %w", err)
	}
	scopeValues(form, scope)
	encoded := form.Encode()
	r.Body = io.NopCloser(strings.NewReader(encoded))
	r.ContentLength = int64(len(encoded))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return nil
}

// newPublicMetricsProxy relays share-scoped read queries to the metrics backend
// (the same Prometheus-compatible endpoint the authenticated Grafana datasource
// uses). backendToken, when set, authenticates the server-side relay; it is
// never accepted from, nor exposed to, the client.
func newPublicMetricsProxy(backend, backendToken string, shares ShareResolver, runScopeSecret string, log *slog.Logger) (http.Handler, error) {
	u, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	base := rp.Director
	backendToken = strings.TrimSpace(backendToken)
	rp.Director = func(r *http.Request) {
		base(r)
		// Never leak the viewer's cookies or credentials upstream.
		r.Header.Del("Cookie")
		r.Header.Del("Authorization")
		if backendToken != "" {
			r.Header.Set("Authorization", "Bearer "+backendToken)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope, ok := shareScopeFromRequest(r, shares, runScopeSecret)
		if !ok {
			// Fail closed, and stay indistinguishable from an unknown token.
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/public/metrics/")
		if !publicMetricsPathAllowed(rel) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := scopeRequest(r, scope); err != nil {
			log.Warn("public metrics: reject request", "err", err, "path", rel)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		r.URL.Path = "/" + rel
		rp.ServeHTTP(w, r)
	}), nil
}

// shareScopeFromRequest resolves the scope cookie into a run scope. The cookie
// carries EITHER a signed run-scope token (an authenticated in-app viewer, see
// runscope.go) or a public share token; both live under the same cookie name so
// Grafana's single keepCookies entry forwards whichever applies.
func shareScopeFromRequest(r *http.Request, shares ShareResolver, runScopeSecret string) (ShareScope, bool) {
	c, err := r.Cookie(shareCookieName)
	if err != nil || c.Value == "" {
		return ShareScope{}, false
	}
	// Signed run-scope token: an authorised in-app viewer. No window bound — the
	// user picks the dashboard time range freely.
	if runID, ok := verifyRunScope(runScopeSecret, c.Value); ok {
		return ShareScope{RunID: runID}, true
	}
	if shares == nil {
		return ShareScope{}, false
	}
	scope, err := shares.ResolveShare(r.Context(), c.Value)
	if err != nil || scope.RunID == "" {
		return ShareScope{}, false
	}
	return scope, true
}

// shareSessionTTL bounds how long the browser keeps the share cookie when the
// share itself never expires.
const shareSessionTTL = 12 * time.Hour

// publicShareSessionResponse is what the share page needs to build the embed
// URLs: the run the token exposes and the window to pin the dashboards to.
type publicShareSessionResponse struct {
	RunID string `json:"runId"`
	From  int64  `json:"from,omitempty"` // unix millis
	To    int64  `json:"to,omitempty"`   // unix millis; 0 while the run is live
}

// servePublicShareSession handles GET /public/share/{token}/session. It converts
// the token in the URL into an HttpOnly cookie that Grafana forwards to the
// scoped metrics datasource, and returns the run window the dashboards should
// show. An unknown / revoked / expired token 404s like any other.
func (g *Gateway) servePublicShareSession(w http.ResponseWriter, r *http.Request) {
	if g.shareScopes == nil {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/public/share/")
	token, ok := strings.CutSuffix(rest, "/session")
	if !ok || token == "" || strings.Contains(token, "/") {
		http.NotFound(w, r)
		return
	}
	scope, err := g.shareScopes.ResolveShare(r.Context(), token)
	if err != nil || scope.RunID == "" {
		http.NotFound(w, r)
		return
	}

	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     shareCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(shareSessionTTL / time.Second),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	resp := publicShareSessionResponse{RunID: scope.RunID}
	if !scope.From.IsZero() {
		resp.From = scope.From.UnixMilli()
	}
	if !scope.To.IsZero() {
		resp.To = scope.To.UnixMilli()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}
