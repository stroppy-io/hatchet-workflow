package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
)

func TestMonitorProxyPreservesPathAndInjectsBearer(t *testing.T) {
	agentToken, verifier := testAgentToken(t)
	var gotPath string
	var gotAuth string
	var calls int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer backend.Close()

	proxy, err := newMonitorProxy(backend.URL, "monitor-token", verifier)
	if err != nil {
		t.Fatalf("monitor proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://control.example/select/0/prometheus/api/v1/query?x=1", nil)
	req.Header.Set("Authorization", "Bearer "+agentToken)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := gotPath, "/select/0/prometheus/api/v1/query?x=1"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if got, want := gotAuth, "Bearer monitor-token"; got != want {
		t.Fatalf("authorization = %q, want %q", got, want)
	}
	if got, want := calls, 1; got != want {
		t.Fatalf("backend calls = %d, want %d", got, want)
	}
}

func TestMonitorProxyRewritesJSONLineLabelsBeforeBackend(t *testing.T) {
	agentToken, verifier := testAgentToken(t)
	var gotAuth string
	var gotEvent map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotEvent); err != nil {
			t.Fatalf("decode backend body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer backend.Close()

	proxy, err := newMonitorProxy(backend.URL, "monitor-token", verifier)
	if err != nil {
		t.Fatalf("monitor proxy: %v", err)
	}

	body := `{"run_id":"forged","machine_id":"forged","tenant_id":"forged","message":"ok"}` + "\n"
	req := httptest.NewRequest(http.MethodPost, "http://control.example/insert/jsonline", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+agentToken)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := gotAuth, "Bearer monitor-token"; got != want {
		t.Fatalf("authorization = %q, want %q", got, want)
	}
	if got, want := gotEvent["tenant_id"], "tenant-1"; got != want {
		t.Fatalf("tenant_id = %q, want %q", got, want)
	}
	if got, want := gotEvent["run_id"], "run-1"; got != want {
		t.Fatalf("run_id = %q, want %q", got, want)
	}
	if got, want := gotEvent["machine_id"], "node-1"; got != want {
		t.Fatalf("machine_id = %q, want %q", got, want)
	}
	if got, want := gotEvent["message"], "ok"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestMonitorProxyRejectsInvalidBearerBeforeBackend(t *testing.T) {
	_, verifier := testAgentToken(t)
	var calls int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer backend.Close()

	proxy, err := newMonitorProxy(backend.URL, "monitor-token", verifier)
	if err != nil {
		t.Fatalf("monitor proxy: %v", err)
	}

	for _, auth := range []string{"", "Bearer stale"} {
		req := httptest.NewRequest(http.MethodPost, "http://control.example/insert/jsonline", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)
		if got, want := rec.Code, http.StatusUnauthorized; got != want {
			t.Fatalf("auth %q status = %d, want %d", auth, got, want)
		}
	}
	if got := calls; got != 0 {
		t.Fatalf("backend was called %d times", got)
	}
}

func testAgentToken(t *testing.T) (string, AgentTokenVerifier) {
	t.Helper()
	svc, err := agentdomain.NewTokenService("test-secret")
	if err != nil {
		t.Fatalf("agent token service: %v", err)
	}
	token, err := svc.IssueAgentToken("tenant-1", "run-1", "node-1", agentdomain.TaskQueue("node-1"))
	if err != nil {
		t.Fatalf("issue agent token: %v", err)
	}
	return token, svc
}

func TestGatewayRoutesMonitoringTrafficToProxy(t *testing.T) {
	var gotPaths []string
	g := &Gateway{
		monitorProxy: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPaths = append(gotPaths, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	for _, path := range []string{"/insert/jsonline", "/select/0/prometheus/api/v1/query"} {
		rec := httptest.NewRecorder()
		g.serveHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if got, want := rec.Code, http.StatusNoContent; got != want {
			t.Fatalf("%s status = %d, want %d", path, got, want)
		}
	}

	if got, want := strings.Join(gotPaths, ","), "/insert/jsonline,/select/0/prometheus/api/v1/query"; got != want {
		t.Fatalf("routed paths = %q, want %q", got, want)
	}
}
