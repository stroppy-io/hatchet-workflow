package gateway

import (
	"bufio"
	"io"
	"net"
	"strings"
	"time"
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
