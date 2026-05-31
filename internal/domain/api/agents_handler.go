package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
)

// agentInfo is the per-machine row the run-detail UI renders. Agents are now
// Temporal workers (no HTTP poll registry), so there is no live agent
// liveness to report; the row is derived from the run's planned topology.
type agentInfo struct {
	MachineID    string `json:"machine_id"`
	Role         string `json:"role"`
	Host         string `json:"host,omitempty"`
	InternalHost string `json:"internal_host,omitempty"`
	AgentPort    int    `json:"agent_port,omitempty"`
	Registered   bool   `json:"registered"`
	Healthy      bool   `json:"healthy"`
	HealthError  string `json:"health_error,omitempty"`
	LastSeenAt   string `json:"last_seen_at,omitempty"`
}

// runAgents returns the run's deployment targets. With agents running as
// Temporal workers there is no HTTP poll registry to query for live
// liveness; we derive the planned targets from the run's saved RunConfig
// (machine roles × counts). When the run record is missing we return an
// empty list rather than 404 so the UI degrades gracefully.
func (s *Server) runAgents(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	rec, err := s.runs.Get(r.Context(), tenantID, runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rec == nil {
		writeJSON(w, http.StatusOK, []agentInfo{})
		return
	}

	out := make([]agentInfo, 0)
	for _, m := range rec.Cfg.Machines {
		for i := 0; i < m.Count; i++ {
			out = append(out, agentInfo{
				MachineID: runID + "-" + string(m.Role) + "-" + itoa(i),
				Role:      string(m.Role),
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// itoa is a tiny std-lib-free int-to-string helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
