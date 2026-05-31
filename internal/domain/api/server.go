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

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// Server is the HTTP server exposing external + UI APIs. Run execution now
// lives in Temporal; this server launches RunWorkflows via App and queries
// their live state, persisting only run metadata via RunStore.
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

	// runs persists run metadata records (resolved config + identity). Live
	// status is queried from Temporal via s.app.Status, not stored here.
	runs RunStore

	// listenAddr is the server's HTTP listen address, used to derive the
	// download URL Docker agents use to fetch package .debs.
	listenAddr string

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

	// spaFS serves the embedded SPA files. If nil, SPA is not served.
	spaFS http.FileSystem
}

// NewServer creates an HTTP server backed by the App.
// monitoringURL is the vmauth base URL (empty = monitoring disabled).
// grafanaURL is the Grafana base URL (empty = Grafana integration disabled).
func NewServer(app *App, logger *zap.Logger, pool *pgxpool.Pool, jwtSecret, monitoringURL, monitoringToken, grafanaURL, listenAddr string) *Server {
	s := &Server{
		app:             app,
		logger:          logger,
		hub:             newWSHub(),
		runs:            postgres.NewRunRecordStorage(pool),
		listenAddr:      listenAddr,
		cancelledRuns:   make(map[string]bool),
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
	// Wire settings getter so the App can access current cloud settings from DB.
	app.settingsFunc = s.settingsForTenant
	// Wire monitoring config so the App can derive OTLP/metrics endpoints.
	app.monitoringURL = monitoringURL
	app.monitoringToken = monitoringToken
	// Wire accountID resolver so the App can set per-tenant victoria accountID.
	app.accountIDFunc = func(tenantID string) int32 {
		id, _ := s.tenantAccountID(context.Background(), tenantID)
		return id
	}
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
		r.Get("/suites/{id}/items", s.listSuiteItems)
		r.Get("/suites/{id}/batches/{batchID}/cross", s.crossCompareBatch)
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
			r.Post("/suites/{id}/clone", s.cloneSuite)
			r.Post("/suites/{id}/run", s.launchSuite)
			r.Post("/suites/{id}/batches/{batchID}/cancel", s.cancelBatch)
			r.Post("/suites/{id}/items", s.createSuiteItem)
			r.Put("/suites/{id}/items/{itemID}", s.updateSuiteItem)
			r.Delete("/suites/{id}/items/{itemID}", s.deleteSuiteItem)
			r.Post("/suites/{id}/items/reorder", s.reorderSuiteItems)
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
// run metadata record, then launches its RunWorkflow on Temporal. Used by
// both runStart (single) and the suite launcher.
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
	// Reject duplicate run ids: a run record with this id already exists.
	if existing, _ := s.runs.Get(ctx, tenantID, cfg.ID); existing != nil {
		return &enqueueError{code: http.StatusConflict, err: fmt.Errorf("run %q already exists", cfg.ID)}
	}

	// Persist run metadata first so listRuns/runStatus can resolve the run
	// even before its workflow surfaces a state.
	rec := types.RunRecord{
		ID:          cfg.ID,
		TenantID:    tenantID,
		Name:        cfg.Name,
		Description: cfg.Description,
		SuiteID:     cfg.SuiteID,
		Provider:    string(cfg.Provider),
		CreatedAt:   time.Now().UTC(),
		Cfg:         *cfg,
	}
	if err := s.runs.Create(ctx, rec); err != nil {
		return &enqueueError{code: http.StatusInternalServerError, err: err}
	}
	// Launch the RunWorkflow on Temporal.
	if err := s.app.Start(ctx, tenantID, *cfg); err != nil {
		// Roll back the record so a failed launch doesn't leave a ghost run.
		_ = s.runs.Delete(ctx, tenantID, cfg.ID)
		return &enqueueError{code: http.StatusInternalServerError, err: err}
	}
	return nil
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

// statusString maps a common.Status enum to the legacy node/run status
// strings the SPA understands ("pending"/"running"/"done"/"failed"/...).
func statusString(st common.Status) string {
	switch st {
	case common.Status_STATUS_PENDING, common.Status_STATUS_ALLOCATED,
		common.Status_STATUS_RETRY_WAIT, common.Status_STATUS_UNSPECIFIED:
		return "pending"
	case common.Status_STATUS_RUNNING, common.Status_STATUS_DEPLOYMENT,
		common.Status_STATUS_DEPLOYED, common.Status_STATUS_CANCELLING:
		return "running"
	case common.Status_STATUS_COMPLETED:
		return "done"
	case common.Status_STATUS_FAILED:
		return "failed"
	case common.Status_STATUS_CANCELLED:
		return "cancelled"
	case common.Status_STATUS_SKIPPED:
		return "done"
	default:
		return "pending"
	}
}

// nodeStatus mirrors the legacy dag.NodeStatus JSON the SPA renders.
type nodeStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// canonicalPhase maps a workflow stage label (the per-target "<targetID>/<label>"
// label, or a top-level stage) onto the bare canonical phase id the SPA's
// RunOverview groups by (web/src/components/RunOverview.tsx phaseGroups). Many
// concrete recipe steps collapse onto one SPA phase (every DB's install_* →
// install_db). Returns "" to DROP the stage from the SPA view (server-internal
// steps that aren't a user-facing phase).
func canonicalPhase(label string) string {
	switch label {
	case "deploy":
		return "machines"
	case "build_recipe", "bootstrap":
		return "" // server-side plan compile / base apt prep — not a UI phase
	case "install_postgres", "install_mysql", "install_picodata",
		"install_cockroach", "install_ydb", "install_patroni":
		return "install_db"
	case "config_postgres", "config_mysql", "config_picodata",
		"config_cockroach", "config_ydb", "config_patroni",
		"init_cockroach", "init_ydb", "start_ydb_db":
		return "configure_db"
	case "install_etcd":
		return "install_etcd"
	case "config_etcd":
		return "configure_etcd"
	case "install_pgbouncer":
		return "install_pgbouncer"
	case "config_pgbouncer":
		return "configure_pgbouncer"
	case "install_haproxy", "install_proxysql":
		return "install_proxy"
	case "config_haproxy", "config_proxysql":
		return "configure_proxy"
	case "install_monitor":
		return "install_monitor"
	case "config_monitor":
		return "configure_monitor"
	case "install_stroppy":
		return "install_stroppy"
	case "run_stroppy":
		return "run_stroppy"
	case "teardown":
		return "teardown"
	default:
		return ""
	}
}

// collapseStatus merges the statuses of several concrete stages that map to one
// SPA phase. Precedence mirrors the SPA's groupStatus: any failed → failed; any
// cancelled → cancelled; all done → done; any running/done → running; else
// pending.
func collapseStatus(ss []string) string {
	if len(ss) == 0 {
		return "pending"
	}
	allDone, anyRunning := true, false
	for _, s := range ss {
		switch s {
		case "failed":
			return "failed"
		case "cancelled":
			return "cancelled"
		case "done":
			anyRunning = true
		case "running":
			allDone, anyRunning = false, true
		default:
			allDone = false
		}
	}
	if allDone {
		return "done"
	}
	if anyRunning {
		return "running"
	}
	return "pending"
}

// stagesToNodes converts the Temporal RunState stages into the SPA's nodes[]
// shape (id/status/error). It strips the "<targetID>/" prefix, maps each stage
// label onto its canonical SPA phase id, and aggregates the many concrete
// per-target stages that share a phase (e.g. install_monitor on every machine)
// into a single node whose status is the collapsed group status. Stages with no
// SPA phase (build_recipe, bootstrap) are dropped.
func stagesToNodes(stages []*workflowpb.Stage) []nodeStatus {
	order := make([]string, 0, len(stages))
	byPhase := make(map[string][]string, len(stages))
	for _, st := range stages {
		label := st.GetName()
		if label == "" {
			label = st.GetNodeExecutionId()
		}
		// Drop the "<targetID>/" prefix → bare step label.
		if i := strings.LastIndexByte(label, '/'); i >= 0 {
			label = label[i+1:]
		}
		phase := canonicalPhase(label)
		if phase == "" {
			continue
		}
		s := statusString(st.GetStatus())
		if _, seen := byPhase[phase]; !seen {
			order = append(order, phase)
		}
		byPhase[phase] = append(byPhase[phase], s)
	}
	out := make([]nodeStatus, 0, len(order))
	for _, phase := range order {
		out = append(out, nodeStatus{ID: phase, Status: collapseStatus(byPhase[phase])})
	}
	return out
}

// runStatus translates the live Temporal RunState into the legacy snapshot
// JSON shape the SPA expects: nodes[] (id/status/error), state.run_config,
// state.targets, started_at/finished_at, and an overall job_state.
func (s *Server) runStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	rec, err := s.runs.Get(r.Context(), tenantID, runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rec == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Live status from Temporal (best-effort — a freshly-launched run may not
	// answer the query yet, in which case we report "pending").
	var nodes []nodeStatus
	overall := "pending"
	if st, qerr := s.app.Status(r.Context(), runID); qerr == nil && st != nil {
		nodes = stagesToNodes(st.GetStages())
		overall = statusString(st.GetStatus())
	} else if qerr != nil {
		s.logger.Debug("runStatus: temporal query failed", zap.String("run_id", runID), zap.Error(qerr))
	}
	if nodes == nil {
		nodes = []nodeStatus{}
	}
	if s.cancelledRuns[runID] {
		overall = "cancelled"
	}

	cfgJSON, _ := json.Marshal(rec.Cfg)
	resp := map[string]any{
		"graph":      "",
		"nodes":      nodes,
		"started_at": rec.CreatedAt,
		"state": map[string]any{
			"provider":          rec.Provider,
			"run_config":        json.RawMessage(cfgJSON),
			"targets":           []any{},
			"effective_configs": run.ComputeEffectiveConfigs(&rec.Cfg),
		},
		"job_state": overall,
	}
	writeJSON(w, http.StatusOK, resp)
}

// runRenderedConfigs returns the per-component config files that the agent
// rendered (or will render) for a given run — postgresql.conf, ydb.yaml,
// pgbouncer.ini, etc. Same shape as the dry-run preview, derived from the
// run's saved RunConfig so it works while a run is in progress.
func (s *Server) runRenderedConfigs(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	rec, err := s.runs.Get(r.Context(), tenantID, runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rec == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	cfg := rec.Cfg
	writeJSON(w, http.StatusOK, map[string]any{
		"rendered_configs": run.BuildRenderedConfigs(&cfg),
	})
}

// runSummary mirrors the legacy dag.RunSummary JSON the SPA's runs list
// renders. Status comes from a best-effort Temporal query per run.
type runSummary struct {
	ID          string       `json:"id"`
	Nodes       []nodeStatus `json:"nodes"`
	Total       int          `json:"total"`
	Done        int          `json:"done"`
	Failed      int          `json:"failed"`
	Pending     int          `json:"pending"`
	StartedAt   time.Time    `json:"started_at,omitempty"`
	FinishedAt  time.Time    `json:"finished_at,omitempty"`
	DBKind      string       `json:"db_kind,omitempty"`
	Provider    string       `json:"provider,omitempty"`
	Script      string       `json:"script,omitempty"`
	Duration    string       `json:"duration,omitempty"`
	VUs         int          `json:"vus,omitempty"`
	DBVersion   string       `json:"db_version,omitempty"`
	NodeCount   int          `json:"node_count,omitempty"`
	PresetID    string       `json:"preset_id,omitempty"`
	Cancelled   bool         `json:"cancelled,omitempty"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	SuiteID     string       `json:"suite_id,omitempty"`
	RunPresetID string       `json:"run_preset_id,omitempty"`
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())

	recs, err := s.runs.List(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]runSummary, len(recs))
	for i := range recs {
		rec := &recs[i]
		cfg := rec.Cfg
		out[i] = runSummary{
			ID:          rec.ID,
			Nodes:       []nodeStatus{},
			DBKind:      string(cfg.Database.Kind),
			Provider:    string(cfg.Provider),
			Script:      cfg.Stroppy.Script,
			Duration:    cfg.Stroppy.Duration,
			VUs:         cfg.Stroppy.VUs,
			DBVersion:   cfg.Database.Version,
			PresetID:    cfg.PresetID,
			Name:        rec.Name,
			Description: rec.Description,
			SuiteID:     rec.SuiteID,
			RunPresetID: cfg.RunPresetID,
			StartedAt:   rec.CreatedAt,
		}
		if s.cancelledRuns[rec.ID] {
			out[i].Cancelled = true
		}
	}

	// Live status from Temporal is a per-run query round-trip. Run them in
	// parallel with a bounded pool and a short per-query deadline so one stale /
	// panic-looping workflow (whose GetRunWorkflowState query hangs) can't stall
	// the whole list — that run just renders as pending.
	const (
		queryConc    = 8
		queryTimeout = 2 * time.Second
	)
	sem := make(chan struct{}, queryConc)
	var wg sync.WaitGroup
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			qctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
			defer cancel()
			st, qerr := s.app.Status(qctx, recs[i].ID)
			if qerr != nil || st == nil {
				return // ignore → "unknown" (empty nodes + pending counters)
			}
			sum := &out[i]
			sum.Nodes = stagesToNodes(st.GetStages())
			sum.Total = len(sum.Nodes)
			for _, n := range sum.Nodes {
				switch n.Status {
				case "done":
					sum.Done++
				case "failed":
					sum.Failed++
				default:
					sum.Pending++
				}
			}
			sum.NodeCount = sum.Total
		}(i)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	rec, err := s.runs.Get(r.Context(), tenantID, runID)
	if err != nil || rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}
	// Request cancellation of the run's Temporal workflow.
	if err := s.app.Cancel(r.Context(), runID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.cancelledRuns[runID] = true
	s.logger.Info("cancelling run", zap.String("run_id", runID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling", "run_id": runID})
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	// Check exists.
	rec, err := s.runs.Get(r.Context(), tenantID, runID)
	if err != nil || rec == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Best-effort cancel of the underlying workflow before dropping the record.
	if err := s.app.Cancel(r.Context(), runID); err != nil {
		s.logger.Debug("deleteRun: cancel workflow failed", zap.String("run_id", runID), zap.Error(err))
	}
	// Clean up Docker resources (best-effort).
	s.cleanupRunResources(runID)

	// Delete the run record.
	if err := s.runs.Delete(r.Context(), tenantID, runID); err != nil {
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
		if compareVersions(v, types.MinStroppyVersionString()) >= 0 {
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

// recDBKind extracts the database kind from a run record, defaulting to
// "postgres" when the record (or kind) is unset.
func recDBKind(rec *types.RunRecord) string {
	if rec == nil || rec.Cfg.Database.Kind == "" {
		return "postgres"
	}
	return string(rec.Cfg.Database.Kind)
}

// runTimeWindow derives a [start, end] metrics window for a run. The record's
// CreatedAt is the start. The end is the run's finish time when terminal,
// otherwise "now" (the run is still in flight). Finish time isn't persisted,
// so a terminal run uses the record's updated bound via "now" — good enough
// for a metrics query that pads ±30s at the boundaries.
func (s *Server) runTimeWindow(ctx context.Context, rec *types.RunRecord) (time.Time, time.Time) {
	if rec == nil {
		return time.Time{}, time.Time{}
	}
	start := rec.CreatedAt
	end := time.Now()
	if st, err := s.app.Status(ctx, rec.ID); err == nil && st != nil {
		switch st.GetStatus() {
		case common.Status_STATUS_COMPLETED, common.Status_STATUS_FAILED,
			common.Status_STATUS_CANCELLED, common.Status_STATUS_SKIPPED:
			// Terminal: use the latest finished_at across stages if available.
			for _, stg := range st.GetStages() {
				if fa := stg.GetFinishedAt(); fa != nil {
					if t := fa.AsTime(); t.After(start) && t.Before(end) {
						end = t
					}
				}
			}
		}
	}
	return start, end
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
	// Load run record to get timestamps and db_kind.
	rec, _ := s.runs.Get(r.Context(), tenantID, runID)
	dbKind := recDBKind(rec)

	collector := s.metricsCollector(accountID, dbKind)
	tr, err := parseTimeRange(r)
	if err != nil {
		if rec != nil && !rec.CreatedAt.IsZero() {
			start, end := s.runTimeWindow(r.Context(), rec)
			tr = metrics.TimeRange{Start: start.Add(-30 * time.Second), End: end.Add(30 * time.Second)}
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

	recA, _ := s.runs.Get(r.Context(), tenantID, runA)
	recB, _ := s.runs.Get(r.Context(), tenantID, runB)

	// Auto-resolve time range from run records if not provided.
	tr, err := parseTimeRange(r)
	if err != nil {
		if recA == nil || recB == nil {
			http.Error(w, "runs not found and no explicit time range provided", http.StatusBadRequest)
			return
		}
		startA, endA := s.runTimeWindow(r.Context(), recA)
		startB, endB := s.runTimeWindow(r.Context(), recB)
		start := startA
		if !startB.IsZero() && startB.Before(start) {
			start = startB
		}
		end := endA
		if endB.After(end) {
			end = endB
		}
		if start.IsZero() || end.IsZero() {
			http.Error(w, "runs have no timestamps and no explicit time range provided", http.StatusBadRequest)
			return
		}
		// Add padding to capture metrics at boundaries.
		tr = metrics.TimeRange{Start: start.Add(-30 * time.Second), End: end.Add(30 * time.Second)}
	}

	// Use dbKind from run A for metric queries.
	collector := s.metricsCollector(accountID, recDBKind(recA))

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
	type enriched struct {
		*metrics.Comparison
		ConfigA json.RawMessage `json:"config_a,omitempty"`
		ConfigB json.RawMessage `json:"config_b,omitempty"`
	}
	resp := enriched{Comparison: comp}
	if recA != nil {
		if b, err := json.Marshal(recA.Cfg); err == nil {
			resp.ConfigA = b
		}
	}
	if recB != nil {
		if b, err := json.Marshal(recB.Cfg); err == nil {
			resp.ConfigB = b
		}
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
	if err := s.runs.SetBaseline(r.Context(), tenantID, name, body.RunID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "baseline": name, "run_id": body.RunID})
}

func (s *Server) getBaseline(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	name := chi.URLParam(r, "name")
	runID, err := s.runs.GetBaseline(r.Context(), tenantID, name)
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
	baselines, err := s.runs.ListBaselines(r.Context(), tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, baselines)
}
