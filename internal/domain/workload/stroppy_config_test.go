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

func TestInstallCommandDownloadsStroppyReleaseThroughGateway(t *testing.T) {
	script := installCommand(&domain.Workload{StroppyVersion: "5.1.2"}, "http://server:8080/")

	for _, want := range []string{
		"http://server:8080/api/binaries/stroppy/5.1.2/stroppy_linux_amd64.tar.gz",
		"curl -fsSL",
		"tar tzf",
		"install -m 0755",
		"/usr/local/bin/stroppy",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install command missing %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "binary resolver is pending") {
		t.Fatalf("install command still contains placeholder:\n%s", script)
	}
}

func TestStroppyDownloadURLRoutesNightlyCommits(t *testing.T) {
	got := stroppyDownloadURL("http://server:8080", "abcdef1234567890")
	want := "http://server:8080/api/binaries/stroppy_nightly/abcdef1/stroppy"
	if got != want {
		t.Fatalf("download URL = %q, want %q", got, want)
	}
}
