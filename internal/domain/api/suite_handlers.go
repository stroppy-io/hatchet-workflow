package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
	Policy      postgres.SuitePolicy `json:"policy"`
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
	policy := s.Policy
	if policy.Mode == "" {
		policy = postgres.DefaultSuitePolicy()
	}
	return suiteItem{
		ID: s.ID, Name: s.Name, Description: s.Description, Items: items,
		Policy:    policy,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
	}
}

type suiteReq struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Items       []postgres.SuiteItem  `json:"items"`
	Policy      *postgres.SuitePolicy `json:"policy,omitempty"`
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
	policy := postgres.DefaultSuitePolicy()
	if req.Policy != nil {
		policy = *req.Policy
	}
	if err := st.Create(r.Context(), postgres.Suite{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description, Items: req.Items,
		Policy: policy,
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
	policy := existing.Policy
	if req.Policy != nil {
		policy = *req.Policy
	}
	if err := st.Update(r.Context(), postgres.Suite{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description, Items: req.Items,
		Policy: policy,
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
	if s.scheduler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "scheduler not initialised"})
		return
	}

	rpStorage := postgres.NewRunPresetStorage(s.pool)

	// Snapshot the suite's execution policy at launch time. Edits made
	// AFTER launch don't retroactively change the running batch.
	policy := su.Policy
	if policy.Mode == "" {
		policy = postgres.DefaultSuitePolicy()
	}
	policyJSON, _ := json.Marshal(policy)

	// Resolve every step's merged config FIRST. Validation happens here so
	// we fail the launch before any DB writes if any preset is missing or
	// any overrides JSON is malformed.
	batchID := uuid.New().String()
	now := time.Now().UnixMilli()

	type planned struct {
		runID    string
		position int
		cfg      types.RunConfig
	}
	var plan []planned

	for i, it := range su.Items {
		rp, err := rpStorage.Get(r.Context(), tenantID, it.RunPresetID)
		if err != nil || rp == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("suite item %d: run preset %q not found", i, it.RunPresetID),
			})
			return
		}
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
		cfg.ID = fmt.Sprintf("run-%d-s%d", now, i)
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
		plan = append(plan, planned{runID: cfg.ID, position: i, cfg: cfg})
	}

	// Persist the entire batch as queued job_runs. Sequential ordering is
	// enforced inside ClaimNext via a position-blockers check (a step at
	// position N is only claimable once all earlier steps are terminal).
	// suite_runs index row is inserted alongside so the existing UI lookup
	// keeps working.
	for _, p := range plan {
		cfg := p.cfg
		// Resolve preset/package/probe + quota + cost — same pipeline as
		// single-run enqueue.
		if err := s.resolveRunPreset(r.Context(), tenantID, &cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("step %d: %s", p.position, err.Error())})
			return
		}
		if err := s.resolveRunPackage(r.Context(), tenantID, &cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("step %d: %s", p.position, err.Error())})
			return
		}
		if err := s.probeRunWorkload(r.Context(), cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("step %d probe: %s", p.position, err.Error())})
			return
		}
		run.FillMachinesFromTopology(&cfg)

		cost := run.EstimateRunCost(cfg)
		cfgJSON, _ := json.Marshal(&cfg)
		if err := s.scheduler.Jobs().Enqueue(r.Context(), postgres.JobRun{
			RunID:       cfg.ID,
			TenantID:    tenantID,
			BatchID:     batchID,
			SuiteID:     id,
			RunPresetID: cfg.RunPresetID,
			Position:    p.position,
			Config:      cfgJSON,
			SuitePolicy: policyJSON,
			Cost: postgres.JobCost{
				CPUs: cost.CPUs, MemoryMB: cost.MemoryMB, DiskGB: cost.DiskGB,
				VMCount: cost.VMCount, RunsRunning: cost.RunsRunning,
			},
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("enqueue step %d failed: %s", p.position, err.Error()),
			})
			return
		}
		_ = st.RecordRun(r.Context(), postgres.SuiteRunRow{
			SuiteID: id, BatchID: batchID, RunID: cfg.ID,
			TenantID: tenantID, Position: p.position,
		})
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"suite_id": id, "batch_id": batchID, "items": len(plan),
	})
}

// cancelBatch transitions every job in a launched batch to cancelled.
// Queued rows go straight to state=cancelled; running ones get their
// worker context cancel()'d via scheduler.CancelRun and finish naturally.
func (s *Server) cancelBatch(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	batchID := chi.URLParam(r, "batchID")
	if s.scheduler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "scheduler not initialised"})
		return
	}
	rows, err := s.scheduler.Jobs().ListByBatch(r.Context(), tenantID, batchID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cancelled := 0
	for _, j := range rows {
		if j.State == postgres.JobStateFinished || j.State == postgres.JobStateFailed || j.State == postgres.JobStateCancelled {
			continue
		}
		if ok, _ := s.scheduler.CancelRun(r.Context(), tenantID, j.RunID); ok {
			cancelled++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"suite_id": id, "batch_id": batchID, "cancelled": cancelled,
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
