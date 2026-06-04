package workload

import (
	"regexp"
	"strings"
	"testing"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
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
	}, &domain.Database{Kind: domain.Database_KIND_POSTGRES}, map[string]string{
		deploymentbuilder.LabelServerAddr: "https://control.example",
		deploymentbuilder.LabelRunID:      "run-1",
	}, databaseTarget{Host: "10.0.0.2", Port: 5432}, 4, "agent-token")

	for _, pattern := range []string{
		`"url":\s+"postgresql://postgres@10.0.0.2:5432/postgres\?sslmode=disable"`,
		`"LOAD_WORKERS":\s+"4"`,
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
	if strings.Contains(rendered, DBHostPlaceholder) || strings.Contains(rendered, DBPortPlaceholder) {
		t.Fatalf("runtime stroppy config still contains DB placeholders:\n%s", rendered)
	}
	if !strings.Contains(rendered, `"-q"`) {
		t.Fatalf("stroppy config should default k6 to quiet mode:\n%s", rendered)
	}
}

func TestWorkloadDeploymentRendererWritesRuntimeDBEndpoint(t *testing.T) {
	port := uint32(5432)
	sshPort := uint32(22)
	spec := &topologypb.TopologySpec{
		Labels: map[string]string{
			deploymentbuilder.LabelServerAddr: "http://server:8080",
			deploymentbuilder.LabelRunID:      "run-1",
		},
		Components: []*topologypb.Component{
			{Id: "postgres-master", Kind: topologypb.Component_KIND_DATABASE, Engine: "fake", Role: "master"},
			component(RunnerNodeID, RunnerNodeID),
		},
		Nodes: []*topologypb.Node{
			{Id: "postgres-master", ComponentIds: []string{"postgres-master"}},
			node(RunnerNodeID, []string{RunnerNodeID}),
		},
		Connections: []*topologypb.Connection{
			{
				FromComponentId: RunnerNodeID,
				ToComponentId:   "postgres-master",
				Kind:            topologypb.Connection_KIND_FLOW,
				Protocol:        topologypb.Connection_PROTOCOL_TCP,
				Mode:            topologypb.Connection_MODE_STREAM,
				EndpointName:    "postgres",
				Port:            &port,
			},
		},
	}
	state := &deploymentpb.InfrastructureState{
		Provider: deploymentpb.Provider_PROVIDER_DOCKER,
		Machines: []*deploymentpb.MachineState{
			{
				NodeId:             "postgres-master",
				ProviderResourceId: "postgres-container",
				Status:             common.Status_STATUS_DEPLOYED,
				Endpoints:          []*deploymentpb.Endpoint{{Name: "private", Address: "10.0.0.2", Port: &sshPort}},
			},
			{
				NodeId:             RunnerNodeID,
				ProviderResourceId: "runner-container",
				Status:             common.Status_STATUS_DEPLOYED,
				Endpoints:          []*deploymentpb.Endpoint{{Name: "private", Address: "10.0.0.3", Port: &sshPort}},
				AllocatedQuotas: []*deploymentpb.Quota_Allocation{
					{
						Info: &deploymentpb.Quota_Info{
							Provider: deploymentpb.Provider_PROVIDER_DOCKER,
							Name:     "host.cpuCores",
							Units:    "cores",
						},
						Used: 4,
					},
				},
			},
		},
	}

	plan, err := deploymentbuilder.BuildPlan(spec, state, deploymentbuilder.BuildOptions{
		Database:    &domain.Database{Kind: domain.Database_KIND_POSTGRES},
		Workload:    testWorkload(),
		Renderers:   deploymentbuilder.NewRegistry(fakeDBRenderer{}, DeploymentRenderer{}),
		AgentTokens: map[string]string{RunnerNodeID: "agent-token"},
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}

	config := deploymentByID(plan, RunnerNodeID).GetSteps()[2].GetWriteFile().GetText()
	for _, want := range []string{
		`postgresql://postgres@10.0.0.2:5432/postgres?sslmode=disable`,
		`"LOAD_WORKERS"`,
		`"4"`,
		`"-q"`,
		`"otlpHttpEndpoint"`,
		`"server:8080"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}
	if strings.Contains(config, DBHostPlaceholder) || strings.Contains(config, DBPortPlaceholder) {
		t.Fatalf("config still contains DB placeholders:\n%s", config)
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

type fakeDBRenderer struct{}

func (r fakeDBRenderer) Supports(component *topologypb.Component) bool {
	return component.GetEngine() == "fake"
}

func (r fakeDBRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	return &deploymentpb.ComponentDeployment{
		ComponentId:    ctx.Component.GetId(),
		NodeId:         ctx.Node.GetId(),
		GlobalPriority: 10,
		NodePriority:   10,
		Steps:          []*deploymentpb.AgentStep{deploymentbuilder.CallCmdStep("run", 1, "true")},
		Status:         common.Status_STATUS_PENDING,
	}, nil
}

func testWorkload() *domain.Workload {
	return &domain.Workload{
		StroppyVersion: "5.1.2",
		Script:         "tpcc/procs",
		Execution: &domain.Workload_Execution{
			Vus:          1,
			Limit:        &domain.Workload_Execution_Duration{Duration: "10s"},
			NoThresholds: true,
		},
		Parameters: &domain.Workload_Parameters{
			PoolSize:    2,
			ScaleFactor: 1,
		},
	}
}

func deploymentByID(plan *deploymentpb.DeploymentPlan, componentID string) *deploymentpb.ComponentDeployment {
	for _, component := range plan.GetComponents() {
		if component.GetComponentId() == componentID {
			return component
		}
	}
	return nil
}
