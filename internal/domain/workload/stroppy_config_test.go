package workload

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestRenderStroppyConfigRoutesOTLPThroughServerAddress(t *testing.T) {
	w := &domain.Workload{
		Protocol: domain.Workload_PROTOCOL_PG,
		Segments: []*domain.Workload_Segment{{
			Name:   "workload",
			Script: "tpcc/tx",
			Execution: &domain.Workload_Execution{
				Vus: proto.Uint32(1),
				Limit: &domain.Workload_Execution_Duration{
					Duration: "1m",
				},
			},
		}},
	}
	rendered := renderStroppyConfigJSON(w, w.GetSegments()[0], &domain.Database{Kind: domain.Database_KIND_POSTGRES}, map[string]string{
		deploymentbuilder.LabelServerAddr: "https://control.example",
		deploymentbuilder.LabelRunID:      "run-1",
	}, databaseTarget{Host: "10.0.0.2", Port: 5432}, 4, "agent-token")

	for _, pattern := range []string{
		`"url":\s+"postgresql://postgres:stroppy_postgres@10.0.0.2:5432/postgres\?sslmode=disable"`,
		`"LOAD_WORKERS":\s+"4"`,
		`"otlpHttpEndpoint":\s+"control.example"`,
		`"otlpHttpExporterUrlPath":\s+"/insert/0/opentelemetry/v1/metrics"`,
		`"otlpEndpointInsecure":\s+false`,
		`"otlpMetricsPrefix":\s+"stroppy_run_1_"`,
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
	if !regexp.MustCompile(`"K6_SETUP_TIMEOUT":\s+"20m"`).MatchString(rendered) {
		t.Fatalf("stroppy config should extend k6 setup timeout through env:\n%s", rendered)
	}
	if strings.Contains(rendered, `"--setup-timeout"`) {
		t.Fatalf("stroppy config must not pass unsupported k6 setup timeout flag:\n%s", rendered)
	}
}

func TestRenderStroppyConfigSetsPicodataBulkSize(t *testing.T) {
	w := &domain.Workload{
		Protocol: domain.Workload_PROTOCOL_PICODATA,
		Segments: []*domain.Workload_Segment{{
			Name:   "workload",
			Script: "tpcc/tx",
			Execution: &domain.Workload_Execution{
				Vus: proto.Uint32(1),
				Limit: &domain.Workload_Execution_Duration{
					Duration: "1m",
				},
			},
		}},
	}
	rendered := renderStroppyConfigJSON(w, w.GetSegments()[0], &domain.Database{Kind: domain.Database_KIND_PICODATA}, map[string]string{
		deploymentbuilder.LabelServerAddr: "https://control.example",
		deploymentbuilder.LabelRunID:      "run-1",
	}, databaseTarget{Host: "10.0.0.2", Port: 5432}, 4, "agent-token")

	for _, want := range []string{
		`"driverType":\s+"picodata"`,
		`"bulkSize":\s+1`,
	} {
		if !regexp.MustCompile(want).MatchString(rendered) {
			t.Fatalf("picodata config missing pattern %q:\n%s", want, rendered)
		}
	}
}

func TestDriverTypeURLMatchesProtocolRegistry(t *testing.T) {
	tests := []struct {
		name       string
		protocol   domain.Workload_Protocol
		target     databaseTarget
		driverType string
		url        string
	}{
		{
			name:       "postgres",
			protocol:   domain.Workload_PROTOCOL_PG,
			target:     databaseTarget{Host: "10.0.0.2", Port: 5432}.withDefaults(&domain.Database{Kind: domain.Database_KIND_POSTGRES}),
			driverType: "postgres",
			url:        "postgresql://postgres:stroppy_postgres@10.0.0.2:5432/postgres?sslmode=disable",
		},
		{
			name:       "mysql",
			protocol:   domain.Workload_PROTOCOL_MYSQL,
			target:     databaseTarget{Host: "10.0.0.2", Port: 3306},
			driverType: "mysql",
			url:        "root@tcp(10.0.0.2:3306)/stroppy",
		},
		{
			name:       "picodata",
			protocol:   domain.Workload_PROTOCOL_PICODATA,
			target:     databaseTarget{Host: "10.0.0.2", Port: 5432},
			driverType: "picodata",
			url:        "postgres://admin:T0psecret@10.0.0.2:5432?sslmode=disable",
		},
		{
			name:       "ydb grpc",
			protocol:   domain.Workload_PROTOCOL_YDB_GRPC,
			target:     databaseTarget{Host: "10.0.0.2", Port: 2136},
			driverType: "ydb",
			url:        "grpc://10.0.0.2:2136/Root/testdb",
		},
		{
			name:       "managed ydb grpcs",
			protocol:   domain.Workload_PROTOCOL_YDB_GRPCS,
			target:     databaseTarget{Host: "ydb.serverless.yandexcloud.net", Port: 2135, DatabasePath: "/ru-central1/b1g/test"},
			driverType: "ydb",
			url:        "grpcs://ydb.serverless.yandexcloud.net:2135/?database=/ru-central1/b1g/test",
		},
		{
			name:       "cockroach",
			protocol:   domain.Workload_PROTOCOL_COCKROACH,
			target:     databaseTarget{Host: "10.0.0.2", Port: 26257},
			driverType: "postgres",
			url:        "postgresql://10.0.0.2:26257/defaultdb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverType, driverURL := driverTypeURL(tt.protocol, tt.target)
			driverURL = resolveDatabasePath(driverURL, tt.target.DatabasePath)
			if driverType != tt.driverType || driverURL != tt.url {
				t.Fatalf("driver = (%q, %q), want (%q, %q)", driverType, driverURL, tt.driverType, tt.url)
			}
		})
	}
}

func TestEffectiveProtocolDefaultsFromDatabaseKind(t *testing.T) {
	tests := []struct {
		kind domain.Database_Kind
		want domain.Workload_Protocol
	}{
		{domain.Database_KIND_POSTGRES, domain.Workload_PROTOCOL_PG},
		{domain.Database_KIND_MYSQL, domain.Workload_PROTOCOL_MYSQL},
		{domain.Database_KIND_MARIADB, domain.Workload_PROTOCOL_MYSQL},
		{domain.Database_KIND_YDB, domain.Workload_PROTOCOL_YDB_GRPC},
		{domain.Database_KIND_YDB_MANAGED, domain.Workload_PROTOCOL_YDB_GRPCS},
		{domain.Database_KIND_COCKROACH, domain.Workload_PROTOCOL_COCKROACH},
		{domain.Database_KIND_PICODATA, domain.Workload_PROTOCOL_PICODATA},
	}

	for _, tt := range tests {
		if got := effectiveProtocol(domain.Workload_PROTOCOL_UNSPECIFIED, &domain.Database{Kind: tt.kind}); got != tt.want {
			t.Fatalf("effective protocol for %s = %s, want %s", tt.kind, got, tt.want)
		}
	}
}

func TestWorkloadConnectionUsesManagedYDBPort(t *testing.T) {
	_, port, protocol := workloadConnection(&domain.Workload{Protocol: domain.Workload_PROTOCOL_YDB_GRPCS}, &topologypb.Component{})
	if port != 2135 || protocol != topologypb.Connection_PROTOCOL_GRPC {
		t.Fatalf("managed ydb workload connection = port %d protocol %s, want 2135/%s", port, protocol, topologypb.Connection_PROTOCOL_GRPC)
	}
}

func TestPatchStroppyConfigOverrideSubstitutesRuntimeCredentials(t *testing.T) {
	file := &common.File{
		Info: &common.File_Info{Path: "/etc/stroppy-cloud/workload/stroppy-config.json"},
		Content: &common.File_Text{Text: `{
  "version": "1",
  "drivers": {
    "0": {
      "driverType": "postgres",
      "url": "postgresql://__STROPPY_DB_USER__:__STROPPY_DB_PASSWORD__@__STROPPY_DB_HOST__:__STROPPY_DB_PORT__/postgres?sslmode=disable"
    }
  }
}`},
	}

	patched, err := patchStroppyConfigFile(file, map[string]string{
		deploymentbuilder.LabelServerAddr: "http://server:8080",
		deploymentbuilder.LabelRunID:      "run-1",
	}, databaseTarget{Host: "10.0.0.2", Port: 5432}.withDefaults(&domain.Database{Kind: domain.Database_KIND_POSTGRES}), "agent-token")
	if err != nil {
		t.Fatalf("patch stroppy config: %v", err)
	}

	text := patched.GetText()
	for _, want := range []string{
		`postgresql://postgres:stroppy_postgres@10.0.0.2:5432/postgres?sslmode=disable`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("patched config missing %q:\n%s", want, text)
		}
	}
	for _, pattern := range []string{
		`"otlpHttpEndpoint":\s+"server:8080"`,
		`"otlpMetricsPrefix":\s+"stroppy_run_1_"`,
	} {
		if !regexp.MustCompile(pattern).MatchString(text) {
			t.Fatalf("patched config missing pattern %q:\n%s", pattern, text)
		}
	}
	for _, forbidden := range []string{DBHostPlaceholder, DBPortPlaceholder, DBUserPlaceholder, DBPasswordPlaceholder} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("patched config still contains %q:\n%s", forbidden, text)
		}
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
		`postgresql://postgres:stroppy_postgres@10.0.0.2:5432/postgres?sslmode=disable`,
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
		"existing stroppy:",
		"installed stroppy:",
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
	if strings.Contains(script, "exit 0\nfi\nmkdir -p /usr/local/bin") {
		t.Fatalf("install command exits before refreshing an existing stroppy binary:\n%s", script)
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
		Segments: []*domain.Workload_Segment{{
			Name:   "workload",
			Script: "tpcc/procs",
			Execution: &domain.Workload_Execution{
				Vus:          proto.Uint32(1),
				Limit:        &domain.Workload_Execution_Duration{Duration: "10s"},
				NoThresholds: true,
			},
			Parameters: &domain.Workload_Parameters{
				PoolSize:    2,
				ScaleFactor: 1,
			},
		}},
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

func TestK6Args(t *testing.T) {
	dur := func(d string) *domain.Workload_Execution_Duration {
		return &domain.Workload_Execution_Duration{Duration: d}
	}
	iters := func(n uint32) *domain.Workload_Execution_Iterations {
		return &domain.Workload_Execution_Iterations{Iterations: n}
	}
	cases := []struct {
		name string
		exec *domain.Workload_Execution
		want []string
	}{
		{
			name: "full profile, extra args appended last",
			exec: &domain.Workload_Execution{
				Vus: proto.Uint32(4), Limit: dur("5m"), Quiet: proto.Bool(true),
				NoThresholds: true, ExtraArgs: []string{"--max-duration", "1h"},
			},
			want: []string{"-q", "--vus", "4", "--duration", "5m", "--no-thresholds", "--max-duration", "1h"},
		},
		{
			name: "vus omitted => no --vus",
			exec: &domain.Workload_Execution{Limit: dur("30s")},
			want: []string{"-q", "--duration", "30s"},
		},
		{
			name: "limit omitted => no duration/iterations flag",
			exec: &domain.Workload_Execution{Vus: proto.Uint32(2)},
			want: []string{"-q", "--vus", "2"},
		},
		{
			name: "iterations limit",
			exec: &domain.Workload_Execution{Vus: proto.Uint32(1), Limit: iters(100)},
			want: []string{"-q", "--vus", "1", "--iterations", "100"},
		},
		{
			name: "quiet absent keeps default -q",
			exec: &domain.Workload_Execution{Limit: dur("1m")},
			want: []string{"-q", "--duration", "1m"},
		},
		{
			name: "quiet explicit false drops -q",
			exec: &domain.Workload_Execution{Quiet: proto.Bool(false), Limit: dur("1m")},
			want: []string{"--duration", "1m"},
		},
		{
			name: "only extra args (all managed flags off)",
			exec: &domain.Workload_Execution{Quiet: proto.Bool(false), ExtraArgs: []string{"--max-duration", "24h"}},
			want: []string{"--max-duration", "24h"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := k6Args(tc.exec)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("k6Args = %v, want %v", got, tc.want)
			}
		})
	}
}
