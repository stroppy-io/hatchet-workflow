package victoria

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// LogsClient is the HTTP client for VictoriaLogs (ingest + LogsQL query). The
// agent ingests command logs with full labels (dag_id/node_execution_id/
// component_id/machine_id); the control plane queries them back (H54).
type LogsClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewLogsClient builds a VictoriaLogs client.
func NewLogsClient(baseURL, token string) *LogsClient {
	return &LogsClient{baseURL: baseURL, token: token, httpClient: &http.Client{Timeout: 30 * time.Second}}
}

// Ingest writes log records (one JSON object each) via the jsonline endpoint.
// Each record should carry `_msg`, `_time`, and label fields. accountID 0 = default.
func (c *LogsClient) Ingest(ctx context.Context, accountID int, records []map[string]any) error {
	if len(records) == 0 {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range records {
		if err := enc.Encode(r); err != nil { // Encode appends a newline (jsonline)
			return fmt.Errorf("vlogs: marshal: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/insert/jsonline", &buf)
	if err != nil {
		return fmt.Errorf("vlogs: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/stream+json")
	if accountID > 0 {
		req.Header.Set("AccountID", strconv.Itoa(accountID))
	}
	c.auth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vlogs: post: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("vlogs: ingest status %d", resp.StatusCode)
	}
	return nil
}

// Query runs a LogsQL query and returns up to limit records (each a field map).
func (c *LogsClient) Query(ctx context.Context, logsql string, limit int) ([]map[string]any, error) {
	params := url.Values{"query": {logsql}}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/select/logsql/query",
		bytes.NewBufferString(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("vlogs: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.auth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vlogs: query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vlogs: query status %d", resp.StatusCode)
	}
	var out []map[string]any
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, sc.Err()
}

// BaseURL returns the configured VictoriaLogs base URL (for deep links).
func (c *LogsClient) BaseURL() string { return c.baseURL }

func (c *LogsClient) auth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
