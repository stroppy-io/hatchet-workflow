package victoria

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

// LogsClient queries VictoriaLogs via the LogsQL HTTP API.
type LogsClient struct {
	baseURL    string
	token      string // bearer token for vmauth (empty = no auth)
	httpClient *http.Client
}

// NewLogsClient creates a VictoriaLogs query client. baseURL is the VictoriaLogs
// root that exposes /select/logsql/query (e.g. http://victoria-logs:9428 or
// <base>/select/0 for a multi-tenant gateway). token may be empty to disable
// bearer auth.
func NewLogsClient(baseURL, token string) *LogsClient {
	return &LogsClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// LogsQuery is a LogsQL query plus its optional time window and row limit.
type LogsQuery struct {
	Query string    // LogsQL expression
	Start time.Time // inclusive lower bound (zero = unbounded)
	End   time.Time // inclusive upper bound (zero = unbounded)
	Limit uint32    // max rows (0 = backend default)
}

// Query executes a LogsQL query against /select/logsql/query and returns the raw
// JSON-lines response body. The caller is responsible for closing the returned
// ReadCloser and decoding each JSON object line.
func (c *LogsClient) Query(ctx context.Context, q LogsQuery) (io.ReadCloser, error) {
	values := url.Values{}
	values.Set("query", q.Query)
	if q.Limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", q.Limit))
	}
	if !q.Start.IsZero() {
		values.Set("start", q.Start.UTC().Format(time.RFC3339Nano))
	}
	if !q.End.IsZero() {
		values.Set("end", q.End.UTC().Format(time.RFC3339Nano))
	}

	endpoint := c.baseURL + "/select/logsql/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("vlogs: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vlogs: query: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("vlogs: status %d: %s", resp.StatusCode, body)
	}
	return resp.Body, nil
}

// Write appends log lines to VictoriaLogs via /insert/jsonline. The caller is
// responsible for setting run_id / node_execution_id / component_id on the
// lines; this method only maps the proto shape to the JSONL wire fields used by
// the read path.
func (c *LogsClient) Write(ctx context.Context, lines []*monitor.LogLine) error {
	if len(lines) == 0 {
		return nil
	}
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, line := range lines {
		if line == nil || line.GetRunId() == "" || line.GetLine() == "" {
			continue
		}
		row := map[string]any{
			"_msg":                     line.GetLine(),
			"run_id":                   line.GetRunId(),
			"node_execution_id":        line.GetNodeExecutionId(),
			"parent_node_execution_id": line.GetParentNodeExecutionId(),
			"phase":                    line.GetPhase(),
			"stage_name":               line.GetStageName(),
			"component_id":             line.GetComponentId(),
			"machine_id":               line.GetMachineId(),
			"step_id":                  line.GetStepId(),
			"action":                   line.GetAction(),
			"mentions":                 strings.Join(line.GetMentions(), ","),
			"unit":                     line.GetUnit(),
			"source":                   sourceName(line.GetSource()),
			"stream":                   streamName(line.GetStream()),
		}
		if line.GetObservedAt() != nil {
			row["_time"] = line.GetObservedAt().AsTime().UTC().Format(time.RFC3339Nano)
		}
		if line.GetLineNo() > 0 {
			row["line_no"] = line.GetLineNo()
		}
		if err := enc.Encode(row); err != nil {
			return fmt.Errorf("vlogs: encode line: %w", err)
		}
	}
	if body.Len() == 0 {
		return nil
	}

	endpoint := c.baseURL + "/insert/jsonline"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return fmt.Errorf("vlogs: build write request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("AccountID", "0")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vlogs: write: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("vlogs: write status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func sourceName(source monitor.Source) string {
	switch source {
	case monitor.Source_SOURCE_COMMAND:
		return "command"
	case monitor.Source_SOURCE_JOURNALD:
		return "journald"
	case monitor.Source_SOURCE_FILE:
		return "file"
	case monitor.Source_SOURCE_SERVER:
		return "server"
	default:
		return ""
	}
}

func streamName(stream monitor.Stream) string {
	switch stream {
	case monitor.Stream_STREAM_STDOUT:
		return "stdout"
	case monitor.Stream_STREAM_STDERR:
		return "stderr"
	default:
		return ""
	}
}
