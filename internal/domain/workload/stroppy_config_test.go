package workload

import (
	"regexp"
	"strings"
	"testing"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestRenderStroppyConfigRoutesOTLPThroughServerAddress(t *testing.T) {
	rendered := renderStroppyConfigJSON(&domain.Workload{
		Script:   "tpcc/tx",
		Protocol: domain.Workload_PROTOCOL_PG,
		Execution: &domain.Workload_Execution{
			Vus: 1,
			Limit: &domain.Workload_Execution_Duration{
				Duration: "1m",
			},
		},
	}, map[string]string{
		deploymentbuilder.LabelServerAddr: "https://control.example",
		deploymentbuilder.LabelRunID:      "run-1",
	}, "", "agent-token")

	for _, pattern := range []string{
		`"otlpHttpEndpoint":\s+"control.example"`,
		`"otlpHttpExporterUrlPath":\s+"/insert/0/opentelemetry/v1/metrics"`,
		`"otlpEndpointInsecure":\s+false`,
		`"otlpMetricsPrefix":\s+"run_1_"`,
		`"otlpHeaders":\s+"Authorization=Bearer agent-token"`,
	} {
		if !regexp.MustCompile(pattern).MatchString(rendered) {
			t.Fatalf("stroppy config missing pattern %q:\n%s", pattern, rendered)
		}
	}
	for _, forbidden := range []string{"vmauth", "victoria", "MONITORING_URL"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("stroppy config leaked %q:\n%s", forbidden, rendered)
		}
	}
}
