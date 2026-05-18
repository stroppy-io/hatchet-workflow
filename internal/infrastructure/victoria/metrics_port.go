package victoria

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// MetricsAdapter implements testing.MetricsPort against a VictoriaMetrics
// Prometheus-compatible API. It picks the largest available window
// (-1h..now by default) and a coarse 30s step; callers compute aggregates
// themselves.
type MetricsAdapter struct {
	cli *Client
}

// NewMetricsAdapter wires a Client into a MetricsPort implementation.
func NewMetricsAdapter(cli *Client) *MetricsAdapter { return &MetricsAdapter{cli: cli} }

// Query runs the supplied PromQL and returns the result as a proto series
// list. test_run_id is included as a label filter when the query contains
// "$TEST_RUN_ID"; otherwise tenant/testrun scoping is the caller's
// responsibility.
func (a *MetricsAdapter) Query(
	ctx context.Context,
	tenant *iampb.TenantId,
	testRunID *testingpb.TestRunId,
	query string,
) (*testingpb.MetricSeriesList, error) {
	if a.cli == nil {
		return &testingpb.MetricSeriesList{}, nil
	}
	now := time.Now()
	from := now.Add(-1 * time.Hour)
	step := 30 * time.Second

	expanded := expandPlaceholders(query, tenant.GetValue(), testRunID.GetValue())

	res, err := a.cli.QueryRange(ctx, expanded, from, now, step)
	if err != nil {
		return nil, fmt.Errorf("victoria: %w", err)
	}
	out := &testingpb.MetricSeriesList{}
	for _, s := range res.Data.Result {
		series := &testingpb.MetricSeries{
			Name:   s.Metric["__name__"],
			Labels: s.Metric,
		}
		for _, v := range s.Values {
			ts := time.Unix(int64(v.Timestamp()), 0)
			fv, err := strconv.ParseFloat(v.Val(), 64)
			if err != nil {
				continue
			}
			series.Points = append(series.Points, &testingpb.MetricPoint{
				Ts:    timestamppb.New(ts),
				Value: fv,
			})
		}
		out.Series = append(out.Series, series)
	}
	return out, nil
}

// expandPlaceholders performs minimal substitution; full templating is the
// caller's job. Replaces $TEST_RUN_ID and $TENANT_ID inside the query.
func expandPlaceholders(query, tenantID, testRunID string) string {
	out := query
	for _, r := range [][2]string{
		{"$TEST_RUN_ID", testRunID},
		{"$TENANT_ID", tenantID},
	} {
		out = replaceAll(out, r[0], r[1])
	}
	return out
}

func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	out := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, new...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}
