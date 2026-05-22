package dag

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

// dockerAgentImage is the local-Docker host image: systemd-in-docker with the agent
// (auto-downloaded + started by its systemd unit), stroppy, and the metric exporters
// baked in (deployments/docker/agent.Dockerfile → `stroppy-agent:latest`). systemdImage
// is the bare fallback used only when the agent image is absent (no agent → the run
// cannot progress past deploy, so build the agent image for real runs).
const (
	dockerAgentImage = "stroppy-agent:latest"
	systemdImage     = "jrei/systemd-ubuntu:22.04"
)

// MaterializeDeploymentIntent derives the provider-specific DeploymentIntent from a
// TestPreset's topology — one Spec per machine (Wizard step 5). It is pure: no IPs,
// tokens or cloud-init (those are resolved at provision time). Empty/unknown
// provider yields an error so the Wizard can surface a diagnostic.
func MaterializeDeploymentIntent(preset *domain.TestPreset, provider deployment.Provider) (*deployment.DeploymentIntent, error) {
	machines := preset.GetTopology().GetMachines()
	specs := make([]*deployment.DeploymentIntent_Spec, 0, len(machines))
	for _, m := range machines {
		spec := &deployment.DeploymentIntent_Spec{Id: m.GetId()}
		switch provider {
		case deployment.Provider_PROVIDER_DOCKER:
			spec.Spec = &deployment.DeploymentIntent_Spec_DockerContainer{DockerContainer: &deployment.Docker_Container{
				Image:         dockerAgentImage,
				FallbackImage: systemdImage,
				Privileged:    true,   // systemd-in-docker
				CgroupnsMode:  "host", // systemd-in-docker
			}}
		case deployment.Provider_PROVIDER_YANDEX:
			spec.Spec = &deployment.DeploymentIntent_Spec_YandexVm{YandexVm: &deployment.Yandex_Vm{
				Cores:      m.GetCores(),
				MemoryGb:   m.GetMemoryGb(),
				BootDiskGb: m.GetDiskGb(),
			}}
		default:
			return nil, fmt.Errorf("cannot materialize deployment: unsupported provider %s", provider)
		}
		specs = append(specs, spec)
	}
	return &deployment.DeploymentIntent{Provider: provider, Specs: specs}, nil
}

// CompileTestPresetPreview compiles the static execution DAG BLUEPRINT for a
// TestPreset (Wizard step 6 / Review). It builds the same dag SubmitTestRun would,
// but with a PREVIEW deployment (no IPs/tokens — just the provider-shaped oneof) and
// no provisioner registration — it never runs, persists or schedules anything. Use it
// purely to render nodes/edges/teardown/parallelism.
func CompileTestPresetPreview(preset *domain.TestPreset) *primitive.Dag {
	dep := BuildDeployment(preset, &system.Network{}, nil)
	return BuildTestDag(preset, dep, Deps{Install: RecipeInstallBuilder{}})
}
