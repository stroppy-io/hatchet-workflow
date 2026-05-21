package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// agentInfo is the per-machine row the run-detail UI renders. Combines
// what the snapshot recorded at provisioning time (id/role/host) with the
// live registry state (last seen, alive). For yandex runs we additionally
// poke the agent's /health endpoint so the operator sees real liveness
// even between polls.
type agentInfo struct {
	MachineID    string `json:"machine_id"`
	Role         string `json:"role"`
	Host         string `json:"host,omitempty"`
	InternalHost string `json:"internal_host,omitempty"`
	AgentPort    int    `json:"agent_port,omitempty"`
	Registered   bool   `json:"registered"` // present in s.agents
	Healthy      bool   `json:"healthy"`    // /health responded <500
	HealthError  string `json:"health_error,omitempty"`
	LastSeenAt   string `json:"last_seen_at,omitempty"`
}

// runAgents returns the list of agents that belong to this run, with live
// status. Source of truth: the run's snapshot.state.targets — that's the
// VMs/containers terraform actually created. The runtime agents map adds
// "currently registered" liveness; for yandex we also issue a 1s /health
// probe to differentiate "registered but unreachable" from "lost touch".
func (s *Server) runAgents(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	snap, err := s.app.Storage().Load(r.Context(), tenantID, runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if snap == nil || snap.State == nil {
		writeJSON(w, http.StatusOK, []agentInfo{})
		return
	}

	provider := snap.State.Provider
	out := make([]agentInfo, 0, len(snap.State.Targets))

	s.agentsMu.RLock()
	registered := make(map[string]bool, len(s.agents))
	for id := range s.agents {
		registered[id] = true
	}
	s.agentsMu.RUnlock()

	for _, t := range snap.State.Targets {
		ai := agentInfo{
			MachineID:    t.ID,
			Role:         t.Role,
			Host:         t.Host,
			InternalHost: t.InternalHost,
			AgentPort:    t.AgentPort,
			Registered:   registered[t.ID],
		}
		// Liveness probe — kept short so the handler stays responsive even
		// for runs with many machines. Only meaningful for yandex (docker
		// agents don't expose a public /health since they live on the
		// server's network namespace).
		if provider == "yandex" {
			ai.Healthy, ai.HealthError = probeAgentHealth(r.Context(), t.Host, t.AgentPort)
		} else {
			// Docker: registered ⇒ healthy (poll loop is the heartbeat).
			ai.Healthy = ai.Registered
		}
		out = append(out, ai)
	}

	// If snapshot has nothing yet (e.g. run is still queued / pre-machines),
	// fall back to the run config so the UI at least shows the planned
	// targets with role labels.
	if len(out) == 0 && len(snap.State.RunConfig) > 0 {
		var cfg types.RunConfig
		if err := json.Unmarshal(snap.State.RunConfig, &cfg); err == nil {
			for _, m := range cfg.Machines {
				for i := 0; i < m.Count; i++ {
					out = append(out, agentInfo{
						MachineID: runID + "-" + string(m.Role) + "-" + itoa(i),
						Role:      string(m.Role),
					})
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, out)
}

// probeAgentHealth issues a 1s GET to the agent's /health and reports
// success when the request lands with any non-5xx status. Errors get
// surfaced verbatim so the UI can show "connection refused", "timeout",
// etc.
func probeAgentHealth(ctx context.Context, host string, port int) (bool, string) {
	if host == "" || port == 0 {
		return false, "host or port missing in target"
	}
	cl := &http.Client{Timeout: time.Second}
	url := "http://" + host + ":" + itoa(port) + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := cl.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return false, http.StatusText(resp.StatusCode)
	}
	return true, ""
}

// itoa is a tiny std-lib-free int-to-string helper to avoid an extra
// strconv import in the hot path.
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
