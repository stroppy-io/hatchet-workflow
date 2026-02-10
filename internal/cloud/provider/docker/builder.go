package docker

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

// DockerContainerConfig is serialized as JSON into Resource.ResourceYaml
// and used by the Docker DeploymentService to create containers.
type DockerContainerConfig struct {
	ImageName     string            `json:"image_name"`
	ContainerName string            `json:"container_name"`
	NetworkName   string            `json:"network_name"`
	Env           map[string]string `json:"env"`
}

type CloudBuilder struct {
	Config *ProviderConfig
}

func NewCloudBuilder(config *ProviderConfig) *CloudBuilder {
	return &CloudBuilder{Config: config}
}

func (b *CloudBuilder) BuildNetwork(
	template *crossplane.Network_Template,
) (*crossplane.Network, error) {
	networkConfig := map[string]string{
		"network_name": b.Config.NetworkName,
		"driver":       "bridge",
	}
	configJSON, err := json.Marshal(networkConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal docker network config: %w", err)
	}
	ref := &crossplane.Ref{
		Name:      template.GetIdentifier().GetName(),
		Namespace: "docker",
	}
	return &crossplane.Network{
		Template: template,
		Resource: &crossplane.Resource{
			Ref: &crossplane.ExtRef{
				Ref:        ref,
				ApiVersion: ApiVersion,
				Kind:       KindDockerNetwork,
			},
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: string(configJSON),
			Status:       crossplane.Resource_STATUS_CREATING,
		},
	}, nil
}

func (b *CloudBuilder) BuildSubnet(
	_ *crossplane.Network,
	template *crossplane.Subnet_Template,
) (*crossplane.Subnet, error) {
	subnetConfig := map[string]string{
		"type": "docker-managed",
	}
	configJSON, err := json.Marshal(subnetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal docker subnet config: %w", err)
	}
	ref := &crossplane.Ref{
		Name:      template.GetIdentifier().GetName(),
		Namespace: "docker",
	}
	return &crossplane.Subnet{
		Template: template,
		Resource: &crossplane.Resource{
			Ref: &crossplane.ExtRef{
				Ref:        ref,
				ApiVersion: ApiVersion,
				Kind:       KindDockerSubnet,
			},
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: string(configJSON),
			Status:       crossplane.Resource_STATUS_CREATING,
		},
	}, nil
}

func (b *CloudBuilder) BuildVm(
	_ *crossplane.Network,
	_ *crossplane.Subnet,
	template *crossplane.Vm_Template,
) (*crossplane.Vm, error) {
	env := extractEnvFromCloudInit(template.GetCloudInit())

	containerConfig := &DockerContainerConfig{
		ImageName:     b.Config.EdgeWorkerImage,
		ContainerName: template.GetIdentifier().GetName(),
		NetworkName:   b.Config.NetworkName,
		Env:           env,
	}
	configJSON, err := json.Marshal(containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal docker container config: %w", err)
	}
	ref := &crossplane.Ref{
		Name:      template.GetIdentifier().GetName(),
		Namespace: "docker",
	}
	return &crossplane.Vm{
		Template: template,
		Resource: &crossplane.Resource{
			Ref: &crossplane.ExtRef{
				Ref:        ref,
				ApiVersion: ApiVersion,
				Kind:       KindDockerContainer,
			},
			ResourceYaml: string(configJSON),
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			Status:       crossplane.Resource_STATUS_CREATING,
			UsingQuotas: []*crossplane.Quota{
				{
					Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_DOCKER,
					Kind:    crossplane.Quota_KIND_VM,
					Current: 1,
				},
			},
		},
	}, nil
}

// extractEnvFromCloudInit parses environment variables from the cloud-init runcmd entries.
// The install-edge-worker.sh cloud-init command has the format:
//
//	["bash", "-c", "echo '<base64>' | base64 -d > ... && ... KEY='VALUE' KEY2='VALUE2'"]
//
// We extract KEY='VALUE' pairs from the last part of the bash command.
func extractEnvFromCloudInit(ci *crossplane.CloudInit) map[string]string {
	env := make(map[string]string)
	if ci == nil {
		return env
	}
	for _, cmd := range ci.GetRuncmd() {
		values := cmd.GetValues()
		if len(values) < 3 || values[0] != "bash" || values[1] != "-c" {
			continue
		}
		cmdStr := values[2]
		parseShellEnvVars(cmdStr, env)
	}
	return env
}

// parseShellEnvVars extracts KEY='VALUE' pairs from a shell command string.
// The format from InstallEdgeWorkerCloudInitCmd is: KEY='value' (single-quoted).
func parseShellEnvVars(cmdStr string, env map[string]string) {
	// Find environment variable assignments after the script path.
	// They appear as: KEY='value' separated by spaces.
	// The script invocation ends with the script path followed by KEY='value' pairs.
	idx := strings.LastIndex(cmdStr, "&&")
	if idx < 0 {
		return
	}
	tail := cmdStr[idx+2:]
	// tail looks like: ' '/tmp/install-edge-worker.sh' KEY1='val1' KEY2='val2'
	parts := splitShellArgs(tail)
	for _, part := range parts {
		eqIdx := strings.Index(part, "=")
		if eqIdx <= 0 || eqIdx >= len(part)-1 {
			continue
		}
		key := part[:eqIdx]
		value := part[eqIdx+1:]
		// Keys should be uppercase identifiers
		if !isEnvKey(key) {
			continue
		}
		// Remove surrounding single quotes from value
		value = strings.Trim(value, "'")
		env[key] = value
	}
}

// splitShellArgs naively splits a string by spaces, respecting single-quoted segments.
func splitShellArgs(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inQuote:
			inQuote = true
			current.WriteByte(ch)
		case ch == '\'' && inQuote:
			inQuote = false
			current.WriteByte(ch)
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func isEnvKey(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

// NewUniqueContainerName generates a unique container name using an identifier and ULID.
func NewUniqueContainerName(baseName string) string {
	return fmt.Sprintf("%s-%s", baseName, ids.NewUlid().Lower().String())
}
