package run

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"go.uber.org/zap"

	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraformprov"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

// dbConnectPort returns the port stroppy connects to for a given (kind,
// protocol). Single source of truth — the protocol registry decides. When
// the run config didn't pin a protocol, falls back to the kind's default.
func dbConnectPort(kind types.DatabaseKind, protocol types.Protocol) int {
	if protocol == "" {
		protocol = types.DefaultProtocol(kind)
	}
	if meta, ok := types.Protocols[protocol]; ok {
		return meta.Port
	}
	return 0
}

// networkTask creates a Docker network (for docker provider).
type networkTask struct {
	cfg      types.NetworkConfig
	provider types.Provider
	deployer *agent.DockerDeployer
	state    *State
	runID    string
}

func (t *networkTask) Execute(nc *NodeContext) error {
	switch t.provider {
	case types.ProviderDocker:
		return t.dockerNetwork(nc)
	case types.ProviderYandex:
		return t.yandexNetwork(nc)
	default:
		nc.Log().Info("network phase: skipping (handled by terraform)", zap.String("provider", string(t.provider)))
		return nil
	}
}

func (t *networkTask) yandexNetwork(nc *NodeContext) error {
	// VPC/subnet creation is handled as part of the machines terraform apply.
	// Log the intent for observability; no error so the pipeline proceeds.
	nc.Log().Info("network phase: Yandex Cloud VPC/subnet will be provisioned by terraform in machines phase",
		zap.String("cidr", t.cfg.CIDR),
		zap.String("zone", t.cfg.Zone),
	)
	return nil
}

func (t *networkTask) dockerNetwork(nc *NodeContext) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("network: docker client: %w", err)
	}
	defer cli.Close()

	netName := fmt.Sprintf("stroppy-%s", t.runID)
	nc.Log().Info("creating docker network", zap.String("name", netName))

	// Reuse if already exists.
	nets, err := cli.NetworkList(nc, network.ListOptions{})
	if err == nil {
		for _, n := range nets {
			if n.Name == netName {
				t.state.SetNetworkID(n.ID)
				nc.Log().Info("docker network already exists, reusing", zap.String("id", n.ID))
				return nil
			}
		}
	}

	resp, err := cli.NetworkCreate(nc, netName, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{"stroppy": "true"},
	})
	if err != nil {
		return fmt.Errorf("network: docker create: %w", err)
	}

	t.state.SetNetworkID(resp.ID)
	nc.Log().Info("docker network created", zap.String("id", resp.ID))

	return nil
}

// machinesTask provisions containers (docker) or VMs (cloud).
// On completion it populates State with agent targets.
type machinesTask struct {
	runCfg     types.RunConfig
	state      *State
	deployer   *agent.DockerDeployer
	serverAddr string
	settings   *types.ServerSettings
	jwtIssuer  *auth.JWTIssuer
	tenantID   string
}

func (t *machinesTask) Execute(nc *NodeContext) error {
	switch t.runCfg.Provider {
	case types.ProviderDocker:
		return t.dockerMachines(nc)
	case types.ProviderYandex:
		return t.yandexMachines(nc)
	default:
		return fmt.Errorf("machines: unsupported provider %q (supported: %s, %s)", t.runCfg.Provider, types.ProviderDocker, types.ProviderYandex)
	}
}

func (t *machinesTask) dockerMachines(nc *NodeContext) error {
	if t.deployer == nil {
		return fmt.Errorf("machines: DockerDeployer is nil")
	}

	ctx := context.Context(nc)
	var dbTargets []agent.Target
	var proxyTargets []agent.Target
	var ydbStorageTargets []agent.Target
	var ydbDatabaseTargets []agent.Target

	// Deploy database machines.
	for _, spec := range t.runCfg.Machines {
		zones := placementZones(spec.Placement)
		for i := range spec.Count {
			machineID := fmt.Sprintf("%s-%s-%d", t.runCfg.ID, spec.Role, i)
			port := agent.DefaultAgentPort

			nc.Log().Info("deploying container",
				zap.String("machine_id", machineID),
				zap.String("role", string(spec.Role)),
			)

			// Generate agent JWT token (valid for 24h).
			agentToken := ""
			if t.jwtIssuer != nil {
				token, err := t.jwtIssuer.Issue(auth.Claims{
					UserID:   machineID,
					Username: machineID,
					TenantID: t.tenantID,
					Role:     "operator",
				}, 24*time.Hour)
				if err == nil {
					agentToken = token
				}
			}

			result, err := t.deployer.Deploy(ctx, machineID, t.serverAddr, agentToken, port)
			if err != nil {
				return fmt.Errorf("machines: deploy %s: %w", machineID, err)
			}
			t.state.AddContainerID(result.ContainerID)

			// Agent polls server for commands — no inbound port needed.
			// InternalHost (container name) is used for inter-container communication
			// (e.g., stroppy connecting to the DB container).
			target := agent.Target{
				ID:           machineID,
				InternalHost: result.ContainerName,
				Zone:         zoneForInstance(spec.Placement, zones, i),
			}

			switch spec.Role {
			case types.RoleDatabase, types.RoleYDBStorage, types.RoleYDBDatabase:
				dbTargets = append(dbTargets, target)
				if spec.Role == types.RoleYDBStorage {
					ydbStorageTargets = append(ydbStorageTargets, target)
				}
				if spec.Role == types.RoleYDBDatabase {
					ydbDatabaseTargets = append(ydbDatabaseTargets, target)
				}
				if len(dbTargets) == 1 {
					// First DB target is master -- store for stroppy to
					// connect. Port comes from the protocol registry so
					// switching protocols (e.g. ydb-grpc → ydb-pgwire)
					// flips the port automatically.
					dbPort := dbConnectPort(t.runCfg.Database.Kind, t.runCfg.Stroppy.Protocol)
					t.state.SetDBEndpoint(result.ContainerName, dbPort)
				}
			case types.RoleProxy:
				proxyTargets = append(proxyTargets, target)
			case types.RoleStroppy:
				t.state.SetStroppyTarget(target)
			}

			nc.Log().Info("container deployed",
				zap.String("machine_id", machineID),
				zap.String("container_id", result.ContainerID[:12]),
			)
		}
	}

	t.state.SetDBTargets(dbTargets)
	t.state.SetProxyTargets(proxyTargets)
	t.state.SetYDBStorageTargets(ydbStorageTargets)
	t.state.SetYDBDatabaseTargets(ydbDatabaseTargets)

	// If proxy is present, stroppy connects through proxy instead of directly to DB.
	// Exception: Picodata — picodata-go driver does topology discovery and needs direct connection.
	if len(proxyTargets) > 0 && t.runCfg.Database.Kind != types.DatabasePicodata {
		proxyHost := proxyTargets[0].InternalHost
		if proxyHost == "" {
			proxyHost = proxyTargets[0].Host
		}
		switch t.runCfg.Database.Kind {
		case types.DatabasePostgres:
			t.state.SetDBEndpoint(proxyHost, 5000) // HAProxy write port
		case types.DatabaseMySQL, types.DatabaseMariaDB:
			t.state.SetDBEndpoint(proxyHost, 6033) // ProxySQL client port
		case types.DatabaseYDB:
			t.state.SetDBEndpoint(proxyHost, 2136)
		}
	}

	nc.Log().Info("all machines provisioned",
		zap.Int("db", len(dbTargets)),
	)
	return nil
}

// runSubnetCIDR derives a unique /16 CIDR from the run ID.
// Given a base like "10.0.0.0/8", it hashes the runID to pick the second octet (1-254),
// producing e.g. "10.42.0.0/16". This avoids collisions between concurrent runs.
func runSubnetCIDR(runID string, _ string) string {
	var h byte
	for _, b := range []byte(runID) {
		h = h*31 + b
	}
	octet := int(h%254) + 1 // 1..254
	return fmt.Sprintf("10.%d.0.0/16", octet)
}

func runSubnetCIDRs(runID string, zones []string) map[string]*deployment.Yandex_Subnet {
	var h byte
	for _, b := range []byte(runID) {
		h = h*31 + b
	}
	octet := int(h%254) + 1
	out := make(map[string]*deployment.Yandex_Subnet, len(zones))
	for i, zone := range zones {
		out[zone] = &deployment.Yandex_Subnet{
			Zone: zone,
			Cidr: fmt.Sprintf("10.%d.%d.0/24", octet, i),
		}
	}
	return out
}

// managedYdbInput builds the deployment.Yandex_ManagedYdb message (the
// managed_ydb tfvar) from the run's ydb-managed topology. Only set when the run
// targets Managed YDB; nil otherwise so the single terraform root skips the
// managed resources entirely.
func managedYdbInput(m *types.YDBManagedTopology, yc types.YandexCloudSettings, runID string) *deployment.Yandex_ManagedYdb {
	// Managed YDB DB name must be unique within the folder; a stable
	// runID-derived suffix keeps tear-down/re-apply idempotent.
	dbName := fmt.Sprintf("stroppy-%s", strings.ReplaceAll(runID, "_", "-"))
	if len(dbName) > 63 {
		dbName = dbName[:63]
	}
	// YC expects a zone-less location ("ru-central1") for managed YDB; yc.Zone
	// is the per-VM zone ("ru-central1-b"), so trim the trailing zone suffix.
	locationID := yc.Zone
	if i := strings.LastIndex(locationID, "-"); i > 0 && len(locationID)-i <= 3 {
		locationID = locationID[:i]
	}

	out := &deployment.Yandex_ManagedYdb{Name: dbName, FolderId: yc.FolderID, LocationId: locationID}

	mgType := string(m.Type)
	if mgType == "" {
		mgType = string(types.YDBManagedKindServerless)
	}
	if mgType == string(types.YDBManagedKindDedicated) {
		groups := uint32(m.StorageGroups)
		if groups < 1 {
			groups = 1
		}
		storageType := m.StorageType
		if storageType == "" {
			storageType = "ssd"
		}
		ded := &deployment.Yandex_ManagedYdb_Dedicated{
			ResourcePresetId: m.ResourcePresetID,
			StorageConfig: &deployment.Yandex_ManagedYdb_StorageConfig{
				GroupCount: groups, StorageTypeId: storageType,
			},
		}
		if m.AutoScale != nil {
			pct := m.AutoScale.CPUUtilizationPct
			if pct <= 0 {
				pct = 70
			}
			ded.ScalePolicy = &deployment.Yandex_ManagedYdb_ScalePolicy{
				Policy: &deployment.Yandex_ManagedYdb_ScalePolicy_Auto_{Auto: &deployment.Yandex_ManagedYdb_ScalePolicy_Auto{
					MinSize: uint32(m.AutoScale.MinSize), MaxSize: uint32(m.AutoScale.MaxSize), CpuUtilizationPercent: uint32(pct),
				}},
			}
		} else {
			size := uint32(m.NodeCount)
			if size < 1 {
				size = 1
			}
			ded.ScalePolicy = &deployment.Yandex_ManagedYdb_ScalePolicy{
				Policy: &deployment.Yandex_ManagedYdb_ScalePolicy_Fixed_{Fixed: &deployment.Yandex_ManagedYdb_ScalePolicy_Fixed{Size: size}},
			}
		}
		out.Kind = &deployment.Yandex_ManagedYdb_Dedicated_{Dedicated: ded}
	} else {
		sl := &deployment.Yandex_ManagedYdb_Serverless{}
		if m.ThrottlingRCUs > 0 {
			sl.ThrottlingRcuLimit = uint32(m.ThrottlingRCUs)
		}
		out.Kind = &deployment.Yandex_ManagedYdb_Serverless_{Serverless: sl}
	}
	return out
}

// yandexMachines provisions the run's Yandex Cloud infrastructure through the
// single terraform root (deployments/terraform/yandex). It builds the typed
// deployment.Yandex.Input (network + compute VMs + optional managed YDB) — which
// IS the terraform tfvars — applies it via the provisioner, then reads the
// resulting VM and managed-YDB endpoints back from Yandex.Output into State.
func (t *machinesTask) yandexMachines(nc *NodeContext) error {
	isManaged := t.runCfg.Database.Kind == types.DatabaseYDBManaged
	if t.settings == nil {
		return fmt.Errorf("machines: server settings not configured for Yandex Cloud provider")
	}

	cloud := t.settings.Cloud
	if err := cloud.ValidateCloud(); err != nil {
		return fmt.Errorf("machines: %w", err)
	}
	yc := cloud.Yandex
	if err := yc.Validate(); err != nil {
		return fmt.Errorf("machines: %w", err)
	}

	// Cloud VMs reach the control plane over the PUBLIC address (cloud.server_addr,
	// the cloud-init callback address) — NOT the docker-internal AGENT_SERVER_ADDR
	// (t.serverAddr) used by in-cluster docker agents, which a YC VM can't resolve.
	serverAddr := cloud.ServerAddr
	if serverAddr == "" {
		serverAddr = t.serverAddr
	}
	if serverAddr == "" {
		return fmt.Errorf("machines: cloud server_addr must be configured for cloud provider")
	}
	// Determine binary URL for cloud-init (defaults to self-serve via the gateway).
	binaryURL := cloud.BinaryURL
	if binaryURL == "" {
		binaryURL = serverAddr + "/agent/binary"
	}

	// Build VM specs for each machine. For Managed YDB the topology has no
	// DB-side machines — only the stroppy client VM(s).
	vmSpecs := make(map[string]*deployment.Yandex_Vm)
	vmRoles := make(map[string]types.MachineRole)
	vmZones := make(map[string]string)
	subnetZones := map[string]struct{}{}

	for _, spec := range t.runCfg.Machines {
		if isManaged && spec.Role != types.RoleStroppy {
			continue // managed YDB: no DB-side machines (defensive skip)
		}
		zones := yandexPlacementZones(spec.Placement, yc.Zone)
		for i := range spec.Count {
			machineID := fmt.Sprintf("%s-%s-%d", t.runCfg.ID, spec.Role, i)
			// Per-VM zone only for self-hosted; managed VMs sit in the network
			// default zone (managed subnets are zone-fixed).
			vmZone := ""
			if !isManaged {
				vmZone = zoneForInstance(spec.Placement, zones, i)
				if vmZone == "" {
					vmZone = yc.Zone
				}
				if vmZone != "" {
					subnetZones[vmZone] = struct{}{}
				}
			}

			// Generate agent JWT token (valid for 24h).
			agentToken := ""
			if t.jwtIssuer != nil {
				token, err := t.jwtIssuer.Issue(auth.Claims{
					UserID:   machineID,
					Username: machineID,
					TenantID: t.tenantID,
					Role:     "operator",
				}, 24*time.Hour)
				if err == nil {
					agentToken = token
				}
			}

			cloudInit, ciErr := agent.GenerateCloudInit(agent.CloudInitParams{
				BinaryURL:    binaryURL,
				ServerAddr:   serverAddr,
				AgentPort:    agent.DefaultAgentPort,
				MachineID:    machineID,
				AgentToken:   agentToken,
				SSHUser:      yc.SSHUser,
				SSHPublicKey: yc.SSHPublicKey,
			})
			if ciErr != nil {
				return fmt.Errorf("machines: generate cloud-init for %s: %w", machineID, ciErr)
			}

			cores := spec.CPUs
			if cores == 0 {
				cores = 2
			}
			memGB := spec.MemoryMB / 1024
			if memGB == 0 {
				memGB = 4
			}
			// Yandex Cloud requires memory to be a multiple of the core count.
			if cores > 0 && memGB%cores != 0 {
				memGB = ((memGB + cores - 1) / cores) * cores
			}
			diskGB := spec.DiskGB
			if diskGB == 0 {
				diskGB = 50
			}

			diskType := spec.DiskType
			if diskType == "" {
				diskType = "network-ssd"
			}
			// YC io-m3 requires disk size be a multiple of 93 GiB.
			diskGB = roundIOM3GB(diskGB, diskType)

			nc.Log().Info("preparing VM",
				zap.String("machine_id", machineID),
				zap.String("role", string(spec.Role)),
				zap.Int("cores", cores),
				zap.Int("memory_gb", memGB),
				zap.Int("disk_gb", diskGB),
			)

			secondary := make([]*deployment.Yandex_Disk, 0, len(spec.SecondaryDisks))
			for _, d := range spec.SecondaryDisks {
				if d.DeviceName == "" || d.SizeGB <= 0 {
					continue
				}
				dt := d.Type
				if dt == "" {
					dt = "network-ssd"
				}
				secondary = append(secondary, &deployment.Yandex_Disk{
					DeviceName: d.DeviceName,
					SizeGb:     uint32(roundIOM3GB(d.SizeGB, dt)),
					Type:       dt,
				})
			}

			netAccel := "standard"
			if yc.SoftwareAcceleratedNetwork {
				netAccel = "software_accelerated"
			}

			vmSpecs[machineID] = &deployment.Yandex_Vm{
				Cores:               uint32(cores),
				MemoryGb:            uint64(memGB),
				BootDiskGb:          uint64(diskGB),
				BootDiskType:        diskType,
				Zone:                vmZone,
				PublicIp:            yc.AssignPublicIP,
				UserData:            cloudInit,
				NetworkAcceleration: netAccel,
				SecondaryDisks:      secondary,
			}
			vmRoles[machineID] = spec.Role
			vmZones[machineID] = vmZone
		}
	}

	// Per-run platform_id overrides the global setting.
	platformID := t.runCfg.PlatformID
	if platformID == "" {
		platformID = yc.PlatformID
	}
	if platformID == "" {
		platformID = "standard-v2"
	}

	// Unique subnet name + CIDR per run avoids collisions across concurrent runs.
	subnetName := fmt.Sprintf("%s-%s", yc.NetworkName, t.runCfg.ID)
	subnetCIDR := runSubnetCIDR(t.runCfg.ID, yc.SubnetCIDR)
	var subnetMap map[string]*deployment.Yandex_Subnet
	if !isManaged && len(subnetZones) > 1 {
		zones := make([]string, 0, len(subnetZones))
		for z := range subnetZones {
			zones = append(zones, z)
		}
		sort.Strings(zones)
		subnetMap = runSubnetCIDRs(t.runCfg.ID, zones)
	}

	// The proto Input IS the terraform tfvars (see deployment/yandex.proto):
	// protojson(Input) yields terraform.tfvars.json matching variables.tf 1:1.
	input := &deployment.Yandex_Input{
		Network: &deployment.Yandex_Network{
			Name:      subnetName,
			NetworkId: yc.NetworkID,
			Cidr:      subnetCIDR,
			Zone:      yc.Zone,
			Subnets:   subnetMap,
		},
		Compute: &deployment.Yandex_Compute{
			PlatformId:       platformID,
			ImageId:          yc.ImageID,
			SerialPortEnable: true,
			Vms:              vmSpecs,
		},
	}
	if isManaged {
		input.ManagedYdb = managedYdbInput(t.runCfg.Database.YDBManaged, yc, t.runCfg.ID)
	}

	env := map[string]string{
		"YC_TOKEN":     yc.Token,
		"YC_CLOUD_ID":  yc.CloudID,
		"YC_FOLDER_ID": yc.FolderID,
		"YC_ZONE":      yc.Zone,
	}

	// Record the workdir BEFORE apply so a server crash mid-apply still leaves
	// enough state for teardown to `terraform destroy` partially-created infra.
	// The provisioner keys the workdir by run id; teardown rebuilds a cold actor.
	t.state.SetTerraformWdId(t.runCfg.ID)
	nc.SaveSnapshot()

	nc.Log().Info("running terraform apply for Yandex Cloud",
		zap.String("run_id", t.runCfg.ID),
		zap.Bool("managed_ydb", isManaged),
		zap.Int("vm_count", len(vmSpecs)),
	)

	ctx := context.Context(nc)
	y, err := terraformprov.Apply(ctx, t.runCfg.ID, &deployment.Yandex{Input: input}, env)
	if err != nil {
		return fmt.Errorf("machines: %w", err)
	}
	out := y.GetOutput()

	// Populate state with targets from the typed Yandex.Output.
	var dbTargets []agent.Target
	var proxyTargets []agent.Target
	var ydbStorageTargets []agent.Target
	var ydbDatabaseTargets []agent.Target

	for name, role := range vmRoles {
		vmInfo, ok := out.GetVms()[name]
		if !ok {
			return fmt.Errorf("machines: terraform output missing IP for VM %q", name)
		}
		ip := vmInfo.GetPublicIp()
		if ip == "" {
			ip = vmInfo.GetInternalIp()
		}
		target := agent.Target{
			ID:           name,
			Host:         ip,
			InternalHost: vmInfo.GetInternalIp(),
			Zone:         vmZones[name],
		}
		switch role {
		case types.RoleDatabase, types.RoleYDBStorage, types.RoleYDBDatabase:
			dbTargets = append(dbTargets, target)
			if role == types.RoleYDBStorage {
				ydbStorageTargets = append(ydbStorageTargets, target)
			}
			if role == types.RoleYDBDatabase {
				ydbDatabaseTargets = append(ydbDatabaseTargets, target)
			}
		case types.RoleProxy:
			proxyTargets = append(proxyTargets, target)
		case types.RoleStroppy:
			t.state.SetStroppyTarget(target)
		}
	}

	t.state.SetDBTargets(dbTargets)
	t.state.SetProxyTargets(proxyTargets)
	t.state.SetYDBStorageTargets(ydbStorageTargets)
	t.state.SetYDBDatabaseTargets(ydbDatabaseTargets)

	// Managed YDB: the SQL endpoint comes from the managed-YDB terraform output.
	if isManaged {
		myo := out.GetManagedYdb()
		if myo == nil || myo.GetYdbApiEndpoint() == "" {
			return fmt.Errorf("machines: terraform produced no managed YDB endpoint")
		}
		endpoint := myo.GetYdbApiEndpoint()
		dbPath := myo.GetDatabasePath()
		host, port := parseYDBEndpoint(endpoint)
		if host == "" {
			return fmt.Errorf("machines: cannot parse managed YDB endpoint %q", endpoint)
		}
		if port == 0 {
			port = types.Protocols[types.ProtocolYDBGRPCS].Port
		}
		// Share the resolved endpoint/path via the topology pointer dbDriverURL reads.
		managed := t.runCfg.Database.YDBManaged
		managed.Endpoint = endpoint
		managed.DatabasePath = dbPath
		t.state.SetDBEndpoint(host, port)
		t.state.SetEffectiveConfig("database", map[string]string{
			"kind":     "ydb-managed",
			"type":     myo.GetType(),
			"endpoint": endpoint,
			"db_path":  dbPath,
		})
		nc.Log().Info("managed YDB provisioned",
			zap.String("endpoint", endpoint),
			zap.String("database_path", dbPath),
		)
		return nil
	}

	// Self-hosted: pick the SQL endpoint stroppy hits (post-loop; map order is
	// non-deterministic). YDB split mode prefers a compute node.
	if len(dbTargets) > 0 {
		dbPort := dbConnectPort(t.runCfg.Database.Kind, t.runCfg.Stroppy.Protocol)
		endpoint := dbTargets[0]
		if t.runCfg.Database.Kind == types.DatabaseYDB && len(ydbDatabaseTargets) > 0 {
			endpoint = ydbDatabaseTargets[0]
		}
		host := endpoint.InternalHost
		if host == "" {
			host = endpoint.Host
		}
		t.state.SetDBEndpoint(host, dbPort)
	}

	// Proxy present → stroppy connects through it (except Picodata: direct).
	if len(proxyTargets) > 0 && t.runCfg.Database.Kind != types.DatabasePicodata {
		proxyHost := proxyTargets[0].InternalHost
		if proxyHost == "" {
			proxyHost = proxyTargets[0].Host
		}
		switch t.runCfg.Database.Kind {
		case types.DatabasePostgres:
			t.state.SetDBEndpoint(proxyHost, 5000) // HAProxy write port
		case types.DatabaseMySQL, types.DatabaseMariaDB:
			t.state.SetDBEndpoint(proxyHost, 6033) // ProxySQL client port
		case types.DatabaseYDB:
			t.state.SetDBEndpoint(proxyHost, 2136)
		}
	}

	nc.Log().Info("Yandex Cloud VMs provisioned",
		zap.Int("db", len(dbTargets)),
		zap.Int("proxy", len(proxyTargets)),
	)
	return nil
}

// --- Managed YDB (Yandex Cloud) ---

// parseYDBEndpoint splits a YC ydb_api_endpoint URL into host and port.
// Returns ("", 0) on parse failure so callers can fall back to defaults.
// Examples of inputs we expect:
//
//	grpcs://ydb.serverless.yandexcloud.net:2135/?database=/ru-central1/...
//	grpcs://ydb.api.cloud.yandex.net:2135/...
func parseYDBEndpoint(raw string) (string, int) {
	s := strings.TrimPrefix(raw, "grpcs://")
	s = strings.TrimPrefix(s, "grpc://")
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	host, portStr, ok := strings.Cut(s, ":")
	if !ok {
		return "", 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0
	}
	return host, port
}

// teardownTask destroys containers and network.
type teardownTask struct {
	provider types.Provider
	state    *State
	deployer *agent.DockerDeployer
	settings *types.ServerSettings
}

func (t *teardownTask) Execute(nc *NodeContext) error {
	switch t.provider {
	case types.ProviderDocker:
		return t.dockerTeardown(nc)
	case types.ProviderYandex:
		return t.yandexTeardown(nc)
	default:
		nc.Log().Warn("teardown: provider not supported, skipping",
			zap.String("provider", string(t.provider)))
		return nil // not an error — unsupported providers just skip teardown
	}
}

func (t *teardownTask) dockerTeardown(nc *NodeContext) error {
	ctx := context.Context(nc)

	// Remove containers.
	for _, cid := range t.state.ContainerIDs() {
		nc.Log().Info("removing container", zap.String("id", cid[:12]))
		if err := t.deployer.Stop(ctx, cid); err != nil {
			nc.Log().Warn("failed to remove container", zap.String("id", cid[:12]), zap.Error(err))
		}
	}

	// Remove network.
	netID := t.state.NetworkID()
	if netID != "" {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err == nil {
			nc.Log().Info("removing docker network", zap.String("id", netID[:12]))
			cli.NetworkRemove(ctx, netID)
			cli.Close()
		}
	}

	nc.Log().Info("teardown complete")
	return nil
}

func (t *teardownTask) yandexTeardown(nc *NodeContext) error {
	wdIdStr := t.state.TerraformWdId()
	if wdIdStr == "" {
		nc.Log().Info("teardown: no terraform working directory recorded, skipping")
		return nil
	}

	actor := t.state.TerraformActor()
	if actor == nil {
		nc.Log().Warn("teardown: no terraform actor in state, creating new one")
		var err error
		actor, err = terraform.NewActor()
		if err != nil {
			return fmt.Errorf("teardown: create terraform actor: %w", err)
		}
	}

	// Re-derive YC creds from settings so a recovered teardown (different
	// process) authenticates against the same folder Apply ran against.
	// In the "actor was alive" path the workdir already has env baked in;
	// the redundant overrides are harmless.
	yc := t.settings.Cloud.Yandex
	env := terraform.TfEnv{
		"YC_TOKEN":     yc.Token,
		"YC_CLOUD_ID":  yc.CloudID,
		"YC_FOLDER_ID": yc.FolderID,
		"YC_ZONE":      yc.Zone,
	}

	nc.Log().Info("running terraform destroy for Yandex Cloud", zap.String("wd_id", wdIdStr))

	ctx := context.Context(nc)
	// DestroyExisting falls back to rebuilding the workdir record from
	// disk when the in-memory map is empty (post-restart recovery). Same
	// behaviour as DestroyTerraform when the workdir is registered.
	if err := actor.DestroyExisting(ctx, terraform.NewWdId(wdIdStr), terraform.WithEnv(env)); err != nil {
		return fmt.Errorf("teardown: terraform destroy: %w", err)
	}

	nc.Log().Info("teardown complete: Yandex Cloud resources destroyed")
	return nil
}
