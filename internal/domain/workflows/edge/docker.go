package edge

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	dockerClient "github.com/docker/docker/client"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

const (
	bridgeDriver = "bridge"

	containerMetadataDockerIPKey      = "docker.network.ipv4"
	containerMetadataPlacementNodeKey = "docker.placement.node"
	containerMetadataLogicalNameKey   = "docker.logical_name"

	containerLabelManagedByKey = "managed_by"
	containerLabelManagedByVal = "hatchet-edge"
	containerLabelRunIDKey     = "run_id"
	containerLabelWorkerIPKey  = "worker_ip"
	containerLabelLogicalKey   = "logical_name"

	dockerConfigDirEnvName = "DOCKER_CONFIG"
	dockerConfigFileName   = "config.json"
)

type ContainerRunner struct {
	client      *dockerClient.Client
	networkName string
	networkID   string
	subnet      string
	logger      hatchetLogger
	mu          sync.Mutex
	containers  map[string]string
}

type startedContainer struct {
	name string
	id   string
}

type hatchetLogger interface {
	Log(message string)
}

type runContainerOptions struct {
	dockerTarget       bool
	runID              string
	workerInternal     string
	publishPorts       bool
	primaryContainerID string // when set, share this container's network namespace
}

func NewContainerRunner(networkName string, logger hatchetLogger) (*ContainerRunner, error) {
	cli, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &ContainerRunner{
		client:      cli,
		networkName: networkName,
		logger:      logger,
		containers:  make(map[string]string),
	}, nil
}

func (r *ContainerRunner) DeployContainersForTarget(
	ctx context.Context,
	runSettings *stroppy.RunSettings,
	workerInternalIP string,
	workerInternalCidr string,
	containers []*provision.Container,
) error {
	opts := runContainerOptions{publishPorts: true}
	if runSettings.GetTarget() == deployment.Target_TARGET_DOCKER {
		if err := r.setNetworkName(runSettings.GetSettings().GetDocker().GetNetworkName()); err != nil {
			return err
		}
		r.setSubnet(workerInternalCidr)
		opts = runContainerOptions{
			dockerTarget:   true,
			runID:          runSettings.GetRunId(),
			workerInternal: workerInternalIP,
			publishPorts:   false,
		}
	}
	return r.deployContainers(ctx, containers, opts)
}

func (r *ContainerRunner) deployContainers(
	ctx context.Context,
	containers []*provision.Container,
	opts runContainerOptions,
) error {
	started := make([]startedContainer, 0, len(containers))

	for i, c := range containers {
		if c == nil {
			return fmt.Errorf("container spec is nil")
		}

		runOpts := opts
		if i > 0 && opts.dockerTarget && len(started) > 0 {
			runOpts.primaryContainerID = started[0].id
		}

		sc, err := r.runContainer(ctx, c, runOpts)
		if err != nil {
			r.rollbackBatch(ctx, started)
			return fmt.Errorf("failed to run container %q: %w", c.GetName(), err)
		}

		r.mu.Lock()
		r.containers[sc.name] = sc.id
		r.mu.Unlock()
		started = append(started, sc)
	}

	return nil
}

func (r *ContainerRunner) Cleanup(ctx context.Context) {
	r.mu.Lock()
	tracked := make(map[string]string, len(r.containers))
	for name, id := range r.containers {
		tracked[name] = id
	}
	r.mu.Unlock()

	for name, id := range tracked {
		if err := r.stopContainer(ctx, id); err != nil {
			r.logf("failed to cleanup container %s (%s): %v", name, id, err)
			continue
		}

		r.mu.Lock()
		delete(r.containers, name)
		r.mu.Unlock()
	}
}

func (r *ContainerRunner) Close() error {
	return r.client.Close()
}

func (r *ContainerRunner) runContainer(
	ctx context.Context,
	c *provision.Container,
	opts runContainerOptions,
) (startedContainer, error) {
	if err := r.ensureNetwork(ctx); err != nil {
		return startedContainer{}, err
	}

	if err := r.pullImage(ctx, c.GetImage()); err != nil {
		return startedContainer{}, fmt.Errorf("failed to pull image %q: %w", c.GetImage(), err)
	}

	containerCfg := toContainerConfig(c, opts)
	hostCfg, err := toHostConfig(c, opts)
	if err != nil {
		return startedContainer{}, fmt.Errorf("failed to map host config for container %q: %w", c.GetName(), err)
	}
	containerName := containerRuntimeName(c, opts)

	// When sharing a primary container's network namespace, skip network config
	// entirely — Docker rejects EndpointsConfig for container-mode networking.
	var networkCfg *network.NetworkingConfig
	if opts.primaryContainerID == "" {
		networkCfg = toNetworkConfig(r.getNetworkName(), r.getNetworkID(), c, opts)
	}

	resp, err := r.client.ContainerCreate(
		ctx,
		containerCfg,
		hostCfg,
		networkCfg,
		nil,
		containerName,
	)
	if err != nil {
		return startedContainer{}, fmt.Errorf("failed to create container %q: %w", containerName, err)
	}

	if err := r.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = r.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return startedContainer{}, fmt.Errorf("failed to start container %q: %w", containerName, err)
	}

	return startedContainer{name: containerName, id: resp.ID}, nil
}

func (r *ContainerRunner) ensureNetwork(ctx context.Context) error {
	r.mu.Lock()
	if r.networkID != "" {
		r.mu.Unlock()
		return nil
	}
	networkName := r.networkName
	subnet := r.subnet
	r.mu.Unlock()

	inspect, err := r.client.NetworkInspect(ctx, networkName, network.InspectOptions{})
	if err == nil {
		r.setNetworkID(inspect.ID)
		return nil
	}

	createOpts := network.CreateOptions{Driver: bridgeDriver}
	if subnet != "" {
		createOpts.IPAM = &network.IPAM{
			Config: []network.IPAMConfig{
				{Subnet: subnet},
			},
		}
	}

	resp, createErr := r.client.NetworkCreate(ctx, networkName, createOpts)
	if createErr != nil {
		inspect, inspectErr := r.client.NetworkInspect(ctx, networkName, network.InspectOptions{})
		if inspectErr == nil {
			r.setNetworkID(inspect.ID)
			return nil
		}
		return fmt.Errorf("failed to create docker network %s: %w", networkName, createErr)
	}

	r.setNetworkID(resp.ID)
	return nil
}

func (r *ContainerRunner) pullImage(ctx context.Context, imageName string) error {
	pullOptions := image.PullOptions{}
	if registryAuth, ok := registryAuthForImage(imageName); ok {
		pullOptions.RegistryAuth = registryAuth
		r.logf("docker image pull %q: registry auth enabled", imageName)
	} else {
		r.logf("docker image pull %q: registry auth not found, using anonymous pull", imageName)
	}

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 30 * time.Second
	b.MaxElapsedTime = 5 * time.Minute

	return backoff.Retry(func() error {
		reader, err := r.client.ImagePull(ctx, imageName, pullOptions)
		if err != nil {
			r.logf("docker image pull %q failed: %v", imageName, err)
			return err
		}
		defer reader.Close()

		if err := r.logDockerStream(fmt.Sprintf("docker image pull %q", imageName), reader); err != nil {
			r.logf("docker image pull %q stream error: %v", imageName, err)
			return err
		}
		return nil
	}, backoff.WithContext(b, ctx))
}

type dockerConfig struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

type dockerAuthEntry struct {
	Auth          string `json:"auth"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	IdentityToken string `json:"identitytoken"`
}

func registryAuthForImage(imageName string) (string, bool) {
	configPath, ok := resolveDockerConfigPath()
	if !ok {
		return "", false
	}

	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		return "", false
	}

	var cfg dockerConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return "", false
	}

	entry, ok := findAuthEntry(cfg.Auths, imageName)
	if !ok {
		return "", false
	}

	authCfg, ok := toRegistryAuthConfig(entry)
	if !ok {
		return "", false
	}

	encoded, err := encodeAuthConfig(authCfg)
	if err != nil {
		return "", false
	}
	return encoded, true
}

func resolveDockerConfigPath() (string, bool) {
	configDir := os.Getenv(dockerConfigDirEnvName)
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		configDir = filepath.Join(homeDir, ".docker")
	}

	configPath := filepath.Join(configDir, dockerConfigFileName)
	if _, err := os.Stat(configPath); err != nil {
		return "", false
	}
	return configPath, true
}

func findAuthEntry(auths map[string]dockerAuthEntry, imageName string) (dockerAuthEntry, bool) {
	if len(auths) == 0 {
		return dockerAuthEntry{}, false
	}

	registryHost := normalizeRegistryHost(registryHostForImage(imageName))
	for key, entry := range auths {
		if normalizeRegistryHost(key) == registryHost {
			return entry, true
		}
	}

	if registryHost == "docker.io" {
		if entry, ok := auths["https://index.docker.io/v1/"]; ok {
			return entry, true
		}
	}

	return dockerAuthEntry{}, false
}

func registryHostForImage(imageName string) string {
	parts := strings.Split(imageName, "/")
	if len(parts) == 0 {
		return "docker.io"
	}

	first := parts[0]
	if !strings.Contains(first, ".") && !strings.Contains(first, ":") && first != "localhost" {
		return "docker.io"
	}
	return first
}

func normalizeRegistryHost(host string) string {
	normalized := strings.TrimSpace(strings.ToLower(host))
	normalized = strings.TrimPrefix(normalized, "https://")
	normalized = strings.TrimPrefix(normalized, "http://")
	normalized = strings.TrimSuffix(normalized, "/")
	if idx := strings.IndexByte(normalized, '/'); idx >= 0 {
		normalized = normalized[:idx]
	}
	if normalized == "index.docker.io" || normalized == "registry-1.docker.io" {
		return "docker.io"
	}
	return normalized
}

func toRegistryAuthConfig(entry dockerAuthEntry) (registry.AuthConfig, bool) {
	authCfg := registry.AuthConfig{
		Username:      entry.Username,
		Password:      entry.Password,
		IdentityToken: entry.IdentityToken,
	}
	if authCfg.Username != "" || authCfg.Password != "" || authCfg.IdentityToken != "" {
		return authCfg, true
	}

	if entry.Auth == "" {
		return registry.AuthConfig{}, false
	}

	decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return registry.AuthConfig{}, false
		}
	}
	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 {
		return registry.AuthConfig{}, false
	}
	return registry.AuthConfig{
		Username: credentials[0],
		Password: credentials[1],
	}, true
}

func encodeAuthConfig(cfg registry.AuthConfig) (string, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(raw), nil
}

type dockerStreamMessage struct {
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func (r *ContainerRunner) logDockerStream(prefix string, stream io.Reader) error {
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		r.logf("%s: %s", prefix, line)

		var msg dockerStreamMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ErrorDetail.Message != "" {
			return fmt.Errorf("%s", msg.ErrorDetail.Message)
		}
		if msg.Error != "" {
			return fmt.Errorf("%s", msg.Error)
		}
	}
	return scanner.Err()
}

func (r *ContainerRunner) stopContainer(ctx context.Context, containerID string) error {
	timeoutSeconds := 10
	stopErr := r.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSeconds})
	removeErr := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	if stopErr != nil && removeErr != nil {
		return fmt.Errorf(
			"failed to stop container %s: %w; failed to remove container %s: %v",
			containerID,
			stopErr,
			containerID,
			removeErr,
		)
	}
	if stopErr != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerID, stopErr)
	}
	if removeErr != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerID, removeErr)
	}
	return nil
}

func (r *ContainerRunner) rollbackBatch(ctx context.Context, started []startedContainer) {
	for i := len(started) - 1; i >= 0; i-- {
		name := started[i].name
		containerID := started[i].id

		if err := r.stopContainer(ctx, containerID); err != nil {
			r.logf("failed to rollback container %s (%s): %v", name, containerID, err)
			continue
		}

		r.mu.Lock()
		delete(r.containers, name)
		r.mu.Unlock()
	}
}

func (r *ContainerRunner) logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if r.logger != nil {
		r.logger.Log(msg)
	}
	logger.StdLog().Printf(format, args...)
}

func (r *ContainerRunner) getNetworkID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.networkID
}

func (r *ContainerRunner) getNetworkName() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.networkName
}

func (r *ContainerRunner) setNetworkID(networkID string) {
	r.mu.Lock()
	r.networkID = networkID
	r.mu.Unlock()
}

func (r *ContainerRunner) setSubnet(subnet string) {
	r.mu.Lock()
	r.subnet = subnet
	r.mu.Unlock()
}

func (r *ContainerRunner) setNetworkName(networkName string) error {
	if networkName == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.networkID != "" && r.networkName != networkName {
		return fmt.Errorf("docker network is already initialized as %q", r.networkName)
	}
	r.networkName = networkName
	return nil
}

func containerRuntimeName(c *provision.Container, opts runContainerOptions) string {
	base := containerLogicalName(c)
	if !opts.dockerTarget {
		return base
	}

	parts := []string{sanitizeDockerNamePart(opts.runID), sanitizeDockerNamePart(opts.workerInternal), sanitizeDockerNamePart(base)}
	return strings.Trim(strings.Join(parts, "-"), "-")
}

func containerLogicalName(c *provision.Container) string {
	if c.GetName() != "" {
		return c.GetName()
	}
	if c.GetId() != "" {
		return c.GetId()
	}
	return "container"
}

func sanitizeDockerNamePart(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteByte(ch + ('a' - 'A'))
		case ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case ch == '.' || ch == '-' || ch == '_':
			b.WriteByte(ch)
		default:
			b.WriteByte('-')
		}
	}

	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "c"
	}
	return out
}
