package gateway

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestGateway builds a Gateway without dialing Temporal (grpc.NewClient is
// lazy, so an unroutable host is fine for the HTTP-only tests).
func newTestGateway(t *testing.T, cfg Config) *Gateway {
	t.Helper()
	cfg.TemporalHostPort = "127.0.0.1:1"
	cfg.CacheDir = t.TempDir()
	g, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(g.Close)
	return g
}

func TestServeAgentBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "stroppy-agent")
	if err := os.WriteFile(bin, []byte("AGENT-ELF"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := newTestGateway(t, Config{AgentBinaryPath: bin})

	rr := httptest.NewRecorder()
	g.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/agent/binary", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "AGENT-ELF" {
		t.Fatalf("body = %q", rr.Body.String())
	}
}

func TestServeAgentBinary_NotConfigured(t *testing.T) {
	g := newTestGateway(t, Config{})
	rr := httptest.NewRecorder()
	g.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/agent/binary", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestServeArtifact_LocalFile(t *testing.T) {
	art := filepath.Join(t.TempDir(), "stroppy")
	if err := os.WriteFile(art, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := newTestGateway(t, Config{Artifacts: map[string]string{"stroppy": art}})

	rr := httptest.NewRecorder()
	g.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/artifacts/stroppy", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("body = %q", rr.Body.String())
	}
}

func TestServeArtifact_HTTPUpstreamCached(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte("STROPPY-BIN"))
	}))
	defer upstream.Close()

	g := newTestGateway(t, Config{Artifacts: map[string]string{"stroppy": upstream.URL}})

	for i := range 2 {
		rr := httptest.NewRecorder()
		g.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/artifacts/stroppy", nil))
		if rr.Code != http.StatusOK || rr.Body.String() != "STROPPY-BIN" {
			t.Fatalf("req %d: status=%d body=%q", i, rr.Code, rr.Body.String())
		}
	}
	if hits != 1 {
		t.Fatalf("upstream hits = %d, want 1 (second request must hit cache)", hits)
	}
}

func TestServeArtifact_Unknown(t *testing.T) {
	g := newTestGateway(t, Config{})
	rr := httptest.NewRecorder()
	g.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/artifacts/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// TestAptProxyMatcher checks the cmux matcher splits apt forward-proxy requests
// from the gateway's own origin-form routes.
func TestAptProxyMatcher(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"GET http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease HTTP/1.1\r\n", true},
		{"CONNECT apt.postgresql.org:443 HTTP/1.1\r\n", true},
		{"GET /agent/binary HTTP/1.1\r\n", false},
		{"GET /artifacts/stroppy HTTP/1.1\r\n", false},
		{"POST /healthz HTTP/1.1\r\n", false},
	}
	for _, c := range cases {
		if got := aptProxyMatcher(strings.NewReader(c.line)); got != c.want {
			t.Errorf("aptProxyMatcher(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

// TestPipeApt verifies the gateway pipes an apt connection raw to the backend.
func TestPipeApt(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		c, err := backend.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 4)
		_, _ = io.ReadFull(c, buf)
		_, _ = c.Write([]byte("pong:" + string(buf)))
	}()

	g := newTestGateway(t, Config{AptBackend: backend.Addr().String()})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		g.pipeApt(c)
	}()

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 9)
	if _, err := io.ReadFull(c, out); err != nil {
		t.Fatal(err)
	}
	if string(out) != "pong:ping" {
		t.Fatalf("pipeApt roundtrip = %q, want pong:ping", string(out))
	}
}

func TestHealthzAndNotFound(t *testing.T) {
	g := newTestGateway(t, Config{})

	rr := httptest.NewRecorder()
	g.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rr.Code)
	}

	rr = httptest.NewRecorder()
	g.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("notfound = %d, want 404", rr.Code)
	}
	_, _ = io.Discard.Write(nil)
}
