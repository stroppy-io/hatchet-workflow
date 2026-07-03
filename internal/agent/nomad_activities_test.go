package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"

	dslnomad "github.com/stroppy-io/stroppy-cloud/internal/dsl/nomad"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// newTestActivities builds an Activities whose heartbeat is a no-op:
// activity.RecordHeartbeat (the default) panics outside a real Temporal
// activity context, and these tests call the Nomad activities directly with
// context.Background()/t.Context(), not through a Temporal worker.
func newTestActivities() *Activities {
	return NewActivities(WithHeartbeater(func(context.Context, ...any) {}))
}

// fakeNomadServer serves the small subset of the Nomad HTTP API the gateway
// agent activities call: register/info/allocations/deregister for jobs,
// allocation info, and the streaming fs/logs endpoint. pollsBeforeRunning
// controls how many "Jobs().Allocations" polls return a pending allocation
// before the allocation flips to running, so tests can exercise the
// submit-and-poll loop.
type fakeNomadServer struct {
	pollsBeforeRunning int32
	pollCount          atomic.Int32
	allocClientStatus  string // overrides the terminal status once polls are exhausted; "" means running

	// allocs, when non-nil, replaces the default single-alloc response
	// entirely: the handler serves this fixed list on every poll instead of
	// synthesizing one from pollsBeforeRunning/allocClientStatus. Used to
	// exercise reschedule scenarios (stale failed alloc + running
	// replacement) where the shape of the allocation list itself is what's
	// under test.
	allocs []*api.AllocationListStub
}

func newFakeNomadServer(t *testing.T, fake *fakeNomadServer) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("PUT /v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Job *api.Job
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(t, w, api.JobRegisterResponse{EvalID: "eval-1"})
	})

	mux.HandleFunc("GET /v1/job/{id}/allocations", func(w http.ResponseWriter, r *http.Request) {
		fake.pollCount.Add(1)
		if fake.allocs != nil {
			writeJSON(t, w, fake.allocs)
			return
		}
		n := fake.pollCount.Load()
		status := api.AllocClientStatusRunning
		desc := "running"
		if n <= fake.pollsBeforeRunning {
			status = api.AllocClientStatusPending
			desc = "pending"
		} else if fake.allocClientStatus != "" {
			status = fake.allocClientStatus
			desc = "alloc failed on purpose"
		}
		allocs := []*api.AllocationListStub{
			{
				ID:                "alloc-1",
				NodeID:            "node-1",
				JobID:             r.PathValue("id"),
				TaskGroup:         "postgres-node-1",
				ClientStatus:      status,
				ClientDescription: desc,
			},
		}
		writeJSON(t, w, allocs)
	})

	mux.HandleFunc("GET /v1/job/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		status := "running"
		writeJSON(t, w, api.Job{ID: &id, Status: &status})
	})

	mux.HandleFunc("DELETE /v1/job/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, struct{ EvalID string }{EvalID: "eval-stop-1"})
	})

	mux.HandleFunc("GET /v1/allocation/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, api.Allocation{ID: r.PathValue("id"), NodeID: "node-1"})
	})

	mux.HandleFunc("GET /v1/client/fs/logs/{id}", func(w http.ResponseWriter, r *http.Request) {
		frame := api.StreamFrame{
			Data:   []byte("log line one\nlog line two\n"),
			File:   "postgres.stdout.0",
			Offset: 27,
		}
		writeJSON(t, w, frame)
	})

	return httptest.NewServer(mux)
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("write fake nomad response: %v", err)
	}
}

func testJobJSON(t *testing.T) []byte {
	t.Helper()
	svc := &dslpb.ServiceSpec{
		Name:    "postgres",
		Image:   "postgres:17",
		Network: "host",
		Health:  &dslpb.HealthCheck{Http: ":8008/health", Timeout: "30s"},
	}
	job, err := dslnomad.BuildJob(svc, []dslnomad.NodeRef{{NodeID: "node-1", PrivateIP: "10.0.0.1"}})
	if err != nil {
		t.Fatalf("BuildJob: %v", err)
	}
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	return data
}

func TestNomadSubmitJobActivityHappyPath(t *testing.T) {
	fake := &fakeNomadServer{pollsBeforeRunning: 2}
	server := newFakeNomadServer(t, fake)
	defer server.Close()
	t.Setenv("STROPPY_NOMAD_ADDR", server.URL)

	a := newTestActivities()
	out, err := a.NomadSubmitJobActivity(t.Context(), &NomadSubmitJobInput{
		JobJSON:      testJobJSON(t),
		PollInterval: time.Millisecond,
		WaitTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NomadSubmitJobActivity: %v", err)
	}
	if got, want := out.JobID, "postgres"; got != want {
		t.Fatalf("JobID = %q, want %q", got, want)
	}
	if got, want := out.EvalID, "eval-1"; got != want {
		t.Fatalf("EvalID = %q, want %q", got, want)
	}
	if len(out.Allocs) != 1 {
		t.Fatalf("Allocs = %v, want 1 entry", out.Allocs)
	}
	if got, want := out.Allocs[0].ClientStatus, api.AllocClientStatusRunning; got != want {
		t.Fatalf("Allocs[0].ClientStatus = %q, want %q", got, want)
	}
	if fake.pollCount.Load() < fake.pollsBeforeRunning+1 {
		t.Fatalf("expected at least %d polls, got %d", fake.pollsBeforeRunning+1, fake.pollCount.Load())
	}
}

func TestNomadSubmitJobActivityFailsOnFailedAlloc(t *testing.T) {
	fake := &fakeNomadServer{allocClientStatus: api.AllocClientStatusFailed}
	server := newFakeNomadServer(t, fake)
	defer server.Close()
	t.Setenv("STROPPY_NOMAD_ADDR", server.URL)

	a := newTestActivities()
	_, err := a.NomadSubmitJobActivity(t.Context(), &NomadSubmitJobInput{
		JobJSON:      testJobJSON(t),
		PollInterval: time.Millisecond,
		WaitTimeout:  5 * time.Second,
	})
	if err == nil {
		t.Fatalf("NomadSubmitJobActivity: want error, got nil")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Fatalf("error = %v, want it to mention the failure", err)
	}
}

// TestNomadSubmitJobActivitySurvivesReschedule exercises Nomad's normal
// reschedule behavior: a transient container failure produces a failed
// allocation whose NextAllocation points at its replacement, and the
// replacement (same TaskGroup, higher CreateIndex) is running. The job is
// healthy as a whole once the replacement runs, so the activity must
// succeed rather than surface the stale failed alloc as a fatal error.
func TestNomadSubmitJobActivitySurvivesReschedule(t *testing.T) {
	fake := &fakeNomadServer{
		allocs: []*api.AllocationListStub{
			{
				ID:                "alloc-1",
				NodeID:            "node-1",
				TaskGroup:         "postgres-node-1",
				ClientStatus:      api.AllocClientStatusFailed,
				ClientDescription: "transient failure, rescheduled",
				NextAllocation:    "alloc-2",
				CreateIndex:       10,
			},
			{
				ID:                "alloc-2",
				NodeID:            "node-1",
				TaskGroup:         "postgres-node-1",
				ClientStatus:      api.AllocClientStatusRunning,
				ClientDescription: "running",
				CreateIndex:       11,
			},
		},
	}
	server := newFakeNomadServer(t, fake)
	defer server.Close()
	t.Setenv("STROPPY_NOMAD_ADDR", server.URL)

	a := newTestActivities()
	out, err := a.NomadSubmitJobActivity(t.Context(), &NomadSubmitJobInput{
		JobJSON:      testJobJSON(t),
		PollInterval: time.Millisecond,
		WaitTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NomadSubmitJobActivity: %v", err)
	}
	if got, want := out.JobID, "postgres"; got != want {
		t.Fatalf("JobID = %q, want %q", got, want)
	}
}

// TestNomadSubmitJobActivityFailsOnTerminalFailedAlloc verifies a failed
// allocation with no replacement (NextAllocation empty) is still fatal: this
// is not a reschedule-in-progress, it's a terminal failure.
func TestNomadSubmitJobActivityFailsOnTerminalFailedAlloc(t *testing.T) {
	fake := &fakeNomadServer{
		allocs: []*api.AllocationListStub{
			{
				ID:                "alloc-1",
				NodeID:            "node-1",
				TaskGroup:         "postgres-node-1",
				ClientStatus:      api.AllocClientStatusFailed,
				ClientDescription: "terminal failure",
				NextAllocation:    "",
				CreateIndex:       10,
			},
		},
	}
	server := newFakeNomadServer(t, fake)
	defer server.Close()
	t.Setenv("STROPPY_NOMAD_ADDR", server.URL)

	a := newTestActivities()
	_, err := a.NomadSubmitJobActivity(t.Context(), &NomadSubmitJobInput{
		JobJSON:      testJobJSON(t),
		PollInterval: time.Millisecond,
		WaitTimeout:  5 * time.Second,
	})
	if err == nil {
		t.Fatalf("NomadSubmitJobActivity: want error, got nil")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Fatalf("error = %v, want it to mention the failure", err)
	}
}

func TestNomadSubmitJobActivityRejectsEmptyPayload(t *testing.T) {
	a := newTestActivities()
	_, err := a.NomadSubmitJobActivity(t.Context(), &NomadSubmitJobInput{})
	if err == nil {
		t.Fatalf("NomadSubmitJobActivity with empty payload: want error, got nil")
	}
}

func TestNomadJobStatusActivity(t *testing.T) {
	fake := &fakeNomadServer{}
	server := newFakeNomadServer(t, fake)
	defer server.Close()
	t.Setenv("STROPPY_NOMAD_ADDR", server.URL)

	a := newTestActivities()
	out, err := a.NomadJobStatusActivity(t.Context(), &NomadJobStatusInput{JobID: "postgres"})
	if err != nil {
		t.Fatalf("NomadJobStatusActivity: %v", err)
	}
	if got, want := out.Status, "running"; got != want {
		t.Fatalf("Status = %q, want %q", got, want)
	}
	if len(out.Allocs) != 1 {
		t.Fatalf("Allocs = %v, want 1 entry", out.Allocs)
	}
}

func TestNomadStopJobActivity(t *testing.T) {
	fake := &fakeNomadServer{}
	server := newFakeNomadServer(t, fake)
	defer server.Close()
	t.Setenv("STROPPY_NOMAD_ADDR", server.URL)

	a := newTestActivities()
	out, err := a.NomadStopJobActivity(t.Context(), &NomadStopJobInput{JobID: "postgres", Purge: true})
	if err != nil {
		t.Fatalf("NomadStopJobActivity: %v", err)
	}
	if got, want := out.EvalID, "eval-stop-1"; got != want {
		t.Fatalf("EvalID = %q, want %q", got, want)
	}
}

func TestNomadAllocLogsActivity(t *testing.T) {
	fake := &fakeNomadServer{}
	server := newFakeNomadServer(t, fake)
	defer server.Close()
	t.Setenv("STROPPY_NOMAD_ADDR", server.URL)

	a := newTestActivities()
	out, err := a.NomadAllocLogsActivity(t.Context(), &NomadAllocLogsInput{
		AllocID: "alloc-1",
		Task:    "postgres",
		LogType: "stdout",
	})
	if err != nil {
		t.Fatalf("NomadAllocLogsActivity: %v", err)
	}
	if got, want := out.Data, "log line one\nlog line two\n"; got != want {
		t.Fatalf("Data = %q, want %q", got, want)
	}
}
