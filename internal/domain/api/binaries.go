package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// Server-side cache + proxy for the binaries the agents fetch during the
// install_* phases (node_exporter, vmagent, stroppy, vector, postgres_exporter,
// mysqld_exporter). Without this, every Yandex VM hits github.com/release-
// assets.githubusercontent.com directly — which is flaky from YC and adds
// minutes per install. With this, the server downloads each artifact once,
// stores it on a persistent volume, and streams it to every subsequent
// agent over the LAN.

// binaryUpstreams maps `name → URL template`. {ver} and {file} are replaced
// at lookup time. Adding a new binary = one line here. Templates are pinned
// so the agent can't ask the server to fetch arbitrary URLs (single-tenant,
// but defence-in-depth on what's effectively a server-side fetcher).
var binaryUpstreams = map[string]string{
	"node_exporter":     "https://github.com/prometheus/node_exporter/releases/download/v{ver}/{file}",
	"vmagent":           "https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v{ver}/{file}",
	"stroppy":           "https://github.com/stroppy-io/stroppy/releases/download/v{ver}/{file}",
	"vector":            "https://packages.timber.io/vector/{ver}/{file}",
	"postgres_exporter": "https://github.com/prometheus-community/postgres_exporter/releases/download/v{ver}/{file}",
	"mysqld_exporter":   "https://github.com/prometheus/mysqld_exporter/releases/download/v{ver}/{file}",
}

// binaryCacheDir is the persistent location of cached artifacts. Backed by
// the `binaries` named volume in docker-compose.yaml so a server rebuild
// keeps the cache warm. Override via STROPPY_BINARY_CACHE_DIR.
func binaryCacheDir() string {
	if d := os.Getenv("STROPPY_BINARY_CACHE_DIR"); d != "" {
		return d
	}
	return "/var/lib/stroppy-cache/binaries"
}

// inflight collapses concurrent first-hit downloads of the same artifact
// into a single upstream fetch. Without this, a fleet booting 30 VMs in
// parallel would each independently hit github 30 times.
var inflight singleflight.Group

// serveCachedBinary handles GET /api/binaries/{name}/{version}/{filename}.
// Cache miss → fetch upstream once → save → stream. Cache hit → stream.
func (s *Server) serveCachedBinary(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")
	filename := chi.URLParam(r, "filename")

	if !validIdent(name) || !validIdent(version) || !validFilename(filename) {
		http.Error(w, "invalid path component", http.StatusBadRequest)
		return
	}
	tmpl, ok := binaryUpstreams[name]
	if !ok {
		http.Error(w, "unknown binary", http.StatusNotFound)
		return
	}
	upstream := strings.NewReplacer("{ver}", version, "{file}", filename).Replace(tmpl)

	cacheDir := binaryCacheDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s", name, version, filename))

	if _, err := os.Stat(cachePath); err == nil {
		// Cache hit — stream straight from disk. Agents pull MBs at a time;
		// http.ServeFile handles range requests + content-type.
		s.logger.Debug("binary cache hit", zap.String("name", name), zap.String("ver", version))
		http.ServeFile(w, r, cachePath)
		return
	}

	// Single-flight the upstream fetch keyed by the final cache path.
	// All concurrent waiters block here; only one issues the GET to github.
	_, err, _ := inflight.Do(cachePath, func() (any, error) {
		// Re-check inside the critical section — another waiter might have
		// finished while we were queueing.
		if _, err := os.Stat(cachePath); err == nil {
			return nil, nil
		}
		return nil, downloadToCache(s.logger, upstream, cachePath)
	})
	if err != nil {
		s.logger.Warn("binary upstream fetch failed",
			zap.String("name", name), zap.String("ver", version),
			zap.String("upstream", upstream), zap.Error(err))
		http.Error(w, "upstream fetch failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	http.ServeFile(w, r, cachePath)
}

// downloadToCache fetches `upstream` to a temp file in the same dir as
// `dst` and atomically renames into place. The atomic rename means a
// crashed download never leaves a half-written cache entry that future
// hits would happily serve.
func downloadToCache(logger *zap.Logger, upstream, dst string) error {
	logger.Info("binary cache miss, fetching upstream",
		zap.String("upstream", upstream), zap.String("dst", dst))
	cl := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, upstream, nil)
	if err != nil {
		return err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream %d %s", resp.StatusCode, resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// validIdent / validFilename keep the path-substitution into the URL
// template strict — no slashes, no dots that escape the cache dir, no
// length explosions. Anything else is a 400.
func validIdent(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r == '-' || r == '_' || r == '.' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func validFilename(s string) bool {
	if s == "" || len(s) > 256 || strings.Contains(s, "/") || strings.Contains(s, "..") {
		return false
	}
	return true
}

// PrewarmBinaries optionally fetches all upstreams listed in the supplied
// version map at server start so the first agent doesn't pay the
// cold-cache penalty. Errors are logged but non-fatal — agents will retry
// the cache miss path themselves.
func (s *Server) PrewarmBinaries(versions map[string]string) {
	cacheDir := binaryCacheDir()
	for name, ver := range versions {
		tmpl, ok := binaryUpstreams[name]
		if !ok {
			continue
		}
		// Filename heuristic differs per binary; skip prewarm if we don't
		// know it. Could be made explicit by extending the manifest.
		filename := guessFilename(name, ver)
		if filename == "" {
			continue
		}
		dst := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s", name, ver, filename))
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		upstream := strings.NewReplacer("{ver}", ver, "{file}", filename).Replace(tmpl)
		if err := downloadToCache(s.logger, upstream, dst); err != nil &&
			!errors.Is(err, os.ErrExist) {
			s.logger.Warn("prewarm failed", zap.String("name", name), zap.Error(err))
		}
	}
}

// guessFilename returns the canonical archive name for a binary+version.
// Conservative — only filled for the entries that follow predictable
// naming. Unknown names skip prewarm and fall back to lazy on first
// agent miss.
func guessFilename(name, ver string) string {
	switch name {
	case "node_exporter":
		return fmt.Sprintf("node_exporter-%s.linux-amd64.tar.gz", ver)
	case "vmagent":
		return fmt.Sprintf("vmutils-linux-amd64-v%s.tar.gz", ver)
	case "stroppy":
		return "stroppy_linux_amd64.tar.gz"
	case "vector":
		return fmt.Sprintf("vector-%s-x86_64-unknown-linux-musl.tar.gz", ver)
	case "postgres_exporter":
		return fmt.Sprintf("postgres_exporter-%s.linux-amd64.tar.gz", ver)
	case "mysqld_exporter":
		return fmt.Sprintf("mysqld_exporter-%s.linux-amd64.tar.gz", ver)
	}
	return ""
}
