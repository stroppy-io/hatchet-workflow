package victoria

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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
