package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

// buildShareSnapshot freezes a run into the snapshot-shaped JSON the share
// viewer renders (same shape as runStatus): nodes[] from the live Temporal
// state, plus state.run_config / started_at from the persisted record.
func (s *Server) buildShareSnapshot(ctx context.Context, rec *types.RunRecord) []byte {
	nodes := []nodeStatus{}
	if st, err := s.app.Status(ctx, rec.ID); err == nil && st != nil {
		nodes = stagesToNodes(st.GetStages())
	}
	cfgJSON, _ := json.Marshal(rec.Cfg)
	snap := map[string]any{
		"graph":      "",
		"nodes":      nodes,
		"started_at": rec.CreatedAt,
		"state": map[string]any{
			"provider":   rec.Provider,
			"run_config": json.RawMessage(cfgJSON),
			"targets":    []any{},
		},
	}
	b, _ := json.Marshal(snap)
	return b
}

// createShareLink freezes the run snapshot + metrics into a read-only share record.
func (s *Server) createShareLink(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	userID := auth.GetClaims(r.Context()).UserID
	runID := chi.URLParam(r, "runID")

	// Load run record.
	rec, err := s.runs.Get(r.Context(), tenantID, runID)
	if err != nil || rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}

	// Collect metrics.
	accountID, _ := s.tenantAccountID(r.Context(), tenantID)
	dbKind := recDBKind(rec)
	collector := s.metricsCollector(accountID, dbKind)

	winStart, winEnd := s.runTimeWindow(r.Context(), rec)
	start := winStart.Add(-30 * time.Second)
	end := winEnd.Add(30 * time.Second)
	if end.Before(start) || end.IsZero() {
		end = time.Now()
	}

	runMetrics, err := collector.Collect(r.Context(), runID, metrics.TimeRange{Start: start, End: end})
	if err != nil {
		s.logger.Warn("share: failed to collect metrics", zap.Error(err))
		// Continue with empty metrics — the record is still valuable.
		runMetrics = &metrics.RunMetrics{}
	}

	metricsJSON, _ := json.Marshal(runMetrics)
	// Freeze a snapshot-shaped blob the share viewer renders: nodes[] (live
	// from Temporal), plus state.run_config / started_at from the record.
	snapJSON := s.buildShareSnapshot(r.Context(), rec)

	// Generate token.
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	q := pgdb.New(s.pool)
	if err := q.CreateSharedRun(r.Context(), pgdb.CreateSharedRunParams{
		Token:     token,
		RunID:     runID,
		TenantID:  tenantID,
		Snapshot:  string(snapJSON),
		Metrics:   string(metricsJSON),
		CreatedBy: userID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create share: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
		"url":   "/share/" + token,
	})
}

// getSharedRun returns the frozen snapshot + metrics for a share token. No auth required.
func (s *Server) getSharedRun(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	q := pgdb.New(s.pool)
	row, err := q.GetSharedRun(r.Context(), token)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// Return as a single JSON object with both snapshot and metrics.
	resp := struct {
		RunID     string          `json:"run_id"`
		Snapshot  json.RawMessage `json:"snapshot"`
		Metrics   json.RawMessage `json:"metrics"`
		CreatedAt string          `json:"created_at"`
	}{
		RunID:     row.RunID,
		Snapshot:  json.RawMessage(row.Snapshot),
		Metrics:   json.RawMessage(row.Metrics),
		CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(resp)
}
