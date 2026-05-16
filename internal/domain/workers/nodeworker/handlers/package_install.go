package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// PackageInstallHandler dispatches an InstallPackage action to the agent via Hub.
// agentID and machineID are resolved at construction time by the caller.
type PackageInstallHandler struct {
	hub       HubPort
	agentID   string
	machineID string
	timeout   time.Duration
}

func NewPackageInstallHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *PackageInstallHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &PackageInstallHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *PackageInstallHandler) Kind() string { return "PackageInstallTask" }

func (h *PackageInstallHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.PackageInstallTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("PackageInstallHandler: unmarshal: %w", err)
	}
	action := &agentpb.Action{
		Verb: &agentpb.Action_InstallPackage{
			InstallPackage: &agentpb.InstallPackage{
				// Package resolved by catalog lookup in full implementation.
				// Skeleton: no package payload — handler requires real agent wiring.
			},
		},
	}
	// task.GetPackageId() and task.GetRole() are used in the full implementation
	// to resolve the package from catalog and dispatch to the correct machine role.
	report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, action, h.timeout)
	if err != nil {
		return nil, fmt.Errorf("PackageInstallHandler: dispatch: %w", err)
	}
	if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		return nil, fmt.Errorf("PackageInstallHandler: agent reported failure: %s", report.GetError())
	}
	return nil, nil
}
