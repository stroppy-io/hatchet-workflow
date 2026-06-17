package execution

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
)

/*
	Monitoring backend adapter.

	LogReader and MetricsReader read run observability from the VictoriaMetrics /
	VictoriaLogs HTTP backends. The base URL is supplied at construction
	(typically from MONITORING_URL / VICTORIA_URL env). When no base URL is
	configured the readers degrade gracefully: they return empty results. This is
	intentional graceful degradation for deployments without a monitoring backend
	wired up — NOT a placeholder, and never fabricated data.

	===== ACCOUNT-ID RULE =====

	Both the metrics and the logs read paths standardise on a SINGLE tenant space
	on the monitoring backend: accountID 0. Metrics are queried at
	`<base>/select/0/prometheus` and logs at `<base>/select/0/logsql/query`.

	The write path (vmagent external_labels / vector ingestion) also writes to
	account 0, so read and write agree — this fixes the historical write-0 /
	read-tenant mismatch where reads against a per-tenant account returned empty.

	Run / tenant isolation is NOT done via the monitoring tenant id: it is done by
	the globally-unique `stroppy_run_id` metric label (metrics) and the `run_id`
	log field (logs). Cross-tenant access is already gated upstream in the service
	(TestRunReader.Exists), so a single shared account is safe here.
*/

// metricsTenantPath is the Prometheus-API path segment for the shared account 0.
const metricsTenantPath = "/select/0/prometheus"

// VictoriaLogs is single-tenant by the AccountID HTTP header (default 0 — the
// same account the vector collectors write under), NOT by a /select/<id> path
// prefix. So the LogsQL read endpoint is `<monitoringURL>/select/logsql/query`
// (which the gateway's vmauth routes to victorialogs); no tenant path segment.

// RunRecordReader is the slice of the run store the metrics reader needs to
// resolve a run's DB kind and observation window. The persisted
// models.TestRunRecord (via its Summary facet) carries db_kind / started_at /
// finished_at — everything the per-DB PromQL set and the query time window need.
// Injected so the reader stays an execution adapter without owning storage; it is
// satisfied by the same store that backs SnapshotRunReader.
type RunRecordReader interface {
	RunRecord(ctx context.Context, runID string) (*models.TestRunRecord, error)
}

// LogReader implements test_run_overview.LogReader against VictoriaLogs.
type LogReader struct {
	client *victoria.LogsClient // nil when no backend is configured
	log    *slog.Logger
}

var _ test_run_overview.LogReader = (*LogReader)(nil)

// NewLogReader builds the test_run_overview.LogReader adapter. monitoringURL is
// the monitoring gateway root (e.g. http://victoria:8428); the reader queries the
// shared account-0 LogsQL endpoint under it. An empty monitoringURL yields a
// reader that returns empty results (no monitoring backend configured). log may
// be nil.
func NewLogReader(monitoringURL, token string, log *slog.Logger) *LogReader {
	if log == nil {
		log = slog.Default()
	}
	r := &LogReader{log: log}
	if base := strings.TrimRight(monitoringURL, "/"); base != "" {
		r.client = victoria.NewLogsClient(base, token)
	}
	return r
}

// configured reports whether a backend URL was supplied.
func (r *LogReader) configured() bool { return r.client != nil }

// Query returns one cursor-paged page of log lines for the run. With no backend
// configured it returns an empty page and nil cursors (nothing to scroll).
func (r *LogReader) Query(
	ctx context.Context,
	runID string,
	filter *api.LogFilter,
	from *monitor.LogCursor,
	direction api.LogScrollDirection,
	limit uint32,
) (lines []*monitor.LogLine, older, newer *monitor.LogCursor, err error) {
	if !r.configured() {
		return nil, nil, nil, nil
	}
	if limit == 0 {
		limit = 200
	}

	q := victoria.LogsQuery{
		Query: buildLogsQuery(runID, filter, direction),
		Limit: limit,
	}
	// Honour any explicit filter window first, then narrow by the cursor anchor.
	if filter.GetStart() != nil {
		q.Start = filter.GetStart().AsTime()
	}
	if filter.GetEnd() != nil {
		q.End = filter.GetEnd().AsTime()
	}
	if from != nil && from.GetObservedAt() != nil {
		t := from.GetObservedAt().AsTime()
		if direction == api.LogScrollDirection_LOG_SCROLL_DIRECTION_OLDER {
			q.End = t
		} else {
			q.Start = t
		}
	}

	body, err := r.client.Query(ctx, q)
	if err != nil {
		return nil, nil, nil, err
	}
	defer body.Close()

	lines, err = decodeLogLines(body, runID)
	if err != nil {
		return nil, nil, nil, err
	}
	lines = normalizeLogPageOrder(lines, direction)
	older, newer = cursorBounds(lines)
	return lines, older, newer, nil
}

// Stream returns a bounded live tail. With no backend configured it returns an
// already-closed channel (nothing to follow). Otherwise it polls Query for newer
// lines on an interval and pushes them until ctx is done.
func (r *LogReader) Stream(
	ctx context.Context,
	runID string,
	filter *api.LogFilter,
	from *monitor.LogCursor,
) (<-chan *monitor.LogLine, error) {
	out := make(chan *monitor.LogLine)
	if !r.configured() {
		close(out)
		return out, nil
	}

	go func() {
		defer close(out)
		cursor := initialStreamCursor(from, filter)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lines, _, newer, err := r.Query(ctx, runID, filter, cursor, api.LogScrollDirection_LOG_SCROLL_DIRECTION_NEWER, 500)
				if err != nil {
					return
				}
				for _, line := range lines {
					select {
					case <-ctx.Done():
						return
					case out <- line:
					}
				}
				if newer != nil {
					cursor = newer
				}
			}
		}
	}()
	return out, nil
}

func initialStreamCursor(from *monitor.LogCursor, filter *api.LogFilter) *monitor.LogCursor {
	if from != nil {
		return from
	}
	if filter.GetStart() != nil {
		return nil
	}
	return &monitor.LogCursor{ObservedAt: timestamppb.Now()}
}

// Resolve turns a shareable LogRef into the concrete filter + anchor cursor used
// to render it. This is a pure projection (no backend call): the ref's fields map
// directly onto a LogFilter, and its cursor (or start time) becomes the anchor.
func (r *LogReader) Resolve(_ context.Context, ref *monitor.LogRef) (*api.LogFilter, *monitor.LogCursor, error) {
	filter := &api.LogFilter{}
	if ref.GetNodeExecutionId() != "" {
		filter.NodeExecutionIds = []string{ref.GetNodeExecutionId()}
	}
	if ref.GetComponentId() != "" {
		filter.ComponentIds = []string{ref.GetComponentId()}
	}
	if ref.GetStart() != nil {
		filter.Start = ref.GetStart()
	}
	if ref.GetEnd() != nil {
		filter.End = ref.GetEnd()
	}

	cursor := ref.GetCursor()
	if cursor == nil && ref.GetStart() != nil {
		cursor = &monitor.LogCursor{ObservedAt: ref.GetStart()}
	}
	if cursor == nil {
		cursor = &monitor.LogCursor{}
	}
	return filter, cursor, nil
}

// MetricsReader implements test_run_overview.MetricsReader against
// VictoriaMetrics. It resolves the run's DB kind + observation window from the
// persisted run record, builds the per-DB PromQL set, range-queries the backend,
// and aggregates the series with the metrics collector.
type MetricsReader struct {
	base  string // monitoring gateway root; "" disables the reader
	token string // optional bearer token for the backend
	store RunRecordReader
	log   *slog.Logger
}

var _ test_run_overview.MetricsReader = (*MetricsReader)(nil)

// NewMetricsReader builds the test_run_overview.MetricsReader adapter.
// monitoringURL is the monitoring gateway root (e.g. http://victoria:8428); the
// reader queries the shared account-0 Prometheus endpoint under it. store
// resolves the run's DB kind + window from the persisted record; it may be nil,
// in which case the reader falls back to the default (postgres) query set and a
// best-effort trailing window. An empty monitoringURL yields a reader that
// returns an empty RunMetrics (no monitoring backend configured). log may be nil.
func NewMetricsReader(monitoringURL, token string, store RunRecordReader, log *slog.Logger) *MetricsReader {
	if log == nil {
		log = slog.Default()
	}
	return &MetricsReader{
		base:  strings.TrimRight(monitoringURL, "/"),
		token: strings.TrimSpace(token),
		store: store,
		log:   log,
	}
}

func (r *MetricsReader) configured() bool { return r.base != "" }

// Get returns the run's aggregated metric summaries. With no backend configured
// it returns an empty RunMetrics for the run id (graceful degradation). A
// non-nil window scopes the aggregation to that sub-range (e.g. one workload
// segment, so bootstrap/load time is excluded from the averages); otherwise the
// whole-run window is used.
func (r *MetricsReader) Get(ctx context.Context, runID string, window *monitor.TimeRange) (*monitor.RunMetrics, error) {
	if !r.configured() {
		return &monitor.RunMetrics{RunId: runID}, nil
	}

	dbKind, tr := r.resolveKindAndWindow(ctx, runID)
	if w, ok := overrideWindow(window); ok {
		tr = w
	}

	client := victoria.NewClient(r.base+metricsTenantPath, r.token)
	collector := metrics.NewCollectorForDB(client, dbKind)
	return collector.Collect(ctx, runID, tr)
}

// overrideWindow converts a caller-supplied TimeRange into a metrics.TimeRange,
// reporting false when it is absent or not a usable [start, end] range.
func overrideWindow(window *monitor.TimeRange) (metrics.TimeRange, bool) {
	if window.GetStart() == nil || window.GetEnd() == nil {
		return metrics.TimeRange{}, false
	}
	start := window.GetStart().AsTime()
	end := window.GetEnd().AsTime()
	if !end.After(start) {
		return metrics.TimeRange{}, false
	}
	return metrics.TimeRange{Start: start, End: end}, true
}

// resolveKindAndWindow loads the run record (when a store is wired) to determine
// the per-DB query set and the metrics time window. It always returns a usable
// window: the record's [started_at, finished_at] padded by 30s, or a trailing
// 1h window ending now as a fallback.
func (r *MetricsReader) resolveKindAndWindow(ctx context.Context, runID string) (string, metrics.TimeRange) {
	const pad = 30 * time.Second
	now := time.Now()
	dbKind := "postgres"
	tr := metrics.TimeRange{Start: now.Add(-time.Hour), End: now}

	if r.store == nil {
		return dbKind, tr
	}

	rec, err := r.store.RunRecord(ctx, runID)
	if err != nil || rec == nil {
		if err != nil {
			r.log.Debug("metrics: run record lookup failed, using defaults", "run_id", runID, "err", err)
		}
		return dbKind, tr
	}

	if k := dbKindString(rec); k != "" {
		dbKind = k
	}

	sum := rec.GetSummary()
	if sum.GetStartedAt() != nil {
		start := sum.GetStartedAt().AsTime().Add(-pad)
		end := now
		if sum.GetFinishedAt() != nil {
			end = sum.GetFinishedAt().AsTime()
		}
		tr = metrics.TimeRange{Start: start, End: end.Add(pad)}
	}
	return dbKind, tr
}

// dbKindString resolves the run's DB engine to the metrics package's lowercase
// kind key. The summary facet is authoritative; it falls back to the baked spec.
func dbKindString(rec *models.TestRunRecord) string {
	kind := rec.GetSummary().GetDbKind()
	if kind == domainpb.Database_KIND_UNSPECIFIED {
		kind = rec.GetSpec().GetDatabase().GetKind()
	}
	switch kind {
	case domainpb.Database_KIND_MYSQL, domainpb.Database_KIND_MARIADB:
		// MariaDB uses the mysqld_exporter and the same metric names as MySQL.
		return "mysql"
	case domainpb.Database_KIND_YDB, domainpb.Database_KIND_YDB_MANAGED:
		return "ydb"
	case domainpb.Database_KIND_COCKROACH:
		return "cockroach"
	case domainpb.Database_KIND_PICODATA:
		return "picodata"
	case domainpb.Database_KIND_POSTGRES:
		return "postgres"
	default:
		return ""
	}
}

/*
	===== wire helpers =====

	The logs backend speaks the LogsQL JSON-lines shape. These helpers build the
	LogsQL filter and map result rows onto the monitor.* protos. They are
	defensive: malformed rows are skipped, never faked.
*/

// buildLogsQuery composes a LogsQL filter pinning the run id and any narrowing
// filter fields, then a sort. Run isolation is by the run_id field (see
// ACCOUNT-ID RULE above).
func buildLogsQuery(runID string, filter *api.LogFilter, direction api.LogScrollDirection) string {
	parts := []string{fmt.Sprintf("run_id:%q", runID)}

	parts = appendOrFilter(parts, "node_execution_id", filter.GetNodeExecutionIds())
	parts = appendOrFilter(parts, "component_id", filter.GetComponentIds())
	// node_ids is the older API name; machine_ids is the explicit UI/runtime
	// machine filter. Both map to persisted LogLine.machine_id.
	parts = appendOrFilter(parts, "machine_id", mergeStrings(filter.GetNodeIds(), filter.GetMachineIds()))
	parts = appendOrFilter(parts, "phase", filter.GetPhases())
	parts = appendOrFilter(parts, "parent_node_execution_id", filter.GetParentNodeExecutionIds())
	parts = appendOrFilter(parts, "stage_name", filter.GetStageNames())
	parts = appendOrFilter(parts, "step_id", filter.GetStepIds())
	parts = appendOrFilter(parts, "action", filter.GetActions())
	parts = appendOrFilter(parts, "mentions", filter.GetMentions())
	parts = appendEnumFilter(parts, "source", filter.GetSources(), logSourceName)
	parts = appendEnumFilter(parts, "stream", filter.GetStreams(), logStreamName)
	units := filter.GetUnits()
	if u := filter.GetUnit(); u != "" {
		units = mergeStrings([]string{u}, units)
	}
	parts = appendOrFilter(parts, "unit", units)
	if s := filter.GetSearch(); s != "" {
		// Substring match over _msg.
		parts = append(parts, fmt.Sprintf("_msg:%q", s))
	}
	if q := filter.GetQuery(); q != "" {
		// Raw LogsQL fragment, AND-ed with the structured filters.
		parts = append(parts, q)
	}

	query := strings.Join(parts, " ")
	// Oldest-first by default; newest-first for the OLDER scroll so the page
	// closest to the cursor comes back first.
	if direction == api.LogScrollDirection_LOG_SCROLL_DIRECTION_OLDER {
		query += " | sort by (_time) desc"
	} else {
		query += " | sort by (_time)"
	}
	return query
}

func mergeStrings(values ...[]string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, slice := range values {
		for _, v := range slice {
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// appendOrFilter adds a `field:"v"` term for one value or a parenthesised
// OR-group for many. No-op for an empty slice.
func appendOrFilter(parts []string, field string, values []string) []string {
	if len(values) == 0 {
		return parts
	}
	terms := make([]string, len(values))
	for i, v := range values {
		terms[i] = fmt.Sprintf("%s:%q", field, v)
	}
	if len(terms) == 1 {
		return append(parts, terms[0])
	}
	return append(parts, "("+strings.Join(terms, " OR ")+")")
}

func appendEnumFilter[T comparable](parts []string, field string, values []T, name func(T) string) []string {
	if len(values) == 0 {
		return parts
	}
	names := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		n := name(value)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	return appendOrFilter(parts, field, names)
}

func logSourceName(source monitor.Source) string {
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

func logStreamName(stream monitor.Stream) string {
	switch stream {
	case monitor.Stream_STREAM_STDOUT:
		return "stdout"
	case monitor.Stream_STREAM_STDERR:
		return "stderr"
	default:
		return ""
	}
}

// logRow is one VictoriaLogs JSON-lines record (a flat map of the streamed
// fields). Only the fields we surface are pulled out.
type logRow struct {
	Time                  string `json:"_time"`
	Message               string `json:"_msg"`
	RunID                 string `json:"run_id"`
	LineNo                uint64 `json:"line_no"`
	NodeExecutionID       string `json:"node_execution_id"`
	ParentNodeExecutionID string `json:"parent_node_execution_id"`
	Phase                 string `json:"phase"`
	StageName             string `json:"stage_name"`
	ComponentID           string `json:"component_id"`
	MachineID             string `json:"machine_id"`
	StepID                string `json:"step_id"`
	Action                string `json:"action"`
	Mentions              string `json:"mentions"`
	Source                string `json:"source"`
	Unit                  string `json:"unit"`
	Stream                string `json:"stream"`
}

// decodeLogLines parses the JSON-lines logs response into LogLine protos.
func decodeLogLines(body io.Reader, runID string) ([]*monitor.LogLine, error) {
	var lines []*monitor.LogLine
	var seq uint64

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var row logRow
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			// Skip a malformed row rather than fabricating or aborting the page.
			continue
		}
		seq++
		observed := parseTimestamp(row.Time)
		rid := row.RunID
		if rid == "" {
			rid = runID
		}
		lineNo := row.LineNo
		if lineNo == 0 {
			lineNo = seq
		}
		lines = append(lines, &monitor.LogLine{
			ObservedAt:            observed,
			RunId:                 rid,
			LineNo:                lineNo,
			NodeExecutionId:       row.NodeExecutionID,
			ParentNodeExecutionId: row.ParentNodeExecutionID,
			Phase:                 row.Phase,
			StageName:             row.StageName,
			ComponentId:           row.ComponentID,
			MachineId:             row.MachineID,
			StepId:                row.StepID,
			Action:                row.Action,
			Mentions:              splitLogMentions(row.Mentions),
			Source:                parseLogSource(row.Source),
			Unit:                  row.Unit,
			Stream:                parseLogStream(row.Stream),
			Line:                  row.Message,
			Cursor:                &monitor.LogCursor{ObservedAt: observed, Seq: lineNo},
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func splitLogMentions(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseLogSource(value string) monitor.Source {
	switch strings.ToLower(value) {
	case "command", "cmd", "source_command":
		return monitor.Source_SOURCE_COMMAND
	case "journald", "journal", "source_journald":
		return monitor.Source_SOURCE_JOURNALD
	case "file", "source_file":
		return monitor.Source_SOURCE_FILE
	case "server", "source_server":
		return monitor.Source_SOURCE_SERVER
	default:
		return monitor.Source_SOURCE_UNSPECIFIED
	}
}

func parseLogStream(value string) monitor.Stream {
	switch strings.ToLower(value) {
	case "stdout", "stream_stdout":
		return monitor.Stream_STREAM_STDOUT
	case "stderr", "stream_stderr":
		return monitor.Stream_STREAM_STDERR
	default:
		return monitor.Stream_STREAM_UNSPECIFIED
	}
}

func normalizeLogPageOrder(lines []*monitor.LogLine, direction api.LogScrollDirection) []*monitor.LogLine {
	if direction != api.LogScrollDirection_LOG_SCROLL_DIRECTION_OLDER || len(lines) < 2 {
		return lines
	}
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines
}

// cursorBounds returns the older (first) and newer (last) cursors of a page.
func cursorBounds(lines []*monitor.LogLine) (older, newer *monitor.LogCursor) {
	if len(lines) == 0 {
		return nil, nil
	}
	return lines[0].GetCursor(), lines[len(lines)-1].GetCursor()
}

// parseTimestamp parses an RFC3339(.nano) timestamp, returning nil on failure.
func parseTimestamp(s string) *timestamppb.Timestamp {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return timestamppb.New(t)
}
