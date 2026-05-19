package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// PackageInstallHandler turns a PackageInstallTask into a sequence of
// agent verbs: pre-install RunShell (per command), then either apt-get
// install (AptSource) or dpkg -i a fetched .deb (DebBlobSource).
// Dispatches to every machine listed in TargetMachineIds.
type PackageInstallHandler struct {
	hub     HubPort
	timeout time.Duration
}

func NewPackageInstallHandler(hub HubPort, timeout time.Duration) *PackageInstallHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &PackageInstallHandler{hub: hub, timeout: timeout}
}

func (h *PackageInstallHandler) Kind() string { return "PackageInstallTask" }

func (h *PackageInstallHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.PackageInstallTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("PackageInstallHandler: unmarshal: %w", err)
	}
	targets := task.GetTargetMachineIds()
	if len(targets) == 0 {
		return nil, fmt.Errorf("PackageInstallHandler: target_machine_ids must be non-empty (legacy role-only fan-out no longer supported)")
	}
	dagRunID := node.GetDagRunId().GetValue()
	for _, machineID := range targets {
		if err := h.installOn(ctx, dagRunID, machineID, &task); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// installOn resolves a single machine_id → agent_id binding and executes
// the install sequence for that host.
func (h *PackageInstallHandler) installOn(ctx context.Context, dagRunID, machineID string, task *taskspb.PackageInstallTask) error {
	agentID, ok := h.hub.ResolveByMachine(dagRunID, machineID)
	if !ok {
		return fmt.Errorf("PackageInstallHandler: no agent registered for machine_id=%s (dag_run=%s)", machineID, dagRunID)
	}

	for _, cmd := range task.GetPreInstall() {
		if strings.TrimSpace(cmd) == "" {
			continue
		}
		if err := h.dispatchShell(ctx, agentID, machineID, cmd); err != nil {
			return fmt.Errorf("PackageInstallHandler: pre_install %q: %w", cmd, err)
		}
	}

	if len(task.GetAptPackages()) > 0 {
		argv := append([]string{"apt-get", "install", "-y"}, task.GetAptPackages()...)
		if err := h.dispatchArgv(ctx, agentID, machineID, argv); err != nil {
			return fmt.Errorf("PackageInstallHandler: apt install: %w", err)
		}
		return nil
	}

	if url := task.GetDebBlobUrl(); url != "" {
		sum := task.GetDebBlobSha256()
		if sum == "" {
			return fmt.Errorf("PackageInstallHandler: deb_blob_url set without deb_blob_sha256 (upload the .deb via /packages/{id}/deb so the server can compute the integrity hash)")
		}
		putAction := &agentpb.Action{
			Verb: &agentpb.Action_PutFile{
				PutFile: &agentpb.PutFile{
					Path: "/tmp/pkg.deb",
					Content: &agentpb.PutFile_Fetch{
						Fetch: &agentpb.FetchSpec{Url: url, Sha256: sum},
					},
					Mode:          0o644,
					CreateParents: true,
				},
			},
		}
		if err := h.dispatch(ctx, agentID, machineID, putAction); err != nil {
			return fmt.Errorf("PackageInstallHandler: fetch deb: %w", err)
		}
		dpkgAction := &agentpb.Action{
			Verb: &agentpb.Action_RunShell{
				RunShell: &agentpb.RunShell{
					Argv:  []string{"dpkg", "-i", "/tmp/pkg.deb"},
					Shell: false,
				},
			},
		}
		if err := h.dispatch(ctx, agentID, machineID, dpkgAction); err != nil {
			return fmt.Errorf("PackageInstallHandler: dpkg -i: %w", err)
		}
		return nil
	}

	// Empty package payload — treat as no-op for this machine.
	return nil
}

func (h *PackageInstallHandler) dispatchShell(ctx context.Context, agentID, machineID, cmd string) error {
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: []string{"sh", "-c", cmd}, Shell: true},
		},
	}
	return h.dispatch(ctx, agentID, machineID, action)
}

func (h *PackageInstallHandler) dispatchArgv(ctx context.Context, agentID, machineID string, argv []string) error {
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: argv, Shell: false},
		},
	}
	return h.dispatch(ctx, agentID, machineID, action)
}

func (h *PackageInstallHandler) dispatch(ctx context.Context, agentID, machineID string, action *agentpb.Action) error {
	report, err := h.hub.Dispatch(ctx, agentID, machineID, action, h.timeout)
	if err != nil {
		return fmt.Errorf("dispatch: %w", err)
	}
	if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		return fmt.Errorf("agent reported failure: %s", report.GetError())
	}
	return nil
}

// Compile-time guard: role parameter on PackageInstallTask must remain
// present for legacy compatibility / observability — we don't currently
// branch on it.
var _ = catalogpb.MachineRole_MACHINE_ROLE_DATABASE
