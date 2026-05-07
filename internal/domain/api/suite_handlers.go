package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

// --- response shapes ---

type suiteResp struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	Description      string                    `json:"description"`
	Policy           postgres.SuitePolicy      `json:"policy"`
	Comparison       postgres.ComparisonConfig `json:"comparison"`
	DBPresetIDs      []string                  `json:"db_preset_ids"`
	Provider         string                    `json:"provider"`
	PlatformID       string                    `json:"platform_id,omitempty"`
	Network          json.RawMessage           `json:"network"`
	StroppyMachine   json.RawMessage           `json:"stroppy_machine"`
	CronExpr         string                    `json:"cron_expr,omitempty"`
	Timezone         string                    `json:"timezone"`
	Enabled          bool                      `json:"enabled"`
	NextFireAt       *time.Time                `json:"next_fire_at,omitempty"`
	LastFireAt       *time.Time                `json:"last_fire_at,omitempty"`
	LastBatchID      string                    `json:"last_batch_id,omitempty"`
	ConcurrentPolicy string                    `json:"concurrent_policy"`
	CatchupMode      string                    `json:"catchup_mode"`
	RetentionRuns    int                       `json:"retention_runs"`
	CreatedAt        string                    `json:"created_at"`
	UpdatedAt        string                    `json:"updated_at"`
	Items            []suiteItemResp           `json:"items,omitempty"`
	Batches          []suiteBatchSummary       `json:"batches,omitempty"`
}

type suiteItemResp struct {
	ID        string          `json:"id"`
	SuiteID   string          `json:"suite_id"`
	Position  int             `json:"position"`
	Name      string          `json:"name"`
	Workload  json.RawMessage `json:"workload"`
	Enabled   bool            `json:"enabled"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

type suiteBatchSummary struct {
	BatchID   string          `json:"batch_id"`
	Trigger   string          `json:"trigger,omitempty"`
	FireAt    *time.Time      `json:"fire_at,omitempty"`
	CreatedAt string          `json:"created_at"`
	Total     int             `json:"total"`
	Finished  int             `json:"finished"`
	Failed    int             `json:"failed"`
	Cancelled int             `json:"cancelled"`
	Running   int             `json:"running"`
	Queued    int             `json:"queued"`
	Runs      []suiteRunBrief `json:"runs"`
}

type suiteRunBrief struct {
	RunID       string `json:"run_id"`
	SuiteItemID string `json:"suite_item_id,omitempty"`
	DBPresetID  string `json:"db_preset_id,omitempty"`
	Position    int    `json:"position"`
	State       string `json:"state"`
}

// --- request shapes ---

type suiteReq struct {
	Name             string                     `json:"name"`
	Description      string                     `json:"description"`
	Policy           *postgres.SuitePolicy      `json:"policy,omitempty"`
	Comparison       *postgres.ComparisonConfig `json:"comparison,omitempty"`
	DBPresetIDs      []string                   `json:"db_preset_ids,omitempty"`
	Provider         string                     `json:"provider,omitempty"`
	PlatformID       string                     `json:"platform_id,omitempty"`
	Network          json.RawMessage            `json:"network,omitempty"`
	StroppyMachine   json.RawMessage            `json:"stroppy_machine,omitempty"`
	CronExpr         string                     `json:"cron_expr,omitempty"`
	Timezone         string                     `json:"timezone,omitempty"`
	Enabled          *bool                      `json:"enabled,omitempty"`
	ConcurrentPolicy string                     `json:"concurrent_policy,omitempty"`
	CatchupMode      string                     `json:"catchup_mode,omitempty"`
	RetentionRuns    int                        `json:"retention_runs,omitempty"`
}

type suiteItemReq struct {
	Name     string          `json:"name"`
	Workload json.RawMessage `json:"workload"`
	Position *int            `json:"position,omitempty"`
	Enabled  *bool           `json:"enabled,omitempty"`
}

type reorderReq struct {
	Order map[string]int `json:"order"`
}

// --- helpers ---

func suiteToResp(su postgres.Suite) suiteResp {
	r := suiteResp{
		ID: su.ID, Name: su.Name, Description: su.Description,
		Policy: su.Policy, Comparison: su.Comparison,
		DBPresetIDs: su.DBPresetIDs,
		Provider:    su.Provider, PlatformID: su.PlatformID,
		Network: su.Network, StroppyMachine: su.StroppyMachine,
		CronExpr: su.CronExpr, Timezone: su.Timezone, Enabled: su.Enabled,
		NextFireAt: su.NextFireAt, LastFireAt: su.LastFireAt,
		LastBatchID:      su.LastBatchID,
		ConcurrentPolicy: su.ConcurrentPolicy, CatchupMode: su.CatchupMode,
		RetentionRuns: su.RetentionRuns,
		CreatedAt:     su.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     su.UpdatedAt.Format(time.RFC3339),
	}
	if r.Policy.Mode == "" {
		r.Policy = postgres.DefaultSuitePolicy()
	}
	if r.DBPresetIDs == nil {
		r.DBPresetIDs = []string{}
	}
	if len(r.Network) == 0 {
		r.Network = json.RawMessage(`{}`)
	}
	if len(r.StroppyMachine) == 0 {
		r.StroppyMachine = json.RawMessage(`{}`)
	}
	return r
}

func itemToResp(it postgres.SuiteItem) suiteItemResp {
	wl := it.Workload
	if len(wl) == 0 {
		wl = json.RawMessage("{}")
	}
	return suiteItemResp{
		ID: it.ID, SuiteID: it.SuiteID, Position: it.Position,
		Name: it.Name, Workload: wl, Enabled: it.Enabled,
		CreatedAt: it.CreatedAt.Format(time.RFC3339),
		UpdatedAt: it.UpdatedAt.Format(time.RFC3339),
	}
}

// computeNextFire wraps the cron parser with the suite's timezone.
func computeNextFire(cronExpr, tz string, from time.Time) *time.Time {
	if cronExpr == "" {
		return nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil || loc == nil {
		loc = time.UTC
	}
	next, err := nextCronFire(cronExpr, from.In(loc))
	if err != nil {
		return nil
	}
	utc := next.UTC()
	return &utc
}

// --- suite CRUD ---

func (s *Server) listSuites(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	st := postgres.NewSuiteStorage(s.pool)
	rows, err := st.List(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]suiteResp, 0, len(rows))
	for _, su := range rows {
		out = append(out, suiteToResp(su))
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
	resp := suiteToResp(*su)
	if items, err := st.ListItems(r.Context(), tenantID, id); err == nil {
		for _, it := range items {
			resp.Items = append(resp.Items, itemToResp(it))
		}
	}
	if s.scheduler != nil {
		if jobs, err := s.scheduler.Jobs().ListBySuite(r.Context(), tenantID, id); err == nil {
			resp.Batches = summariseBatches(jobs)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) createSuite(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	var req suiteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	if req.CronExpr != "" {
		if _, err := nextCronFire(req.CronExpr, time.Now()); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cron_expr: " + err.Error()})
			return
		}
	}
	st := postgres.NewSuiteStorage(s.pool)
	id := uuid.New().String()
	policy := postgres.DefaultSuitePolicy()
	if req.Policy != nil {
		policy = *req.Policy
	}
	cmp := postgres.ComparisonConfig{}
	if req.Comparison != nil {
		cmp = *req.Comparison
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	provider := req.Provider
	if provider == "" {
		provider = "docker"
	}
	su := postgres.Suite{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description,
		Policy: policy, Comparison: cmp,
		DBPresetIDs: req.DBPresetIDs,
		Provider:    provider, PlatformID: req.PlatformID,
		Network: req.Network, StroppyMachine: req.StroppyMachine,
		CronExpr: req.CronExpr, Timezone: tz, Enabled: enabled,
		NextFireAt:       computeNextFire(req.CronExpr, tz, time.Now()),
		ConcurrentPolicy: defaultStr(req.ConcurrentPolicy, "forbid"),
		CatchupMode:      defaultStr(req.CatchupMode, "skip"),
		RetentionRuns:    intDefault(req.RetentionRuns, 30),
	}
	if err := st.Create(r.Context(), su); err != nil {
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
	if req.CronExpr != "" {
		if _, err := nextCronFire(req.CronExpr, time.Now()); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cron_expr: " + err.Error()})
			return
		}
	}
	updated := *existing
	if req.Name != "" {
		updated.Name = req.Name
	}
	updated.Description = req.Description
	if req.Policy != nil {
		updated.Policy = *req.Policy
	}
	if req.Comparison != nil {
		updated.Comparison = *req.Comparison
	}
	if req.DBPresetIDs != nil {
		updated.DBPresetIDs = req.DBPresetIDs
	}
	if req.Provider != "" {
		updated.Provider = req.Provider
	}
	updated.PlatformID = req.PlatformID
	if len(req.Network) > 0 {
		updated.Network = req.Network
	}
	if len(req.StroppyMachine) > 0 {
		updated.StroppyMachine = req.StroppyMachine
	}
	cronChanged := req.CronExpr != updated.CronExpr
	updated.CronExpr = req.CronExpr
	if req.Timezone != "" {
		updated.Timezone = req.Timezone
	}
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if req.ConcurrentPolicy != "" {
		updated.ConcurrentPolicy = req.ConcurrentPolicy
	}
	if req.CatchupMode != "" {
		updated.CatchupMode = req.CatchupMode
	}
	if req.RetentionRuns > 0 {
		updated.RetentionRuns = req.RetentionRuns
	}
	if cronChanged || req.Enabled != nil {
		updated.NextFireAt = nil
		if updated.Enabled && updated.CronExpr != "" {
			updated.NextFireAt = computeNextFire(updated.CronExpr, updated.Timezone, time.Now())
		}
	}
	if err := st.Update(r.Context(), updated); err != nil {
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

// --- suite items (workloads) CRUD ---

func (s *Server) listSuiteItems(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	st := postgres.NewSuiteStorage(s.pool)
	items, err := st.ListItems(r.Context(), tenantID, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]suiteItemResp, 0, len(items))
	for _, it := range items {
		out = append(out, itemToResp(it))
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

func (s *Server) createSuiteItem(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	suiteID := chi.URLParam(r, "id")
	var req suiteItemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Workload) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workload required"})
		return
	}
	// Sanity-check the workload decodes as StroppyConfig.
	var probe types.StroppyConfig
	if err := json.Unmarshal(req.Workload, &probe); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid workload: " + err.Error()})
		return
	}
	st := postgres.NewSuiteStorage(s.pool)
	pos := -1
	if req.Position != nil {
		pos = *req.Position
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	itemID := uuid.New().String()
	if err := st.CreateItem(r.Context(), postgres.SuiteItem{
		ID: itemID, SuiteID: suiteID, TenantID: tenantID,
		Position: pos, Name: req.Name,
		Workload: req.Workload, Enabled: enabled,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": itemID})
}

func (s *Server) updateSuiteItem(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	itemID := chi.URLParam(r, "itemID")
	var req suiteItemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	st := postgres.NewSuiteStorage(s.pool)
	existing, err := st.GetItem(r.Context(), tenantID, itemID)
	if err != nil || existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if len(req.Workload) > 0 {
		var probe types.StroppyConfig
		if err := json.Unmarshal(req.Workload, &probe); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid workload: " + err.Error()})
			return
		}
		existing.Workload = req.Workload
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := st.UpdateItem(r.Context(), *existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteSuiteItem(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	itemID := chi.URLParam(r, "itemID")
	st := postgres.NewSuiteStorage(s.pool)
	if err := st.DeleteItem(r.Context(), tenantID, itemID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) reorderSuiteItems(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	suiteID := chi.URLParam(r, "id")
	var req reorderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Order) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order required"})
		return
	}
	st := postgres.NewSuiteStorage(s.pool)
	if err := st.ReorderItems(r.Context(), tenantID, suiteID, req.Order); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reordered"})
}

// --- launch + cancel ---

func (s *Server) launchSuite(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	if s.scheduler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "scheduler not initialised"})
		return
	}
	batchID, count, skipped, err := s.runSuiteOnce(r.Context(), tenantID, id, "manual", nil)
	if err != nil {
		writeJSON(w, errStatusOr(err, http.StatusInternalServerError), map[string]string{"error": err.Error()})
		return
	}
	resp := map[string]any{
		"suite_id": id, "batch_id": batchID, "items": count,
	}
	if len(skipped) > 0 {
		resp["skipped"] = skipped
	}
	writeJSON(w, http.StatusAccepted, resp)
}

// runSuiteOnce expands the suite's (db_preset × workload) matrix into
// job_runs. One launch produces len(suite.DBPresetIDs) * len(enabled items)
// rows minus any (preset, workload) combinations whose script is not
// supported by that preset's protocol — those are skipped with a warning
// rather than failing the whole launch.
//
// Returns (batchID, enqueuedCount, skippedPairs, error). On the cron path,
// fireAt becomes part of the per-row idempotency key so a duplicate cron
// tick across two servers cannot double-enqueue any matrix cell.
func (s *Server) runSuiteOnce(
	ctx context.Context, tenantID, suiteID, trigger string, fireAt *time.Time,
) (string, int, []string, error) {
	st := postgres.NewSuiteStorage(s.pool)
	su, err := st.Get(ctx, tenantID, suiteID)
	if err != nil {
		return "", 0, nil, err
	}
	if su == nil {
		return "", 0, nil, &enqueueError{code: http.StatusNotFound, err: errors.New("suite not found")}
	}
	if len(su.DBPresetIDs) == 0 {
		return "", 0, nil, &enqueueError{code: http.StatusBadRequest, err: errors.New("suite has no database presets configured")}
	}
	items, err := st.ListItems(ctx, tenantID, suiteID)
	if err != nil {
		return "", 0, nil, err
	}
	enabled := items[:0]
	for _, it := range items {
		if it.Enabled {
			enabled = append(enabled, it)
		}
	}
	if len(enabled) == 0 {
		return "", 0, nil, &enqueueError{code: http.StatusBadRequest, err: errors.New("suite has no enabled workloads")}
	}

	// concurrent_policy=forbid: skip if any prior batch row is still alive.
	if su.ConcurrentPolicy == "forbid" && su.LastBatchID != "" {
		prev, _ := s.scheduler.Jobs().ListByBatch(ctx, tenantID, su.LastBatchID)
		for _, j := range prev {
			if j.State == postgres.JobStateQueued ||
				j.State == postgres.JobStateClaimed ||
				j.State == postgres.JobStateRunning {
				return "", 0, nil, &enqueueError{
					code: http.StatusConflict,
					err:  errors.New("previous batch still running (concurrent_policy=forbid)"),
				}
			}
		}
	}

	policy := su.Policy
	if policy.Mode == "" {
		policy = postgres.DefaultSuitePolicy()
	}
	policyJSON, _ := json.Marshal(policy)

	// Pre-load every preset once; the cartesian below would otherwise hit
	// the DB N*M times.
	q := pgdb.New(s.pool)
	presetByID := map[string]pgdb.Preset{}
	for _, pid := range su.DBPresetIDs {
		row, err := q.GetPreset(ctx, pgdb.GetPresetParams{ID: pid, TenantID: tenantID})
		if err != nil {
			return "", 0, nil, &enqueueError{
				code: http.StatusBadRequest,
				err:  fmt.Errorf("preset %s not found: %w", pid, err),
			}
		}
		presetByID[pid] = row
	}

	// Decode shared infra blobs once.
	var network types.NetworkConfig
	if len(su.Network) > 0 {
		_ = json.Unmarshal(su.Network, &network)
	}
	if network.CIDR == "" {
		network.CIDR = "10.10.0.0/24"
	}
	var stroppyMachine *types.MachineSpec
	if len(su.StroppyMachine) > 0 && string(su.StroppyMachine) != "{}" {
		var m types.MachineSpec
		if err := json.Unmarshal(su.StroppyMachine, &m); err == nil {
			stroppyMachine = &m
		}
	}

	batchID := uuid.New().String()
	now := time.Now().UnixMilli()
	enqueued := 0
	var skipped []string

	pos := 0
	for _, pid := range su.DBPresetIDs {
		preset := presetByID[pid]
		kind := types.DatabaseKind(preset.DbKind)

		for _, it := range enabled {
			var workload types.StroppyConfig
			if err := json.Unmarshal(it.Workload, &workload); err != nil {
				return "", 0, nil, &enqueueError{
					code: http.StatusBadRequest,
					err:  fmt.Errorf("item %s: invalid workload: %w", it.ID, err),
				}
			}

			// Resolve protocol: explicit on workload wins, else kind default.
			protocol := workload.Protocol
			if protocol == "" {
				protocol = types.DefaultProtocol(kind)
			}

			// Compatibility skip — a single matrix can mix engines, so a
			// workload designed for postgres will simply be skipped on a
			// picodata target rather than failing the whole launch.
			if !types.ScriptSupported(kind, protocol, workload.Script) {
				skipped = append(skipped, fmt.Sprintf(
					"%s × %s (script %q not supported on %s/%s)",
					preset.Name, it.Name, workload.Script, kind, protocol,
				))
				continue
			}
			workload.Protocol = protocol
			if stroppyMachine != nil && workload.Machine == nil {
				workload.Machine = stroppyMachine
			}

			cfg := types.RunConfig{
				ID:         fmt.Sprintf("run-%d-%d", now, pos),
				Name:       fmt.Sprintf("%s × %s", preset.Name, it.Name),
				Provider:   types.Provider(su.Provider),
				PlatformID: su.PlatformID,
				Network:    network,
				Database:   types.DatabaseConfig{Kind: kind, Version: defaultDBVersion(kind)},
				Stroppy:    workload,
				PresetID:   pid,
				SuiteID:    suiteID,
			}

			// Same resolution pipeline as single-run enqueue: applies the
			// preset topology to cfg.Database, expands package, probes the
			// workload, fills machines.
			if err := s.resolveRunPreset(ctx, tenantID, &cfg); err != nil {
				return "", 0, nil, &enqueueError{
					code: http.StatusBadRequest,
					err:  fmt.Errorf("preset %s × item %s: %w", pid, it.ID, err),
				}
			}
			if err := s.resolveRunPackage(ctx, tenantID, &cfg); err != nil {
				return "", 0, nil, &enqueueError{
					code: http.StatusBadRequest,
					err:  fmt.Errorf("preset %s × item %s package: %w", pid, it.ID, err),
				}
			}
			if err := s.probeRunWorkload(ctx, cfg); err != nil {
				return "", 0, nil, &enqueueError{
					code: http.StatusBadRequest,
					err:  fmt.Errorf("preset %s × item %s probe: %w", pid, it.ID, err),
				}
			}
			run.FillMachinesFromTopology(&cfg)

			cost := run.EstimateRunCost(cfg)
			cfgJSON, _ := json.Marshal(&cfg)
			if err := s.scheduler.Jobs().Enqueue(ctx, postgres.JobRun{
				RunID:       cfg.ID,
				TenantID:    tenantID,
				BatchID:     batchID,
				SuiteID:     suiteID,
				SuiteItemID: it.ID,
				DBPresetID:  pid,
				Position:    pos,
				Config:      cfgJSON,
				SuitePolicy: policyJSON,
				Trigger:     trigger,
				FireAt:      fireAt,
				Cost: postgres.JobCost{
					CPUs: cost.CPUs, MemoryMB: cost.MemoryMB, DiskGB: cost.DiskGB,
					VMCount: cost.VMCount, RunsRunning: cost.RunsRunning,
				},
			}); err != nil {
				return "", 0, nil, fmt.Errorf("enqueue cell (%s × %s): %w", pid, it.ID, err)
			}
			enqueued++
			pos++
		}
	}
	if enqueued == 0 {
		return "", 0, skipped, &enqueueError{
			code: http.StatusBadRequest,
			err:  errors.New("no compatible (preset × workload) pairs to launch"),
		}
	}
	return batchID, enqueued, skipped, nil
}

// defaultDBVersion is the engine version stamped onto every cell when the
// suite/workload doesn't override it. Mirrors the wizard's default version
// dropdown so cron-launched suites pick the same versions a human would.
func defaultDBVersion(kind types.DatabaseKind) string {
	switch kind {
	case types.DatabasePostgres:
		return "17"
	case types.DatabaseMySQL:
		return "8.4"
	case types.DatabaseMariaDB:
		return "11.4"
	case types.DatabasePicodata:
		return "25.3"
	case types.DatabaseYDB:
		return "25.2"
	case types.DatabaseYDBManaged:
		return "managed"
	case types.DatabaseCockroach:
		return "24.2"
	}
	return "latest"
}

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

// --- compare batch vs baseline ---

// compareBatch pairs runs in the requested batch against a baseline batch
// by (db_preset_id, suite_item_id) — the matrix cell coordinate. Stable
// across reorders and additions/removals as long as the cell identity is
// preserved.
func (s *Server) compareBatch(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	suiteID := chi.URLParam(r, "id")
	batchID := chi.URLParam(r, "batchID")
	baseline := r.URL.Query().Get("baseline")

	if s.scheduler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "scheduler not initialised"})
		return
	}
	current, err := s.scheduler.Jobs().ListByBatch(r.Context(), tenantID, batchID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baselineBatch, err := s.resolveBaselineBatch(r.Context(), tenantID, suiteID, batchID, baseline)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type pair struct {
		DBPresetID  string `json:"db_preset_id"`
		SuiteItemID string `json:"suite_item_id"`
		Position    int    `json:"position"`
		RunA        string `json:"run_a"`
		RunB        string `json:"run_b"`
	}
	type cellKey struct{ db, item string }
	baseByCell := map[cellKey]postgres.JobRun{}
	for _, j := range baselineBatch {
		baseByCell[cellKey{j.DBPresetID, j.SuiteItemID}] = j
	}
	out := make([]pair, 0, len(current))
	for _, j := range current {
		p := pair{
			DBPresetID: j.DBPresetID, SuiteItemID: j.SuiteItemID,
			Position: j.Position, RunA: j.RunID,
		}
		if b, ok := baseByCell[cellKey{j.DBPresetID, j.SuiteItemID}]; ok {
			p.RunB = b.RunID
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"suite_id":          suiteID,
		"batch_id":          batchID,
		"baseline_batch_id": baselineBatchID(baselineBatch),
		"pairs":             out,
	})
}

func (s *Server) resolveBaselineBatch(ctx context.Context, tenantID, suiteID, currentBatchID, strategy string) ([]postgres.JobRun, error) {
	if strategy == "" {
		return nil, nil
	}
	if strategy != "previous" && strategy != "first" {
		return s.scheduler.Jobs().ListByBatch(ctx, tenantID, strategy)
	}
	all, err := s.scheduler.Jobs().ListBySuite(ctx, tenantID, suiteID)
	if err != nil {
		return nil, err
	}
	type batchRow struct {
		batchID string
		when    time.Time
	}
	seen := map[string]batchRow{}
	for _, j := range all {
		if j.BatchID == "" || j.BatchID == currentBatchID {
			continue
		}
		b, ok := seen[j.BatchID]
		if !ok || j.CreatedAt.Before(b.when) {
			seen[j.BatchID] = batchRow{batchID: j.BatchID, when: j.CreatedAt}
		}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	var pick batchRow
	for _, b := range seen {
		if pick.batchID == "" {
			pick = b
			continue
		}
		if strategy == "previous" && b.when.After(pick.when) {
			pick = b
		}
		if strategy == "first" && b.when.Before(pick.when) {
			pick = b
		}
	}
	return s.scheduler.Jobs().ListByBatch(ctx, tenantID, pick.batchID)
}

// --- shared helpers ---

func summariseBatches(jobs []postgres.JobRun) []suiteBatchSummary {
	if len(jobs) == 0 {
		return nil
	}
	idx := map[string]*suiteBatchSummary{}
	order := []string{}
	for _, j := range jobs {
		if j.BatchID == "" {
			continue
		}
		b, ok := idx[j.BatchID]
		if !ok {
			b = &suiteBatchSummary{
				BatchID:   j.BatchID,
				Trigger:   j.Trigger,
				FireAt:    j.FireAt,
				CreatedAt: j.CreatedAt.Format(time.RFC3339),
			}
			idx[j.BatchID] = b
			order = append(order, j.BatchID)
		}
		b.Total++
		switch j.State {
		case postgres.JobStateFinished:
			b.Finished++
		case postgres.JobStateFailed:
			b.Failed++
		case postgres.JobStateCancelled:
			b.Cancelled++
		case postgres.JobStateRunning, postgres.JobStateClaimed:
			b.Running++
		case postgres.JobStateQueued:
			b.Queued++
		}
		b.Runs = append(b.Runs, suiteRunBrief{
			RunID: j.RunID, SuiteItemID: j.SuiteItemID,
			DBPresetID: j.DBPresetID,
			Position:   j.Position, State: string(j.State),
		})
	}
	out := make([]suiteBatchSummary, 0, len(order))
	for _, id := range order {
		out = append(out, *idx[id])
	}
	return out
}

func baselineBatchID(rows []postgres.JobRun) string {
	if len(rows) == 0 {
		return ""
	}
	return rows[0].BatchID
}

func errStatusOr(err error, fallback int) int {
	var ee *enqueueError
	if errors.As(err, &ee) {
		return ee.code
	}
	return fallback
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func intDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
