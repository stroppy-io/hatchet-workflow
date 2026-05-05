package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

type suiteItem struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Items       []postgres.SuiteItem `json:"items"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
	Runs        []suiteRunSummary    `json:"runs,omitempty"`
}

type suiteRunSummary struct {
	BatchID   string `json:"batch_id"`
	RunID     string `json:"run_id"`
	Position  int    `json:"position"`
	CreatedAt string `json:"created_at"`
}

func suiteToItem(s postgres.Suite) suiteItem {
	items := s.Items
	if items == nil {
		items = []postgres.SuiteItem{}
	}
	return suiteItem{
		ID: s.ID, Name: s.Name, Description: s.Description, Items: items,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
	}
}

type suiteReq struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Items       []postgres.SuiteItem `json:"items"`
}

func (s *Server) listSuites(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	st := postgres.NewSuiteStorage(s.pool)
	rows, err := st.List(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]suiteItem, 0, len(rows))
	for _, su := range rows {
		out = append(out, suiteToItem(su))
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

func (s *Server) getSuite(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	st := postgres.NewSuiteStorage(s.pool)
	su, err := st.Get(r.Context(), tenantID, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if su == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	item := suiteToItem(*su)
	if rows, err := st.ListRuns(r.Context(), tenantID, id); err == nil {
		for _, rr := range rows {
			item.Runs = append(item.Runs, suiteRunSummary{
				BatchID: rr.BatchID, RunID: rr.RunID, Position: rr.Position,
				CreatedAt: rr.CreatedAt.Format(time.RFC3339),
			})
		}
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) createSuite(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	var req suiteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	st := postgres.NewSuiteStorage(s.pool)
	id := uuid.New().String()
	if err := st.Create(r.Context(), postgres.Suite{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description, Items: req.Items,
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "create failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *Server) updateSuite(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	var req suiteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	st := postgres.NewSuiteStorage(s.pool)
	existing, err := st.Get(r.Context(), tenantID, id)
	if err != nil || existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if req.Name == "" {
		req.Name = existing.Name
	}
	if err := st.Update(r.Context(), postgres.Suite{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description, Items: req.Items,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteSuite(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	st := postgres.NewSuiteStorage(s.pool)
	if err := st.Delete(r.Context(), tenantID, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// suiteLaunchReq is the body for POST /suites/{id}/run. Each entry in
// `overrides` is a JSON patch object applied (top-level merged) on top of the
// resolved run-preset config for the corresponding suite item. Indexed by
// position; missing entries fall back to the suite's stored overrides.
type suiteLaunchReq struct {
	Overrides   map[int]json.RawMessage `json:"overrides,omitempty"`
	NamePrefix  string                  `json:"name_prefix,omitempty"`
	Description string                  `json:"description,omitempty"`
}

func (s *Server) launchSuite(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	var req suiteLaunchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = suiteLaunchReq{}
	}

	st := postgres.NewSuiteStorage(s.pool)
	su, err := st.Get(r.Context(), tenantID, id)
	if err != nil || su == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "suite not found"})
		return
	}
	if len(su.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "suite has no items"})
		return
	}

	rpStorage := postgres.NewRunPresetStorage(s.pool)

	// Build all configs up-front so we fail before launching anything if a
	// referenced run-preset is missing or malformed. Sequential launch
	// happens in a goroutine afterwards.
	type prepared struct {
		cfg      types.RunConfig
		position int
	}
	batchID := uuid.New().String()
	var plan []prepared

	for i, it := range su.Items {
		rp, err := rpStorage.Get(r.Context(), tenantID, it.RunPresetID)
		if err != nil || rp == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("suite item %d: run preset %q not found", i, it.RunPresetID),
			})
			return
		}
		// Stored item overrides first, then per-launch overrides. JSON merge
		// is shallow at the top level — the SPA sends partial RunConfig
		// patches like {"stroppy":{"vus":200}} and expects them to replace
		// the matching top-level subtree, which mirrors how `rerun_config`
		// behaves on the existing rerun flow.
		base := rp.Config
		if len(it.Overrides) > 0 {
			base = mergeJSONShallow(base, it.Overrides)
		}
		if patch, ok := req.Overrides[i]; ok && len(patch) > 0 {
			base = mergeJSONShallow(base, patch)
		}
		var cfg types.RunConfig
		if err := json.Unmarshal(base, &cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("suite item %d: invalid merged config: %s", i, err),
			})
			return
		}
		cfg.ID = fmt.Sprintf("run-%d-s%d", time.Now().UnixMilli(), i)
		cfg.SuiteID = id
		cfg.RunPresetID = it.RunPresetID
		if req.NamePrefix != "" {
			if cfg.Name == "" {
				cfg.Name = fmt.Sprintf("%s #%d", req.NamePrefix, i+1)
			} else {
				cfg.Name = fmt.Sprintf("%s — %s", req.NamePrefix, cfg.Name)
			}
		}
		if req.Description != "" {
			cfg.Description = req.Description
		}
		plan = append(plan, prepared{cfg: cfg, position: i})
	}

	// Launch sequentially in the background so each run completes (or fails)
	// before the next starts. Fire-and-forget to the executor; failures are
	// surfaced via the run snapshot.
	go func() {
		ctx := context.Background()
		for _, step := range plan {
			cfg := step.cfg
			cfg.ID = fmt.Sprintf("run-%d-s%d", time.Now().UnixMilli(), step.position)

			// Resolve preset topology, package, probe.
			if err := s.resolveRunPreset(ctx, tenantID, &cfg); err != nil {
				s.logger.Error("suite launch: resolve preset failed", zap.String("suite", id), zap.Int("pos", step.position), zap.Error(err))
				continue
			}
			if err := s.resolveRunPackage(ctx, tenantID, &cfg); err != nil {
				s.logger.Error("suite launch: resolve package failed", zap.String("suite", id), zap.Error(err))
				continue
			}
			run.FillMachinesFromTopology(&cfg)

			runCtx, cancel := context.WithCancel(ctx)
			s.runCancelsMu.Lock()
			s.runCancels[cfg.ID] = cancel
			s.runTenants[cfg.ID] = tenantID
			s.runCancelsMu.Unlock()

			_ = st.RecordRun(ctx, postgres.SuiteRunRow{
				SuiteID: id, BatchID: batchID, RunID: cfg.ID,
				TenantID: tenantID, Position: step.position,
			})

			activeRuns.Inc()
			done := make(chan struct{})
			go func(c types.RunConfig) {
				defer close(done)
				defer activeRuns.Dec()
				if err := s.app.Start(runCtx, tenantID, c); err != nil {
					s.logger.Error("suite run failed", zap.String("run_id", c.ID), zap.Error(err))
				}
			}(cfg)
			<-done

			s.runCancelsMu.Lock()
			delete(s.runCancels, cfg.ID)
			delete(s.runTenants, cfg.ID)
			s.runCancelsMu.Unlock()
			cancel()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"suite_id": id, "batch_id": batchID, "items": len(plan),
	})
}

// mergeJSONShallow performs a top-level merge: any key present in patch
// replaces the matching key in base. Other keys in base are preserved. Both
// inputs must be JSON objects; on parse failure base is returned untouched
// (keeps the launch flow tolerant of garbage).
func mergeJSONShallow(base, patch json.RawMessage) json.RawMessage {
	var b, p map[string]json.RawMessage
	if err := json.Unmarshal(base, &b); err != nil {
		return base
	}
	if err := json.Unmarshal(patch, &p); err != nil {
		return base
	}
	for k, v := range p {
		b[k] = v
	}
	out, err := json.Marshal(b)
	if err != nil {
		return base
	}
	return out
}
