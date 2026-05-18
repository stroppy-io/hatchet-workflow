package main

import (
	"context"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	systemsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
)

// systemLogAdapter bridges agentsvc.LogIngester to systemsvc.Service.IngestLogs.
// Kept in cmd because it is the wiring layer that knows both packages.
type systemLogAdapter struct {
	sys *systemsvc.Service
}

func (a systemLogAdapter) IngestLogs(ctx context.Context, lines []agentsvc.AgentLogLine) error {
	if len(lines) == 0 {
		return nil
	}
	out := make([]systemsvc.AgentLogLine, len(lines))
	for i, l := range lines {
		out[i] = systemsvc.AgentLogLine{
			DagRunID:  l.DagRunID,
			NodeRunID: l.NodeRunID,
			CommandID: l.CommandID,
			Timestamp: l.Timestamp,
			Stream:    l.Stream,
			Line:      l.Line,
		}
	}
	return a.sys.IngestLogs(ctx, out)
}
