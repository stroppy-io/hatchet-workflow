package provider

import (
	"context"
	"encoding/json"
	"fmt"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// dockerProvider is the builtin Provider implementation: one container per
// requested machine, running the stroppy-agent bootstrap (same env
// convention as the pre-DSL renderDockerInput/agentdomain path), with no
// terraform module involved.
type dockerProvider struct {
	exec dockerExec
}

// NewDocker builds the docker builtin Provider. exec performs the actual
// container lifecycle (see dockerExec for the adaptation gap against the
// real docker.Executor runner).
func NewDocker(exec dockerExec) Provider {
	return &dockerProvider{exec: exec}
}

// dockerParams is the dynamic shape of a docker ProviderRef.ParamsJson: it
// plays the role that terraform's variables.tf/tfvars play for the
// terraform-module provider, since the docker builtin has no module to
// derive a schema from.
type dockerParams struct {
	Image      string            `json:"image"`
	Network    string            `json:"network"`
	ServerAddr string            `json:"server_addr"`
	BinaryURL  string            `json:"binary_url"`
	RunID      string            `json:"run_id"`
	Env        map[string]string `json:"env"`
}

func decodeDockerParams(ref *dslpb.ProviderRef) (dockerParams, error) {
	var params dockerParams
	raw := ref.GetParamsJson()
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return params, fmt.Errorf("decode docker provider params_json: %w", err)
		}
	}
	if params.Image == "" {
		return params, fmt.Errorf("docker provider params: image is required")
	}
	if params.ServerAddr == "" {
		return params, fmt.Errorf("docker provider params: server_addr is required")
	}
	if params.RunID == "" {
		return params, fmt.Errorf("docker provider params: run_id is required")
	}
	if params.Network == "" {
		params.Network = dockerNetworkName(params.RunID)
	}
	return params, nil
}

func (p *dockerProvider) Provision(ctx context.Context, ref *dslpb.ProviderRef, groups []*dslpb.MachineGroup) (map[string][]*deploymentpb.MachineState, error) {
	params, err := decodeDockerParams(ref)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]*deploymentpb.MachineState, len(groups))
	for _, group := range groups {
		for idx := uint32(0); idx < group.GetCount(); idx++ {
			nodeID := fmt.Sprintf("%s-%d", group.GetName(), idx)
			containerName := dockerContainerName(params.RunID, nodeID)

			env, err := agentdomain.Env(nodeID, agentdomain.Bootstrap{
				ServerAddr: params.ServerAddr,
				BinaryURL:  params.BinaryURL,
				RunID:      params.RunID,
				ExtraEnv:   params.Env,
			})
			if err != nil {
				return nil, fmt.Errorf("render agent bootstrap env for %q: %w", nodeID, err)
			}

			spec := ContainerSpec{
				Name:    containerName,
				Image:   params.Image,
				Network: params.Network,
				Env:     env,
				Labels: map[string]string{
					"stroppy.cloud/run_id":  params.RunID,
					"stroppy.cloud/node_id": nodeID,
					"group":                 group.GetName(),
				},
			}

			state, err := p.exec.EnsureContainer(ctx, spec)
			if err != nil {
				return nil, fmt.Errorf("ensure container %q: %w", containerName, err)
			}

			result[group.GetName()] = append(result[group.GetName()], &deploymentpb.MachineState{
				NodeId:             nodeID,
				ProviderResourceId: state.ID,
				Status:             common.Status_STATUS_DEPLOYED,
				Endpoints: []*deploymentpb.Endpoint{
					{
						Name:    "private",
						Address: state.InternalIP,
						Labels:  map[string]string{"scope": "private"},
					},
				},
				Labels: map[string]string{
					"node_id": nodeID,
					"group":   group.GetName(),
				},
			})
		}
	}
	return result, nil
}

func (p *dockerProvider) Destroy(ctx context.Context, ref *dslpb.ProviderRef) error {
	params, err := decodeDockerParams(ref)
	if err != nil {
		return err
	}
	if err := p.exec.RemoveContainers(ctx, params.Network); err != nil {
		return fmt.Errorf("remove docker containers for network %q: %w", params.Network, err)
	}
	return nil
}

func dockerNetworkName(runID string) string {
	return "stroppy-" + runID
}

func dockerContainerName(runID, nodeID string) string {
	return "stroppy-" + runID + "-" + nodeID
}
