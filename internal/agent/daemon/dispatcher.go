package daemon

import (
	"context"
	"sync"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// VerbHandler executes a single action and returns a Report.
// The CommandId field is left empty — Dispatcher fills it in.
type VerbHandler func(ctx context.Context, action *agentpb.Action) *agentpb.Report

// Dispatcher routes incoming Commands to registered VerbHandlers,
// accumulates completed Reports and LogLines, and drains them for Poll.
type Dispatcher struct {
	mu       sync.Mutex
	reports  []*agentpb.Report
	loglines []*agentpb.LogLine

	verbs map[string]VerbHandler
}

// NewDispatcher returns an empty Dispatcher with no verbs registered.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		verbs: make(map[string]VerbHandler),
	}
}

// Register associates a verb kind name with a handler.
// Kind names must match the oneof field accessor names, e.g. "RunShell".
func (d *Dispatcher) Register(kind string, h VerbHandler) {
	d.mu.Lock()
	d.verbs[kind] = h
	d.mu.Unlock()
}

// Dispatch selects the handler for cmd.Action.Verb, runs it synchronously,
// stamps the Report with command/machine ids, and queues it.
func (d *Dispatcher) Dispatch(ctx context.Context, cmd *agentpb.Command) *agentpb.Report {
	action := cmd.GetAction()
	kind := verbKind(action)

	d.mu.Lock()
	handler, ok := d.verbs[kind]
	d.mu.Unlock()

	var rep *agentpb.Report
	if !ok {
		rep = &agentpb.Report{
			Status:     agentpb.ReportStatus_REPORT_STATUS_FAILED,
			Error:      "unknown verb: " + kind,
			FinishedAt: timestamppb.Now(),
		}
	} else {
		rep = handler(ctx, action)
	}

	rep.CommandId = cmd.GetId()
	rep.MachineId = cmd.GetMachineId()

	d.QueueReport(rep)
	return rep
}

// QueueReport appends a Report to the internal buffer.
func (d *Dispatcher) QueueReport(r *agentpb.Report) {
	d.mu.Lock()
	d.reports = append(d.reports, r)
	d.mu.Unlock()
}

// DrainReports atomically returns all buffered reports/loglines and clears them.
func (d *Dispatcher) DrainReports() *agentpb.AgentReport {
	d.mu.Lock()
	reports := d.reports
	loglines := d.loglines
	d.reports = nil
	d.loglines = nil
	d.mu.Unlock()
	return &agentpb.AgentReport{
		Reports:  reports,
		LogLines: loglines,
	}
}

// verbKind returns the string name of the oneof verb variant.
func verbKind(a *agentpb.Action) string {
	if a == nil {
		return ""
	}
	switch a.GetVerb().(type) {
	case *agentpb.Action_PutFile:
		return "PutFile"
	case *agentpb.Action_RunShell:
		return "RunShell"
	case *agentpb.Action_Systemctl:
		return "Systemctl"
	case *agentpb.Action_InstallPackage:
		return "InstallPackage"
	case *agentpb.Action_WaitFor:
		return "WaitFor"
	default:
		return ""
	}
}
