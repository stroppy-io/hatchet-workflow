package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

// schedAPI is what Server needs from the scheduler. Defined locally so we
// don't pull the scheduler package into api/ (which would create a cycle:
// scheduler imports api.App as Runner).
type schedAPI interface {
	Jobs() *postgres.JobStorage
	CancelRun(ctx context.Context, tenantID, runID string) (bool, error)
}

// Server is the HTTP server exposing agent, external, and UI APIs.
type Server struct {
	app    *App
	logger *zap.Logger
	hub    *wsHub
	pool   *pgxpool.Pool

	jwtIssuer *auth.JWTIssuer

	// monitoringURL is the vmauth base URL (env MONITORING_URL).
	// Empty means monitoring is disabled.
	monitoringURL string
	// monitoringToken is the bearer token for vmauth (env MONITORING_TOKEN).
	monitoringToken string

	// grafanaURL is the Grafana base URL (env GRAFANA_URL).
	// Empty means Grafana integration is disabled.
	grafanaURL string

	// grafanaDashboards maps dashboard names to UIDs (hardcoded defaults).
	grafanaDashboards map[string]string

	// agentRegistry tracks connected agents by machine ID.
	agentsMu sync.RWMutex
	agents   map[string]agent.Target

	// runCancels tracks cancel functions for running DAG executions.
	runCancelsMu sync.Mutex
	runCancels   map[string]context.CancelFunc
	runTenants   map[string]string // runID → tenantID for concurrent run counting

	// stroppyVersionsCache caches GitHub releases to avoid rate limits.
	stroppyVersionsMu    sync.Mutex
	stroppyVersionsCache []string
	stroppyVersionsAt    time.Time

	// stroppyCommitsCache caches per-commit pre-releases (5min).
	stroppyCommitsMu    sync.Mutex
	stroppyCommitsCache []StroppyCommit
	stroppyCommitsAt    time.Time

	// cancelledRuns tracks runs that were explicitly cancelled by the user.
	cancelledRuns map[string]bool

	// pollClient is the command queue used for agent<->server communication.
	pollClient *agent.PollClient

	// spaFS serves the embedded SPA files. If nil, SPA is not served.
	spaFS http.FileSystem

	// scheduler owns the durable run queue + worker pool. Wired in by
	// main.go right after construction; runStart/launchSuite enqueue here
	// instead of running goroutines inline.
	scheduler schedAPI
}

// SetScheduler wires the durable scheduler. Must be called once before
// the HTTP server starts accepting requests.
func (s *Server) SetScheduler(sch schedAPI) {
	s.scheduler = sch
	// Forward the scheduler's instance UUID + DB pool into PollClient so
	// every dispatched command is durably persisted in agent_commands.
	if g, ok := sch.(interface{ InstanceID() string }); ok && s.pollClient != nil {
		s.pollClient.SetPool(s.pool, g.InstanceID())
	}
}

// SettingsResolver returns a closure the scheduler uses to resolve per-tenant
// settings (quota limits) at admission time.
func (s *Server) SettingsResolver() func(tenantID string) *types.ServerSettings {
	return func(tenantID string) *types.ServerSettings { return s.settingsForTenant(tenantID) }
}

// RecoverChecker returns a closure deciding whether a snapshot is
// recoverable. Provider-aware: docker checks ContainerIDs alive, yandex
// pings each agent's /health.
func (s *Server) RecoverChecker() func(state *dag.RunState) bool {
	return func(state *dag.RunState) bool {
		return s.canRecoverState(state)
	}
}

// NewServer creates an HTTP server backed by the App.
// monitoringURL is the vmauth base URL (empty = monitoring disabled).
// grafanaURL is the Grafana base URL (empty = Grafana integration disabled).
func NewServer(app *App, logger *zap.Logger, pool *pgxpool.Pool, jwtSecret, monitoringURL, monitoringToken, grafanaURL, listenAddr string) *Server {
	pc := agent.NewPollClient(logger)
	s := &Server{
		app:             app,
		logger:          logger,
		hub:             newWSHub(),
		agents:          make(map[string]agent.Target),
		runCancels:      make(map[string]context.CancelFunc),
		runTenants:      make(map[string]string),
		cancelledRuns:   make(map[string]bool),
		pollClient:      pc,
		pool:            pool,
		jwtIssuer:       auth.NewJWTIssuer(jwtSecret),
		monitoringURL:   monitoringURL,
		monitoringToken: monitoringToken,
		grafanaURL:      grafanaURL,
		grafanaDashboards: map[string]string{
			"system":   "stroppy-system",
			"postgres": "stroppy-postgres",
			"mysql":    "stroppy-mysql",
			"picodata": "stroppy-picodata",
			"ydb":      "stroppy-ydb",
			"stroppy":  "stroppy-metrics-v1",
			"compare":  "stroppy-compare",
		},
	}
	// Wire LogSink so executor logs stream to WebSocket clients and VictoriaLogs.
	if monitoringURL != "" {
		s.hub.victoriaLogs = victoria.NewLogsClient(monitoringURL, monitoringToken)
	}
	s.hub.logger = logger
	s.hub.accountIDResolver = func(runID string) int32 {
		id, _ := s.accountIDFromRunID(context.Background(), runID)
		return id
	}
	s.hub.tenantIDResolver = func(runID string) string {
		return s.tenantIDFromRunID(context.Background(), runID)
	}
	app.sink = s.hub
	// Wire settings getter so buildDeps can access current cloud settings from DB.
	app.settingsFunc = s.settingsForTenant
	// Wire monitoring config so buildDeps can derive OTLP/metrics endpoints.
	app.monitoringURL = monitoringURL
	app.monitoringToken = monitoringToken
	// Wire accountID resolver so buildDeps can set per-tenant victoria accountID.
	app.accountIDFunc = func(tenantID string) int32 {
		id, _ := s.tenantAccountID(context.Background(), tenantID)
		return id
	}
	// Wire PollClient as the agent client — all command dispatch goes through polling.
	app.client = pc
	// Wire listen address so Docker agents can reach the server.
	app.listenAddr = listenAddr
	// Wire JWT issuer for agent token generation.
	app.jwtIssuer = s.jwtIssuer
	return s
}

// Router returns the chi router with all routes mounted.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Use(auth.NewAuthMiddleware(s.jwtIssuer, s.pool))

	// --- Prometheus metrics ---
	r.Get("/metrics", metricsHandler().ServeHTTP)

	// --- Health ---
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// --- Agent API ---
	r.Route("/api/agent", func(r chi.Router) {
		r.Post("/register", s.agentRegister)
		r.Post("/poll", s.agentPoll)
		r.Post("/report", s.agentReport)
		r.Post("/logs-batch", s.agentLogBatch)
	})

	// --- Public share links (no auth) ---
	r.Get("/api/share/{token}", s.getSharedRun)

	// --- Auth (public — handled by isPublicPath in middleware) ---
	r.Post("/api/v1/auth/login", s.login)
	r.Post("/api/v1/auth/refresh", s.refresh)
	r.Post("/api/v1/auth/logout", s.logout)

	// --- Authenticated, no tenant required ---
	r.Get("/api/v1/auth/me", s.authMe)
	r.Post("/api/v1/auth/select-tenant", s.selectTenant)
	r.Put("/api/v1/auth/password", s.changePassword)

	// --- Grafana config (infrastructure, not tenant) ---
	r.Get("/api/v1/grafana", s.getGrafanaConfig)

	// --- Tenant-scoped ---
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.TenantRequired())

		// Viewer+
		r.Get("/packages", s.listPackages)
		r.Get("/packages/{id}", s.getPackage)
		r.Get("/packages/{id}/deb", s.downloadPackageDeb)
		r.Get("/runs", s.listRuns)
		r.Get("/queue", s.listQueue)
		r.Get("/quotas", s.getQuotas)
		r.Get("/run/{runID}/status", s.runStatus)
		r.Get("/run/{runID}/logs", s.runLogs)
		r.Get("/run/{runID}/metrics", s.runMetrics)
		r.Get("/run/{runID}/rendered-configs", s.runRenderedConfigs)
		r.Get("/run/{runID}/agents", s.runAgents)
		r.Get("/compare", s.compareRuns)
		r.Get("/stroppy-versions", s.stroppyVersions)
		r.Get("/stroppy-commits", s.stroppyCommits)
		r.Get("/presets", s.listPresetsTenant)
		r.Get("/presets/{id}", s.getPreset)
		r.Get("/run-presets", s.listRunPresets)
		r.Get("/run-presets/{id}", s.getRunPreset)
		r.Get("/suites", s.listSuites)
		r.Get("/suites/{id}", s.getSuite)
		r.Get("/settings", s.getSettings)

		// Operator+
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("operator"))
			r.Post("/run", s.runStart)
			r.Post("/validate", s.runValidate)
			r.Post("/dry-run", s.runDryRun)
			r.Post("/stroppy-config-preview", s.stroppyConfigPreview)
			r.Post("/probe", s.stroppyProbe)
			r.Delete("/run/{runID}", s.deleteRun)
			r.Post("/run/{runID}/cancel", s.cancelRun)
			r.Post("/run/{runID}/share", s.createShareLink)
			r.Put("/baseline/{name}", s.setBaseline)
			r.Get("/baseline/{name}", s.getBaseline)
			r.Get("/baselines", s.listBaselines)
			r.Post("/upload/deb", s.uploadPackage)
			r.Post("/upload/rpm", s.uploadPackage)
			r.Post("/presets", s.createPreset)
			r.Put("/presets/{id}", s.updatePreset)
			r.Delete("/presets/{id}", s.deletePreset)
			r.Post("/presets/{id}/clone", s.clonePreset)
			r.Post("/run-presets", s.createRunPreset)
			r.Put("/run-presets/{id}", s.updateRunPreset)
			r.Delete("/run-presets/{id}", s.deleteRunPreset)
			r.Post("/suites", s.createSuite)
			r.Put("/suites/{id}", s.updateSuite)
			r.Delete("/suites/{id}", s.deleteSuite)
			r.Post("/suites/{id}/run", s.launchSuite)
			r.Post("/suites/{id}/batches/{batchID}/cancel", s.cancelBatch)
			r.Post("/packages", s.createPackage)
			r.Put("/packages/{id}", s.updatePackage)
			r.Delete("/packages/{id}", s.deletePackage)
			r.Post("/packages/{id}/clone", s.clonePackage)
			r.Post("/packages/{id}/deb", s.uploadPackageDeb)
		})

		// Owner+
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("owner"))
			r.Put("/settings", s.updateSettings)
			r.Route("/tenant", func(r chi.Router) {
				r.Get("/members", s.listMembers)
				r.Post("/members", s.addMember)
				r.Put("/members/{userID}", s.updateMember)
				r.Delete("/members/{userID}", s.removeMember)
				r.Get("/tokens", s.listAPITokens)
				r.Post("/tokens", s.createAPIToken)
				r.Delete("/tokens/{id}", s.revokeAPIToken)
			})
		})
	})

	// --- Root only ---
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(auth.RequireRoot())
		r.Get("/tenants", s.listTenantsAdmin)
		r.Post("/tenants", s.createTenantAdmin)
		r.Delete("/tenants/{id}", s.deleteTenantAdmin)
		r.Get("/users", s.listUsersAdmin)
		r.Post("/users", s.createUserAdmin)
		r.Delete("/users/{id}", s.deleteUserAdmin)
		r.Put("/users/{id}/password", s.resetPasswordAdmin)
	})

	// --- UI WebSocket ---
	r.Get("/ws/logs", s.wsLogs)
	r.Get("/ws/logs/{runID}", s.wsLogsRun)

	// --- Package serving ---
	r.Get("/packages/*", http.StripPrefix("/packages/", http.FileServer(http.Dir(uploadDir))).ServeHTTP)

	// --- Agent binary download (public — agent has no token yet at download time) ---
	r.Get("/agent/binary", s.serveBinary)

	// --- Cached binary proxy (public — agents fetch supporting binaries
	//     like node_exporter / vmagent / stroppy through the server so we
	//     don't hammer github from every Yandex VM). Path layout:
	//         /api/binaries/{name}/{version}/{filename}
	//     The server caches each artifact on disk; subsequent agent
	//     requests stream straight from the cache.
	r.Get("/api/binaries/{name}/{version}/{filename}", s.serveCachedBinary)

	// --- SPA (embedded frontend) ---
	if s.spaFS != nil {
		r.Get("/*", s.serveSPA)
		// Unknown paths: SPA fallback for non-API, JSON 404 for API.
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			s.serveSPA(w, r)
		})
	}

	return metricsMiddleware(r)
}

// SetSPA configures the embedded SPA filesystem.
// Call with the sub-directory containing index.html (e.g. web.Dist "dist" subdir).
func (s *Server) SetSPA(fsys fs.FS) {
	s.spaFS = http.FS(fsys)
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	// Try to serve the exact file first.
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	f, err := s.spaFS.Open(path[1:]) // strip leading /
	if err == nil {
		f.Close()
		http.FileServer(s.spaFS).ServeHTTP(w, r)
		return
	}
	// File not found -> serve index.html for client-side routing.
	r.URL.Path = "/"
	http.FileServer(s.spaFS).ServeHTTP(w, r)
}

// ============================================================
// Agent API handlers
// ============================================================

// RegisterRequest is the payload sent by an agent on startup.
type RegisterRequest struct {
	MachineID string `json:"machine_id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
}

func (s *Server) agentRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MachineID string `json:"machine_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Pre-create the healthy channel so PollClient can detect this agent.
	s.pollClient.MarkAgentReady(req.MachineID)

	s.agentsMu.Lock()
	_, existed := s.agents[req.MachineID]
	s.agents[req.MachineID] = agent.Target{ID: req.MachineID}
	s.agentsMu.Unlock()
	if !existed {
		agentCount.Inc()
	}

	s.logger.Info("agent registered", zap.String("machine_id", req.MachineID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) agentPoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MachineID string `json:"machine_id"`
		Host      string `json:"host,omitempty"`
		Port      int    `json:"port,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Re-register this agent on every poll. Lets us rebuild the in-memory
	// agents map after a server restart without requiring agents to call
	// /register again — they already keep polling, so we lift identity out
	// of the poll body.
	if req.MachineID != "" {
		s.agentsMu.Lock()
		_, existed := s.agents[req.MachineID]
		s.agents[req.MachineID] = agent.Target{
			ID: req.MachineID, Host: req.Host, AgentPort: req.Port,
		}
		s.agentsMu.Unlock()
		if !existed {
			agentCount.Inc()
			s.logger.Info("agent observed via poll (re-registered)",
				zap.String("machine_id", req.MachineID))
		}
	}

	// Long-poll: block up to 60s waiting for a command.
	cmd := s.pollClient.Poll(req.MachineID, 60*time.Second)
	if cmd == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Audit the dispatch in agent_commands (best-effort — failure here
	// doesn't block the agent because the in-memory queue is the source of
	// truth right now). Audits let the orphan/reaper logic see what was
	// dispatched even after a server restart.
	if cmd.ID != "" && req.MachineID != "" {
		payload, _ := json.Marshal(cmd)
		go func(runID, machineID string, payload []byte) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := s.pool.Exec(ctx, `
				INSERT INTO agent_commands (run_id, machine_id, payload, state, claimed_by, claimed_at)
				VALUES ($1, $2, $3, 'claimed', $4, NOW())`,
				runID, machineID, string(payload), s.instanceID())
			if err != nil {
				s.logger.Debug("agent_commands insert failed", zap.Error(err))
			}
		}(extractRunID(req.MachineID), req.MachineID, payload)
	}

	writeJSON(w, http.StatusOK, cmd)
}

func (s *Server) agentReport(w http.ResponseWriter, r *http.Request) {
	var report agent.Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.logger.Info("agent report",
		zap.String("command_id", report.CommandID),
		zap.String("status", string(report.Status)),
	)

	// Route report to the waiting PollClient.Send() call.
	s.pollClient.DeliverReport(report)

	// Audit the completion in agent_commands. UPDATE is best-effort and
	// non-blocking — the in-memory channel above is what unblocks the
	// waiting task in this server lifetime.
	if report.CommandID != "" {
		go func(rep agent.Report) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			state := "done"
			if rep.Status == agent.ReportFailed {
				state = "failed"
			}
			_, err := s.pool.Exec(ctx, `
				UPDATE agent_commands
				SET state=$2, completed_at=NOW(), result=$3, error=$4
				WHERE payload::text LIKE '%' || $1 || '%' AND state IN ('pending','claimed')`,
				rep.CommandID, state, rep.Output, rep.Error)
			if err != nil {
				s.logger.Debug("agent_commands update failed", zap.Error(err))
			}
		}(report)
	}

	// Broadcast to WS clients.
	s.hub.broadcast(wsMessage{Type: "report", Payload: report})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// instanceID returns this server process's identifier for audit columns.
// Empty if scheduler isn't wired (test/CLI mode); the field then stores ”.
func (s *Server) instanceID() string {
	if s.scheduler == nil {
		return ""
	}
	type idGetter interface {
		InstanceID() string
	}
	if g, ok := s.scheduler.(idGetter); ok {
		return g.InstanceID()
	}
	return ""
}

func (s *Server) agentLogBatch(w http.ResponseWriter, r *http.Request) {
	var lines []agent.LogLine
	if err := json.NewDecoder(r.Body).Decode(&lines); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for i := range lines {
		line := &lines[i]
		runID := extractRunID(line.MachineID)
		tenantID := s.tenantIDFromRunID(r.Context(), runID)
		s.hub.broadcast(wsMessage{Type: "agent_log", RunID: runID, TenantID: tenantID, Payload: *line})

		if s.monitoringURL != "" {
			accountID, _ := s.accountIDFromRunID(r.Context(), runID)
			ll := *line // capture for goroutine
			go func() {
				vlClient := victoria.NewLogsClient(s.monitoringURL, s.monitoringToken)
				vlClient.IngestWithAccount(accountID, ll.MachineID, ll.CommandID, ll.Action, runID, ll.Stream, ll.Line)
			}()
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// External API handlers
// ============================================================

func (s *Server) runStart(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())

	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("run-%d", time.Now().UnixMilli())
	}
	if err := s.enqueueRun(r.Context(), tenantID, &cfg); err != nil {
		writeJSON(w, errStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": cfg.ID, "status": "queued"})
}

// enqueueError carries an HTTP status code so the runStart / launchSuite
// handlers can map quota / validation / conflict reasons cleanly.
type enqueueError struct {
	code int
	err  error
}

func (e *enqueueError) Error() string { return e.err.Error() }
func errStatus(err error) int {
	if e, ok := err.(*enqueueError); ok {
		return e.code
	}
	return http.StatusInternalServerError
}

// enqueueRun resolves preset/package/probe, validates quotas, persists the
// merged RunConfig in job_runs as queued. Used by both runStart (single)
// and the suite launcher.
func (s *Server) enqueueRun(ctx context.Context, tenantID string, cfg *types.RunConfig) error {
	if err := s.resolveRunPreset(ctx, tenantID, cfg); err != nil {
		return &enqueueError{code: http.StatusBadRequest, err: err}
	}
	if err := s.resolveRunPackage(ctx, tenantID, cfg); err != nil {
		return &enqueueError{code: http.StatusBadRequest, err: err}
	}
	if err := s.probeRunWorkload(ctx, *cfg); err != nil {
		return &enqueueError{code: http.StatusBadRequest, err: fmt.Errorf("workload probe failed: %w", err)}
	}
	run.FillMachinesFromTopology(cfg)

	settings := s.settingsForTenant(tenantID)
	if err := settings.Quotas.ValidateQuotas(cfg); err != nil {
		return &enqueueError{code: http.StatusForbidden, err: fmt.Errorf("quota exceeded: %w", err)}
	}
	if settings.Quotas.MaxQueueDepth > 0 && s.scheduler != nil {
		n, err := s.scheduler.Jobs().CountQueued(ctx, tenantID)
		if err == nil && n >= settings.Quotas.MaxQueueDepth {
			return &enqueueError{code: http.StatusTooManyRequests, err: fmt.Errorf("queue depth limit reached (%d/%d)", n, settings.Quotas.MaxQueueDepth)}
		}
	}
	if snap, _ := s.app.Storage().Load(ctx, tenantID, cfg.ID); snap != nil {
		return &enqueueError{code: http.StatusConflict, err: fmt.Errorf("run %q already exists", cfg.ID)}
	}
	if s.scheduler != nil {
		if existing, _ := s.scheduler.Jobs().Get(ctx, tenantID, cfg.ID); existing != nil {
			return &enqueueError{code: http.StatusConflict, err: fmt.Errorf("run %q already queued", cfg.ID)}
		}
	}

	cost := run.EstimateRunCost(*cfg)
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return &enqueueError{code: http.StatusInternalServerError, err: err}
	}
	if s.scheduler == nil {
		return &enqueueError{code: http.StatusServiceUnavailable, err: fmt.Errorf("scheduler not initialised")}
	}
	return s.scheduler.Jobs().Enqueue(ctx, postgres.JobRun{
		RunID:       cfg.ID,
		TenantID:    tenantID,
		BatchID:     cfg.SuiteID, // suites populate via launchSuite, not here
		SuiteID:     cfg.SuiteID,
		RunPresetID: cfg.RunPresetID,
		Position:    0,
		Config:      cfgJSON,
		Cost: postgres.JobCost{
			CPUs: cost.CPUs, MemoryMB: cost.MemoryMB, DiskGB: cost.DiskGB,
			VMCount: cost.VMCount, RunsRunning: cost.RunsRunning,
		},
	})
}

func (s *Server) runValidate(w http.ResponseWriter, r *http.Request) {
	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.app.Validate(cfg); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "valid"})
}

func (s *Server) runDryRun(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Resolve preset so dry-run sees the full topology.
	if err := s.resolveRunPreset(r.Context(), tenantID, &cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	graphJSON, resolvedCfg, err := s.app.DryRun(cfg)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	// Wrap graph + resolved config into a single response.
	var graph json.RawMessage = graphJSON
	cfgJSON, _ := json.Marshal(resolvedCfg)
	resp := struct {
		Graph           json.RawMessage              `json:"graph"`
		Nodes           json.RawMessage              `json:"nodes"`
		ResolvedConfig  json.RawMessage              `json:"resolved_config,omitempty"`
		EffectiveConfig map[string]map[string]string `json:"effective_config,omitempty"`
		StroppyConfig   string                       `json:"stroppy_config,omitempty"`
		RenderedConfigs map[string]string            `json:"rendered_configs,omitempty"`
	}{}
	// The graph JSON is the full graph object — extract nodes from it.
	var graphObj map[string]json.RawMessage
	if json.Unmarshal(graph, &graphObj) == nil {
		resp.Nodes = graphObj["nodes"]
	}
	resp.Graph = graph
	resp.ResolvedConfig = cfgJSON
	resp.EffectiveConfig = run.ComputeEffectiveConfigs(&cfg)
	resp.RenderedConfigs = run.BuildRenderedConfigs(resolvedCfg)

	// Build stroppy config preview. dbHost="" + dbPort=0 asks BuildStroppyConfigJSON
	// to emit sentinel tokens (substituted at run time) — the real DB endpoint is
	// only known once the DB is provisioned.
	if cfg.Stroppy.ConfigOverrideJSON != "" {
		resp.StroppyConfig = cfg.Stroppy.ConfigOverrideJSON
	} else {
		stroppySettings := types.DefaultStroppySettings()
		if b, err := run.BuildStroppyConfigJSON(cfg.Stroppy, cfg.Database.Kind, "", 0, stroppySettings, cfg.ID, cfg.Database); err == nil {
			resp.StroppyConfig = string(b)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) runStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	snap, err := s.app.storage.Load(r.Context(), tenantID, runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if snap == nil {
		// Run might still be queued (no snapshot yet — scheduler hasn't
		// claimed it). Synthesise a minimal snapshot from the job_runs row
		// so the UI can show "queued" instead of 404'ing.
		if s.scheduler != nil {
			if job, _ := s.scheduler.Jobs().Get(r.Context(), tenantID, runID); job != nil {
				writeJSON(w, http.StatusOK, queuedSnapshotShim(job))
				return
			}
		}
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Annotate with current job state so UI can surface queued/running/etc.
	if s.scheduler != nil {
		if job, _ := s.scheduler.Jobs().Get(r.Context(), tenantID, runID); job != nil {
			extra := map[string]any{
				"graph":          json.RawMessage(snap.GraphJSON),
				"nodes":          snap.Nodes,
				"state":          snap.State,
				"started_at":     snap.StartedAt,
				"finished_at":    snap.FinishedAt,
				"job_state":      string(job.State),
				"queue_position": job.Position,
				"batch_id":       job.BatchID,
			}
			writeJSON(w, http.StatusOK, extra)
			return
		}
	}
	writeJSON(w, http.StatusOK, snap)
}

// queuedSnapshotShim returns a minimal snapshot-shaped response for jobs
// that haven't started executing yet (no `runs` row exists). The UI
// reuses RunDetail rendering for this — empty nodes + job_state="queued".
func queuedSnapshotShim(job *postgres.JobRun) map[string]any {
	return map[string]any{
		"graph":          "",
		"nodes":          []any{},
		"state":          nil,
		"job_state":      string(job.State),
		"batch_id":       job.BatchID,
		"queue_position": job.Position,
		"created_at":     job.CreatedAt,
	}
}

// runRenderedConfigs returns the per-component config files that the agent
// rendered (or will render) for a given run — postgresql.conf, ydb.yaml,
// pgbouncer.ini, etc. Same shape as the dry-run preview, derived from the
// run's saved RunConfig so it works while a run is in progress.
func (s *Server) runRenderedConfigs(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	snap, err := s.app.storage.Load(r.Context(), tenantID, runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if snap == nil || snap.State == nil || len(snap.State.RunConfig) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var cfg types.RunConfig
	if err := json.Unmarshal(snap.State.RunConfig, &cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "decode run config: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"rendered_configs": run.BuildRenderedConfigs(&cfg),
	})
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())

	runs, err := s.app.Storage().List(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for i := range runs {
		if s.cancelledRuns[runs[i].ID] {
			runs[i].Cancelled = true
		}
	}
	// Append queued jobs that don't have a snapshot yet so they appear in
	// the runs list as "Queued" instead of being invisible until the
	// scheduler claims them.
	if s.scheduler != nil {
		known := make(map[string]bool, len(runs))
		for _, r2 := range runs {
			known[r2.ID] = true
		}
		rows, err := s.pool.Query(r.Context(), `
			SELECT run_id, COALESCE(batch_id,''), COALESCE(suite_id,''), COALESCE(run_preset_id,''),
			       config, created_at
			FROM job_runs
			WHERE tenant_id=$1 AND state='queued'
			ORDER BY created_at DESC`, tenantID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var (
					id, batchID, suiteID, runPresetID, cfgStr string
					createdAt                                 any
				)
				if err := rows.Scan(&id, &batchID, &suiteID, &runPresetID, &cfgStr, &createdAt); err != nil {
					continue
				}
				if known[id] {
					continue
				}
				var cfg types.RunConfig
				_ = json.Unmarshal([]byte(cfgStr), &cfg)
				summary := dag.RunSummary{
					ID: id, Total: 0, Done: 0, Pending: 1,
					DBKind: string(cfg.Database.Kind), Provider: string(cfg.Provider),
					Script: cfg.Stroppy.Script, Duration: cfg.Stroppy.Duration,
					VUs: cfg.Stroppy.VUs, DBVersion: cfg.Database.Version,
					PresetID: cfg.PresetID, Name: cfg.Name, Description: cfg.Description,
					SuiteID: suiteID, RunPresetID: runPresetID,
				}
				runs = append(runs, summary)
			}
		}
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	// Two cancel paths: if the scheduler owns this run (queued or running),
	// it short-circuits to either UPDATE state=cancelled (queued) or
	// cancel() the worker context (running). Legacy in-memory map is kept
	// as a fallback for tests / cases where the scheduler is not wired.
	if s.scheduler != nil {
		ok, err := s.scheduler.CancelRun(r.Context(), tenantID, runID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if ok {
			s.cancelledRuns[runID] = true
			writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling", "run_id": runID})
			return
		}
	}

	// Fallback: in-memory cancel map (recovery flow before durable jobs).
	if _, err := s.app.Storage().Load(r.Context(), tenantID, runID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}
	s.runCancelsMu.Lock()
	cancel, ok := s.runCancels[runID]
	s.runCancelsMu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not active or already finished"})
		return
	}
	s.logger.Info("cancelling run", zap.String("run_id", runID))
	cancel()
	s.cancelledRuns[runID] = true
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling", "run_id": runID})
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	// Check exists.
	snap, err := s.app.Storage().Load(r.Context(), tenantID, runID)
	if err != nil || snap == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Clean up Docker resources (best-effort).
	s.cleanupRunResources(runID)

	// Delete from storage.
	if err := s.app.Storage().Delete(r.Context(), tenantID, runID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "run_id": runID})
}

// cleanupRunResources removes Docker containers and networks associated with a run.
func (s *Server) cleanupRunResources(runID string) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		s.logger.Warn("cleanup: docker client failed", zap.Error(err))
		return
	}
	defer cli.Close()

	ctx := context.Background()

	// Remove containers matching this run (name pattern: stroppy-agent-{runID}-*)
	containers, _ := cli.ContainerList(ctx, container.ListOptions{All: true})
	prefix := fmt.Sprintf("stroppy-agent-%s-", runID)
	for _, c := range containers {
		for _, name := range c.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, prefix) {
				s.logger.Info("cleanup: removing container", zap.String("name", cleanName))
				cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			}
		}
	}

	// Remove network for this run (name pattern: stroppy-{runID})
	netName := fmt.Sprintf("stroppy-%s", runID)
	networks, _ := cli.NetworkList(ctx, network.ListOptions{})
	for _, n := range networks {
		if n.Name == netName {
			s.logger.Info("cleanup: removing network", zap.String("name", n.Name))
			cli.NetworkRemove(ctx, n.ID)
		}
	}
}

// RecoverOrCleanupRuns attempts to resume incomplete runs whose Docker containers
// are still alive. Runs that cannot be recovered are marked as failed and their
// resources are cleaned up. Called on server startup.
//
// For recovery, we query all runs across all tenants.
func (s *Server) RecoverOrCleanupRuns() {
	ctx := context.Background()

	// Query all runs across all tenants for recovery.
	rows, err := s.pool.Query(ctx, "SELECT id, tenant_id, snapshot FROM runs ORDER BY created_at DESC")
	if err != nil {
		s.logger.Warn("recovery: failed to list runs", zap.Error(err))
		return
	}
	defer rows.Close()

	type runInfo struct {
		id       string
		tenantID string
		snap     *dag.Snapshot
	}

	var incompleteRuns []runInfo
	for rows.Next() {
		var id, tenantID, data string
		if err := rows.Scan(&id, &tenantID, &data); err != nil {
			continue
		}
		var snap dag.Snapshot
		if json.Unmarshal([]byte(data), &snap) != nil {
			continue
		}
		// Check if any nodes are pending.
		pending := false
		for _, n := range snap.Nodes {
			if n.Status == dag.StatusPending {
				pending = true
				break
			}
		}
		if pending {
			incompleteRuns = append(incompleteRuns, runInfo{id: id, tenantID: tenantID, snap: &snap})
		}
	}

	// Track run IDs that are being recovered so we don't clean up their containers.
	activeRunIDs := make(map[string]bool)

	for _, r := range incompleteRuns {
		s.logger.Info("recovery: found incomplete run", zap.String("id", r.id), zap.String("tenant", r.tenantID))

		if r.snap.State == nil {
			s.logger.Warn("recovery: no state saved, marking as failed", zap.String("id", r.id))
			s.markRunFailed(ctx, r.tenantID, r.snap, r.id)
			s.cleanupRunResources(r.id)
			continue
		}

		if s.canRecoverRun(r.snap.State) {
			s.logger.Info("recovery: containers alive, resuming run", zap.String("id", r.id))
			activeRunIDs[r.id] = true
			go s.recoverRun(r.id, r.tenantID, r.snap)
		} else {
			s.logger.Warn("recovery: containers dead, marking as failed", zap.String("id", r.id))
			s.markRunFailed(ctx, r.tenantID, r.snap, r.id)
			s.cleanupRunResources(r.id)
		}
	}

	// Clean up orphaned containers that don't belong to any active/recovering run.
	s.cleanupOrphanedContainers(activeRunIDs)
}

// CleanupOrphanedRuns is kept for backward compatibility; it delegates to RecoverOrCleanupRuns.
func (s *Server) CleanupOrphanedRuns() {
	s.RecoverOrCleanupRuns()
}

// CleanupOrphanResourcesOnly nukes Docker containers/networks left behind
// by previous server lifetimes WITHOUT touching run snapshots — that path
// is owned by the scheduler now. Safe to call on every server start.
func (s *Server) CleanupOrphanResourcesOnly() {
	// Only Docker provisions local resources we'd want to GC; Yandex VMs
	// stay alive in the cloud across server restarts and are owned by the
	// run's terraform workdir.
	s.cleanupOrphanedContainers(map[string]bool{})
}

// canRecoverRun checks whether the Docker containers from a previous run are still running.
func (s *Server) canRecoverRun(state *dag.RunState) bool {
	if state == nil || len(state.ContainerIDs) == 0 {
		return false
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}
	defer cli.Close()

	for _, cid := range state.ContainerIDs {
		inspect, err := cli.ContainerInspect(context.Background(), cid)
		if err == nil && inspect.State.Running {
			return true // at least one container alive -- worth trying
		}
	}

	return false
}

// canRecoverState dispatches by provider — docker uses ContainerInspect
// (current behaviour), yandex pings each registered agent's /health since
// VMs don't show up in our local Docker daemon. Used by the scheduler's
// startup recovery sweep.
func (s *Server) canRecoverState(state *dag.RunState) bool {
	if state == nil {
		return false
	}
	switch state.Provider {
	case "yandex":
		return s.canRecoverYandex(state)
	default:
		return s.canRecoverRun(state)
	}
}

// canRecoverYandex does a 3-second HTTP ping of every target's agent.
// Recovery is deemed possible if at least one agent answers — even partial
// fleet liveness is enough to drive the DAG forward to teardown.
func (s *Server) canRecoverYandex(state *dag.RunState) bool {
	if len(state.Targets) == 0 {
		return false
	}
	cl := &http.Client{Timeout: 3 * time.Second}
	for _, t := range state.Targets {
		host := t.Host
		if host == "" {
			host = t.InternalHost
		}
		if host == "" {
			continue
		}
		port := t.AgentPort
		if port == 0 {
			port = 8090
		}
		url := fmt.Sprintf("http://%s:%d/health", host, port)
		resp, err := cl.Get(url)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 500 {
			return true
		}
	}
	return false
}

// recoverRun rebuilds state from a snapshot and resumes execution.
// Registers the cancel function so users can cancel recovered runs.
func (s *Server) recoverRun(runID, tenantID string, snap *dag.Snapshot) {
	ctx, cancel := context.WithCancel(context.Background())

	s.runCancelsMu.Lock()
	s.runCancels[runID] = cancel
	s.runTenants[runID] = tenantID
	s.runCancelsMu.Unlock()

	activeRuns.Inc()
	defer func() {
		activeRuns.Dec()
		s.runCancelsMu.Lock()
		delete(s.runCancels, runID)
		delete(s.runTenants, runID)
		s.runCancelsMu.Unlock()
		cancel()
	}()

	if err := s.app.RecoverRun(ctx, tenantID, snap); err != nil {
		s.logger.Error("recovery: run failed",
			zap.String("id", runID), zap.Error(err))
	}
}

// markRunFailed marks all pending nodes in a snapshot as failed.
func (s *Server) markRunFailed(ctx context.Context, tenantID string, snap *dag.Snapshot, runID string) {
	if snap == nil {
		return
	}
	changed := false
	for i := range snap.Nodes {
		if snap.Nodes[i].Status == dag.StatusPending {
			snap.Nodes[i].Status = dag.StatusFailed
			snap.Nodes[i].Error = "server restarted -- run orphaned"
			changed = true
		}
	}
	if changed {
		_ = s.app.Storage().Save(ctx, tenantID, runID, snap)
	}
}

// cleanupOrphanedContainers removes stroppy-agent containers and networks
// that don't belong to any actively recovering run.
func (s *Server) cleanupOrphanedContainers(activeRunIDs map[string]bool) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		s.logger.Warn("orphan cleanup: docker client failed", zap.Error(err))
		return
	}
	defer cli.Close()

	ctx := context.Background()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return
	}

	removed := 0
	for _, c := range containers {
		for _, name := range c.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if !strings.HasPrefix(cleanName, "stroppy-agent-") {
				continue
			}
			// Check if this container belongs to an active run.
			belongsToActive := false
			for runID := range activeRunIDs {
				prefix := fmt.Sprintf("stroppy-agent-%s-", runID)
				if strings.HasPrefix(cleanName, prefix) {
					belongsToActive = true
					break
				}
			}
			if !belongsToActive {
				s.logger.Info("orphan cleanup: removing container", zap.String("name", cleanName))
				cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
				removed++
			}
		}
	}

	if removed > 0 {
		s.logger.Info("orphan cleanup: removed stale containers", zap.Int("count", removed))
	}

	// Remove stroppy-* networks not belonging to active runs.
	networks, _ := cli.NetworkList(ctx, network.ListOptions{})
	for _, n := range networks {
		if !strings.HasPrefix(n.Name, "stroppy-") || strings.Contains(n.Name, "cloud") {
			continue
		}
		// Check if network belongs to active run (name pattern: stroppy-{runID}).
		belongsToActive := false
		for runID := range activeRunIDs {
			if n.Name == fmt.Sprintf("stroppy-%s", runID) {
				belongsToActive = true
				break
			}
		}
		if !belongsToActive {
			s.logger.Info("orphan cleanup: removing network", zap.String("name", n.Name))
			cli.NetworkRemove(ctx, n.ID)
		}
	}
}

// extractRunID gets the run ID from a machine ID.
// Machine IDs follow the pattern "{runID}-{role}-{index}".
// extractDBKind gets the database kind from a run snapshot.
func extractDBKind(snap *dag.Snapshot) string {
	if snap == nil || snap.State == nil {
		return "postgres" // default
	}
	rcBytes := snap.State.RunConfig
	if rcBytes == nil {
		return "postgres"
	}
	var cfg struct {
		Database struct {
			Kind string `json:"kind"`
		} `json:"database"`
	}
	if err := json.Unmarshal(rcBytes, &cfg); err != nil {
		return "postgres"
	}
	if cfg.Database.Kind != "" {
		return cfg.Database.Kind
	}
	return "postgres"
}

func extractRunID(machineID string) string {
	// Find the last two "-" separated segments and strip them.
	parts := strings.Split(machineID, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	return machineID
}

// ============================================================
// Agent binary download
// ============================================================

// stroppyVersions returns available stroppy versions from GitHub releases (cached 5min).
func (s *Server) stroppyVersions(w http.ResponseWriter, r *http.Request) {
	s.stroppyVersionsMu.Lock()
	if time.Since(s.stroppyVersionsAt) < 5*time.Minute && len(s.stroppyVersionsCache) > 0 {
		cached := s.stroppyVersionsCache
		s.stroppyVersionsMu.Unlock()
		writeJSON(w, http.StatusOK, cached)
		return
	}
	s.stroppyVersionsMu.Unlock()

	// Fetch from GitHub API.
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet,
		"https://api.github.com/repos/stroppy-io/stroppy/releases?per_page=50", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "github: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("github releases %d: %s", resp.StatusCode, string(body))})
		return
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "github parse: " + err.Error()})
		return
	}

	// Filter: non-draft release tags >= 4.1.0. Stable releases are returned
	// first so the wizard can default to the latest stable while still letting
	// users pick RCs/prereleases from the same list.
	var stable, prerelease []string
	for _, rel := range releases {
		if rel.Draft || strings.HasPrefix(rel.TagName, stroppyNightlyPfx) {
			continue
		}
		v := strings.TrimPrefix(rel.TagName, "v")
		if compareVersions(v, "4.1.0") >= 0 {
			if rel.Prerelease {
				prerelease = append(prerelease, v)
			} else {
				stable = append(stable, v)
			}
		}
	}
	versions := append(stable, prerelease...)

	// Cache.
	s.stroppyVersionsMu.Lock()
	s.stroppyVersionsCache = versions
	s.stroppyVersionsAt = time.Now()
	s.stroppyVersionsMu.Unlock()

	writeJSON(w, http.StatusOK, versions)
}

// compareVersions compares two semver strings. Returns -1, 0, or 1.
func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var va, vb int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &va)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &vb)
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

func (s *Server) serveBinary(w http.ResponseWriter, r *http.Request) {
	binPath, err := agent.SelfBinaryPath()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=stroppy-agent")
	http.ServeFile(w, r, binPath)
}

// ============================================================
// Stroppy commit binaries via per-commit pre-releases (nightly-{short_sha})
// ============================================================
//
// Workflow on stroppy-io/stroppy CI publishes a pre-release tagged
// `nightly-<short_sha>` with a single asset `stroppy` (raw linux/amd64 binary)
// per push to main. Anonymous downloads work directly from
// https://github.com/stroppy-io/stroppy/releases/download/nightly-<short>/stroppy
// so the agent fetches them without going through this server.

const (
	stroppyRepoOwner  = "stroppy-io"
	stroppyRepoName   = "stroppy"
	stroppyNightlyPfx = "nightly-"
	stroppyNightlyBin = "stroppy"
)

// StroppyCommit describes a per-commit pre-release available for download.
type StroppyCommit struct {
	Short       string    `json:"short"`          // 7-char SHA (the suffix after "nightly-")
	Tag         string    `json:"tag"`            // full release tag (e.g. "nightly-abc1234")
	Name        string    `json:"name,omitempty"` // release name if set
	DownloadURL string    `json:"download_url"`   // anonymous direct asset URL
	PublishedAt time.Time `json:"published_at"`
}

// stroppyCommits returns recent per-commit pre-releases (cached 5min).
func (s *Server) stroppyCommits(w http.ResponseWriter, r *http.Request) {
	s.stroppyCommitsMu.Lock()
	if time.Since(s.stroppyCommitsAt) < 5*time.Minute && len(s.stroppyCommitsCache) > 0 {
		cached := s.stroppyCommitsCache
		s.stroppyCommitsMu.Unlock()
		writeJSON(w, http.StatusOK, cached)
		return
	}
	s.stroppyCommitsMu.Unlock()

	commits, err := s.fetchStroppyCommits(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	s.stroppyCommitsMu.Lock()
	s.stroppyCommitsCache = commits
	s.stroppyCommitsAt = time.Now()
	s.stroppyCommitsMu.Unlock()

	writeJSON(w, http.StatusOK, commits)
}

// fetchStroppyCommits queries the public releases endpoint and filters
// pre-releases tagged `nightly-<short_sha>`.
func (s *Server) fetchStroppyCommits(ctx context.Context) ([]StroppyCommit, error) {
	listURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/releases?per_page=50",
		stroppyRepoOwner, stroppyRepoName,
	)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github releases %d: %s", resp.StatusCode, string(body))
	}

	var releases []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("github releases parse: %w", err)
	}

	out := make([]StroppyCommit, 0, len(releases))
	for _, rel := range releases {
		if rel.Draft || !rel.Prerelease {
			continue
		}
		if !strings.HasPrefix(rel.TagName, stroppyNightlyPfx) {
			continue
		}
		short := strings.TrimPrefix(rel.TagName, stroppyNightlyPfx)
		var dlURL string
		for _, a := range rel.Assets {
			if a.Name == stroppyNightlyBin {
				dlURL = a.BrowserDownloadURL
				break
			}
		}
		if dlURL == "" {
			continue
		}
		out = append(out, StroppyCommit{
			Short:       short,
			Tag:         rel.TagName,
			Name:        rel.Name,
			DownloadURL: dlURL,
			PublishedAt: rel.PublishedAt,
		})
	}
	return out, nil
}

// ============================================================
// WebSocket for UI log streaming
// ============================================================

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) wsLogs(w http.ResponseWriter, r *http.Request) {
	s.handleWS(w, r, "")
}

func (s *Server) wsLogsRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")
	s.handleWS(w, r, runID)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request, filterRunID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("ws upgrade failed", zap.Error(err))
		return
	}
	tenantID := auth.TenantID(r.Context())
	s.hub.addClient(conn, filterRunID, tenantID)
}

// Agents returns currently registered agent targets.
func (s *Server) Agents() map[string]agent.Target {
	s.agentsMu.RLock()
	defer s.agentsMu.RUnlock()
	cp := make(map[string]agent.Target, len(s.agents))
	for k, v := range s.agents {
		cp[k] = v
	}
	return cp
}

// ============================================================
// Log query handler (VictoriaLogs)
// ============================================================

func (s *Server) runLogs(w http.ResponseWriter, r *http.Request) {
	if s.monitoringURL == "" {
		http.Error(w, "log storage not configured (no MONITORING_URL)", http.StatusServiceUnavailable)
		return
	}

	tenantID := auth.TenantID(r.Context())
	accountID, err := s.tenantAccountID(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	runID := chi.URLParam(r, "runID")

	// Build VictoriaLogs LogsQL query with optional pipes.
	query := fmt.Sprintf(`run_id:"%s"`, runID)
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	limit := r.URL.Query().Get("limit")
	search := r.URL.Query().Get("search")

	// Text search: simple substring match on _msg field.
	if search != "" {
		query += fmt.Sprintf(` _msg:"%s"`, strings.ReplaceAll(search, `"`, `\"`))
	}

	// Filter by action (phase) if provided — exact match on action stream.
	if actions := r.URL.Query()["action"]; len(actions) > 0 {
		parts := make([]string, len(actions))
		for i, a := range actions {
			parts[i] = fmt.Sprintf(`action:"%s"`, strings.ReplaceAll(a, `"`, `\"`))
		}
		if len(parts) == 1 {
			query += " " + parts[0]
		} else {
			query += " (" + strings.Join(parts, " OR ") + ")"
		}
	}

	// Filter by role (e.g. database, ydb-storage, stroppy, proxy) — vector
	// log events carry `role`; executor events do not, so a role filter
	// scopes the result to vector-shipped DB stdout.
	if roles := r.URL.Query()["role"]; len(roles) > 0 {
		parts := make([]string, len(roles))
		for i, v := range roles {
			parts[i] = fmt.Sprintf(`role:"%s"`, strings.ReplaceAll(v, `"`, `\"`))
		}
		if len(parts) == 1 {
			query += " " + parts[0]
		} else {
			query += " (" + strings.Join(parts, " OR ") + ")"
		}
	}

	// Filter by systemd unit (postgresql.service, mysql.service,
	// ydbd-storage.service, …) for narrowing DB logs to one engine.
	if units := r.URL.Query()["unit"]; len(units) > 0 {
		parts := make([]string, len(units))
		for i, v := range units {
			parts[i] = fmt.Sprintf(`unit:"%s"`, strings.ReplaceAll(v, `"`, `\"`))
		}
		if len(parts) == 1 {
			query += " " + parts[0]
		} else {
			query += " (" + strings.Join(parts, " OR ") + ")"
		}
	}

	// Filter by machine_id — useful when one VM out of N is misbehaving.
	if mids := r.URL.Query()["machine_id"]; len(mids) > 0 {
		parts := make([]string, len(mids))
		for i, v := range mids {
			parts[i] = fmt.Sprintf(`machine_id:"%s"`, strings.ReplaceAll(v, `"`, `\"`))
		}
		if len(parts) == 1 {
			query += " " + parts[0]
		} else {
			query += " (" + strings.Join(parts, " OR ") + ")"
		}
	}

	// Sort direction: "desc" returns newest first (for chat-like UI), default is "asc".
	dir := r.URL.Query().Get("dir")
	if dir == "desc" {
		query += " | sort by (_time) desc"
	} else {
		query += " | sort by (_time)"
	}
	if limit != "" {
		query += " | limit " + limit
	}

	vlURL := fmt.Sprintf("%s/select/logsql/query?query=%s",
		s.monitoringURL, url.QueryEscape(query))
	if start != "" {
		vlURL += "&start=" + url.QueryEscape(start)
	}
	if end != "" {
		vlURL += "&end=" + url.QueryEscape(end)
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, vlURL, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if s.monitoringToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.monitoringToken)
	}
	if accountID > 0 {
		req.Header.Set("AccountID", fmt.Sprintf("%d", accountID))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ============================================================
// Metrics handlers
// ============================================================

// MetricsRequest is parsed from query parameters for metrics endpoints.
type MetricsRequest struct {
	Start string `json:"start"` // RFC3339
	End   string `json:"end"`   // RFC3339
}

// tenantIDFromRunID resolves tenantID by looking up the run's tenant.
func (s *Server) tenantIDFromRunID(ctx context.Context, runID string) string {
	var tenantID string
	s.pool.QueryRow(ctx, "SELECT tenant_id FROM runs WHERE id = $1 LIMIT 1", runID).Scan(&tenantID)
	return tenantID
}

// accountIDFromRunID resolves accountID by looking up the run's tenant, then the tenant's account_id.
func (s *Server) accountIDFromRunID(ctx context.Context, runID string) (int32, error) {
	tenantID := s.tenantIDFromRunID(ctx, runID)
	if tenantID == "" {
		return 0, fmt.Errorf("lookup tenant for run %s: not found", runID)
	}
	return s.tenantAccountID(ctx, tenantID)
}

// tenantAccountID looks up the account_id for a tenant.
func (s *Server) tenantAccountID(ctx context.Context, tenantID string) (int32, error) {
	q := pgdb.New(s.pool)
	t, err := q.GetTenant(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("get tenant: %w", err)
	}
	if !t.AccountID.Valid {
		return 0, fmt.Errorf("tenant %s has no account_id", tenantID)
	}
	return t.AccountID.Int32, nil
}

// metricsCollector returns a metrics.Collector configured for the given accountID and DB kind.
func (s *Server) metricsCollector(accountID int32, dbKind string) *metrics.Collector {
	prefix := fmt.Sprintf("%s/select/%d/prometheus", s.monitoringURL, accountID)
	return metrics.NewCollectorForDB(victoria.NewClient(prefix, s.monitoringToken), dbKind)
}

// logsBaseURL returns the VictoriaLogs query base URL for the given accountID.
func (s *Server) logsBaseURL(accountID int32) string {
	return fmt.Sprintf("%s/select/%d", s.monitoringURL, accountID)
}

// getGrafanaConfig handles GET /api/v1/grafana.
func (s *Server) getGrafanaConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"url":           s.grafanaURL,
		"embed_enabled": s.grafanaURL != "",
		"dashboards":    s.grafanaDashboards,
	})
}

func (s *Server) runMetrics(w http.ResponseWriter, r *http.Request) {
	if s.monitoringURL == "" {
		http.Error(w, "metrics not configured (no MONITORING_URL)", http.StatusServiceUnavailable)
		return
	}

	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	accountID, err := s.tenantAccountID(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Load snapshot to get timestamps and db_kind.
	snap, _ := s.app.storage.Load(r.Context(), tenantID, runID)
	dbKind := extractDBKind(snap)

	collector := s.metricsCollector(accountID, dbKind)
	tr, err := parseTimeRange(r)
	if err != nil {
		if snap != nil && !snap.StartedAt.IsZero() {
			end := snap.FinishedAt
			if end.IsZero() {
				end = time.Now()
			}
			tr = metrics.TimeRange{Start: snap.StartedAt.Add(-30 * time.Second), End: end.Add(30 * time.Second)}
		} else {
			http.Error(w, "no time range provided and run has no timestamps", http.StatusBadRequest)
			return
		}
	}

	result, err := collector.Collect(r.Context(), runID, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) compareRuns(w http.ResponseWriter, r *http.Request) {
	if s.monitoringURL == "" {
		http.Error(w, "metrics not configured (no MONITORING_URL)", http.StatusServiceUnavailable)
		return
	}

	tenantID := auth.TenantID(r.Context())

	accountID, err := s.tenantAccountID(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	runA := r.URL.Query().Get("a")
	runB := r.URL.Query().Get("b")
	if runA == "" || runB == "" {
		http.Error(w, "query params 'a' and 'b' (run IDs) are required", http.StatusBadRequest)
		return
	}

	// Auto-resolve time range from run snapshots if not provided.
	tr, err := parseTimeRange(r)
	if err != nil {
		// Try to derive from run snapshots.
		snapA, _ := s.app.storage.Load(r.Context(), tenantID, runA)
		snapB, _ := s.app.storage.Load(r.Context(), tenantID, runB)
		if snapA == nil || snapB == nil {
			http.Error(w, "runs not found and no explicit time range provided", http.StatusBadRequest)
			return
		}
		start := snapA.StartedAt
		if !snapB.StartedAt.IsZero() && snapB.StartedAt.Before(start) {
			start = snapB.StartedAt
		}
		end := snapA.FinishedAt
		if !snapB.FinishedAt.IsZero() && snapB.FinishedAt.After(end) {
			end = snapB.FinishedAt
		}
		if start.IsZero() || end.IsZero() {
			http.Error(w, "runs have no timestamps and no explicit time range provided", http.StatusBadRequest)
			return
		}
		// Add padding to capture metrics at boundaries.
		tr = metrics.TimeRange{Start: start.Add(-30 * time.Second), End: end.Add(30 * time.Second)}
	}

	// Use dbKind from run A for metric queries.
	snapA2, _ := s.app.storage.Load(r.Context(), tenantID, runA)
	collector := s.metricsCollector(accountID, extractDBKind(snapA2))

	metricsA, err := collector.Collect(r.Context(), runA, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "run A: " + err.Error()})
		return
	}

	metricsB, err := collector.Collect(r.Context(), runB, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "run B: " + err.Error()})
		return
	}

	threshold := 5.0 // default
	if t := r.URL.Query().Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil && v > 0 {
			threshold = v
		}
	}
	comp := metrics.Compare(metricsA, metricsB, threshold)
	comp.Start = tr.Start
	comp.End = tr.End

	// Enrich with run configs so the UI can show hardware info.
	snapAFull, _ := s.app.storage.Load(r.Context(), tenantID, runA)
	snapBFull, _ := s.app.storage.Load(r.Context(), tenantID, runB)
	type enriched struct {
		*metrics.Comparison
		ConfigA json.RawMessage `json:"config_a,omitempty"`
		ConfigB json.RawMessage `json:"config_b,omitempty"`
	}
	resp := enriched{Comparison: comp}
	if snapAFull != nil && snapAFull.State != nil {
		resp.ConfigA = snapAFull.State.RunConfig
	}
	if snapBFull != nil && snapBFull.State != nil {
		resp.ConfigB = snapBFull.State.RunConfig
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseTimeRange(r *http.Request) (metrics.TimeRange, error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" || endStr == "" {
		return metrics.TimeRange{}, fmt.Errorf("query params 'start' and 'end' (RFC3339) are required")
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return metrics.TimeRange{}, fmt.Errorf("invalid 'start': %w", err)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return metrics.TimeRange{}, fmt.Errorf("invalid 'end': %w", err)
	}

	return metrics.TimeRange{Start: start, End: end}, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ============================================================
// Baseline management handlers
// ============================================================

func (s *Server) setBaseline(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	name := chi.URLParam(r, "name")
	var body struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RunID == "" {
		http.Error(w, "request body must contain run_id", http.StatusBadRequest)
		return
	}
	if err := s.app.Storage().SetBaseline(r.Context(), tenantID, name, body.RunID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "baseline": name, "run_id": body.RunID})
}

func (s *Server) getBaseline(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	name := chi.URLParam(r, "name")
	runID, err := s.app.Storage().GetBaseline(r.Context(), tenantID, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if runID == "" {
		http.Error(w, "baseline not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"baseline": name, "run_id": runID})
}

func (s *Server) listBaselines(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	baselines, err := s.app.Storage().ListBaselines(r.Context(), tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, baselines)
}
