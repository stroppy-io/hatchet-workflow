//go:build integration

// Package integration holds the end-to-end demo test for the docker provider.
//
// Unlike a unit test, this drives the REAL deployed topology: a control-plane
// server runs as a compose service (server-demo) on the shared network; the
// wizard bakes + launches a run; Temporal deploys a real agent CONTAINER that
// knows ONLY the server address; that container bootstraps itself by downloading
// the agent binary from the server, connects back to Temporal through the
// server's gRPC proxy as a worker, and a worker session drives it to apt-install
// postgres (apt routed through the server's cache) + fetch the stand-in stroppy
// (from the server's artifact cache) + run it. We observe the whole pipeline via
// the Overview projection until terminal.
//
// Preconditions (see `make demo-up`):
//   - docker compose --profile demo up -d --build  (temporal, postgres, minio,
//     server-demo) — server-demo publishes api gRPC on :8081 and the gateway on
//     :8080.
//   - the stroppy-agent:latest image is built.
//
// Run with: go test -tags=integration ./tests/integration/ -run E2E -v -timeout 15m
package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

func terminal(s common.Status) bool {
	switch s {
	case common.Status_STATUS_COMPLETED,
		common.Status_STATUS_FAILED,
		common.Status_STATUS_CANCELLED,
		common.Status_STATUS_SKIPPED:
		return true
	}
	return false
}

func apiAddr() string {
	if v := os.Getenv("STROPPY_API_ADDR"); v != "" {
		return v
	}
	return "127.0.0.1:8081"
}

func TestE2E_WizardToOverview(t *testing.T) {
	conn, err := grpc.NewClient(apiAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial api %s: %v", apiAddr(), err)
	}
	defer conn.Close()

	wizard := api.NewTestWizardServiceClient(conn)
	overview := api.NewTestRunOverviewServiceClient(conn)

	ctx := context.Background()
	const tenant = "demo-tenant"

	// 1. Wizard: open a draft. InitialForm seeds a valid single-node postgres +
	//    workload + docker provider, so Compute already marks it ready.
	start, err := wizard.StartTestWizard(ctx, &api.StartTestWizardRequest{TenantId: tenant, Name: "e2e"})
	if err != nil {
		t.Fatalf("StartTestWizard (is server-demo up at %s?): %v", apiAddr(), err)
	}
	draftID := start.GetDraft().GetEntity().GetId()
	if !start.GetDraft().GetReady() {
		t.Fatalf("draft not ready after start; errors=%v", start.GetDraft().GetErrors())
	}
	t.Logf("draft %s ready", draftID)

	// 2. Finish + start: bake domain.TestRun and launch TestWorkflow.
	fin, err := wizard.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
		TenantId: tenant, DraftId: draftID, Start: true,
	})
	if err != nil {
		t.Fatalf("FinishTestWizard: %v", err)
	}
	runID := fin.GetRun().GetEntity().GetId()
	t.Logf("run %s launched", runID)

	// 3. Observe via Overview until terminal.
	deadline := time.Now().Add(12 * time.Minute)
	var last common.Status
	for time.Now().Before(deadline) {
		ov, err := overview.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{TenantId: tenant, RunId: runID})
		if err != nil {
			t.Logf("overview err (retrying): %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		o := ov.GetOverview()
		if o.GetStatus() != last {
			last = o.GetStatus()
			t.Logf("status=%v progress=%d%% stages=%d", o.GetStatus(), o.GetProgressPct(), len(o.GetPipeline().GetRoots()))
			for _, n := range o.GetPipeline().GetRoots() {
				t.Logf("  stage %-22s %v", n.GetName(), n.GetStatus())
			}
		}
		if terminal(o.GetStatus()) {
			if o.GetStatus() == common.Status_STATUS_COMPLETED {
				t.Logf("run COMPLETED")
				assertRouting(t, o.GetPipeline().GetRoots())
				return
			}
			t.Fatalf("run ended non-completed: %v", o.GetStatus())
		}
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("timeout: last status %v", last)
}

// assertRouting proves Temporal placed each component's steps on its OWN machine:
// pipeline nodes are namespaced "<instanceID>/<step>", so the postgres-install
// step and the run-stroppy step must carry DIFFERENT instance prefixes (database
// machine vs workload machine). Same prefix => both ran on one machine (the old
// shared-queue misroute bug).
func assertRouting(t *testing.T, roots []*monitor.PipelineNode) {
	t.Helper()
	prefix := func(suffix string) string {
		for _, n := range roots {
			name := n.GetName()
			if strings.HasSuffix(name, "/"+suffix) {
				return strings.TrimSuffix(name, "/"+suffix)
			}
		}
		return ""
	}
	dbNode := prefix("install postgresql-16")
	wlNode := prefix("run stroppy")
	if dbNode == "" {
		t.Fatalf("no '<instance>/install postgresql-16' node found; nodes=%v", names(roots))
	}
	if wlNode == "" {
		t.Fatalf("no '<instance>/run stroppy' node found; nodes=%v", names(roots))
	}
	if dbNode == wlNode {
		t.Fatalf("routing FAILED: postgres install and stroppy both ran on instance %q (expected different machines)", dbNode)
	}
	t.Logf("routing OK: postgres install on %q, stroppy on %q (distinct machines)", dbNode, wlNode)
}

func names(roots []*monitor.PipelineNode) []string {
	out := make([]string, 0, len(roots))
	for _, n := range roots {
		out = append(out, n.GetName())
	}
	return out
}
