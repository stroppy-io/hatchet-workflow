package workflows

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	dockerexec "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/docker"
	terraformexec "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/activity"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type QuotaManager interface {
	Reserve(ctx context.Context, tenantID, runID, workflowID string, plan *deploymentpb.InfrastructurePlan, refs []*workflowpb.QuotaRequestRef) ([]*workflowpb.QuotaAllocationRef, error)
	Commit(ctx context.Context, tenantID, runID string) ([]*workflowpb.QuotaAllocationRef, error)
	Release(ctx context.Context, tenantID, runID string) (uint32, error)
}

type NetworkManager interface {
	Reserve(ctx context.Context, tenantID, runID, workflowID string, plan *deploymentpb.InfrastructurePlan) (string, error)
	Commit(ctx context.Context, tenantID, runID string) (string, error)
	Release(ctx context.Context, tenantID, runID string) (uint32, error)
}

type RunLogWriter interface {
	Write(context.Context, []*monitor.LogLine) error
}

type deploymentActivities struct {
	quotas   QuotaManager
	networks NetworkManager
	logs     RunLogWriter
}

func NewDeploymentActivities(quotas QuotaManager, networks NetworkManager, logs RunLogWriter) workflowpb.DeploymentServiceActivities {
	return &deploymentActivities{quotas: quotas, networks: networks, logs: logs}
}

func (a *deploymentActivities) AcquireNetworkActivity(ctx context.Context, req *workflowpb.AcquireNetworkActivityRequest) (*workflowpb.AcquireNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.networks == nil || req.GetPlan().GetProvider() != deploymentpb.Provider_PROVIDER_YANDEX {
		return &workflowpb.AcquireNetworkActivityResponse{}, nil
	}
	info := activity.GetInfo(ctx)
	cidr, err := a.networks.Reserve(ctx, req.GetTenantId(), req.GetRunId(), info.WorkflowExecution.ID, req.GetPlan())
	if err != nil {
		return nil, err
	}
	return &workflowpb.AcquireNetworkActivityResponse{NetworkCidr: cidr}, nil
}

func (a *deploymentActivities) CommitNetworkActivity(ctx context.Context, req *workflowpb.CommitNetworkActivityRequest) (*workflowpb.CommitNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.networks == nil {
		return &workflowpb.CommitNetworkActivityResponse{}, nil
	}
	cidr, err := a.networks.Commit(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.CommitNetworkActivityResponse{NetworkCidr: cidr}, nil
}

func (a *deploymentActivities) ReleaseNetworkActivity(ctx context.Context, req *workflowpb.ReleaseNetworkActivityRequest) (*workflowpb.ReleaseNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.networks == nil {
		return &workflowpb.ReleaseNetworkActivityResponse{}, nil
	}
	released, err := a.networks.Release(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.ReleaseNetworkActivityResponse{Released: released}, nil
}

func (a *deploymentActivities) AcquireQuotasActivity(ctx context.Context, req *workflowpb.AcquireQuotasActivityRequest) (*workflowpb.AcquireQuotasActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.quotas != nil {
		info := activity.GetInfo(ctx)
		allocations, err := a.quotas.Reserve(ctx, req.GetTenantId(), req.GetRunId(), info.WorkflowExecution.ID, req.GetPlan(), req.GetQuotaRequests())
		if err != nil {
			return nil, err
		}
		return &workflowpb.AcquireQuotasActivityResponse{QuotaAllocations: allocations}, nil
	}
	allocations := echoQuotaAllocations(req.GetQuotaRequests())
	return &workflowpb.AcquireQuotasActivityResponse{QuotaAllocations: allocations}, nil
}

func (a *deploymentActivities) CommitQuotasActivity(ctx context.Context, req *workflowpb.CommitQuotasActivityRequest) (*workflowpb.CommitQuotasActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.quotas == nil {
		return &workflowpb.CommitQuotasActivityResponse{}, nil
	}
	allocations, err := a.quotas.Commit(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.CommitQuotasActivityResponse{QuotaAllocations: allocations}, nil
}

func (a *deploymentActivities) ReleaseQuotasActivity(ctx context.Context, req *workflowpb.ReleaseQuotasActivityRequest) (*workflowpb.ReleaseQuotasActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.quotas == nil {
		return &workflowpb.ReleaseQuotasActivityResponse{}, nil
	}
	released, err := a.quotas.Release(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.ReleaseQuotasActivityResponse{Released: released}, nil
}

func (a *deploymentActivities) DockerPullActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	stderr := newServerLogWriter(ctx, a.logs, serverLogContextFromDocker(req.GetLogContext()), monitor.Stream_STREAM_STDERR)
	executor, err := dockerexec.NewExecutor(dockerexec.WithStderr(io.MultiWriter(os.Stderr, stderr)))
	if err != nil {
		return nil, err
	}
	defer stderr.Flush()
	defer executor.Close()
	return withHeartbeat(ctx, func() (*deploymentpb.Docker_Output, error) {
		return executor.Pull(ctx, req)
	})
}

func (a *deploymentActivities) DockerUpActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	stderr := newServerLogWriter(ctx, a.logs, serverLogContextFromDocker(req.GetLogContext()), monitor.Stream_STREAM_STDERR)
	executor, err := dockerexec.NewExecutor(dockerexec.WithStderr(io.MultiWriter(os.Stderr, stderr)))
	if err != nil {
		return nil, err
	}
	defer stderr.Flush()
	defer executor.Close()
	return withHeartbeat(ctx, func() (*deploymentpb.Docker_Output, error) {
		return executor.Up(ctx, req)
	})
}

func (a *deploymentActivities) DockerDownActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	stderr := newServerLogWriter(ctx, a.logs, serverLogContextFromDocker(req.GetLogContext()), monitor.Stream_STREAM_STDERR)
	executor, err := dockerexec.NewExecutor(dockerexec.WithStderr(io.MultiWriter(os.Stderr, stderr)))
	if err != nil {
		return nil, err
	}
	defer stderr.Flush()
	defer executor.Close()
	return withHeartbeat(ctx, func() (*deploymentpb.Docker_Output, error) {
		return executor.Down(ctx, req)
	})
}

func (a *deploymentActivities) TerraformPlanActivity(ctx context.Context, req *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return a.runTerraformActivity(ctx, req, deploymentpb.Terraform_ACTION_PLAN)
}

func (a *deploymentActivities) TerraformApplyActivity(ctx context.Context, req *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return a.runTerraformActivity(ctx, req, deploymentpb.Terraform_ACTION_APPLY)
}

func (a *deploymentActivities) TerraformDestroyActivity(ctx context.Context, req *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return a.runTerraformActivity(ctx, req, deploymentpb.Terraform_ACTION_DESTROY)
}

func (a *deploymentActivities) runTerraformActivity(ctx context.Context, req *deploymentpb.Terraform_Input, action deploymentpb.Terraform_Action) (*deploymentpb.Terraform_Output, error) {
	if req == nil {
		return nil, fmt.Errorf("terraform input is required")
	}
	clone := proto.Clone(req).(*deploymentpb.Terraform_Input)
	if clone.Operation == nil {
		clone.Operation = &deploymentpb.Terraform_Operation{}
	}
	clone.Operation.Action = action
	stdout, stderr := a.terraformLogWriters(ctx, clone.Operation.GetLogContext())
	executor := terraformexec.NewExecutor(
		terraformexec.WithStdout(io.MultiWriter(os.Stdout, stdout)),
		terraformexec.WithStderr(io.MultiWriter(os.Stderr, stderr)),
	)
	return withHeartbeat(ctx, func() (*deploymentpb.Terraform_Output, error) {
		out, err := executor.Execute(ctx, clone)
		stdout.Flush()
		stderr.Flush()
		return out, err
	})
}

func (a *deploymentActivities) terraformLogWriters(ctx context.Context, lc *deploymentpb.Terraform_Operation_LogContext) (*serverLogWriter, *serverLogWriter) {
	stdout := newServerLogWriter(ctx, a.logs, serverLogContextFromTerraform(lc), monitor.Stream_STREAM_STDOUT)
	stderr := newServerLogWriter(ctx, a.logs, serverLogContextFromTerraform(lc), monitor.Stream_STREAM_STDERR)
	return stdout, stderr
}

type serverLogContext struct {
	runID                 string
	nodeExecutionID       string
	parentNodeExecutionID string
	phase                 string
	stageName             string
	action                string
	unit                  string
	mentions              []string
}

type serverLogWriter struct {
	mu      sync.Mutex
	ctx     context.Context
	sink    RunLogWriter
	lc      serverLogContext
	stream  monitor.Stream
	buf     []byte
	enabled bool
}

func newServerLogWriter(ctx context.Context, sink RunLogWriter, lc serverLogContext, stream monitor.Stream) *serverLogWriter {
	return &serverLogWriter{ctx: ctx, sink: sink, lc: lc, stream: stream, enabled: sink != nil && lc.runID != ""}
}

func (w *serverLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(p) == 0 {
		return 0, nil
	}
	if !w.enabled {
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	lines := make([]string, 0)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		raw := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		lines = append(lines, strings.TrimSuffix(raw, "\r"))
	}
	w.shipLocked(lines)
	return len(p), nil
}

func (w *serverLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.enabled || len(w.buf) == 0 {
		return
	}
	line := strings.TrimSuffix(string(w.buf), "\r")
	w.buf = nil
	w.shipLocked([]string{line})
}

func (w *serverLogWriter) shipLocked(lines []string) {
	if !w.enabled || len(lines) == 0 {
		return
	}
	out := make([]*monitor.LogLine, 0, len(lines))
	now := timestamppb.Now()
	for _, line := range lines {
		if line == "" {
			continue
		}
		out = append(out, &monitor.LogLine{
			ObservedAt:            now,
			RunId:                 w.lc.runID,
			NodeExecutionId:       w.lc.nodeExecutionID,
			ParentNodeExecutionId: w.lc.parentNodeExecutionID,
			Phase:                 w.lc.phase,
			StageName:             w.lc.stageName,
			Action:                w.lc.action,
			Mentions:              append([]string(nil), w.lc.mentions...),
			Source:                monitor.Source_SOURCE_SERVER,
			Unit:                  w.lc.unit,
			Stream:                w.stream,
			Line:                  line,
		})
	}
	if len(out) == 0 {
		return
	}
	_ = w.sink.Write(w.ctx, out)
}

func serverLogContextFromTerraform(lc *deploymentpb.Terraform_Operation_LogContext) serverLogContext {
	if lc == nil {
		return serverLogContext{}
	}
	return serverLogContext{
		runID:                 lc.GetRunId(),
		nodeExecutionID:       lc.GetNodeExecutionId(),
		parentNodeExecutionID: lc.GetParentNodeExecutionId(),
		phase:                 lc.GetPhase(),
		stageName:             lc.GetStageName(),
		action:                lc.GetAction(),
		unit:                  lc.GetUnit(),
		mentions:              append([]string(nil), lc.GetMentions()...),
	}
}

func serverLogContextFromDocker(lc *deploymentpb.Docker_Input_LogContext) serverLogContext {
	if lc == nil {
		return serverLogContext{}
	}
	return serverLogContext{
		runID:                 lc.GetRunId(),
		nodeExecutionID:       lc.GetNodeExecutionId(),
		parentNodeExecutionID: lc.GetParentNodeExecutionId(),
		phase:                 lc.GetPhase(),
		stageName:             lc.GetStageName(),
		action:                lc.GetAction(),
		unit:                  lc.GetUnit(),
		mentions:              append([]string(nil), lc.GetMentions()...),
	}
}

// withHeartbeat records a Temporal activity heartbeat every 20s for the duration
// of fn. Long-running deployment activities (terraform apply/destroy, which can
// run for minutes) MUST heartbeat: with HeartbeatTimeout=1m and no heartbeats,
// Temporal declares the activity timed-out and retries it while the first
// terraform process is still running, colliding on the terraform state lock and
// failing the run (while leaking the half-applied infrastructure).
func withHeartbeat[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	beatCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-beatCtx.Done():
				return
			case <-ticker.C:
				activity.RecordHeartbeat(ctx)
			}
		}
	}()
	return fn()
}

func echoQuotaAllocations(refs []*workflowpb.QuotaRequestRef) []*workflowpb.QuotaAllocationRef {
	allocations := make([]*workflowpb.QuotaAllocationRef, 0, len(refs))
	for _, ref := range refs {
		request := ref.GetRequest()
		if request == nil {
			continue
		}
		allocations = append(allocations, &workflowpb.QuotaAllocationRef{
			NodeId: ref.GetNodeId(),
			Allocation: &deploymentpb.Quota_Allocation{
				Info: request.GetInfo(),
				Used: request.GetRequest(),
			},
		})
	}
	return allocations
}
