//go:build integration

// Package tests holds end-to-end tests that drive the real Connect API of a
// running stroppy-cloud control plane (gateway on :8080) with the docker
// provider. They are gated behind the `integration` build tag and expect a
// live stack (see Makefile `docker-up` / `scripts/smoke.sh`).
//
// Run:
//
//	E2E_BASE_URL=http://127.0.0.1:8080 go test -tags=integration -timeout 30m -run TestE2E ./tests/
package tests

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/apiconnect"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deployment "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

const (
	defaultBaseURL = "http://127.0.0.1:8080"
	adminLogin     = "admin"
	adminPassword  = "admin"

	// stroppyVersion must be >= STROPPY_MIN_VERSION on the server.
	stroppyVersion = "5.1.2"
	// tpcc/procs exercises every recipe phase with a tiny footprint.
	defaultScript = "tpcc/procs"
)

func baseURL() string {
	if v := os.Getenv("E2E_BASE_URL"); v != "" {
		return v
	}
	return defaultBaseURL
}

func login() string {
	if v := os.Getenv("E2E_LOGIN"); v != "" {
		return v
	}
	return adminLogin
}

func password() string {
	if v := os.Getenv("E2E_PASSWORD"); v != "" {
		return v
	}
	return adminPassword
}

// e2e bundles authenticated Connect clients plus the resolved tenant.
type e2e struct {
	http        *http.Client
	base        string
	token       string
	tenantID    string
	iam         apiconnect.IamServiceClient
	wizard      apiconnect.TestWizardServiceClient
	runs        apiconnect.TestRunServiceClient
	overview    apiconnect.TestRunOverviewServiceClient
	dbPresets   apiconnect.DatabasePresetServiceClient
	testPresets apiconnect.TestPresetServiceClient
	settings    apiconnect.TenantSettingsServiceClient
	quota       apiconnect.QuotaServiceClient
}

// authInterceptor injects "Authorization: Bearer <token>" on every client call,
// for both unary and server-streaming RPCs (the overview subscription).
type authInterceptor struct{ token string }

func (a authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			req.Header().Set("Authorization", "Bearer "+a.token)
		}
		return next(ctx, req)
	}
}

func (a authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", "Bearer "+a.token)
		return conn
	}
}

func (a authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// setup logs in as admin, builds authenticated clients and resolves the tenant.
func setup(t *testing.T) *e2e {
	t.Helper()
	base := baseURL()
	hc := &http.Client{Timeout: 30 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	noauth := apiconnect.NewIamServiceClient(hc, base)
	lr, err := noauth.Login(ctx, &api.LoginRequest{
		Login:    login(),
		Password: password(),
	})
	if err != nil {
		t.Fatalf("login %s@%s: %v", login(), base, err)
	}
	token := lr.GetTokens().GetAccessToken()
	if token == "" {
		t.Fatal("login: empty access token")
	}

	opt := connect.WithInterceptors(authInterceptor{token: token})
	// Streaming (overview subscription) must not be killed by a client timeout;
	// it is bounded by the call context instead.
	streamHC := &http.Client{}
	e := &e2e{
		http:        hc,
		base:        base,
		token:       token,
		iam:         apiconnect.NewIamServiceClient(hc, base, opt),
		wizard:      apiconnect.NewTestWizardServiceClient(hc, base, opt),
		runs:        apiconnect.NewTestRunServiceClient(hc, base, opt),
		overview:    apiconnect.NewTestRunOverviewServiceClient(streamHC, base, opt),
		dbPresets:   apiconnect.NewDatabasePresetServiceClient(hc, base, opt),
		testPresets: apiconnect.NewTestPresetServiceClient(hc, base, opt),
		settings:    apiconnect.NewTenantSettingsServiceClient(hc, base, opt),
		quota:       apiconnect.NewQuotaServiceClient(hc, base, opt),
	}

	tr, err := e.iam.ListMyTenants(ctx, &api.ListMyTenantsRequest{})
	if err != nil {
		t.Fatalf("ListMyTenants: %v", err)
	}
	if len(tr.GetTenants()) == 0 {
		t.Fatal("ListMyTenants: no tenants for admin")
	}
	e.tenantID = tr.GetTenants()[0].GetId()
	t.Logf("tenant=%s (%s)", e.tenantID, tr.GetTenants()[0].GetName())
	return e
}

// dockerPostgresDatabase builds a typed, provider-agnostic single-node
// PostgreSQL database (the server bakes topology + infrastructure_plan).
func dockerPostgresDatabase(version string) *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Version: version,
				Engine: &domain.DatabaseParams_Postgres{
					Postgres: &domain.PostgresParams{},
				},
			},
		},
	}
}

// tinyWorkload builds a minimal stroppy workload (vus=1, scale=1, short).
func tinyWorkload(vus uint32, scale float64, duration string) *domain.Workload {
	return &domain.Workload{
		StroppyVersion: stroppyVersion,
		Script:         defaultScript,
		Protocol:       domain.Workload_PROTOCOL_PG,
		Execution: &domain.Workload_Execution{
			Vus:          vus,
			Limit:        &domain.Workload_Execution_Duration{Duration: duration},
			NoThresholds: true,
		},
		Parameters: &domain.Workload_Parameters{
			PoolSize:    2,
			ScaleFactor: scale,
		},
	}
}

// launchDockerRun drives the wizard and returns the launched run's id.
func (e *e2e) launchDockerRun(t *testing.T, name string, vus uint32, scale float64, duration string) string {
	return e.launchDockerRunRec(t, name, vus, scale, duration).GetEntity().GetId()
}

// launchDockerRunRec drives the wizard (start -> patch -> finish+start) and
// returns the launched run record (status is PENDING right after launch, before
// the workflow flips it to RUNNING). It fails the test if the draft is not ready.
func (e *e2e) launchDockerRunRec(t *testing.T, name string, vus uint32, scale float64, duration string) *models.TestRunRecord {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sd, err := e.wizard.StartTestWizard(ctx, &api.StartTestWizardRequest{
		TenantId: e.tenantID,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("StartTestWizard: %v", err)
	}
	draftID := sd.GetDraft().GetEntity().GetId()
	if draftID == "" {
		t.Fatal("StartTestWizard: empty draft id")
	}

	pr, err := e.wizard.PatchTestWizard(ctx, &api.PatchTestWizardRequest{
		TenantId: e.tenantID,
		DraftId:  draftID,
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Database: dockerPostgresDatabase("16"),
		Workload: tinyWorkload(vus, scale, duration),
	})
	if err != nil {
		t.Fatalf("PatchTestWizard: %v", err)
	}
	if !pr.GetDraft().GetReady() {
		for _, fe := range pr.GetDraft().GetErrors() {
			t.Logf("draft error: %s -> %s", fe.GetField(), fe.GetMessage())
		}
		t.Fatal("PatchTestWizard: draft not ready after docker/postgres/tpcc patch")
	}

	fr, err := e.wizard.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
		TenantId: e.tenantID,
		DraftId:  draftID,
		Start:    true,
	})
	if err != nil {
		t.Fatalf("FinishTestWizard: %v", err)
	}
	rec := fr.GetRun()
	if rec.GetEntity().GetId() == "" {
		t.Fatal("FinishTestWizard: empty run id (start=true)")
	}
	t.Logf("launched run %s status=%s (draft %s)", rec.GetEntity().GetId(), rec.GetStatus(), draftID)
	return rec
}

func isTerminal(s common.Status) bool {
	switch s {
	case common.Status_STATUS_COMPLETED,
		common.Status_STATUS_FAILED,
		common.Status_STATUS_CANCELLED,
		common.Status_STATUS_SKIPPED:
		return true
	default:
		return false
	}
}

// getRun fetches the current run record.
func (e *e2e) getRun(t *testing.T, runID string) *models.TestRunRecord {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	gr, err := e.runs.GetTestRun(ctx, &api.GetTestRunRequest{
		TenantId: e.tenantID,
		Id:       runID,
	})
	if err != nil {
		t.Fatalf("GetTestRun(%s): %v", runID, err)
	}
	return gr.GetRun()
}

// waitTerminal polls GetTestRun until the run reaches a terminal status.
func (e *e2e) waitTerminal(t *testing.T, runID string, timeout time.Duration) *models.TestRunRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last common.Status = common.Status_STATUS_UNSPECIFIED
	for {
		rec := e.getRun(t, runID)
		if rec.GetStatus() != last {
			last = rec.GetStatus()
			t.Logf("run %s status=%s progress=%d%%", runID, last, rec.GetSummary().GetProgressPct())
		}
		if isTerminal(rec.GetStatus()) {
			return rec
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not reach terminal state within %s (last=%s)", runID, timeout, last)
		}
		time.Sleep(5 * time.Second)
	}
}

// pollStatuses polls GetTestRun until terminal, returning the ordered list of
// DISTINCT statuses observed (consecutive duplicates collapsed). Transient states
// between polls may be missed; the overview stream captures them exhaustively.
func (e *e2e) pollStatuses(t *testing.T, runID string, interval, timeout time.Duration) []common.Status {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var seq []common.Status
	for {
		st := e.getRun(t, runID).GetStatus()
		if len(seq) == 0 || seq[len(seq)-1] != st {
			seq = append(seq, st)
			t.Logf("run %s -> %s", runID, st)
		}
		if isTerminal(st) {
			return seq
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s not terminal within %s (seq=%v)", runID, timeout, seq)
		}
		time.Sleep(interval)
	}
}

// containsSubsequence reports whether want appears as an ordered (not necessarily
// contiguous) subsequence of got.
func containsSubsequence(got, want []common.Status) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

// waitStatus polls until the run reaches one of the wanted statuses (or terminal).
func (e *e2e) waitStatus(t *testing.T, runID string, timeout time.Duration, want ...common.Status) *models.TestRunRecord {
	t.Helper()
	wantSet := map[common.Status]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	deadline := time.Now().Add(timeout)
	for {
		rec := e.getRun(t, runID)
		if wantSet[rec.GetStatus()] || isTerminal(rec.GetStatus()) {
			return rec
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not reach %v within %s (last=%s)", runID, want, timeout, rec.GetStatus())
		}
		time.Sleep(2 * time.Second)
	}
}
