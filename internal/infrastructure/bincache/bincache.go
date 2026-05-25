// Package bincache is the server-side cache + proxy for the binaries agents fetch
// during install (node_exporter, vmagent, stroppy, vector, postgres_exporter,
// mysqld_exporter). Without it every provisioned VM hits github directly — flaky
// from Yandex Cloud and slow. With it the server downloads each artifact once to a
// persistent volume and streams it to every subsequent agent over the LAN. The agent
// recipe is identical for local Docker and YC runs (always fetch from the server),
// which is what proves the path.
package bincache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gopherex/xlog"
	"golang.org/x/sync/singleflight"
)

// upstreams maps name → URL template; {ver} and {file} are substituted at lookup.
// Pinned templates mean the agent can't ask the server to fetch arbitrary URLs.
var upstreams = map[string]string{
	"node_exporter":     "https://github.com/prometheus/node_exporter/releases/download/v{ver}/{file}",
	"vmagent":           "https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v{ver}/{file}",
	"stroppy":           "https://github.com/stroppy-io/stroppy/releases/download/v{ver}/{file}",
	"vector":            "https://packages.timber.io/vector/{ver}/{file}",
	"postgres_exporter": "https://github.com/prometheus-community/postgres_exporter/releases/download/v{ver}/{file}",
	"mysqld_exporter":   "https://github.com/prometheus/mysqld_exporter/releases/download/v{ver}/{file}",
	"cockroach":         "https://storage.googleapis.com/cockroach-release-artifacts-prod/{file}",
	"ydb":               "https://binaries.ydb.tech/release/{ver}/{file}",
}

// RoutePattern is the chi route this handler serves.
const RoutePattern = "/binary/{name}/{version}/{filename}"

func cacheDir() string {
	if d := os.Getenv("STROPPY_BINARY_CACHE_DIR"); d != "" {
		return d
	}
	return "/var/lib/stroppy-cache/binaries"
}

var inflight singleflight.Group

// Handler serves GET /binary/{name}/{version}/{filename}: cache hit streams from
// disk, miss fetches the pinned upstream once (single-flighted), caches atomically,
// then streams.
func Handler(logger *xlog.Logger) http.HandlerFunc {
	log := logger.AppendName("bincache")
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		version := chi.URLParam(r, "version")
		filename := chi.URLParam(r, "filename")
		if !validIdent(name) || !validIdent(version) || !validFilename(filename) {
			http.Error(w, "invalid path component", http.StatusBadRequest)
			return
		}
		tmpl, ok := upstreams[name]
		if !ok {
			http.Error(w, "unknown binary", http.StatusNotFound)
			return
		}
		upstream := strings.NewReplacer("{ver}", version, "{file}", filename).Replace(tmpl)

		dir := cacheDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cachePath := filepath.Join(dir, fmt.Sprintf("%s-%s-%s", name, version, filename))

		if _, err := os.Stat(cachePath); err == nil {
			http.ServeFile(w, r, cachePath)
			return
		}
		_, err, _ := inflight.Do(cachePath, func() (any, error) {
			if _, statErr := os.Stat(cachePath); statErr == nil {
				return nil, nil
			}
			return nil, downloadToCache(log, upstream, cachePath)
		})
		if err != nil {
			log.Warn("binary upstream fetch failed",
				xlog.String("name", name), xlog.String("version", version),
				xlog.String("upstream", upstream), xlog.Error("error", err))
			http.Error(w, "upstream fetch failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		http.ServeFile(w, r, cachePath)
	}
}

// downloadToCache fetches upstream to a temp file in dst's dir and atomically renames
// it into place — a crashed download never leaves a half-written cache entry.
func downloadToCache(log *xlog.Logger, upstream, dst string) error {
	log.Info("binary cache miss, fetching upstream", xlog.String("upstream", upstream))
	cl := &http.Client{Timeout: 10 * time.Minute}
	resp, err := cl.Get(upstream)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
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
			_ = tmp.Close()
			_ = os.Remove(tmpName)
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

func validIdent(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r == '-' || r == '_' || r == '.' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func validFilename(s string) bool {
	if s == "" || len(s) > 128 || strings.Contains(s, "/") || strings.Contains(s, "..") {
		return false
	}
	return true
}
