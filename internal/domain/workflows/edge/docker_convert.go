package edge

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
)

func toContainerConfig(c *provision.Container, opts runContainerOptions) *container.Config {
	cfg := &container.Config{
		Image: c.GetImage(),
	}

	if command := c.GetCommand(); len(command) > 0 {
		cfg.Entrypoint = command
	}
	if args := c.GetArgs(); len(args) > 0 {
		cfg.Cmd = args
	}

	env := c.GetEnv()
	if len(env) > 0 {
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		cfg.Env = make([]string, 0, len(env))
		for _, k := range keys {
			cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", k, env[k]))
		}
	}

	exposed := nat.PortSet{}
	for _, p := range c.GetPorts() {
		if p == nil || p.GetContainerPort() == 0 {
			continue
		}
		dockerPort := nat.Port(fmt.Sprintf("%d/tcp", p.GetContainerPort()))
		exposed[dockerPort] = struct{}{}
	}
	if len(exposed) > 0 {
		cfg.ExposedPorts = exposed
	}

	if opts.dockerTarget {
		logicalName := c.GetMetadata()[containerMetadataLogicalNameKey]
		if logicalName == "" {
			logicalName = containerLogicalName(c)
		}
		cfg.Labels = map[string]string{
			containerLabelManagedByKey: containerLabelManagedByVal,
			containerLabelRunIDKey:     opts.runID,
			containerLabelWorkerIPKey:  opts.workerInternal,
			containerLabelLogicalKey:   logicalName,
		}
	}

	return cfg
}

func toHostConfig(c *provision.Container, opts runContainerOptions) (*container.HostConfig, error) {
	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name:              container.RestartPolicyOnFailure,
			MaximumRetryCount: 3,
		},
	}

	if opts.primaryContainerID != "" {
		hostCfg.NetworkMode = container.NetworkMode("container:" + opts.primaryContainerID)
	}

	if opts.publishPorts {
		if ports := c.GetPorts(); len(ports) > 0 {
			bindings := nat.PortMap{}
			for _, p := range ports {
				if p == nil || p.GetContainerPort() == 0 {
					continue
				}
				dockerPort := nat.Port(fmt.Sprintf("%d/tcp", p.GetContainerPort()))
				hostPort := ""
				if p.HostPort != nil {
					hostPort = strconv.FormatUint(uint64(p.GetHostPort()), 10)
				}
				bindings[dockerPort] = append(bindings[dockerPort], nat.PortBinding{HostPort: hostPort})
			}
			if len(bindings) > 0 {
				hostCfg.PortBindings = bindings
			}
		}
	}

	if volumes := c.GetVolumes(); len(volumes) > 0 {
		hostCfg.Mounts = make([]mount.Mount, 0, len(volumes))
		for _, spec := range volumes {
			mnt, err := parseBindMount(spec)
			if err != nil {
				return nil, err
			}
			hostCfg.Mounts = append(hostCfg.Mounts, mnt)
		}
	}

	return hostCfg, nil
}

func toNetworkConfig(networkName, networkID string, c *provision.Container, opts runContainerOptions) *network.NetworkingConfig {
	endpoint := &network.EndpointSettings{NetworkID: networkID}
	if opts.dockerTarget {
		if ip := strings.TrimSpace(c.GetMetadata()[containerMetadataDockerIPKey]); ip != "" {
			endpoint.IPAMConfig = &network.EndpointIPAMConfig{
				IPv4Address: ip,
			}
		}
	}

	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: endpoint,
		},
	}
}

func parseBindMount(spec string) (mount.Mount, error) {
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return mount.Mount{}, fmt.Errorf("invalid volume format %q, expected source:target[:ro]", spec)
	}

	source := strings.TrimSpace(parts[0])
	target := strings.TrimSpace(parts[1])
	if source == "" || target == "" {
		return mount.Mount{}, fmt.Errorf("invalid volume format %q, source and target are required", spec)
	}

	readOnly := false
	if len(parts) == 3 {
		flag := strings.TrimSpace(parts[2])
		switch flag {
		case "", "rw":
			readOnly = false
		case "ro":
			readOnly = true
		default:
			return mount.Mount{}, fmt.Errorf("invalid volume mode %q in %q, expected ro or rw", flag, spec)
		}
	}

	return mount.Mount{
		Type:     mount.TypeBind,
		Source:   source,
		Target:   target,
		ReadOnly: readOnly,
	}, nil
}
