package api

import (
	"time"

	"net/http"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

// formatTime renders pgx-scanned timestamps as RFC3339 for JSON. Accepts
// time.Time / *time.Time / nil so the same helper handles both nullable
// and required columns.
func formatTime(v any) string {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	case *time.Time:
		if t == nil || t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	}
	return ""
}

type queueRow struct {
	RunID       string           `json:"run_id"`
	BatchID     string           `json:"batch_id,omitempty"`
	SuiteID     string           `json:"suite_id,omitempty"`
	Position    int              `json:"position"`
	State       string           `json:"state"`
	Cost        postgres.JobCost `json:"cost"`
	CreatedAt   string           `json:"created_at"`
	StartedAt   string           `json:"started_at,omitempty"`
	HeartbeatAt string           `json:"heartbeat_at,omitempty"`
	Error       string           `json:"error,omitempty"`
	ConfigName  string           `json:"name,omitempty"`
}

// listQueue returns the tenant's non-terminal runs (pending/running). Drives
// the Queue UI page. Execution state now lives in Temporal, so each run's
// live status is queried best-effort; runs that are terminal (done/failed/
// cancelled) or unreachable are filtered out.
func (s *Server) listQueue(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	recs, err := s.runs.List(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]queueRow, 0, 16)
	for i := range recs {
		rec := &recs[i]
		state := "pending"
		if st, qerr := s.app.Status(r.Context(), rec.ID); qerr == nil && st != nil {
			state = statusString(st.GetStatus())
		}
		// Only surface non-terminal runs in the queue view.
		if state == "done" || state == "failed" || state == "cancelled" {
			continue
		}
		cost := run.EstimateRunCost(rec.Cfg)
		out = append(out, queueRow{
			RunID:      rec.ID,
			SuiteID:    rec.SuiteID,
			State:      state,
			ConfigName: rec.Name,
			CreatedAt:  formatTime(rec.CreatedAt),
			Cost: postgres.JobCost{
				CPUs: cost.CPUs, MemoryMB: cost.MemoryMB, DiskGB: cost.DiskGB,
				VMCount: cost.VMCount, RunsRunning: cost.RunsRunning,
			},
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type quotasResp struct {
	Used    postgres.JobCost `json:"used"`
	Limits  postgres.JobCost `json:"limits"`
	Queue   int              `json:"queue_depth"`
	Running int              `json:"running_count"`
}

// getQuotas surfaces tenant resource limits + a best-effort count of
// in-flight runs. Per-run live status comes from Temporal; the durable
// scheduler's accounting is gone, so "used" cost is summed from the configs
// of currently non-terminal runs.
func (s *Server) getQuotas(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	resp := quotasResp{}
	q := s.settingsForTenant(tenantID).Quotas
	resp.Limits = postgres.JobCost{
		CPUs: q.MaxConcurrentCPUs, MemoryMB: q.MaxConcurrentMemoryMB,
		DiskGB: q.MaxConcurrentDiskGB, VMCount: q.MaxConcurrentVMs,
		RunsRunning: q.MaxConcurrentRuns,
	}

	recs, err := s.runs.List(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	running, queued := 0, 0
	for i := range recs {
		rec := &recs[i]
		state := "pending"
		if st, qerr := s.app.Status(r.Context(), rec.ID); qerr == nil && st != nil {
			state = statusString(st.GetStatus())
		}
		switch state {
		case "running":
			running++
			cost := run.EstimateRunCost(rec.Cfg)
			resp.Used.CPUs += cost.CPUs
			resp.Used.MemoryMB += cost.MemoryMB
			resp.Used.DiskGB += cost.DiskGB
			resp.Used.VMCount += cost.VMCount
			resp.Used.RunsRunning += cost.RunsRunning
		case "pending":
			queued++
		}
	}
	resp.Queue = queued
	resp.Running = running
	writeJSON(w, http.StatusOK, resp)
}
