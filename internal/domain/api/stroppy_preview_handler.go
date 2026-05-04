package api

import (
	"encoding/json"
	"net/http"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

func (s *Server) stroppyConfigPreview(w http.ResponseWriter, r *http.Request) {
	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if cfg.Stroppy.ConfigOverrideJSON != "" {
		writeJSON(w, http.StatusOK, map[string]string{"stroppy_config": cfg.Stroppy.ConfigOverrideJSON})
		return
	}

	stroppySettings := types.DefaultStroppySettings()
	b, err := run.BuildStroppyConfigJSON(cfg.Stroppy, cfg.Database.Kind, "", 0, stroppySettings, cfg.ID, cfg.Database)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"stroppy_config": string(b)})
}
