package metrics

import (
	"strings"
	"testing"
)

func TestStroppyMetricPrefixStartsWithLetter(t *testing.T) {
	runID := "124b8318-b4ea-45ca-9445-4109fd568446"

	if got, want := StroppyMetricPrefix(runID), "stroppy_124b8318_b4ea_45ca_9445_4109fd568446"; got != want {
		t.Fatalf("prefix = %q, want %q", got, want)
	}

	query := RenderQuery(MetricDef{Query: `sum(%p_vus)`}, runID)
	if !strings.Contains(query, "stroppy_124b8318_b4ea_45ca_9445_4109fd568446_vus") {
		t.Fatalf("query does not use stroppy metric prefix: %s", query)
	}
}
