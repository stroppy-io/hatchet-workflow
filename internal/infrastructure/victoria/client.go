// Package victoria is the HTTP client for VictoriaMetrics (PromQL) used to build
// run metric summaries. Recast of internal/old/infrastructure/victoria. Grafana
// dashboards talk to VictoriaMetrics directly (H56); this client backs the
// additive MetricsService summaries only.
package victoria

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client queries a VictoriaMetrics (or Prometheus-compatible) endpoint.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient builds a Client. token (optional) is sent as a Bearer header.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryResult is the /api/v1/query[_range] response envelope.
type QueryResult struct {
	Status string     `json:"status"`
	Data   ResultData `json:"data"`
}

// ResultData holds the result series.
type ResultData struct {
	ResultType string   `json:"resultType"`
	Result     []Series `json:"result"`
}

// Series is one metric series.
type Series struct {
	Metric map[string]string `json:"metric"`
	Values []SamplePair      `json:"values,omitempty"` // query_range
	Value  []any             `json:"value,omitempty"`  // instant query
}

// SamplePair is a [timestamp, "value"] pair.
type SamplePair [2]any

// Float parses the sample's string value.
func (s SamplePair) Float() (float64, bool) {
	if len(s) != 2 {
		return 0, false
	}
	str, ok := s[1].(string)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(str, 64)
	return v, err == nil
}

// QueryInstant runs a PromQL instant query.
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
	params := url.Values{"query": {query}}
	if !ts.IsZero() {
		params.Set("time", strconv.FormatInt(ts.Unix(), 10))
	}
	return c.doQuery(ctx, "/api/v1/query", params)
}

// QueryRange runs a PromQL range query.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	params := url.Values{
		"query": {query},
		"start": {strconv.FormatInt(start.Unix(), 10)},
		"end":   {strconv.FormatInt(end.Unix(), 10)},
		"step":  {strconv.FormatInt(int64(step.Seconds()), 10)},
	}
	return c.doQuery(ctx, "/api/v1/query_range", params)
}

// InstantValue runs an instant query and returns the first series' scalar value.
func (c *Client) InstantValue(ctx context.Context, query string) (float64, bool, error) {
	res, err := c.QueryInstant(ctx, query, time.Time{})
	if err != nil {
		return 0, false, err
	}
	for _, s := range res.Data.Result {
		if len(s.Value) == 2 {
			if str, ok := s.Value[1].(string); ok {
				if v, perr := strconv.ParseFloat(str, 64); perr == nil {
					return v, true, nil
				}
			}
		}
	}
	return 0, false, nil
}

func (c *Client) doQuery(ctx context.Context, path string, params url.Values) (*QueryResult, error) {
	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("victoria: build request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("victoria: query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("victoria: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("victoria: status %d: %s", resp.StatusCode, body)
	}
	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("victoria: decode: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("victoria: query failed: %s", body)
	}
	return &result, nil
}
