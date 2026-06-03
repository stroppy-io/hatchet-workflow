package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/golang/snappy"
	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const (
	promTenantLabel  = "stroppy_tenant_id"
	promRunLabel     = "stroppy_run_id"
	promMachineLabel = "stroppy_machine_id"

	otlpTenantAttr  = "stroppy.tenant.id"
	otlpRunAttr     = "stroppy.run.id"
	otlpMachineAttr = "stroppy.machine.id"

	logTenantField  = "tenant_id"
	logRunField     = "run_id"
	logMachineField = "machine_id"
)

type monitorLabel struct {
	name  string
	value string
}

func rewriteMonitorPayload(path string, body []byte, claims *agentdomain.TokenClaims) ([]byte, error) {
	switch {
	case strings.Contains(path, "/prometheus/api/v1/write"):
		return rewriteRemoteWriteLabels(body, claims)
	case strings.Contains(path, "/opentelemetry/v1/metrics"):
		return rewriteOTLPMetricLabels(body, claims)
	case strings.Contains(path, "/jsonline"):
		return rewriteJSONLineLabels(body, claims)
	default:
		return nil, fmt.Errorf("unsupported insert endpoint %q", path)
	}
}

func rewriteJSONLineLabels(body []byte, claims *agentdomain.TokenClaims) ([]byte, error) {
	lines := bytes.SplitAfter(body, []byte("\n"))
	out := make([]byte, 0, len(body)+128)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		hasNewline := line[len(line)-1] == '\n'
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(bytes.TrimSpace(trimmed)) == 0 {
			out = append(out, line...)
			continue
		}

		var event map[string]any
		if err := json.Unmarshal(trimmed, &event); err != nil {
			return nil, err
		}
		event[logTenantField] = claims.TenantID
		event[logRunField] = claims.RunID
		event[logMachineField] = claims.MachineID

		encoded, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		out = append(out, encoded...)
		if hasNewline {
			out = append(out, '\n')
		}
	}
	return out, nil
}

func rewriteRemoteWriteLabels(body []byte, claims *agentdomain.TokenClaims) ([]byte, error) {
	decoded, err := snappy.Decode(nil, body)
	if err != nil {
		return nil, err
	}
	rewritten, err := rewriteWriteRequestLabels(decoded, promForcedLabels(claims))
	if err != nil {
		return nil, err
	}
	return snappy.Encode(nil, rewritten), nil
}

func rewriteWriteRequestLabels(msg []byte, forced []monitorLabel) ([]byte, error) {
	var out []byte
	for len(msg) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(msg)
		if tagLen < 0 {
			return nil, protowire.ParseError(tagLen)
		}
		rest := msg[tagLen:]
		valueLen := protowire.ConsumeFieldValue(num, typ, rest)
		if valueLen < 0 {
			return nil, protowire.ParseError(valueLen)
		}
		value := rest[:valueLen]
		msg = rest[valueLen:]

		if num == 1 {
			if typ != protowire.BytesType {
				return nil, fmt.Errorf("remote_write timeseries field has wire type %v", typ)
			}
			ts, n := protowire.ConsumeBytes(value)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			rewritten, err := rewriteTimeSeriesLabels(ts, forced)
			if err != nil {
				return nil, err
			}
			out = protowire.AppendTag(out, num, typ)
			out = protowire.AppendBytes(out, rewritten)
			continue
		}

		out = protowire.AppendTag(out, num, typ)
		out = append(out, value...)
	}
	return out, nil
}

func rewriteTimeSeriesLabels(msg []byte, forced []monitorLabel) ([]byte, error) {
	blocked := labelNameSet(forced)
	labels := make([]monitorLabel, 0, len(forced)+8)
	var other []byte

	for len(msg) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(msg)
		if tagLen < 0 {
			return nil, protowire.ParseError(tagLen)
		}
		rest := msg[tagLen:]
		valueLen := protowire.ConsumeFieldValue(num, typ, rest)
		if valueLen < 0 {
			return nil, protowire.ParseError(valueLen)
		}
		value := rest[:valueLen]
		msg = rest[valueLen:]

		if num == 1 {
			if typ != protowire.BytesType {
				return nil, fmt.Errorf("remote_write label field has wire type %v", typ)
			}
			labelMsg, n := protowire.ConsumeBytes(value)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			label, err := parsePromLabel(labelMsg)
			if err != nil {
				return nil, err
			}
			if _, exists := blocked[label.name]; !exists {
				labels = append(labels, label)
			}
			continue
		}

		other = protowire.AppendTag(other, num, typ)
		other = append(other, value...)
	}

	labels = append(labels, forced...)
	sort.SliceStable(labels, func(i, j int) bool {
		return labels[i].name < labels[j].name
	})

	var out []byte
	for _, label := range labels {
		out = protowire.AppendTag(out, 1, protowire.BytesType)
		out = protowire.AppendBytes(out, composePromLabel(label.name, label.value))
	}
	out = append(out, other...)
	return out, nil
}

func parsePromLabel(msg []byte) (monitorLabel, error) {
	var label monitorLabel
	for len(msg) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(msg)
		if tagLen < 0 {
			return label, protowire.ParseError(tagLen)
		}
		rest := msg[tagLen:]
		valueLen := protowire.ConsumeFieldValue(num, typ, rest)
		if valueLen < 0 {
			return label, protowire.ParseError(valueLen)
		}
		value := rest[:valueLen]
		msg = rest[valueLen:]

		if typ != protowire.BytesType {
			continue
		}
		text, n := protowire.ConsumeString(value)
		if n < 0 {
			return label, protowire.ParseError(n)
		}
		switch num {
		case 1:
			label.name = text
		case 2:
			label.value = text
		}
	}
	if label.name == "" {
		return label, fmt.Errorf("remote_write label without name")
	}
	return label, nil
}

func composePromLabel(name, value string) []byte {
	var msg []byte
	msg = protowire.AppendTag(msg, 1, protowire.BytesType)
	msg = protowire.AppendString(msg, name)
	msg = protowire.AppendTag(msg, 2, protowire.BytesType)
	msg = protowire.AppendString(msg, value)
	return msg
}

func rewriteOTLPMetricLabels(body []byte, claims *agentdomain.TokenClaims) ([]byte, error) {
	var req collectormetricspb.ExportMetricsServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	for _, rm := range req.GetResourceMetrics() {
		if rm.Resource == nil {
			rm.Resource = &resourcepb.Resource{}
		}
		rm.Resource.Attributes = upsertOTLPAttrs(rm.Resource.GetAttributes(), otlpForcedAttrs(claims))
		for _, sm := range rm.GetScopeMetrics() {
			for _, metric := range sm.GetMetrics() {
				stripOTLPMetricDatapointIdentity(metric)
			}
		}
	}
	return proto.Marshal(&req)
}

func stripOTLPMetricDatapointIdentity(metric *metricspb.Metric) {
	if metric == nil {
		return
	}
	if gauge := metric.GetGauge(); gauge != nil {
		for _, dp := range gauge.GetDataPoints() {
			dp.Attributes = stripOTLPIdentityAttrs(dp.GetAttributes())
		}
	}
	if sum := metric.GetSum(); sum != nil {
		for _, dp := range sum.GetDataPoints() {
			dp.Attributes = stripOTLPIdentityAttrs(dp.GetAttributes())
		}
	}
	if histogram := metric.GetHistogram(); histogram != nil {
		for _, dp := range histogram.GetDataPoints() {
			dp.Attributes = stripOTLPIdentityAttrs(dp.GetAttributes())
		}
	}
	if histogram := metric.GetExponentialHistogram(); histogram != nil {
		for _, dp := range histogram.GetDataPoints() {
			dp.Attributes = stripOTLPIdentityAttrs(dp.GetAttributes())
		}
	}
	if summary := metric.GetSummary(); summary != nil {
		for _, dp := range summary.GetDataPoints() {
			dp.Attributes = stripOTLPIdentityAttrs(dp.GetAttributes())
		}
	}
}

func upsertOTLPAttrs(attrs []*commonpb.KeyValue, forced []monitorLabel) []*commonpb.KeyValue {
	blocked := otlpIdentityKeys()
	out := make([]*commonpb.KeyValue, 0, len(attrs)+len(forced))
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		if _, exists := blocked[attr.GetKey()]; exists {
			continue
		}
		out = append(out, attr)
	}
	for _, label := range forced {
		out = append(out, &commonpb.KeyValue{
			Key: label.name,
			Value: &commonpb.AnyValue{
				Value: &commonpb.AnyValue_StringValue{StringValue: label.value},
			},
		})
	}
	return out
}

func stripOTLPIdentityAttrs(attrs []*commonpb.KeyValue) []*commonpb.KeyValue {
	blocked := otlpIdentityKeys()
	out := attrs[:0]
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		if _, exists := blocked[attr.GetKey()]; exists {
			continue
		}
		out = append(out, attr)
	}
	return out
}

func promForcedLabels(claims *agentdomain.TokenClaims) []monitorLabel {
	return []monitorLabel{
		{name: promMachineLabel, value: claims.MachineID},
		{name: promRunLabel, value: claims.RunID},
		{name: promTenantLabel, value: claims.TenantID},
	}
}

func otlpForcedAttrs(claims *agentdomain.TokenClaims) []monitorLabel {
	return []monitorLabel{
		{name: otlpMachineAttr, value: claims.MachineID},
		{name: otlpRunAttr, value: claims.RunID},
		{name: otlpTenantAttr, value: claims.TenantID},
	}
}

func labelNameSet(labels []monitorLabel) map[string]struct{} {
	out := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		out[label.name] = struct{}{}
	}
	return out
}

func otlpIdentityKeys() map[string]struct{} {
	return map[string]struct{}{
		otlpTenantAttr:   {},
		otlpRunAttr:      {},
		otlpMachineAttr:  {},
		promTenantLabel:  {},
		promRunLabel:     {},
		promMachineLabel: {},
	}
}
