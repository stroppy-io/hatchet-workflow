package gateway

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	testRun   = "run-aaa"
	otherRun  = "run-bbb"
	testToken = "tok-123"
)

type stubShares struct {
	scope ShareScope
	err   error
}

func (s stubShares) ResolveShare(_ context.Context, token string) (ShareScope, error) {
	if s.err != nil || token != testToken {
		return ShareScope{}, errors.New("no scope")
	}
	return s.scope, nil
}

func liveScope() ShareScope {
	return ShareScope{
		RunID: testRun,
		From:  time.Unix(1000, 0).UTC(),
		To:    time.Unix(2000, 0).UTC(),
	}
}

// scopeValues is the whole security contract: whatever the client sent, the
// forwarded request must carry exactly one run filter — ours.
func TestScopeValuesForcesOurRunFilter(t *testing.T) {
	cases := map[string]url.Values{
		"clean":                        {"query": {"up"}},
		"client extra_filters":         {"query": {"up"}, "extra_filters[]": {`{stroppy_run_id="` + otherRun + `"}`}},
		"client extra_filters no bkt":  {"query": {"up"}, "extra_filters": {`{stroppy_run_id="` + otherRun + `"}`}},
		"client extra_label":           {"query": {"up"}, "extra_label": {"stroppy_run_id=" + otherRun}},
		"client extra_label uppercase": {"query": {"up"}, "EXTRA_LABEL": {"stroppy_run_id=" + otherRun}},
		"many client filters": {
			"query":           {"up"},
			"extra_filters[]": {`{stroppy_run_id="` + otherRun + `"}`, `{stroppy_tenant_id="x"}`},
			"extra_label":     {"stroppy_run_id=" + otherRun},
		},
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			scopeValues(v, liveScope())

			if got := v["extra_label"]; len(got) != 1 || got[0] != "stroppy_run_id="+testRun {
				t.Fatalf("extra_label = %v, want exactly [stroppy_run_id=%s]", got, testRun)
			}
			// VictoriaMetrics ORs multiple extra_filters[] together, so ANY
			// surviving client filter widens the selection back out.
			for key := range v {
				if strings.EqualFold(key, "extra_filters") || strings.EqualFold(key, "extra_filters[]") {
					t.Fatalf("client scoping param %q survived: %v", key, v[key])
				}
				if strings.EqualFold(key, "extra_label") && key != "extra_label" {
					t.Fatalf("case-variant extra_label %q survived", key)
				}
			}
			if v.Get("query") != "up" {
				t.Fatalf("query mutated: %q", v.Get("query"))
			}
		})
	}
}

func TestScopeValuesClampsTimeWindow(t *testing.T) {
	scope := liveScope()
	v := url.Values{"start": {"500"}, "end": {"9999"}, "time": {"1500"}}
	scopeValues(v, scope)

	if v.Get("start") != "1000" {
		t.Fatalf("start = %q, want clamped to 1000", v.Get("start"))
	}
	if v.Get("end") != "2000" {
		t.Fatalf("end = %q, want clamped to 2000", v.Get("end"))
	}
	if v.Get("time") != "1500" {
		t.Fatalf("time = %q, want untouched 1500", v.Get("time"))
	}
}

func TestScopeValuesRunningRunKeepsOpenEnd(t *testing.T) {
	scope := ShareScope{RunID: testRun, From: time.Unix(1000, 0).UTC()} // no To: still running
	v := url.Values{"start": {"500"}, "end": {"999999"}}
	scopeValues(v, scope)

	if v.Get("end") != "999999" {
		t.Fatalf("end = %q, want untouched for a running run", v.Get("end"))
	}
}

func TestPublicMetricsPathAllowed(t *testing.T) {
	allowed := []string{
		"api/v1/query",
		"api/v1/query_range",
		"api/v1/series",
		"api/v1/labels",
		"api/v1/status/buildinfo",
		"api/v1/label/stroppy_run_id/values",
	}
	for _, p := range allowed {
		if !publicMetricsPathAllowed(p) {
			t.Fatalf("%q should be allowed", p)
		}
	}
	denied := []string{
		"api/v1/write",
		"api/v1/export",
		"api/v1/admin/tsdb/delete_series",
		"api/v1/label//values",
		"api/v1/label/a/b/values",
		"",
		"../select/0/prometheus/api/v1/query",
	}
	for _, p := range denied {
		if publicMetricsPathAllowed(p) {
			t.Fatalf("%q should be denied", p)
		}
	}
}

// Grafana's Prometheus datasource sends query_range as a urlencoded POST, so a
// filter injected only into the query string would leave the body unscoped.
func TestScopeRequestRewritesPostForm(t *testing.T) {
	body := url.Values{
		"query":           {"up"},
		"extra_filters[]": {`{stroppy_run_id="` + otherRun + `"}`},
	}.Encode()
	r := httptest.NewRequest(http.MethodPost, "/public/metrics/api/v1/query_range?extra_label=stroppy_run_id="+otherRun, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := scopeRequest(r, liveScope()); err != nil {
		t.Fatalf("scopeRequest: %v", err)
	}

	if got := r.URL.Query()["extra_label"]; len(got) != 1 || got[0] != "stroppy_run_id="+testRun {
		t.Fatalf("query extra_label = %v", got)
	}
	raw, _ := io.ReadAll(r.Body)
	form, err := url.ParseQuery(string(raw))
	if err != nil {
		t.Fatalf("parse rewritten body: %v", err)
	}
	if got := form["extra_label"]; len(got) != 1 || got[0] != "stroppy_run_id="+testRun {
		t.Fatalf("body extra_label = %v, want ours only", got)
	}
	if _, ok := form["extra_filters[]"]; ok {
		t.Fatalf("client extra_filters[] survived in body")
	}
	if r.ContentLength != int64(len(raw)) {
		t.Fatalf("ContentLength = %d, want %d", r.ContentLength, len(raw))
	}
}

func TestScopeRequestRejectsNonFormBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/public/metrics/api/v1/query", strings.NewReader(`{"query":"up"}`))
	r.Header.Set("Content-Type", "application/json")
	if err := scopeRequest(r, liveScope()); err == nil {
		t.Fatal("json body should be rejected")
	}
}

// End-to-end through the proxy: the upstream must only ever see our filter.
func newProxyFixture(t *testing.T, shares ShareResolver) (*httptest.Server, *url.Values) {
	t.Helper()
	var seen url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		seen = r.Form
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	h, err := newPublicMetricsProxy(upstream.URL, "backend-token", shares, "", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newPublicMetricsProxy: %v", err)
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front, &seen
}

func TestProxyForwardsOnlyOurFilter(t *testing.T) {
	front, seen := newProxyFixture(t, stubShares{scope: liveScope()})

	req, _ := http.NewRequest(http.MethodGet, front.URL+`/public/metrics/api/v1/query?query=up&extra_filters[]={stroppy_run_id="`+otherRun+`"}`, nil)
	req.AddCookie(&http.Cookie{Name: shareCookieName, Value: testToken})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := (*seen)["extra_label"]; len(got) != 1 || got[0] != "stroppy_run_id="+testRun {
		t.Fatalf("upstream extra_label = %v", got)
	}
	if _, ok := (*seen)["extra_filters[]"]; ok {
		t.Fatalf("upstream saw client extra_filters[]")
	}
}

func TestProxyFailsClosedWithoutCookie(t *testing.T) {
	front, _ := newProxyFixture(t, stubShares{scope: liveScope()})

	resp, err := http.Get(front.URL + "/public/metrics/api/v1/query?query=up")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (fail closed)", resp.StatusCode)
	}
}

// A revoked/expired/unknown token must be indistinguishable and must not query.
func TestProxyRejectsDeadToken(t *testing.T) {
	front, _ := newProxyFixture(t, stubShares{err: errors.New("revoked")})

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/public/metrics/api/v1/query?query=up", nil)
	req.AddCookie(&http.Cookie{Name: shareCookieName, Value: testToken})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestProxyDeniesNonReadPaths(t *testing.T) {
	front, _ := newProxyFixture(t, stubShares{scope: liveScope()})

	req, _ := http.NewRequest(http.MethodPost, front.URL+"/public/metrics/api/v1/write", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: shareCookieName, Value: testToken})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestProxyStripsViewerCredentials(t *testing.T) {
	var gotAuth, gotCookie string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h, err := newPublicMetricsProxy(upstream.URL, "backend-token", stubShares{scope: liveScope()}, "", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newPublicMetricsProxy: %v", err)
	}
	front := httptest.NewServer(h)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/public/metrics/api/v1/query?query=up", nil)
	req.AddCookie(&http.Cookie{Name: shareCookieName, Value: testToken})
	req.Header.Set("Authorization", "Bearer viewer-supplied")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer backend-token" {
		t.Fatalf("upstream Authorization = %q, want the backend token", gotAuth)
	}
	if gotCookie != "" {
		t.Fatalf("viewer cookie leaked upstream: %q", gotCookie)
	}
}
