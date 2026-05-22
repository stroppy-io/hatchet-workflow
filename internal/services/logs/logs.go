// Package logs implements run.LogsClient (query/stream/deep-link run logs from
// VictoriaLogs) and agent.LogsSink (ingest agent-shipped log lines). Logs are
// labeled by dag_id/node_execution_id/component_id/machine_id; RunService resolves
// run_id -> dag_id and passes the dag id here (H54).
package logs

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
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

// QueryRunLogs returns a page of the run's logs (filtered by dag + optional
// stage/component), ascending by _time, with opaque next_token cursor paging.
//
// VictoriaLogs has no native row cursor, so we page by a (_time, offset) bound:
// the query sorts ascending by _time and is lower-bounded at the cursor time;
// `offset` is the count of rows that share that exact cursor time which a prior
// page already consumed (the stable tiebreak for same-nanosecond lines). A
// next_token is emitted only when a full page (page_size) was returned.
func (c *Client) QueryRunLogs(ctx context.Context, dagID string, req *uipb.QueryRunLogsRequest) (*logspb.LogPage, error) {
	size := int(req.GetPageSize())
	if size <= 0 || size > defaultLimit {
		size = defaultLimit
	}
	cur, err := decodeCursor(req.GetPageToken())
	if err != nil {
		return nil, err
	}
	// Time window: request [start,end] intersected with the cursor lower bound.
	lower := startBound(req.GetStart(), cur)
	upper := endBound(req.GetEnd())
	// Over-fetch the page plus the rows already consumed at the cursor time
	// (offset) plus one extra to detect whether more pages follow.
	fetch := size + cur.offset + 1
	q := logsQL(dagID, req.GetNodeExecutionId(), req.GetComponentId(), windowFilter(lower, upper)) + " | sort by (_time) limit " + strconv.Itoa(fetch)
	records, err := c.vl.Query(ctx, q, fetch)
	if err != nil {
		return nil, err
	}
	// Drop the rows at the cursor's exact time that the previous page consumed.
	if cur.offset > 0 && cur.offset <= len(records) {
		records = records[cur.offset:]
	} else if cur.offset > 0 {
		records = nil
	}
	page := &logspb.LogPage{Lines: make([]*logspb.LogLine, 0, size)}
	full := len(records) > size
	if full {
		records = records[:size]
	}
	for _, r := range records {
		page.Lines = append(page.Lines, recordToLine(r))
	}
	if full && len(page.Lines) > 0 {
		last := page.Lines[len(page.Lines)-1].GetObservedAt().AsTime()
		page.NextToken = encodeCursor(logCursor{at: last, offset: sameTimeTail(page.Lines, last)})
	}
	return page, nil
}

// logCursor is the opaque paging position: the last seen _time and the number
// of trailing rows that share it (consumed so the next page resumes after them).
type logCursor struct {
	at     time.Time
	offset int
}

// startBound returns the effective lower _time bound: the later of the request
// start and the cursor time.
func startBound(start *timestamppb.Timestamp, cur logCursor) time.Time {
	lower := cur.at
	if start != nil {
		if s := start.AsTime(); cur.at.IsZero() || s.After(cur.at) {
			lower = s
		}
	}
	return lower
}

func endBound(end *timestamppb.Timestamp) time.Time {
	if end == nil {
		return time.Time{}
	}
	return end.AsTime()
}

// windowFilter builds the LogsQL _time bound: lower-inclusive (>= so same-time
// rows are kept and de-duped via the cursor offset) and upper-inclusive.
func windowFilter(lower, upper time.Time) string {
	var parts []string
	if !lower.IsZero() {
		parts = append(parts, fmt.Sprintf("_time:>=%q", lower.UTC().Format(time.RFC3339Nano)))
	}
	if !upper.IsZero() {
		parts = append(parts, fmt.Sprintf("_time:<=%q", upper.UTC().Format(time.RFC3339Nano)))
	}
	return strings.Join(parts, " ")
}

// sameTimeTail counts how many trailing lines share the given _time (the offset
// to skip on the next page so same-nanosecond rows aren't re-emitted).
func sameTimeTail(lines []*logspb.LogLine, at time.Time) int {
	n := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if !lines[i].GetObservedAt().AsTime().Equal(at) {
			break
		}
		n++
	}
	return n
}

func encodeCursor(c logCursor) string {
	if c.at.IsZero() {
		return ""
	}
	raw := fmt.Sprintf("%s|%d", c.at.UTC().Format(time.RFC3339Nano), c.offset)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(token string) (logCursor, error) {
	if token == "" {
		return logCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return logCursor{}, fmt.Errorf("logs: invalid page token: %w", err)
	}
	at, off, ok := strings.Cut(string(raw), "|")
	if !ok {
		return logCursor{}, fmt.Errorf("logs: malformed page token")
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return logCursor{}, fmt.Errorf("logs: invalid page token time: %w", err)
	}
	n, err := strconv.Atoi(off)
	if err != nil || n < 0 {
		return logCursor{}, fmt.Errorf("logs: invalid page token offset")
	}
	return logCursor{at: t, offset: n}, nil
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
			"_msg":               l.GetLine(),
			"_time":              l.GetObservedAt().AsTime().UTC().Format(time.RFC3339Nano),
			"tenant_id":          tenantID,
			"dag_id":             l.GetDagId(),
			"node_execution_id":  l.GetNodeExecutionId(),
			"machine_id":         l.GetTarget().GetMachineId(),
			"agent_component_id": l.GetTarget().GetAgentComponentId(),
			"component_id":       l.GetTarget().GetTargetComponentId(),
			"command_id":         l.GetCommandId(),
			"stream":             l.GetStream().String(),
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
