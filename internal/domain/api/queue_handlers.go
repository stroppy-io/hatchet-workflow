package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
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
func formatTimePtr(v any) string { return formatTime(deref(v)) }
func deref(v any) any {
	if p, ok := v.(*any); ok && p != nil {
		return *p
	}
	return v
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

// listQueue returns every queued/claimed/running/recently-finished job for
// the tenant. Drives the Queue UI page and lets users see what's blocked
// behind a quota.
func (s *Server) listQueue(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	if s.scheduler == nil {
		writeJSON(w, http.StatusOK, []queueRow{})
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT run_id, COALESCE(batch_id,''), COALESCE(suite_id,''), position,
		       state, cost, COALESCE(error,''), config,
		       created_at, started_at, heartbeat_at
		FROM job_runs
		WHERE tenant_id=$1 AND state IN ('queued','claimed','running')
		ORDER BY priority DESC, created_at ASC`, tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := make([]queueRow, 0, 16)
	for rows.Next() {
		var (
			q         queueRow
			cost, cfg string
			started   *interface{}
			hb        *interface{}
		)
		// Use generic *interface{} placeholders for nullable timestamps;
		// we rewrite to RFC3339 below when present.
		var started2, hb2 any
		started = &started2
		hb = &hb2
		var createdAt any
		if err := rows.Scan(&q.RunID, &q.BatchID, &q.SuiteID, &q.Position,
			&q.State, &cost, &q.Error, &cfg, &createdAt, started, hb); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(cost), &q.Cost)
		// Surface human-friendly run name pulled from the snapshotted RunConfig.
		var partial struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal([]byte(cfg), &partial)
		q.ConfigName = partial.Name
		q.CreatedAt = formatTime(createdAt)
		q.StartedAt = formatTimePtr(started)
		q.HeartbeatAt = formatTimePtr(hb)
		out = append(out, q)
	}
	writeJSON(w, http.StatusOK, out)
}

type quotasResp struct {
	Used    postgres.JobCost `json:"used"`
	Limits  postgres.JobCost `json:"limits"`
	Queue   int              `json:"queue_depth"`
	Running int              `json:"running_count"`
}

// getQuotas surfaces current resource usage + tenant limits + queue depth.
// Lets the UI tell users why their next run is queued instead of running.
func (s *Server) getQuotas(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	resp := quotasResp{}
	if s.scheduler == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	used, _ := s.scheduler.Jobs().GetUsed(r.Context(), tenantID)
	resp.Used = used
	q := s.settingsForTenant(tenantID).Quotas
	resp.Limits = postgres.JobCost{
		CPUs: q.MaxConcurrentCPUs, MemoryMB: q.MaxConcurrentMemoryMB,
		DiskGB: q.MaxConcurrentDiskGB, VMCount: q.MaxConcurrentVMs,
		RunsRunning: q.MaxConcurrentRuns,
	}
	if n, err := s.scheduler.Jobs().CountQueued(r.Context(), tenantID); err == nil {
		resp.Queue = n
	}
	resp.Running = used.RunsRunning
	writeJSON(w, http.StatusOK, resp)
}
