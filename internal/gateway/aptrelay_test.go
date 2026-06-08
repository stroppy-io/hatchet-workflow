package gateway

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaddyForwardProxyRequestRebuildsAbsoluteURI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ubuntu/pool?x=1", nil)
	req.Host = "archive.ubuntu.com"
	req.Header.Set(forwardProxyHeader, "1")
	req.Header.Set("X-Forwarded-Host", "archive.ubuntu.com")

	out, err := caddyForwardProxyRequest(req)
	if err != nil {
		t.Fatalf("rebuild request: %v", err)
	}
	if got, want := out.URL.String(), "http://archive.ubuntu.com/ubuntu/pool?x=1"; got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
	if got := out.Header.Get(forwardProxyHeader); got != "" {
		t.Fatalf("%s header leaked: %q", forwardProxyHeader, got)
	}
}

func TestCaddyForwardProxyRequestUsesExplicitConnectTarget(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "http://gateway", nil)
	req.Host = "server:8080"
	req.Header.Set(forwardProxyHeader, "1")
	req.Header.Set(forwardProxyTargetHeader, "www.postgresql.org:443")

	out, err := caddyForwardProxyRequest(req)
	if err != nil {
		t.Fatalf("rebuild request: %v", err)
	}
	if got, want := out.URL.Host, "www.postgresql.org:443"; got != want {
		t.Fatalf("connect target = %q, want %q", got, want)
	}
	if got := out.Header.Get(forwardProxyTargetHeader); got != "" {
		t.Fatalf("%s header leaked: %q", forwardProxyTargetHeader, got)
	}
}

func TestGatewayRelaysCaddyNormalizedAptRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	gotLine := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			gotLine <- "accept: " + err.Error()
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		gotLine <- strings.TrimSpace(line)
		_, _ = conn.Write([]byte("HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n"))
	}()

	g := &Gateway{aptBackend: ln.Addr().String(), logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/ubuntu/pool?x=1", nil)
	req.Host = "archive.ubuntu.com"
	req.Header.Set(forwardProxyHeader, "1")
	rec := httptest.NewRecorder()
	g.serveHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := <-gotLine, "GET http://archive.ubuntu.com/ubuntu/pool?x=1 HTTP/1.1"; got != want {
		t.Fatalf("request line = %q, want %q", got, want)
	}
}
