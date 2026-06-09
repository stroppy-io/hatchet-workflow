package workflows

import (
	"strings"
	"testing"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func TestRenderDockerInputInjectsAgentBootstrap(t *testing.T) {
	cfg := workflowRunConfig(t, nil)
	firstMachine := cfg.GetInfrastructurePlan().GetMachines()[0]
	cfg.GetAgentBootstrap().AgentTokens = map[string]string{
		firstMachine.GetNodeId(): "agent-token-" + firstMachine.GetNodeId(),
	}
	cfg.GetAgentBootstrap().AgentTaskQueues = map[string]string{
		firstMachine.GetNodeId(): "secret-queue-" + firstMachine.GetNodeId(),
	}
	input, err := renderDockerInput(&workflowpb.RenderDockerInputWorkflowRequest{
		RunId:          cfg.GetId(),
		Plan:           cfg.GetInfrastructurePlan(),
		AgentBootstrap: cfg.GetAgentBootstrap(),
	})
	if err != nil {
		t.Fatalf("render docker input: %v", err)
	}

	machine := cfg.GetInfrastructurePlan().GetMachines()[0]
	container := input.GetContainers()[dockerResourceName(cfg.GetId(), machine.GetNodeId())]
	if container == nil {
		t.Fatalf("container for %q is missing", machine.GetNodeId())
	}
	if got, want := container.GetEnv()["AGENT_TASK_QUEUE"], "secret-queue-"+machine.GetNodeId(); got != want {
		t.Fatalf("AGENT_TASK_QUEUE = %q, want %q", got, want)
	}
	file := dockerFileByPath(container.GetFiles(), agentdomain.DockerEnvFilePath)
	if file == nil {
		t.Fatalf("%s is missing", agentdomain.DockerEnvFilePath)
	}
	content := string(file.GetContent())
	for _, want := range []string{
		"STROPPY_SERVER_ADDR=http://127.0.0.1:8080",
		"STROPPY_AGENT_BINARY_URL=http://127.0.0.1:8080/agent/binary",
		"STROPPY_MACHINE_ID=" + machine.GetNodeId(),
		"STROPPY_AGENT_TOKEN=agent-token-" + machine.GetNodeId(),
		"AGENT_TASK_QUEUE=secret-queue-" + machine.GetNodeId(),
		"TEMPORAL_NAMESPACE=default",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("docker env file missing %q:\n%s", want, content)
		}
	}
	for _, unwanted := range []string{"HTTP_PROXY=", "HTTPS_PROXY=", "http_proxy=", "https_proxy=", "NO_PROXY=", "no_proxy="} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("docker env file leaked proxy env %q:\n%s", unwanted, content)
		}
	}

	aptProxy := dockerFileByPath(container.GetFiles(), agentdomain.DockerAptProxyFilePath)
	if aptProxy == nil {
		t.Fatalf("%s is missing", agentdomain.DockerAptProxyFilePath)
	}
	aptProxyContent := string(aptProxy.GetContent())
	for _, want := range []string{
		`Acquire::http::Proxy "http://127.0.0.1:8080";`,
		`Acquire::https::Proxy "http://127.0.0.1:8080";`,
	} {
		if !strings.Contains(aptProxyContent, want) {
			t.Fatalf("docker apt proxy config missing %q:\n%s", want, aptProxyContent)
		}
	}
}

func TestRenderTerraformInputInjectsYandexCloudInit(t *testing.T) {
	plan := yandexInfrastructurePlan(t)
	bootstrap := testAgentBootstrap()
	bootstrap.AgentTokens = make(map[string]string, len(plan.GetMachines()))
	for _, machine := range plan.GetMachines() {
		bootstrap.AgentTokens[machine.GetNodeId()] = "agent-token-" + machine.GetNodeId()
	}
	bootstrap.AgentTaskQueues = make(map[string]string, len(plan.GetMachines()))
	for _, machine := range plan.GetMachines() {
		bootstrap.AgentTaskQueues[machine.GetNodeId()] = "secret-queue-" + machine.GetNodeId()
	}
	tfInput, err := renderTerraformInput(&workflowpb.RenderTerraformVariablesWorkflowRequest{
		RunId:          "run-1",
		Plan:           plan,
		AgentBootstrap: bootstrap,
	})
	if err != nil {
		t.Fatalf("render terraform input: %v", err)
	}

	values := tfInput.GetTfvars().GetValues().AsMap()
	compute := values["compute"].(map[string]any)
	vms := compute["vms"].(map[string]any)
	for _, machine := range plan.GetMachines() {
		vm := vms[yandexResourceName("run-1", machine.GetNodeId())].(map[string]any)
		userData := vm["user_data"].(string)
		for _, want := range []string{
			"#cloud-config",
			"STROPPY_SERVER_ADDR=http://127.0.0.1:8080",
			"STROPPY_AGENT_BINARY_URL=http://127.0.0.1:8080/agent/binary",
			"STROPPY_MACHINE_ID=" + machine.GetNodeId(),
			"STROPPY_AGENT_TOKEN=agent-token-" + machine.GetNodeId(),
			"AGENT_TASK_QUEUE=secret-queue-" + machine.GetNodeId(),
			`Acquire::http::Proxy "http://127.0.0.1:8080";`,
			`Acquire::https::Proxy "http://127.0.0.1:8080";`,
			"EnvironmentFile=/etc/stroppy/agent.env",
			"ExecStart=/usr/local/bin/stroppy-agent agent",
		} {
			if !strings.Contains(userData, want) {
				t.Fatalf("cloud-init for %q missing %q:\n%s", machine.GetNodeId(), want, userData)
			}
		}
		for _, unwanted := range []string{"HTTP_PROXY=", "HTTPS_PROXY=", "http_proxy=", "https_proxy=", "NO_PROXY=", "no_proxy="} {
			if strings.Contains(userData, unwanted) {
				t.Fatalf("cloud-init for %q leaked proxy env %q:\n%s", machine.GetNodeId(), unwanted, userData)
			}
		}
	}
}

func TestRenderTerraformInputUsesYandexProviderNetworkSettings(t *testing.T) {
	plan := yandexInfrastructurePlan(t)
	settings := plan.GetSettings().GetYandex()
	settings.Zone = deploymentpb.Yandex_Settings_ZONE_RU_CENTRAL1_B
	settings.AssignPublicIp = false
	settings.SoftwareAcceleratedNetwork = true
	for _, machine := range plan.GetMachines() {
		vm := machine.GetYandex()
		vm.Zone = "ru-central1-d"
		vm.InternalIp = "10.0.0.42"
		vm.PublicIp = true
		vm.NetworkAcceleration = "standard"
	}

	tfInput, err := renderTerraformInput(&workflowpb.RenderTerraformVariablesWorkflowRequest{
		RunId:          "run-1",
		Plan:           plan,
		AgentBootstrap: testAgentBootstrap(),
	})
	if err != nil {
		t.Fatalf("render terraform input: %v", err)
	}

	values := tfInput.GetTfvars().GetValues().AsMap()
	compute := values["compute"].(map[string]any)
	vms := compute["vms"].(map[string]any)
	for _, machine := range plan.GetMachines() {
		vm := vms[yandexResourceName("run-1", machine.GetNodeId())].(map[string]any)
		if got, want := vm["zone"], "ru-central1-b"; got != want {
			t.Fatalf("vm zone = %v, want %q", got, want)
		}
		if got, want := vm["internal_ip"], "auto"; got != want {
			t.Fatalf("vm internal_ip = %v, want %q", got, want)
		}
		if got, want := vm["public_ip"], false; got != want {
			t.Fatalf("vm public_ip = %v, want %v", got, want)
		}
		if got, want := vm["network_acceleration"], "software_accelerated"; got != want {
			t.Fatalf("vm network_acceleration = %v, want %q", got, want)
		}
	}
}

func TestRenderTerraformInputUsesReservedNetworkCIDR(t *testing.T) {
	plan := planWithReservedNetworkCIDR(yandexInfrastructurePlan(t), "10.42.0.0/16")
	tfInput, err := renderTerraformInput(&workflowpb.RenderTerraformVariablesWorkflowRequest{
		RunId:          "run-1",
		Plan:           plan,
		AgentBootstrap: testAgentBootstrap(),
	})
	if err != nil {
		t.Fatalf("render terraform input: %v", err)
	}

	values := tfInput.GetTfvars().GetValues().AsMap()
	network := values["network"].(map[string]any)
	if got, want := network["cidr"].(string), "10.42.0.0/16"; got != want {
		t.Fatalf("network cidr = %q, want %q", got, want)
	}
}

func TestRenderTerraformInputNormalizesStandardV3Memory(t *testing.T) {
	plan := yandexInfrastructurePlan(t)
	plan.GetSettings().GetYandex().PlatformId = deploymentpb.Yandex_Settings_PLATFORM_ID_STANDARD_V3
	machine := plan.GetMachines()[0]
	machine.GetYandex().Cores = 8
	machine.GetYandex().MemoryGb = 12

	tfInput, err := renderTerraformInput(&workflowpb.RenderTerraformVariablesWorkflowRequest{
		RunId:          "run-1",
		Plan:           plan,
		AgentBootstrap: testAgentBootstrap(),
	})
	if err != nil {
		t.Fatalf("render terraform input: %v", err)
	}

	values := tfInput.GetTfvars().GetValues().AsMap()
	compute := values["compute"].(map[string]any)
	vms := compute["vms"].(map[string]any)
	vm := vms[yandexResourceName("run-1", machine.GetNodeId())].(map[string]any)
	switch got := vm["memory_gb"].(type) {
	case float64:
		if got != 16 {
			t.Fatalf("memory_gb = %v, want 16", got)
		}
	case uint64:
		if got != 16 {
			t.Fatalf("memory_gb = %v, want 16", got)
		}
	case int:
		if got != 16 {
			t.Fatalf("memory_gb = %v, want 16", got)
		}
	case string:
		if got != "16" {
			t.Fatalf("memory_gb = %v, want 16", got)
		}
	default:
		t.Fatalf("memory_gb has unexpected type %T: %v", got, got)
	}
}

func dockerFileByPath(files []*deploymentpb.Docker_File, path string) *deploymentpb.Docker_File {
	for _, file := range files {
		if file.GetPath() == path {
			return file
		}
	}
	return nil
}

func yandexInfrastructurePlan(t *testing.T) *deploymentpb.InfrastructurePlan {
	t.Helper()

	run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
		ID:       "run-1",
		Database: postgresDatabase(),
		Workload: workload(),
		Provider: deploymentpb.Provider_PROVIDER_YANDEX,
		Infrastructure: infrastructurebuilder.BuildOptions{
			DefaultSizing: infrastructurebuilder.MachineSizing{CPUCores: 2, MemoryMB: 4096, DiskGB: 50},
			Settings: &deploymentpb.ProviderSettings{
				Settings: &deploymentpb.ProviderSettings_Yandex{
					Yandex: &deploymentpb.Yandex_Settings{
						Token:          "token",
						CloudId:        "cloud-id",
						FolderId:       "folder-id",
						Zone:           deploymentpb.Yandex_Settings_ZONE_RU_CENTRAL1_A,
						NetworkId:      "network-id",
						NetworkName:    "stroppy",
						SubnetCidr:     "10.0.0.0/8",
						PlatformId:     deploymentpb.Yandex_Settings_PLATFORM_ID_STANDARD_V2,
						ImageId:        "image-id",
						AssignPublicIp: true,
						SshUser:        "stroppy",
						SshPublicKey:   "ssh-rsa test",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build yandex test run: %v", err)
	}
	return run.GetInfrastructurePlan()
}
