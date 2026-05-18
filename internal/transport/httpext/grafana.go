package httpext

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// GrafanaHandler serves the legacy /api/v1/grafana shape so SPA / iframe code
// keeps working without a Connect dependency. Source = env vars; the future
// path is SettingsService keys, but this avoids a per-request DB hit.
type GrafanaHandler struct {
	url        string
	dashboards []string
}

// NewGrafanaHandler reads GRAFANA_URL + GRAFANA_DASHBOARDS (comma list) and
// returns a configured handler.
func NewGrafanaHandler() *GrafanaHandler {
	url := os.Getenv("GRAFANA_URL")
	var dashboards []string
	if v := os.Getenv("GRAFANA_DASHBOARDS"); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				dashboards = append(dashboards, p)
			}
		}
	}
	return &GrafanaHandler{url: url, dashboards: dashboards}
}

// Mount registers /api/v1/grafana on the supplied mux.
func (h *GrafanaHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/grafana", h.serve)
}

func (h *GrafanaHandler) serve(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"url":           h.url,
		"embed_enabled": h.url != "",
		"dashboards":    h.dashboards,
	})
}
