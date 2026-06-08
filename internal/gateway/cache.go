package gateway

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

// binaryUpstreams maps `name → URL template` for the /api/binaries/{name}/{ver}/
// {file} cache route. {ver} and {file} are substituted at lookup. Templates are
// pinned so an agent can't make the server fetch arbitrary URLs.
var binaryUpstreams = map[string]string{
	"cockroach":         "https://binaries.cockroachdb.com/{file}",
	"node_exporter":     "https://github.com/prometheus/node_exporter/releases/download/v{ver}/{file}",
	"vmagent":           "https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v{ver}/{file}",
	"stroppy":           "https://github.com/stroppy-io/stroppy/releases/download/v{ver}/{file}",
	"stroppy_nightly":   "https://github.com/stroppy-io/stroppy/releases/download/nightly-{ver}/{file}",
	"postgres_exporter": "https://github.com/prometheus-community/postgres_exporter/releases/download/v{ver}/{file}",
	"mysqld_exporter":   "https://github.com/prometheus/mysqld_exporter/releases/download/v{ver}/{file}",
	"vector":            "https://packages.timber.io/vector/{ver}/{file}",
	"ydbd":              "https://binaries.ydb.tech/release/{ver}/{file}",
}

// inflight collapses concurrent first-hit downloads of the same artifact into a
// single upstream fetch.
var inflight singleflight.Group

// serveAgentBinary streams the linux agent executable. A bare base image curls
// this on boot to bootstrap itself — the binary is never baked into the image.
func (g *Gateway) serveAgentBinary(w http.ResponseWriter, r *http.Request) {
	if g.agentBinaryPath == "" {
		http.Error(w, "agent binary not configured", http.StatusServiceUnavailable)
		return
	}
	if _, err := os.Stat(g.agentBinaryPath); err != nil {
		http.Error(w, "agent binary unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, g.agentBinaryPath)
}

// serveArtifact proxies + caches a named artifact whose real upstream the server
// knows but the agent must not. Used for the stroppy binary: the recipe points
// the agent at http://SERVER/artifacts/stroppy and the server fetches it from
// its configured upstream (e.g. minio / a github release) once, then serves it
// from the local cache to every agent.
func (g *Gateway) serveArtifact(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/artifacts/")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		http.Error(w, "invalid artifact", http.StatusBadRequest)
		return
	}
	upstream, ok := g.artifacts[name]
	if !ok || upstream == "" {
		http.Error(w, "unknown artifact", http.StatusNotFound)
		return
	}
	// A local path (no http scheme) is served straight from disk — used for a
	// stand-in artifact baked into the server image. An http(s) upstream (minio /
	// github release) is proxied + cached.
	if !strings.HasPrefix(upstream, "http://") && !strings.HasPrefix(upstream, "https://") {
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, upstream)
		return
	}
	g.serveCached(w, r, "artifact-"+name, upstream)
}

// serveCachedBinary handles GET /api/binaries/{name}/{version}/{filename}: cache
// miss → fetch the pinned upstream once → save → stream; cache hit → stream.
func (g *Gateway) serveCachedBinary(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/binaries/")
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		http.Error(w, "expected /api/binaries/{name}/{version}/{filename}", http.StatusBadRequest)
		return
	}
	name, version, filename := parts[0], parts[1], parts[2]
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
	g.serveCached(w, r, fmt.Sprintf("%s-%s-%s", name, version, filename), upstream)
}

// serveCached streams the cached copy of upstream (keyed by cacheKey), fetching
// it once on a miss. Concurrent first-hits collapse into a single download.
func (g *Gateway) serveCached(w http.ResponseWriter, r *http.Request, cacheKey, upstream string) {
	if err := os.MkdirAll(g.cacheDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cachePath := filepath.Join(g.cacheDir, cacheKey)

	if _, err := os.Stat(cachePath); err == nil {
		g.logger.Debug("cache hit", "key", cacheKey)
		http.ServeFile(w, r, cachePath)
		return
	}

	_, err, _ := inflight.Do(cachePath, func() (any, error) {
		if _, err := os.Stat(cachePath); err == nil {
			return nil, nil
		}
		return nil, g.downloadToCache(upstream, cachePath)
	})
	if err != nil {
		g.logger.Warn("upstream fetch failed", "upstream", upstream, "err", err)
		http.Error(w, "upstream fetch failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	http.ServeFile(w, r, cachePath)
}

// downloadToCache fetches upstream to a temp file next to dst and atomically
// renames it into place, so a crashed download never leaves a half-written entry.
func (g *Gateway) downloadToCache(upstream, dst string) error {
	g.logger.Info("cache miss, fetching upstream", "upstream", upstream, "dst", dst)
	cl := &http.Client{Timeout: 10 * time.Minute}
	resp, err := cl.Get(upstream)
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
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}

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
