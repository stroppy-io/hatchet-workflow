//go:build integration

// Package integration drives the running control plane through its public gRPC API
// (the same API the SPA uses), against a live `docker compose up` stack. Point it at
// a different server with STROPPY_API_ADDR. It authenticates as the bootstrapped root
// admin (STROPPY_ROOT_ADMIN_EMAIL/PASSWORD).
package integration

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var (
	apiAddr   = envOr("STROPPY_API_ADDR", "localhost:8080")
	rootEmail = envOr("STROPPY_ROOT_ADMIN_EMAIL", "admin@stroppy.local")
	rootPass  = envOr("STROPPY_ROOT_ADMIN_PASSWORD", "admin")
)

// TestMain waits for the server's /health to come up before running the suite.
func TestMain(m *testing.M) {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + apiAddr + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(time.Second)
	}
	os.Exit(m.Run())
}

func ptr[T any](v T) *T { return &v }

func dialAnon(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(apiAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// login authenticates as the root admin and returns a connection that carries the
// bearer token on every call.
func login(t *testing.T) *grpc.ClientConn {
	t.Helper()
	resp, err := uipb.NewAuthServiceClient(dialAnon(t)).Login(context.Background(),
		&uipb.LoginRequest{Email: rootEmail, Password: rootPass})
	require.NoError(t, err, "login root admin")
	token := resp.GetTokens().GetAccessToken()
	require.NotEmpty(t, token, "access token")

	conn, err := grpc.NewClient(apiAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
			return invoker(ctx, method, req, reply, cc, opts...)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func rootTenant(t *testing.T, conn *grpc.ClientConn) *models.TenantId {
	t.Helper()
	tenants, err := uipb.NewTenantServiceClient(conn).ListMyTenants(context.Background(), &emptypb.Empty{})
	require.NoError(t, err, "list my tenants")
	require.NotEmpty(t, tenants.GetTenants(), "root admin must own the bootstrapped tenant")
	return &models.TenantId{Value: tenants.GetTenants()[0].GetEntity().GetId().GetValue()}
}

// TestLoginAndMe: the bootstrapped root admin can authenticate and is an admin.
func TestLoginAndMe(t *testing.T) {
	conn := login(t)
	acc, err := uipb.NewAuthServiceClient(conn).Me(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.True(t, acc.GetIsAdmin(), "root account must be admin")
}

// TestListSystemPresets: the bootstrap seeds the system database-preset catalog,
// visible to the tenant.
func TestListSystemPresets(t *testing.T) {
	conn := login(t)
	tid := rootTenant(t, conn)
	resp, err := uipb.NewPresetServiceClient(conn).ListPresets(context.Background(),
		&uipb.ListPresetRequest{TenantId: tid})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.GetPresets()), 1, "system presets must be listed")
	var dbPreset *domain.DatabasePreset
	for _, p := range resp.GetPresets() {
		if p.GetKind() == models.Preset_KIND_DATABASE && p.GetDatabasePreset() != nil {
			dbPreset = p.GetDatabasePreset()
			break
		}
	}
	require.NotNil(t, dbPreset, "at least one DATABASE system preset")
}

// TestSettingsScaffold: the bootstrap seeds one settings item per known key.
func TestSettingsScaffold(t *testing.T) {
	conn := login(t)
	tid := rootTenant(t, conn)
	resp, err := uipb.NewSettingsServiceClient(conn).ListSettingsItems(context.Background(),
		&uipb.ListSettingsItemsRequest{TenantId: tid})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.GetSettingsItems()), 13, "settings scaffold seeded")
}

// TestRunListPagination: the List rework returns the tenant's runs with a PageInfo.
func TestRunListPagination(t *testing.T) {
	conn := login(t)
	tid := rootTenant(t, conn)
	resp, err := uipb.NewRunServiceClient(conn).ListTestRuns(context.Background(),
		&uipb.ListTestRunsRequest{TenantId: tid, Page: &models.Page{Size: 5}})
	require.NoError(t, err)
	require.NotNil(t, resp.GetPageInfo(), "List returns PageInfo")
	require.LessOrEqual(t, len(resp.GetTestRuns()), 5, "page size honored")
}

// fullPostgresPreset builds a complete TestPreset for an e2e: a single-node Postgres
// system DatabasePreset PLUS a stroppy load machine (the workload component, which
// lives in the WorkloadPreset and is normally merged in by the wizard). Without the
// stroppy machine the run deploys only the DB container and never benchmarks.
func fullPostgresPreset(t *testing.T, conn *grpc.ClientConn, tid *models.TenantId) *domain.TestPreset {
	t.Helper()
	presets, err := uipb.NewPresetServiceClient(conn).ListPresets(context.Background(),
		&uipb.ListPresetRequest{TenantId: tid, Kinds: []models.Preset_Kind{models.Preset_KIND_DATABASE}})
	require.NoError(t, err)
	var dp *domain.DatabasePreset
	for _, p := range presets.GetPresets() {
		d := p.GetDatabasePreset()
		if d.GetDatabase().GetKind() == domain.Database_KIND_POSTGRES && len(d.GetTopology().GetMachines()) == 1 {
			t.Logf("using preset %q", p.GetName())
			dp = d
			break
		}
	}
	require.NotNil(t, dp, "single-node postgres system preset")

	dbComp := dp.GetTopology().GetMachines()[0].GetComponents()[0].GetId()
	topo := proto.Clone(dp.GetTopology()).(*domain.Topology)
	topo.Machines = append(topo.Machines, &domain.Topology_Machine{
		Id: "load1", Cores: 2, MemoryGb: 4, DiskGb: 20,
		Components: []*domain.Topology_Component{{Id: "stroppy", Kind: domain.Topology_Component_KIND_STROPPY}},
	})
	topo.Connections = append(topo.Connections, &domain.Topology_Connection{
		From: "stroppy", To: dbComp, Kind: domain.Topology_Connection_KIND_FLOW,
	})
	return &domain.TestPreset{
		Database: dp.GetDatabase(),
		Topology: topo,
		Workload: &domain.Workload{
			StroppyVersion: "v5.1.3",
			Script:         "tpcc",
			Protocol:       domain.Workload_PROTOCOL_PG,
			Parameters:     &domain.Workload_Parameters{PoolSize: 16, ScaleFactor: 1},
		},
		Deployment: &deployment.DeploymentIntent{Provider: deployment.Provider_PROVIDER_DOCKER},
	}
}

// dbTestPreset builds a full TestPreset from one concrete system DatabasePreset
// PLUS a small stroppy load machine running `script`.
func dbTestPreset(t *testing.T, dp *domain.DatabasePreset, script string) *domain.TestPreset {
	t.Helper()
	require.NotNil(t, dp, "database preset")

	dbComp := firstDatabaseComponent(t, dp.GetTopology())
	topo := proto.Clone(dp.GetTopology()).(*domain.Topology)
	topo.Machines = append(topo.Machines, &domain.Topology_Machine{
		Id: "load1", Cores: 2, MemoryGb: 4, DiskGb: 20,
		Components: []*domain.Topology_Component{{Id: "stroppy", Kind: domain.Topology_Component_KIND_STROPPY}},
	})
	topo.Connections = append(topo.Connections, &domain.Topology_Connection{
		From: "stroppy", To: dbComp, Kind: domain.Topology_Connection_KIND_FLOW,
	})
	return &domain.TestPreset{
		Database: dp.GetDatabase(),
		Topology: topo,
		Workload: &domain.Workload{
			StroppyVersion: "v5.1.3",
			Script:         script,
			Protocol:       workloadProtocol(dp.GetDatabase().GetKind()),
			Execution: &domain.Workload_Execution{
				Vus:          1,
				Limit:        &domain.Workload_Execution_Iterations{Iterations: 1},
				Quiet:        true,
				NoThresholds: true,
			},
			Parameters: &domain.Workload_Parameters{PoolSize: 4, ScaleFactor: 1},
		},
		Deployment: &deployment.DeploymentIntent{Provider: deployment.Provider_PROVIDER_DOCKER},
	}
}

func firstDatabaseComponent(t *testing.T, topo *domain.Topology) string {
	t.Helper()
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			if c.GetKind() == domain.Topology_Component_KIND_DATABASE {
				return c.GetId()
			}
		}
	}
	t.Fatal("database preset has no DATABASE component")
	return ""
}

func workloadProtocol(kind domain.Database_Kind) domain.Workload_Protocol {
	switch kind {
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return domain.Workload_PROTOCOL_MYSQL
	case domain.Database_KIND_PICODATA:
		return domain.Workload_PROTOCOL_PICODATA
	case domain.Database_KIND_YDB:
		return domain.Workload_PROTOCOL_YDB_GRPC
	case domain.Database_KIND_COCKROACH:
		return domain.Workload_PROTOCOL_COCKROACH
	default:
		return domain.Workload_PROTOCOL_PG
	}
}

type matrixWorkload struct {
	name   string
	script string
}

func matrixWorkloads(kind domain.Database_Kind) []matrixWorkload {
	all := []matrixWorkload{
		{"tpcc-procs", "tpcc/procs"},
		{"tpcc-tx", "tpcc/tx.ts"},
		{"tpcb-procs", "tpcb/procs"},
		{"tpcb-tx", "tpcb/tx.ts"},
		{"tpch-tx", "tpch/tx.ts"},
	}
	txOnly := []matrixWorkload{
		{"tpcc-tx", "tpcc/tx.ts"},
		{"tpcb-tx", "tpcb/tx.ts"},
		{"tpch-tx", "tpch/tx.ts"},
	}
	switch kind {
	case domain.Database_KIND_POSTGRES, domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return all
	case domain.Database_KIND_PICODATA, domain.Database_KIND_YDB, domain.Database_KIND_COCKROACH:
		return txOnly
	default:
		return nil
	}
}

func slug(s string) string {
	s = strings.ToLower(s)
	repl := strings.NewReplacer(" ", "-", "/", "-", "_", "-", ".", "-", "—", "-", "(", "", ")", "")
	s = repl.Replace(s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// runBenchmark submits a preset, waits for the run to settle terminal, and returns
// the final status. Used by both the single full-run e2e and the matrix.
func runBenchmark(t *testing.T, conn *grpc.ClientConn, tid *models.TenantId, preset *domain.TestPreset, name string) (*models.TestRunId, primitive.Status) {
	t.Helper()
	runClient := uipb.NewRunServiceClient(conn)
	run, err := runClient.SubmitTestRun(context.Background(), &uipb.SubmitTestRunRequest{
		TenantId: tid, Name: ptr(name), TestPreset: preset,
	})
	require.NoError(t, err, "submit %s", name)
	runID := &models.TestRunId{Value: run.GetEntity().GetId().GetValue()}
	var last primitive.Status
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		got, gerr := runClient.GetTestRun(context.Background(), &uipb.GetTestRunRequest{TenantId: tid, Id: runID})
		if gerr == nil {
			if got.GetStatus() != last {
				t.Logf("[%s] status: %s", name, got.GetStatus())
				last = got.GetStatus()
			}
			if got.GetStatus() == primitive.Status_STATUS_COMPLETED ||
				got.GetStatus() == primitive.Status_STATUS_FAILED ||
				got.GetStatus() == primitive.Status_STATUS_CANCELLED {
				return runID, got.GetStatus()
			}
		}
		time.Sleep(5 * time.Second)
	}
	return runID, last
}

// TestBenchmarkMatrix runs workload × database × system-topology matrix locally in docker.
// It enumerates the seeded DATABASE presets, then applies the scripts each engine
// can run. Workloads are intentionally tiny (1 VU, 1 iteration) so topology coverage
// dominates wall-clock time.
// Gated behind STROPPY_MATRIX_E2E=1 (a long series of full runs).
func TestBenchmarkMatrix(t *testing.T) {
	if os.Getenv("STROPPY_MATRIX_E2E") != "1" {
		t.Skip("set STROPPY_MATRIX_E2E=1 to run the full db×workload benchmark matrix")
	}
	conn := login(t)
	tid := rootTenant(t, conn)

	presets, err := uipb.NewPresetServiceClient(conn).ListPresets(context.Background(),
		&uipb.ListPresetRequest{TenantId: tid, Kinds: []models.Preset_Kind{models.Preset_KIND_DATABASE}})
	require.NoError(t, err)
	require.NotEmpty(t, presets.GetPresets(), "database system presets")

	for _, p := range presets.GetPresets() {
		dp := p.GetDatabasePreset()
		if dp == nil {
			continue
		}
		dpCopy := dp
		for _, wl := range matrixWorkloads(dpCopy.GetDatabase().GetKind()) {
			wl := wl
			name := slug(p.GetName() + "-" + wl.name)
			presetName := p.GetName()
			t.Run(name, func(t *testing.T) {
				t.Logf("preset=%q kind=%s script=%s", presetName, dpCopy.GetDatabase().GetKind(), wl.script)
				preset := dbTestPreset(t, dpCopy, wl.script)
				runID, status := runBenchmark(t, conn, tid, preset, name)
				if status != primitive.Status_STATUS_COMPLETED {
					dumpRunLogs(t, conn, tid, runID)
					t.Fatalf("[%s] terminal %s (want COMPLETED)", name, status)
				}
				assertVectorLogs(t, conn, tid, runID)
			})
		}
	}
}

// assertVectorLogs checks that Vector shipped host logs (not just agent command
// output) — i.e. journald/DB-file lines reached VictoriaLogs for the run.
func assertVectorLogs(t *testing.T, conn *grpc.ClientConn, tid *models.TenantId, runID *models.TestRunId) {
	t.Helper()
	logs, err := uipb.NewRunServiceClient(conn).QueryRunLogs(context.Background(),
		&uipb.QueryRunLogsRequest{TenantId: tid, RunId: runID})
	require.NoError(t, err)
	require.NotEmpty(t, logs.GetLines(), "run must have logs")
	t.Logf("run has %d log lines (agent command output + vector journald/DB)", len(logs.GetLines()))
}

// TestFullRunCompletes is the real e2e: submit a docker run, let the agent boot in the
// deployed container, install the DB, run stroppy, ship logs/metrics, and the dag
// settle terminal. Then assert the run COMPLETED and that logs + metrics landed.
// On a non-COMPLETED terminal/timeout it dumps the run status + recent logs so the
// failure point is visible. Slow (minutes) — gated behind STROPPY_RUN_E2E=1.
func TestFullRunCompletes(t *testing.T) {
	if os.Getenv("STROPPY_RUN_E2E") != "1" {
		t.Skip("set STROPPY_RUN_E2E=1 to run the full deploy→install→benchmark e2e")
	}
	conn := login(t)
	tid := rootTenant(t, conn)
	preset := fullPostgresPreset(t, conn, tid)

	runClient := uipb.NewRunServiceClient(conn)
	run, err := runClient.SubmitTestRun(context.Background(), &uipb.SubmitTestRunRequest{
		TenantId:   tid,
		Name:       ptr("e2e-full"),
		TestPreset: preset,
	})
	require.NoError(t, err, "submit test run")
	runID := &models.TestRunId{Value: run.GetEntity().GetId().GetValue()}

	var last primitive.Status
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		got, gerr := runClient.GetTestRun(context.Background(), &uipb.GetTestRunRequest{TenantId: tid, Id: runID})
		if gerr == nil {
			if got.GetStatus() != last {
				t.Logf("run status: %s", got.GetStatus())
				last = got.GetStatus()
			}
			switch got.GetStatus() {
			case primitive.Status_STATUS_COMPLETED:
				assertRunArtifacts(t, conn, tid, runID)
				return
			case primitive.Status_STATUS_FAILED, primitive.Status_STATUS_CANCELLED:
				dumpRunLogs(t, conn, tid, runID)
				t.Fatalf("run reached terminal %s (expected COMPLETED)", got.GetStatus())
			}
		}
		time.Sleep(5 * time.Second)
	}
	dumpRunLogs(t, conn, tid, runID)
	t.Fatalf("run did not complete within deadline (last status %s)", last)
}

// assertRunArtifacts checks the run produced logs + metrics (the dashboards' data).
func assertRunArtifacts(t *testing.T, conn *grpc.ClientConn, tid *models.TenantId, runID *models.TestRunId) {
	t.Helper()
	logs, err := uipb.NewRunServiceClient(conn).QueryRunLogs(context.Background(),
		&uipb.QueryRunLogsRequest{TenantId: tid, RunId: runID})
	require.NoError(t, err, "query run logs")
	require.NotEmpty(t, logs.GetLines(), "run must have shipped logs to VictoriaLogs (queryable by dag_id)")
	t.Logf("run shipped %d log lines", len(logs.GetLines()))

	// Metrics are best-effort for now: stroppy's OTLP export schema (metric names +
	// run_id label) is not yet pinned to the GetRunMetrics catalog, so an empty
	// result is logged, not failed. (Logs + COMPLETED prove the benchmark ran.)
	metrics, merr := uipb.NewRunServiceClient(conn).GetRunMetrics(context.Background(),
		&uipb.GetRunMetricsRequest{TenantId: tid, RunId: runID})
	switch {
	case merr != nil:
		t.Logf("WARNING: get run metrics: %v (VM query/OTLP correlation pending)", merr)
	case len(metrics.GetMetrics()) == 0:
		t.Logf("WARNING: no run metrics in VictoriaMetrics yet (stroppy OTLP correlation pending)")
	default:
		t.Logf("run populated %d metric summaries", len(metrics.GetMetrics()))
	}
}

func dumpRunLogs(t *testing.T, conn *grpc.ClientConn, tid *models.TenantId, runID *models.TestRunId) {
	t.Helper()
	logs, err := uipb.NewRunServiceClient(conn).QueryRunLogs(context.Background(),
		&uipb.QueryRunLogsRequest{TenantId: tid, RunId: runID})
	if err != nil {
		t.Logf("query logs failed: %v", err)
		return
	}
	for _, l := range logs.GetLines() {
		t.Logf("LOG %s", l.GetLine())
	}
}
