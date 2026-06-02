package execution

import (
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
func NewLogReader(monitoringURL string, log *slog.Logger) *LogReader {
	if log == nil {
		log = slog.Default()
	}
	r := &LogReader{log: log}
	if base := strings.TrimRight(monitoringURL, "/"); base != "" {
		r.client = victoria.NewLogsClient(base, "")
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
		cursor := from
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
func NewMetricsReader(monitoringURL string, store RunRecordReader, log *slog.Logger) *MetricsReader {
	if log == nil {
		log = slog.Default()
	}
	return &MetricsReader{
		base:  strings.TrimRight(monitoringURL, "/"),
		store: store,
		log:   log,
	}
}

func (r *MetricsReader) configured() bool { return r.base != "" }

// Get returns the run's aggregated metric summaries. With no backend configured
// it returns an empty RunMetrics for the run id (graceful degradation).
func (r *MetricsReader) Get(ctx context.Context, runID string) (*monitor.RunMetrics, error) {
	if !r.configured() {
		return &monitor.RunMetrics{RunId: runID}, nil
	}

	dbKind, tr := r.resolveKindAndWindow(ctx, runID)

	client := victoria.NewClient(r.base+metricsTenantPath, r.token)
	collector := metrics.NewCollectorForDB(client, dbKind)
	return collector.Collect(ctx, runID, tr)
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
	case domainpb.Database_KIND_MYSQL:
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
	parts = appendOrFilter(parts, "node_id", filter.GetNodeIds())

	if u := filter.GetUnit(); u != "" {
		parts = append(parts, fmt.Sprintf("unit:%q", u))
	}
	if s := filter.GetSearch(); s != "" {
		// Substring match over _msg.
		parts = append(parts, fmt.Sprintf("_msg:%q", strings.ReplaceAll(s, `"`, `\"`)))
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

// appendOrFilter adds a `field:"v"` term for one value or a parenthesised
// OR-group for many. No-op for an empty slice.
func appendOrFilter(parts []string, field string, values []string) []string {
	if len(values) == 0 {
		return parts
	}
	terms := make([]string, len(values))
	for i, v := range values {
		terms[i] = fmt.Sprintf("%s:%q", field, strings.ReplaceAll(v, `"`, `\"`))
	}
	if len(terms) == 1 {
		return append(parts, terms[0])
	}
	return append(parts, "("+strings.Join(terms, " OR ")+")")
}

// logRow is one VictoriaLogs JSON-lines record (a flat map of the streamed
// fields). Only the fields we surface are pulled out.
type logRow struct {
	Time            string `json:"_time"`
	Message         string `json:"_msg"`
	RunID           string `json:"run_id"`
	NodeExecutionID string `json:"node_execution_id"`
	ComponentID     string `json:"component_id"`
	MachineID       string `json:"machine_id"`
}

// decodeLogLines parses the JSON-lines logs response into LogLine protos.
func decodeLogLines(body io.Reader, runID string) ([]*monitor.LogLine, error) {
	dec := json.NewDecoder(body)
	var lines []*monitor.LogLine
	var seq uint64
	for dec.More() {
		var row logRow
		if err := dec.Decode(&row); err != nil {
			// Skip a malformed row rather than fabricating or aborting the page.
			continue
		}
		seq++
		observed := parseTimestamp(row.Time)
		rid := row.RunID
		if rid == "" {
			rid = runID
		}
		lines = append(lines, &monitor.LogLine{
			ObservedAt:      observed,
			RunId:           rid,
			LineNo:          seq,
			NodeExecutionId: row.NodeExecutionID,
			ComponentId:     row.ComponentID,
			MachineId:       row.MachineID,
			Line:            row.Message,
			Cursor:          &monitor.LogCursor{ObservedAt: observed, Seq: seq},
		})
	}
	return lines, nil
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
