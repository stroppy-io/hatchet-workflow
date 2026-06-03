//go:build integration

package tests

import (
	"testing"
	"time"

	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deployment "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// TestE2E is the core end-to-end flow: login -> wizard (start/patch/finish+start)
// -> poll until terminal -> assert COMPLETED + a sane run summary, all on the
// docker provider against a real running stack.
func TestE2E(t *testing.T) {
	e := setup(t)

	runID := e.launchDockerRun(t, "e2e-docker-pg-tpcc", 1, 1, "10s")

	rec := e.waitTerminal(t, runID, 15*time.Minute)

	if rec.GetStatus() != common.Status_STATUS_COMPLETED {
		t.Fatalf("run %s terminal status = %s, want COMPLETED", runID, rec.GetStatus())
	}

	s := rec.GetSummary()
	if s == nil {
		t.Fatal("completed run has nil summary")
	}
	if s.GetProvider() != deployment.Provider_PROVIDER_DOCKER {
		t.Errorf("summary.provider = %s, want PROVIDER_DOCKER", s.GetProvider())
	}
	if s.GetDbKind() != domain.Database_KIND_POSTGRES {
		t.Errorf("summary.db_kind = %s, want KIND_POSTGRES", s.GetDbKind())
	}
	if s.GetNodeCount() == 0 {
		t.Error("summary.node_count = 0, want > 0")
	}
	if s.GetStroppyVersion() != stroppyVersion {
		t.Errorf("summary.stroppy_version = %q, want %q", s.GetStroppyVersion(), stroppyVersion)
	}
	if d := s.GetDuration().AsDuration(); d <= 0 {
		t.Errorf("summary.duration = %s, want > 0", d)
	}
	if s.GetFinishedAt() == nil {
		t.Error("summary.finished_at is nil for completed run")
	}
	if pct := s.GetProgressPct(); pct != 100 {
		t.Logf("note: summary.progress_pct = %d (expected 100 on completion)", pct)
	}

	t.Logf("OK run=%s nodes=%d duration=%s provider=%s", runID, s.GetNodeCount(),
		s.GetDuration().AsDuration(), s.GetProvider())
}
