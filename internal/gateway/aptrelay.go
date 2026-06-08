package gateway

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	forwardProxyHeader       = "X-Stroppy-Forward-Proxy"
	forwardProxyTargetHeader = "X-Stroppy-Proxy-Target"
)

// aptProxyMatcher is a cmux matcher that recognises an HTTP forward-proxy
// conversation (what `apt` speaks when Acquire::http::Proxy points at us): either
// a CONNECT, or a request whose target is an absolute URI (GET http://mirror/...).
// The gateway's own routes use origin-form targets (GET /agent/binary), so this
// cleanly splits apt traffic from gateway HTTP on the SAME port — agents need one
// address for everything. cmux replays the sniffed bytes, so the matched
// connection still carries the full unmodified request for apt-cacher-ng.
func aptProxyMatcher(r io.Reader) bool {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil {
		return false
	}
	parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
	if len(parts) < 2 {
		return false
	}
	method, target := parts[0], parts[1]
	if method == "CONNECT" {
		return true
	}
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

// pipeApt forwards an apt forward-proxy connection raw to the internal
// apt-cacher-ng (backend), copying bytes both ways. apt-cacher-ng — hidden on the
// compose network, never addressed by agents directly — sees an unmodified proxy
// conversation, so plain GET (cached) and CONNECT (tunnel) both work. This is the
// "apt through us" path: agents proxy apt at the single server address; the server
// relays to the cache. Nothing goes direct to the internet.
func (g *Gateway) pipeApt(client net.Conn) {
	defer client.Close()
	if g.aptBackend == "" {
		return
	}
	up, err := net.DialTimeout("tcp", g.aptBackend, 5*time.Second)
	if err != nil {
		g.logger.Warn("apt backend dial failed", "backend", g.aptBackend, "err", err)
		return
	}
	defer up.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, up); done <- struct{}{} }()
	<-done
}

// serveAptProxyHTTP relays forward-proxy traffic that came through Caddy. Caddy
// accepts the public proxy request and reverse-proxies it to this gateway, but
// normalizes absolute-form requests (GET http://mirror/path) into origin-form
// requests (GET /path) while preserving the target Host. Rebuild the
// absolute-form request before handing it to apt-cacher-ng.
func (g *Gateway) serveAptProxyHTTP(w http.ResponseWriter, r *http.Request) {
	if g.aptBackend == "" {
		http.Error(w, "apt backend is not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodConnect {
		g.tunnelAptProxyHTTP(w, r)
		return
	}

	proxyReq, err := caddyForwardProxyRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	up, err := net.DialTimeout("tcp", g.aptBackend, 5*time.Second)
	if err != nil {
		g.logger.Warn("apt backend dial failed", "backend", g.aptBackend, "err", err)
		http.Error(w, "apt backend unavailable", http.StatusBadGateway)
		return
	}
	defer up.Close()

	if err := proxyReq.WriteProxy(up); err != nil {
		http.Error(w, "write apt proxy request failed", http.StatusBadGateway)
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(up), proxyReq)
	if err != nil {
		http.Error(w, "read apt proxy response failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (g *Gateway) tunnelAptProxyHTTP(w http.ResponseWriter, r *http.Request) {
	proxyReq, err := caddyForwardProxyRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	up, err := net.DialTimeout("tcp", g.aptBackend, 5*time.Second)
	if err != nil {
		g.logger.Warn("apt backend dial failed", "backend", g.aptBackend, "err", err)
		http.Error(w, "apt backend unavailable", http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		up.Close()
		http.Error(w, "response writer does not support hijacking", http.StatusInternalServerError)
		return
	}
	client, rw, err := hj.Hijack()
	if err != nil {
		up.Close()
		return
	}
	defer client.Close()
	defer up.Close()

	if err := proxyReq.Write(up); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() {
		if rw.Reader.Buffered() > 0 {
			_, _ = io.CopyN(up, rw.Reader, int64(rw.Reader.Buffered()))
		}
		_, _ = io.Copy(up, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, up)
		done <- struct{}{}
	}()
	<-done
}

func caddyForwardProxyRequest(r *http.Request) (*http.Request, error) {
	out := r.Clone(r.Context())
	out.RequestURI = ""
	out.Header.Del(forwardProxyHeader)
	out.Header.Del(forwardProxyTargetHeader)
	out.Header.Del("X-Forwarded-For")
	out.Header.Del("X-Forwarded-Host")
	out.Header.Del("X-Forwarded-Proto")

	if r.Method == http.MethodConnect {
		target := r.Header.Get(forwardProxyTargetHeader)
		if target == "" {
			target = r.Host
		}
		if target == "" {
			target = r.URL.Host
		}
		if target == "" {
			return nil, fmt.Errorf("empty CONNECT target")
		}
		out.URL = &url.URL{Host: target}
		out.Host = target
		return out, nil
	}

	if out.URL != nil && out.URL.IsAbs() {
		return out, nil
	}
	if r.Host == "" {
		return nil, fmt.Errorf("empty proxy target host")
	}
	absoluteURL, err := url.Parse("http://" + r.Host + r.URL.RequestURI())
	if err != nil {
		return nil, fmt.Errorf("rebuild proxy URL: %w", err)
	}
	out.URL = absoluteURL
	out.Host = r.Host
	return out, nil
}
