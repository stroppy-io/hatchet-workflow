// Package logs implements run.LogsClient (query/stream/deep-link run logs from
// VictoriaLogs) and agent.LogsSink (ingest agent-shipped log lines). Logs are
// labeled by dag_id/node_execution_id/component_id/machine_id; RunService resolves
// run_id -> dag_id and passes the dag id here (H54).
package logs

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gopherex/xlog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	logspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/logs"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

const defaultLimit = 1000

// Client serves and ingests run logs over VictoriaLogs.
type Client struct {
	*tracing.Entity
	vl *victoria.LogsClient
}

// New builds a logs Client.
func New(logger *xlog.Logger, vl *victoria.LogsClient) *Client {
	return &Client{Entity: tracing.NewEntity(logger.AppendName("LogsClient")), vl: vl}
}

// QueryRunLogs returns a page of the run's logs (filtered by dag + optional stage/component).
//
// TODO(logs): pagination uses LogsQL limit only; next_token (LogCursor paging)
// not yet emitted.
func (c *Client) QueryRunLogs(ctx context.Context, dagID string, req *uipb.QueryRunLogsRequest) (*logspb.LogPage, error) {
	records, err := c.vl.Query(ctx, logsQL(dagID, req.GetNodeExecutionId(), req.GetComponentId(), ""), defaultLimit)
	if err != nil {
		return nil, err
	}
	page := &logspb.LogPage{Lines: make([]*logspb.LogLine, 0, len(records))}
	for _, r := range records {
		page.Lines = append(page.Lines, recordToLine(r))
	}
	return page, nil
}

// StreamRunLogs tails the run's logs (dag-addressed) until the stream context is
// cancelled. RunService resolves run_id -> dag_id.
func (c *Client) StreamRunLogs(ctx context.Context, dagID, nodeExecutionID, componentID string, stream grpc.ServerStreamingServer[logspb.LogLine]) error {
	return c.tail(ctx, dagID, nodeExecutionID, componentID, stream)
}

func (c *Client) tail(ctx context.Context, dagID, node, component string, stream grpc.ServerStreamingServer[logspb.LogLine]) error {
	var since time.Time
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		records, err := c.vl.Query(ctx, logsQL(dagID, node, component, sinceFilter(since)), defaultLimit)
		if err != nil {
			return err
		}
		for _, r := range records {
			line := recordToLine(r)
			if t := line.GetObservedAt().AsTime(); t.After(since) {
				since = t
			}
			if err := stream.Send(line); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// BuildLogLink returns a VictoriaLogs UI deep link for the run's logs.
func (c *Client) BuildLogLink(_ context.Context, dagID string, req *uipb.BuildLogLinkRequest) (string, error) {
	q := logsQL(dagID, req.GetNodeExecutionId(), req.GetComponentId(), "")
	return fmt.Sprintf("%s/select/vmui/?#/?query=%s", c.vl.BaseURL(), url.QueryEscape(q)), nil
}

// SendLogs ingests agent-shipped log lines (agent.LogsSink). The agent also
// ingests directly with full dag labels; this RPC path maps the available fields.
func (c *Client) SendLogs(ctx context.Context, tenantID string, lines []*rtagent.LogLine) error {
	records := make([]map[string]any, 0, len(lines))
	for _, l := range lines {
		records = append(records, map[string]any{
			"_msg":         l.GetLine(),
			"_time":        l.GetObservedAt().AsTime().UTC().Format(time.RFC3339Nano),
			"tenant_id":    tenantID,
			"machine_id":   l.GetTarget().GetMachineId(),
			"component_id": l.GetTarget().GetTargetComponentId(),
			"command_id":   l.GetCommandId(),
			"stream":       l.GetStream().String(),
		})
	}
	return c.vl.Ingest(ctx, 0, records)
}

func logsQL(dagID, node, component, extra string) string {
	var parts []string
	if dagID != "" {
		parts = append(parts, fmt.Sprintf("dag_id:=%q", dagID))
	}
	if node != "" {
		parts = append(parts, fmt.Sprintf("node_execution_id:=%q", node))
	}
	if component != "" {
		parts = append(parts, fmt.Sprintf("component_id:=%q", component))
	}
	if extra != "" {
		parts = append(parts, extra)
	}
	if len(parts) == 0 {
		return "*"
	}
	return strings.Join(parts, " ")
}

func sinceFilter(since time.Time) string {
	if since.IsZero() {
		return ""
	}
	return fmt.Sprintf("_time:>%q", since.UTC().Format(time.RFC3339Nano))
}

func recordToLine(r map[string]any) *logspb.LogLine {
	line := &logspb.LogLine{
		DagId:           str(r, "dag_id"),
		NodeExecutionId: str(r, "node_execution_id"),
		ComponentId:     str(r, "component_id"),
		MachineId:       str(r, "machine_id"),
		Unit:            str(r, "unit"),
		Line:            str(r, "_msg"),
	}
	if t, err := time.Parse(time.RFC3339Nano, str(r, "_time")); err == nil {
		ts := timestamppb.New(t)
		line.ObservedAt = ts
		line.Cursor = &logspb.LogCursor{ObservedAt: ts}
	}
	return line
}

func str(r map[string]any, key string) string {
	if v, ok := r[key].(string); ok {
		return v
	}
	return ""
}
