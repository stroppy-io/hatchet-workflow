package httpext

import (
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// PackagesHandler exposes legacy HTTP routes for package binary I/O.
//
//	POST /packages/{id}/deb  — multipart upload (form key "file")
//	GET  /packages/{id}/deb  — presigned download URL (302) or 404
type PackagesHandler struct {
	catalog *catalog.Service
	log     *zap.Logger
}

// NewPackagesHandler builds the multiplex.
func NewPackagesHandler(c *catalog.Service, log *zap.Logger) *PackagesHandler {
	return &PackagesHandler{catalog: c, log: log}
}

// Mount registers the routes on the supplied mux.
func (h *PackagesHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/packages/", h.dispatch)
}

func (h *PackagesHandler) dispatch(w http.ResponseWriter, r *http.Request) {
	// expected: /packages/{id}/deb
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/packages/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "deb" {
		http.Error(w, "expected /packages/{id}/deb", http.StatusNotFound)
		return
	}
	pkgID := &catalogpb.PackageId{Value: parts[0]}

	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(r.Context())}
	switch r.Method {
	case http.MethodPost:
		h.upload(w, r, pkgID, tenantID)
	case http.MethodGet:
		h.download(w, r, pkgID, tenantID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PackagesHandler) upload(w http.ResponseWriter, r *http.Request, pkgID *catalogpb.PackageId, tenantID *iampb.TenantId) {
	// Cap upload body at 1 GiB to prevent memory exhaustion.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "field 'file' required", http.StatusBadRequest)
		return
	}
	defer f.Close()

	key, err := h.catalog.UploadPackageBinary(r.Context(), pkgID, tenantID, fh.Filename, f, fh.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"key":"`+key+`"}`)
}

func (h *PackagesHandler) download(w http.ResponseWriter, r *http.Request, pkgID *catalogpb.PackageId, tenantID *iampb.TenantId) {
	// Filename is encoded in query string for now (UI knows from package row).
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "?filename= required", http.StatusBadRequest)
		return
	}
	url, err := h.catalog.PresignPackageDownload(r.Context(), pkgID, tenantID, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}
