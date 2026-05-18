// Package httpext hosts plain-HTTP routes that don't fit ConnectRPC.
package httpext

import (
	"context"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

// BinariesHandler bundles non-RPC routes:
//
//	GET /agent/binary                                — current agent binary
//	GET /api/binaries/{name}/{version}/{filename}   — cached artifact proxy
type BinariesHandler struct {
	agentBinaryPath string
	cache           *ops.BinaryCacheService
	log             *zap.Logger
}

// NewBinariesHandler builds the multiplex.
func NewBinariesHandler(agentBinaryPath string, cache *ops.BinaryCacheService, log *zap.Logger) *BinariesHandler {
	return &BinariesHandler{agentBinaryPath: agentBinaryPath, cache: cache, log: log}
}

// Mount registers the routes on the supplied mux.
func (h *BinariesHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/agent/binary", h.serveAgentBinary)
	mux.HandleFunc("/api/binaries/", h.serveCachedBinary)
}

func (h *BinariesHandler) serveAgentBinary(w http.ResponseWriter, r *http.Request) {
	path := h.agentBinaryPath
	if path == "" {
		// Default: serve the running process binary so a single-host install
		// can `curl /agent/binary` to provision VMs.
		exe, err := os.Executable()
		if err != nil {
			http.Error(w, "agent binary not configured", http.StatusServiceUnavailable)
			return
		}
		path = exe
	}
	if _, err := os.Stat(path); err != nil {
		http.Error(w, "agent binary not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="stroppy-cloud"`)
	http.ServeFile(w, r, path)
}

func (h *BinariesHandler) serveCachedBinary(w http.ResponseWriter, r *http.Request) {
	// /api/binaries/{name}/{version}/{filename}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/binaries/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		http.Error(w, "expected /api/binaries/{name}/{version}/{filename}", http.StatusBadRequest)
		return
	}
	if h.cache == nil {
		http.Error(w, "binary cache disabled", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*0)
	if r.Context() != nil {
		ctx = r.Context()
	}
	_ = cancel
	resp, err := h.cache.Resolve(ctx, &agentpb.ResolveArtifactRequest{
		Name:     parts[0],
		Version:  parts[1],
		Filename: parts[2],
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	url := resp.GetDownloadUrl()
	if url == "" {
		http.Error(w, "artifact storage_uri empty", http.StatusNotFound)
		return
	}
	// 302 to presigned/storage URL (works for both S3-presigned and public).
	http.Redirect(w, r, url, http.StatusFound)
}
