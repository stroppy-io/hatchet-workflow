package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
)

// Config configures a Gateway.
type Config struct {
	// TemporalHostPort is the real Temporal frontend the proxy forwards to.
	TemporalHostPort string
	// AgentBinaryPath is the local path to the linux agent executable served at
	// GET /agent/binary. Empty disables the route (503).
	AgentBinaryPath string
	// CacheDir is where proxied artifacts + apt packages are cached.
	CacheDir string
	// Artifacts maps an artifact name (e.g. "stroppy") to the real upstream URL
	// the server fetches it from. Exposed to agents as GET /artifacts/{name}.
	Artifacts map[string]string
	// AptBackend is the internal apt-cacher-ng address (e.g. "apt-cacher-ng:3142")
	// the gateway pipes agent apt traffic to. Empty disables apt forwarding.
	AptBackend string
	// HTTPFallback handles every HTTP/1.1 request that is not one of the gateway's
	// own agent routes (/healthz, /agent/binary, /artifacts/*, /api/binaries/*).
	// Wire the control-plane UI/API router here so the SPA + REST API are served
	// on the SAME port as the gateway. Nil => 404 for unknown paths.
	HTTPFallback http.Handler
	// Logger is the structured logger (defaults to slog.Default()).
	Logger *slog.Logger
}

// Gateway is the single agent-facing entrypoint: a Temporal gRPC proxy and an
// HTTP server (agent binary, artifact cache, apt forward proxy) multiplexed on
// one listener.
type Gateway struct {
	agentBinaryPath string
	cacheDir        string
	artifacts       map[string]string
	aptBackend      string
	logger          *slog.Logger

	fallback http.Handler

	grpc    *grpc.Server
	backend *grpc.ClientConn
	http    *http.Server
}

// New builds a Gateway and its Temporal proxy. It does not listen until Serve.
func New(cfg Config) (*Gateway, error) {
	if cfg.TemporalHostPort == "" {
		return nil, errors.New("gateway: empty TemporalHostPort")
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = "/var/lib/stroppy-cache/binaries"
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	proxySrv, backend, err := newTemporalProxy(cfg.TemporalHostPort)
	if err != nil {
		return nil, err
	}

	g := &Gateway{
		agentBinaryPath: cfg.AgentBinaryPath,
		cacheDir:        cfg.CacheDir,
		artifacts:       cfg.Artifacts,
		aptBackend:      cfg.AptBackend,
		fallback:        cfg.HTTPFallback,
		logger:          logger,
		grpc:            proxySrv,
		backend:         backend,
	}
	g.http = &http.Server{Handler: http.HandlerFunc(g.serveHTTP), ReadHeaderTimeout: 30 * time.Second}
	return g, nil
}

// serveHTTP dispatches the gateway's HTTP routes (agent binary + artifact /
// binary caches). apt is handled out-of-band by the apt-cache TCP relay, not
// here, so this server only sees its own routes.
func (g *Gateway) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/healthz":
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/agent/binary":
		g.serveAgentBinary(w, r)
	case strings.HasPrefix(r.URL.Path, "/artifacts/"):
		g.serveArtifact(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/binaries/"):
		g.serveCachedBinary(w, r)
	case g.fallback != nil:
		// Control-plane UI + REST API (SPA, /api/v1/...) on the same port.
		g.fallback.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

// Serve multiplexes gRPC (Temporal proxy) and HTTP on one listener via cmux and
// blocks until the listener closes or a sub-server errors.
func (g *Gateway) Serve(lis net.Listener) error {
	m := cmux.New(lis)
	// Order matters — most specific first:
	//   1. gRPC: any HTTP/2 connection (the only HTTP/2 client is the Temporal
	//      worker). cmux.HTTP2() matches on the client preface alone — unlike the
	//      SendSettings variant it does not write a settings frame back, which is
	//      more reliable across docker NAT (agents on per-run bridges).
	//   2. apt: HTTP/1.1 forward-proxy conversation (CONNECT / absolute-URI) → piped
	//      raw to apt-cacher-ng. Same port, so agents need ONE address for apt too.
	//   3. everything else: the gateway's own HTTP/1.1 routes.
	grpcL := m.Match(cmux.HTTP2())
	aptL := m.Match(aptProxyMatcher)
	httpL := m.Match(cmux.Any())

	errc := make(chan error, 4)
	go func() { errc <- g.grpc.Serve(grpcL) }()
	go func() {
		if err := g.http.Serve(httpL); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
			return
		}
		errc <- nil
	}()
	go func() {
		for {
			conn, err := aptL.Accept()
			if err != nil {
				errc <- nil
				return
			}
			go g.pipeApt(conn)
		}
	}()
	go func() { errc <- m.Serve() }()

	return <-errc
}

// Close stops the gRPC proxy, HTTP server and the backend Temporal connection.
func (g *Gateway) Close() {
	g.grpc.GracefulStop()
	_ = g.http.Shutdown(context.Background())
	if g.backend != nil {
		_ = g.backend.Close()
	}
}
