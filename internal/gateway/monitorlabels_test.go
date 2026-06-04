package gateway

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/golang/snappy"
	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestRewriteJSONLineLabelsForcesAgentClaims(t *testing.T) {
	body := []byte(`{"run_id":"forged","machine_id":"forged","tenant_id":"forged","message":"ok"}` + "\n")

	rewritten, err := rewriteJSONLineLabels(body, testMonitorClaims())
	if err != nil {
		t.Fatalf("rewrite jsonline: %v", err)
	}

	var event map[string]any
	if err := json.Unmarshal(rewritten, &event); err != nil {
		t.Fatalf("unmarshal rewritten jsonline: %v", err)
	}
	if got, want := event[logTenantField], "tenant-1"; got != want {
		t.Fatalf("tenant_id = %q, want %q", got, want)
	}
	if got, want := event[logRunField], "run-1"; got != want {
		t.Fatalf("run_id = %q, want %q", got, want)
	}
	if got, want := event[logMachineField], "node-1"; got != want {
		t.Fatalf("machine_id = %q, want %q", got, want)
	}
	if got, want := event["message"], "ok"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestRewriteRemoteWriteLabelsForcesAgentClaims(t *testing.T) {
	body := testRemoteWriteBody(t, map[string]string{
		"__name__":         "node_cpu_seconds_total",
		promRunLabel:       "forged-run",
		promMachineLabel:   "forged-node",
		promTenantLabel:    "forged-tenant",
		"source_component": "vmagent",
	})

	rewritten, err := rewriteRemoteWriteLabels(body, testMonitorClaims())
	if err != nil {
		t.Fatalf("rewrite remote_write: %v", err)
	}
	labels := readRemoteWriteLabels(t, rewritten)

	if got, want := labels[promTenantLabel], "tenant-1"; got != want {
		t.Fatalf("%s = %q, want %q", promTenantLabel, got, want)
	}
	if got, want := labels[promRunLabel], "run-1"; got != want {
		t.Fatalf("%s = %q, want %q", promRunLabel, got, want)
	}
	if got, want := labels[promMachineLabel], "node-1"; got != want {
		t.Fatalf("%s = %q, want %q", promMachineLabel, got, want)
	}
	if got, want := labels["source_component"], "vmagent"; got != want {
		t.Fatalf("source_component = %q, want %q", got, want)
	}
}

func TestRewriteOTLPMetricLabelsForcesResourceAndStripsDatapointIdentity(t *testing.T) {
	req := &collectormetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
					testStringAttr("service.name", "stroppy"),
					testStringAttr(otlpRunAttr, "forged-run"),
					testStringAttr(promRunLabel, "forged-prom-run"),
				}},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name: "stroppy_vus",
								Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
									DataPoints: []*metricspb.NumberDataPoint{
										{
											Attributes: []*commonpb.KeyValue{
												testStringAttr("phase", "main"),
												testStringAttr(otlpMachineAttr, "forged-node"),
												testStringAttr(promMachineLabel, "forged-prom-node"),
											},
											Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 1},
										},
									},
								}},
							},
						},
					},
				},
			},
		},
	}
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal otlp request: %v", err)
	}

	rewritten, err := rewriteOTLPMetricLabels(body, testMonitorClaims())
	if err != nil {
		t.Fatalf("rewrite otlp metrics: %v", err)
	}

	var got collectormetricspb.ExportMetricsServiceRequest
	if err := proto.Unmarshal(rewritten, &got); err != nil {
		t.Fatalf("unmarshal rewritten otlp: %v", err)
	}
	resourceAttrs := attrMap(got.GetResourceMetrics()[0].GetResource().GetAttributes())
	if got, want := resourceAttrs["service.name"], "stroppy"; got != want {
		t.Fatalf("service.name = %q, want %q", got, want)
	}
	if got, want := resourceAttrs[promTenantLabel], "tenant-1"; got != want {
		t.Fatalf("%s = %q, want %q", promTenantLabel, got, want)
	}
	if got, want := resourceAttrs[promRunLabel], "run-1"; got != want {
		t.Fatalf("%s = %q, want %q", promRunLabel, got, want)
	}
	if got, want := resourceAttrs[promMachineLabel], "node-1"; got != want {
		t.Fatalf("%s = %q, want %q", promMachineLabel, got, want)
	}
	if _, exists := resourceAttrs[otlpRunAttr]; exists {
		t.Fatalf("%s was not stripped from resource attrs", otlpRunAttr)
	}

	dpAttrs := attrMap(got.GetResourceMetrics()[0].GetScopeMetrics()[0].GetMetrics()[0].GetGauge().GetDataPoints()[0].GetAttributes())
	if got, want := dpAttrs["phase"], "main"; got != want {
		t.Fatalf("phase = %q, want %q", got, want)
	}
	if _, exists := dpAttrs[otlpMachineAttr]; exists {
		t.Fatalf("%s was not stripped from datapoint attrs", otlpMachineAttr)
	}
	if _, exists := dpAttrs[promMachineLabel]; exists {
		t.Fatalf("%s was not stripped from datapoint attrs", promMachineLabel)
	}
}

func testMonitorClaims() *agentdomain.TokenClaims {
	return &agentdomain.TokenClaims{
		TenantID:  "tenant-1",
		RunID:     "run-1",
		MachineID: "node-1",
	}
}

func testRemoteWriteBody(t *testing.T, labels map[string]string) []byte {
	t.Helper()
	var ts []byte
	for _, name := range sortedLabelNames(labels) {
		ts = protowire.AppendTag(ts, 1, protowire.BytesType)
		ts = protowire.AppendBytes(ts, composePromLabel(name, labels[name]))
	}
	var req []byte
	req = protowire.AppendTag(req, 1, protowire.BytesType)
	req = protowire.AppendBytes(req, ts)
	return snappy.Encode(nil, req)
}

func readRemoteWriteLabels(t *testing.T, body []byte) map[string]string {
	t.Helper()
	decoded, err := snappy.Decode(nil, body)
	if err != nil {
		t.Fatalf("decode remote_write: %v", err)
	}
	for len(decoded) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(decoded)
		if tagLen < 0 {
			t.Fatalf("consume request tag: %v", protowire.ParseError(tagLen))
		}
		rest := decoded[tagLen:]
		valueLen := protowire.ConsumeFieldValue(num, typ, rest)
		if valueLen < 0 {
			t.Fatalf("consume request field: %v", protowire.ParseError(valueLen))
		}
		value := rest[:valueLen]
		decoded = rest[valueLen:]
		if num != 1 {
			continue
		}
		if typ != protowire.BytesType {
			t.Fatalf("timeseries wire type = %v, want bytes", typ)
		}
		ts, n := protowire.ConsumeBytes(value)
		if n < 0 {
			t.Fatalf("consume timeseries: %v", protowire.ParseError(n))
		}
		return readTimeSeriesLabels(t, ts)
	}
	t.Fatal("remote_write request has no timeseries")
	return nil
}

func readTimeSeriesLabels(t *testing.T, ts []byte) map[string]string {
	t.Helper()
	out := map[string]string{}
	for len(ts) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(ts)
		if tagLen < 0 {
			t.Fatalf("consume timeseries tag: %v", protowire.ParseError(tagLen))
		}
		rest := ts[tagLen:]
		valueLen := protowire.ConsumeFieldValue(num, typ, rest)
		if valueLen < 0 {
			t.Fatalf("consume timeseries field: %v", protowire.ParseError(valueLen))
		}
		value := rest[:valueLen]
		ts = rest[valueLen:]
		if num != 1 {
			continue
		}
		labelMsg, n := protowire.ConsumeBytes(value)
		if n < 0 {
			t.Fatalf("consume label: %v", protowire.ParseError(n))
		}
		label, err := parsePromLabel(labelMsg)
		if err != nil {
			t.Fatalf("parse label: %v", err)
		}
		out[label.name] = label.value
	}
	return out
}

func sortedLabelNames(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func testStringAttr(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key: key,
		Value: &commonpb.AnyValue{
			Value: &commonpb.AnyValue_StringValue{StringValue: value},
		},
	}
}

func attrMap(attrs []*commonpb.KeyValue) map[string]string {
	out := map[string]string{}
	for _, attr := range attrs {
		out[attr.GetKey()] = attr.GetValue().GetStringValue()
	}
	return out
}
