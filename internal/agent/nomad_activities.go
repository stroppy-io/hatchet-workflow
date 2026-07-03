package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hashicorp/nomad/api"
)

// nomadAddrEnv names the env var pointing the gateway agent at its local
// Nomad API. Nomad runs docker services on run VMs; only the gateway node's
// agent proxies these calls (as Temporal activities) against Nomad, which it
// always reaches over loopback.
const nomadAddrEnv = "STROPPY_NOMAD_ADDR"

const (
	defaultNomadWaitTimeout  = 5 * time.Minute
	defaultNomadPollInterval = 2 * time.Second
)

// newNomadClient builds a fresh Nomad API client for a single activity call.
// A client is cheap to construct (it does no I/O until a request is made), so
// activities build one per call from STROPPY_NOMAD_ADDR (default
// http://127.0.0.1:4646) rather than plumbing one through the Activities
// struct — there is exactly one Nomad reachable from any given agent process
// (the gateway's own, over loopback), so there is nothing to inject in
// practice, and tests can point every call at an httptest server by setting
// the env var.
func newNomadClient() (*api.Client, error) {
	cfg := api.DefaultConfig()
	if addr := os.Getenv(nomadAddrEnv); addr != "" {
		cfg.Address = addr
	}
	return api.NewClient(cfg)
}

// NomadSubmitJobInput carries a JSON-marshaled *api.Job (Temporal activity
// inputs must be serializable; shipping the already-built job avoids forcing
// the workflow to depend on the nomad/api package). WaitTimeout/PollInterval
// default to 5m/2s when zero.
type NomadSubmitJobInput struct {
	JobJSON      json.RawMessage
	WaitTimeout  time.Duration
	PollInterval time.Duration
}

// NomadAllocStatus is a minimal, Temporal-friendly projection of
// api.AllocationListStub.
type NomadAllocStatus struct {
	ID           string
	NodeID       string
	TaskGroup    string
	ClientStatus string
	Description  string
}

// NomadSubmitJobOutput reports the outcome of a submit-and-wait.
type NomadSubmitJobOutput struct {
	JobID  string
	EvalID string
	Allocs []NomadAllocStatus
}

// NomadSubmitJobActivity registers in.JobJSON with Nomad, then polls
// allocations until every task group has a running allocation, a failed
// allocation is observed, or WaitTimeout elapses. It heartbeats every poll so
// the workflow's activity doesn't time out while a job is still scheduling.
func (a *Activities) NomadSubmitJobActivity(
	ctx context.Context,
	in *NomadSubmitJobInput,
) (*NomadSubmitJobOutput, error) {
	if in == nil || len(in.JobJSON) == 0 {
		return nil, fmt.Errorf("NomadSubmitJob: empty job payload")
	}

	var job api.Job
	if err := json.Unmarshal(in.JobJSON, &job); err != nil {
		return nil, fmt.Errorf("NomadSubmitJob: unmarshal job: %w", err)
	}
	jobID := stringVal(job.ID)
	if jobID == "" {
		return nil, fmt.Errorf("NomadSubmitJob: job has no id")
	}
	wantAllocs := len(job.TaskGroups)

	client, err := newNomadClient()
	if err != nil {
		return nil, fmt.Errorf("NomadSubmitJob %q: nomad client: %w", jobID, err)
	}

	regResp, _, err := client.Jobs().Register(&job, nil)
	if err != nil {
		return nil, fmt.Errorf("NomadSubmitJob %q: register: %w", jobID, err)
	}
	a.logger.InfoContext(ctx, "nomad job registered", "job_id", jobID, "eval_id", regResp.EvalID)

	waitTimeout := in.WaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = defaultNomadWaitTimeout
	}
	pollInterval := in.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultNomadPollInterval
	}
	deadline := time.Now().Add(waitTimeout)

	for {
		allocs, _, err := client.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return nil, fmt.Errorf("NomadSubmitJob %q: list allocations: %w", jobID, err)
		}
		a.heartbeat(ctx, "polling", jobID, len(allocs))

		if failed := firstFailedAlloc(allocs); failed != nil {
			return nil, fmt.Errorf(
				"NomadSubmitJob %q: allocation %s failed: %s",
				jobID, failed.ID, failed.ClientDescription,
			)
		}
		if wantAllocs > 0 && countRunning(allocs) >= wantAllocs {
			return &NomadSubmitJobOutput{
				JobID:  jobID,
				EvalID: regResp.EvalID,
				Allocs: toAllocStatuses(allocs),
			}, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf(
				"NomadSubmitJob %q: timed out after %s waiting for %d allocation(s) to run (have %d)",
				jobID, waitTimeout, wantAllocs, countRunning(allocs),
			)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// NomadJobStatusInput identifies a job to inspect.
type NomadJobStatusInput struct {
	JobID string
}

// NomadJobStatusOutput reports a job's current server-side status and its
// allocations' client statuses.
type NomadJobStatusOutput struct {
	Status string
	Allocs []NomadAllocStatus
}

// NomadJobStatusActivity fetches a job's status and its allocations.
func (a *Activities) NomadJobStatusActivity(
	ctx context.Context,
	in *NomadJobStatusInput,
) (*NomadJobStatusOutput, error) {
	if in == nil || in.JobID == "" {
		return nil, fmt.Errorf("NomadJobStatus: empty job id")
	}

	client, err := newNomadClient()
	if err != nil {
		return nil, fmt.Errorf("NomadJobStatus %q: nomad client: %w", in.JobID, err)
	}

	job, _, err := client.Jobs().Info(in.JobID, nil)
	if err != nil {
		return nil, fmt.Errorf("NomadJobStatus %q: info: %w", in.JobID, err)
	}
	allocs, _, err := client.Jobs().Allocations(in.JobID, false, nil)
	if err != nil {
		return nil, fmt.Errorf("NomadJobStatus %q: list allocations: %w", in.JobID, err)
	}

	a.logger.InfoContext(ctx, "nomad job status", "job_id", in.JobID, "status", stringVal(job.Status))
	return &NomadJobStatusOutput{
		Status: stringVal(job.Status),
		Allocs: toAllocStatuses(allocs),
	}, nil
}

// NomadStopJobInput identifies a job to stop.
type NomadStopJobInput struct {
	JobID string
	Purge bool
}

// NomadStopJobOutput reports the evaluation created by the deregister.
type NomadStopJobOutput struct {
	EvalID string
}

// NomadStopJobActivity deregisters (stops) a Nomad job.
func (a *Activities) NomadStopJobActivity(
	ctx context.Context,
	in *NomadStopJobInput,
) (*NomadStopJobOutput, error) {
	if in == nil || in.JobID == "" {
		return nil, fmt.Errorf("NomadStopJob: empty job id")
	}

	client, err := newNomadClient()
	if err != nil {
		return nil, fmt.Errorf("NomadStopJob %q: nomad client: %w", in.JobID, err)
	}

	evalID, _, err := client.Jobs().Deregister(in.JobID, in.Purge, nil)
	if err != nil {
		return nil, fmt.Errorf("NomadStopJob %q: deregister: %w", in.JobID, err)
	}
	a.logger.InfoContext(ctx, "nomad job stopped", "job_id", in.JobID, "purge", in.Purge, "eval_id", evalID)
	return &NomadStopJobOutput{EvalID: evalID}, nil
}

// NomadAllocLogsInput identifies an allocation/task log stream to fetch.
// LogType is "stdout" or "stderr"; empty defaults to "stdout".
type NomadAllocLogsInput struct {
	AllocID string
	Task    string
	LogType string
}

// NomadAllocLogsOutput carries the fetched log content.
type NomadAllocLogsOutput struct {
	Data string
}

// NomadAllocLogsActivity fetches the full current content of one task's
// stdout/stderr log from its allocation (no follow: it reads once from the
// start of the log and returns).
func (a *Activities) NomadAllocLogsActivity(
	ctx context.Context,
	in *NomadAllocLogsInput,
) (*NomadAllocLogsOutput, error) {
	if in == nil || in.AllocID == "" {
		return nil, fmt.Errorf("NomadAllocLogs: empty alloc id")
	}
	logType := in.LogType
	if logType == "" {
		logType = api.FSLogNameStdout
	}

	client, err := newNomadClient()
	if err != nil {
		return nil, fmt.Errorf("NomadAllocLogs %q: nomad client: %w", in.AllocID, err)
	}

	alloc, _, err := client.Allocations().Info(in.AllocID, nil)
	if err != nil {
		return nil, fmt.Errorf("NomadAllocLogs %q: allocation info: %w", in.AllocID, err)
	}

	cancelCh := make(chan struct{})
	frames, errCh := client.AllocFS().Logs(alloc, false, in.Task, logType, api.OriginStart, 0, cancelCh, nil)
	reader := api.NewFrameReader(frames, errCh, cancelCh)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("NomadAllocLogs %q: read logs: %w", in.AllocID, err)
	}
	a.logger.DebugContext(ctx, "nomad alloc logs fetched",
		"alloc_id", in.AllocID, "task", in.Task, "log_type", logType, "bytes", len(data))
	return &NomadAllocLogsOutput{Data: string(data)}, nil
}

// --- helpers -------------------------------------------------------------

func stringVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func firstFailedAlloc(allocs []*api.AllocationListStub) *api.AllocationListStub {
	for _, alloc := range allocs {
		if alloc.ClientStatus == api.AllocClientStatusFailed {
			return alloc
		}
	}
	return nil
}

func countRunning(allocs []*api.AllocationListStub) int {
	n := 0
	for _, alloc := range allocs {
		if alloc.ClientStatus == api.AllocClientStatusRunning {
			n++
		}
	}
	return n
}

func toAllocStatuses(allocs []*api.AllocationListStub) []NomadAllocStatus {
	out := make([]NomadAllocStatus, 0, len(allocs))
	for _, alloc := range allocs {
		out = append(out, NomadAllocStatus{
			ID:           alloc.ID,
			NodeID:       alloc.NodeID,
			TaskGroup:    alloc.TaskGroup,
			ClientStatus: alloc.ClientStatus,
			Description:  alloc.ClientDescription,
		})
	}
	return out
}
