package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

type runPresetItem struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	DBKind      string          `json:"db_kind"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

func runPresetToItem(p postgres.RunPreset) runPresetItem {
	return runPresetItem{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		DBKind:      p.DBKind,
		Config:      p.Config,
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

type runPresetReq struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	DBKind      string          `json:"db_kind"`
	Config      json.RawMessage `json:"config"`
}

// stripIdentityFromConfig removes id/name/description from a RunConfig
// payload before persisting it as a template. Identity belongs to runs and
// presets separately; storing it inside the saved config would force every
// load to overwrite it.
func stripIdentityFromConfig(raw json.RawMessage) json.RawMessage {
	var cfg types.RunConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return raw // best-effort; let the validator catch malformed payloads
	}
	cfg.ID = ""
	cfg.Name = ""
	cfg.Description = ""
	cfg.RunPresetID = ""
	cfg.SuiteID = ""
	out, err := json.Marshal(&cfg)
	if err != nil {
		return raw
	}
	return out
}

func (s *Server) listRunPresets(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	storage := postgres.NewRunPresetStorage(s.pool)
	dbKind := r.URL.Query().Get("db_kind")

	rows, err := storage.List(r.Context(), tenantID, dbKind)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]runPresetItem, 0, len(rows))
	for _, p := range rows {
		out = append(out, runPresetToItem(p))
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

func (s *Server) getRunPreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	storage := postgres.NewRunPresetStorage(s.pool)
	p, err := storage.Get(r.Context(), tenantID, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, runPresetToItem(*p))
}

func (s *Server) createRunPreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	var req runPresetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.DBKind == "" || len(req.Config) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, db_kind, config required"})
		return
	}
	storage := postgres.NewRunPresetStorage(s.pool)
	id := uuid.New().String()
	if err := storage.Create(r.Context(), postgres.RunPreset{
		ID: id, TenantID: tenantID,
		Name: req.Name, Description: req.Description, DBKind: req.DBKind,
		Config: stripIdentityFromConfig(req.Config),
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "name already exists or db error: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *Server) updateRunPreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	var req runPresetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	storage := postgres.NewRunPresetStorage(s.pool)
	existing, err := storage.Get(r.Context(), tenantID, id)
	if err != nil || existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if req.Name == "" {
		req.Name = existing.Name
	}
	cfg := existing.Config
	if len(req.Config) > 0 {
		cfg = stripIdentityFromConfig(req.Config)
	}
	if err := storage.Update(r.Context(), postgres.RunPreset{
		ID: id, TenantID: tenantID,
		Name: req.Name, Description: req.Description,
		DBKind: existing.DBKind, Config: cfg,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteRunPreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	storage := postgres.NewRunPresetStorage(s.pool)
	if err := storage.Delete(r.Context(), tenantID, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
