// Package victoria reads VictoriaLogs and VictoriaMetrics over their HTTP
// APIs, scoping every query to one run by the label the pipelines stamp
// on telemetry.
//
// doc: docs.victoriametrics.com/victorialogs/querying — /select/logsql/query
// (NDJSON), /select/logsql/facets; docs.victoriametrics.com/keyconcepts —
// /api/v1/query_range (Prometheus matrix).
package victoria

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// Config is the stores' location.
type Config struct {
	// LogsURL is VictoriaLogs (vlselect); empty = logs unavailable.
	LogsURL string `mapstructure:"logs_url" validate:"omitempty,url"`
	// MetricsURL is VictoriaMetrics (vmselect); empty = metrics unavailable.
	MetricsURL string `mapstructure:"metrics_url" validate:"omitempty,url"`
	// RunLabel is the label/field carrying the run id on every record.
	RunLabel string `default:"stroppy_run_id" mapstructure:"run_label"`
	// Timeout of one store request.
	Timeout time.Duration `default:"30s" mapstructure:"timeout"`
}

// Logs is the VictoriaLogs client.
type Logs struct {
	base  string
	label string
	hc    *http.Client
}

// NewLogs builds the client (nil when not configured).
func NewLogs(cfg *Config) *Logs {
	if cfg.LogsURL == "" {
		return nil
	}
	return &Logs{base: strings.TrimRight(cfg.LogsURL, "/"), label: cfg.RunLabel, hc: &http.Client{Timeout: cfg.Timeout}}
}

// logsQL builds the LogsQL of a typed query: the run scope, then every
// filter as an `in(...)` on its field, then the free text.
func (l *Logs) logsQL(q observe.LogQuery) string {
	parts := []string{fmt.Sprintf(`%s:=%q`, l.label, q.RunID.String())}
	in := func(field string, values []string) {
		if len(values) == 0 {
			return
		}
		quoted := make([]string, 0, len(values))
		for _, v := range values {
			quoted = append(quoted, strconv.Quote(v))
		}
		parts = append(parts, fmt.Sprintf("%s:in(%s)", field, strings.Join(quoted, ",")))
	}
	in("role", q.Roles)
	in("machine", q.Machines)
	in("container", q.Containers)
	in("stream", q.Streams)
	in("phase", q.Phases)
	in("level", q.Levels)
	if q.Segment != "" {
		parts = append(parts, fmt.Sprintf("segment:=%q", q.Segment))
	}
	if strings.TrimSpace(q.Text) != "" {
		parts = append(parts, fmt.Sprintf("_msg:%s", strconv.Quote(q.Text)))
	}
	return strings.Join(parts, " AND ")
}

// Query implements observe.Logs: the cursor is an RFC3339Nano timestamp;
// older pages end before it, newer pages start after it.
func (l *Logs) Query(ctx context.Context, q observe.LogQuery) (observe.LogPage, error) {
	start, end := q.Start, q.End
	if q.Cursor != "" {
		t, err := time.Parse(time.RFC3339Nano, q.Cursor)
		if err != nil {
			return observe.LogPage{}, errs.Invalid("cursor is not a timestamp")
		}
		if q.Direction == "newer" {
			start = t.Add(time.Nanosecond)
		} else {
			end = t
		}
	}
	lines, err := l.query(ctx, l.logsQL(q), start, end, q.Limit+1)
	if err != nil {
		return observe.LogPage{}, err
	}
	return pageOf(lines, q.Limit, q.Direction == "newer"), nil
}

// Raw runs a LogsQL fragment AND-ed with the run scope.
func (l *Logs) Raw(ctx context.Context, runID uuid.UUID, query string, start, end time.Time, limit int) (observe.LogPage, error) {
	scoped := fmt.Sprintf(`%s:=%q AND (%s)`, l.label, runID.String(), query)
	lines, err := l.query(ctx, scoped, start, end, limit+1)
	if err != nil {
		return observe.LogPage{}, err
	}
	return pageOf(lines, limit, false), nil
}

// pageOf orders newest-first and sets the cursors.
func pageOf(lines []observe.LogLine, limit int, newer bool) observe.LogPage {
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Time.After(lines[j].Time) })
	page := observe.LogPage{Lines: lines}
	if len(lines) > limit {
		page.Truncated = true
		if newer {
			page.Lines = lines[len(lines)-limit:]
		} else {
			page.Lines = lines[:limit]
		}
	}
	if n := len(page.Lines); n > 0 {
		page.Newer = page.Lines[0].Time.Format(time.RFC3339Nano)
		page.Older = page.Lines[n-1].Time.Format(time.RFC3339Nano)
	}
	return page
}

func (l *Logs) query(ctx context.Context, logsql string, start, end time.Time, limit int) ([]observe.LogLine, error) {
	form := url.Values{"query": {logsql}, "limit": {strconv.Itoa(limit)}, "start": {start.UTC().Format(time.RFC3339Nano)}, "end": {end.UTC().Format(time.RFC3339Nano)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.base+"/select/logsql/query", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := l.hc.Do(req)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, "logs store", err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096)) //nolint:errcheck // diagnostic tail only
		return nil, errs.Newf(errs.CodeUnavailable, "logs store: %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var out []observe.LogLine
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var rec map[string]string
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		out = append(out, lineOf(rec))
	}
	return out, sc.Err()
}

// lineOf maps a VictoriaLogs record: the well-known fields become the
// typed columns, the rest lands in Fields.
func lineOf(rec map[string]string) observe.LogLine {
	line := observe.LogLine{Message: rec["_msg"], Fields: map[string]string{}}
	if t, err := time.Parse(time.RFC3339Nano, rec["_time"]); err == nil {
		line.Time = t
	}
	pick := func(dst *string, keys ...string) {
		for _, k := range keys {
			if v, ok := rec[k]; ok && v != "" {
				*dst = v
				delete(rec, k)
				return
			}
		}
	}
	pick(&line.Level, "level", "severity")
	pick(&line.Stream, "stream")
	pick(&line.Role, "role", "stroppy_role")
	pick(&line.Machine, "machine", "stroppy_machine")
	pick(&line.Container, "container", "container_name")
	pick(&line.Phase, "phase")
	pick(&line.Segment, "segment")
	if seq, err := strconv.Atoi(rec["seq"]); err == nil {
		line.Seq = seq
		delete(rec, "seq")
	}
	for k, v := range rec {
		if k == "_msg" || k == "_time" || k == "_stream" || k == "_stream_id" {
			continue
		}
		line.Fields[k] = v
	}
	return line
}

// Facets implements observe.Logs over /select/logsql/facets.
func (l *Logs) Facets(ctx context.Context, runID uuid.UUID, start, end time.Time) ([]run.Facet, error) {
	form := url.Values{"query": {fmt.Sprintf(`%s:=%q`, l.label, runID.String())}, "start": {start.UTC().Format(time.RFC3339Nano)}, "end": {end.UTC().Format(time.RFC3339Nano)}, "limit": {"50"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.base+"/select/logsql/facets", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := l.hc.Do(req)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, "logs store", err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, errs.Newf(errs.CodeUnavailable, "logs store: %d", res.StatusCode)
	}
	var body struct {
		Facets []struct {
			Field  string `json:"field_name"`
			Values []struct {
				Value string `json:"field_value"`
				Hits  int    `json:"hits"`
			} `json:"values"`
		} `json:"facets"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, "logs store: facets", err)
	}
	wanted := map[string]bool{"role": true, "machine": true, "container": true, "stream": true, "phase": true, "level": true, "segment": true}
	out := []run.Facet{}
	for _, f := range body.Facets {
		if !wanted[f.Field] {
			continue
		}
		facet := run.Facet{Field: f.Field, Values: []run.FacetValue{}}
		for _, v := range f.Values {
			facet.Values = append(facet.Values, run.FacetValue{Value: v.Value, Count: v.Hits})
		}
		out = append(out, facet)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out, nil
}

// Metrics is the VictoriaMetrics client.
type Metrics struct {
	base  string
	label string
	hc    *http.Client
}

// NewMetrics builds the client (nil when not configured).
func NewMetrics(cfg *Config) *Metrics {
	if cfg.MetricsURL == "" {
		return nil
	}
	return &Metrics{base: strings.TrimRight(cfg.MetricsURL, "/"), label: cfg.RunLabel, hc: &http.Client{Timeout: cfg.Timeout}}
}

// Query implements observe.Metrics: one range query per catalog key;
// per-key failures are reported, not fatal.
func (m *Metrics) Query(ctx context.Context, q observe.MetricQuery) ([]observe.Series, []error, error) {
	var out []observe.Series
	var failures []error
	for _, def := range q.Metrics {
		raw, err := m.rangeQuery(ctx, observe.KeyExpr(def, m.label, q.RunID), q.Start, q.End, q.Step)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", def.Key, err))
			continue
		}
		series, err := seriesOf(raw, def.Key, def.Title, def.Unit)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", def.Key, err))
			continue
		}
		out = append(out, series...)
	}
	return out, failures, nil
}

// Raw runs PromQL with the run matcher injected into every selector.
func (m *Metrics) Raw(ctx context.Context, runID uuid.UUID, query string, start, end time.Time, step time.Duration) (json.RawMessage, error) {
	return m.rangeQuery(ctx, scopePromQL(query, m.label, runID.String()), start, end, step)
}

// scopePromQL adds the run label matcher to every metric selector; a
// selector without braces gets one, a selector with braces gets the
// matcher prepended (VictoriaMetrics also supports extra_filters, but
// rewriting keeps the scope visible in the query returned to the user).
// Label matchers inside braces, strings, numbers with duration suffixes,
// functions and aggregation keywords are left alone.
func scopePromQL(query, label, runID string) string {
	matcher := fmt.Sprintf(`%s=%q`, label, runID)
	var b strings.Builder
	i := 0
	for i < len(query) {
		c := query[i]
		switch {
		case c == '"' || c == '\'' || c == '`':
			j := i + 1
			for j < len(query) && query[j] != c {
				if query[j] == '\\' {
					j++
				}
				j++
			}
			if j < len(query) {
				j++
			}
			b.WriteString(query[i:j])
			i = j
		case c == '{':
			// Copy the matcher list verbatim.
			j := strings.IndexByte(query[i:], '}')
			if j < 0 {
				b.WriteString(query[i:])
				return b.String()
			}
			b.WriteString(query[i : i+j+1])
			i += j + 1
		case c >= '0' && c <= '9':
			// A number, possibly with a duration suffix (5m, 1h30m).
			j := i
			for j < len(query) && (isIdent(query[j]) || query[j] == '.') {
				j++
			}
			b.WriteString(query[i:j])
			i = j
		case isIdentStart(c):
			j := i
			for j < len(query) && isIdent(query[j]) {
				j++
			}
			name := query[i:j]
			k := j
			for k < len(query) && query[k] == ' ' {
				k++
			}
			switch {
			case isGrouping(name) && k < len(query) && query[k] == '(':
				// by (a, b): a label list, not selectors.
				end := strings.IndexByte(query[k:], ')')
				if end < 0 {
					b.WriteString(query[i:])
					return b.String()
				}
				b.WriteString(query[i : k+end+1])
				i = k + end + 1
			case isKeyword(name), k < len(query) && query[k] == '(':
				b.WriteString(name)
				i = j
			case k < len(query) && query[k] == '{':
				end := strings.IndexByte(query[k:], '}')
				if end < 0 {
					b.WriteString(query[i:])
					return b.String()
				}
				inner := strings.TrimSpace(query[k+1 : k+end])
				b.WriteString(name + "{" + matcher)
				if inner != "" {
					b.WriteString("," + inner)
				}
				b.WriteString("}")
				i = k + end + 1
			default:
				b.WriteString(name + "{" + matcher + "}")
				i = j
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

func isIdentStart(c byte) bool {
	return c == '_' || c == ':' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdent(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

var keywords = map[string]bool{
	"by": true, "without": true, "on": true, "ignoring": true, "group_left": true, "group_right": true, "bool": true,
	"and": true, "or": true, "unless": true, "offset": true, "inf": true, "nan": true, "NaN": true, "Inf": true,
	"sum": true, "avg": true, "min": true, "max": true, "count": true, "stddev": true, "stdvar": true, "topk": true, "bottomk": true,
	"quantile": true, "count_values": true, "group": true,
}

func isKeyword(s string) bool { return keywords[s] }

func isGrouping(s string) bool {
	return s == "by" || s == "without" || s == "on" || s == "ignoring" || s == "group_left" || s == "group_right"
}

func (m *Metrics) rangeQuery(ctx context.Context, expr string, start, end time.Time, step time.Duration) (json.RawMessage, error) {
	form := url.Values{"query": {expr}, "start": {strconv.FormatInt(start.Unix(), 10)}, "end": {strconv.FormatInt(end.Unix(), 10)}, "step": {strconv.FormatInt(int64(step.Seconds()), 10) + "s"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.base+"/api/v1/query_range", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := m.hc.Do(req)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, "metrics store", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32*1024*1024))
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, "metrics store", err)
	}
	if res.StatusCode/100 != 2 {
		return nil, errs.Newf(errs.CodeInvalid, "metrics store: %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// seriesOf maps a Prometheus matrix into series with aggregates.
func seriesOf(raw json.RawMessage, key, title, unit string) ([]observe.Series, error) {
	var body struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][2]any          `json:"values"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("%s", body.Error)
	}
	out := make([]observe.Series, 0, len(body.Data.Result))
	for _, r := range body.Data.Result {
		s := observe.Series{Key: key, Title: title, Unit: unit, Role: r.Metric["role"], Machine: firstOf(r.Metric, "machine", "instance")}
		var values []float64
		for _, v := range r.Values {
			ts, ok := v[0].(float64)
			str, ok2 := v[1].(string)
			if !ok || !ok2 {
				continue
			}
			f, err := strconv.ParseFloat(str, 64)
			if err != nil {
				continue
			}
			s.Points = append(s.Points, [2]float64{ts, f})
			values = append(values, f)
		}
		if len(values) > 0 {
			s.Min, s.Max, s.Last = values[0], values[0], values[len(values)-1]
			sum := 0.0
			for _, f := range values {
				sum += f
				s.Min = min(s.Min, f)
				s.Max = max(s.Max, f)
			}
			s.Avg = sum / float64(len(values))
			sorted := append([]float64(nil), values...)
			sort.Float64s(sorted)
			s.P95 = sorted[(len(sorted)-1)*95/100]
		}
		out = append(out, s)
	}
	return out, nil
}

func firstOf(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}
