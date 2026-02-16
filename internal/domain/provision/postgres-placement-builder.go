package provision

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
)

const (
	imagePostgres          = "postgres:17-alpine"
	imageEtcd              = "quay.io/coreos/etcd:v3.5.17"
	imagePgbouncer         = "edoburu/pgbouncer:latest"
	imageNodeExporter      = "prom/node-exporter:latest"
	imagePostgresExporter  = "prometheuscommunity/postgres-exporter:latest"
	imagePgbouncerExporter = "quay.io/prometheuscommunity/pgbouncer-exporter:latest"
	imageBackup            = "postgres:17-alpine"

	defaultPortPostgres          uint32 = 5432
	defaultPortPgbouncer         uint32 = 6432
	defaultPortEtcdClient        uint32 = 2379
	defaultPortEtcdPeer          uint32 = 2380
	defaultPortNodeExporter      uint32 = 9100
	defaultPortPostgresExporter  uint32 = 9187
	defaultPortPgbouncerExporter uint32 = 9127
	defaultPortPatroniAPI        uint32 = 8008

	defaultPostgresUser     = "postgres"
	defaultPostgresPassword = "postgres"
	defaultPostgresDatabase = "postgres"

	defaultEtcdClusterState = "new"
	defaultEtcdClusterToken = "postgres-etcd-cluster"

	containerMetadataDockerIPKey      = "docker.network.ipv4"
	containerMetadataPlacementNodeKey = "docker.placement.node"
	containerMetadataLogicalNameKey   = "docker.logical_name"
)

type postgresPlacementBuilder struct {
	itemsByName map[string]*provision.PlacementIntent_Item
	order       []string
	network     *deployment.Network
}

func newPostgresPlacementBuilder(network *deployment.Network) *postgresPlacementBuilder {
	return &postgresPlacementBuilder{
		itemsByName: map[string]*provision.PlacementIntent_Item{},
		order:       make([]string, 0),
		network:     network,
	}
}

func (p *postgresPlacementBuilder) BuildForPostgresInstance(
	t *database.Database_Template_PostgresInstance,
) (*provision.PlacementIntent, error) {
	if t == nil {
		return nil, fmt.Errorf("database template postgres_instance is nil")
	}
	if err := p.addInstanceTemplate(t.PostgresInstance); err != nil {
		return nil, err
	}
	return p.finalize()
}

func (p *postgresPlacementBuilder) BuildForPostgresCluster(
	t *database.Database_Template_PostgresCluster,
) (*provision.PlacementIntent, error) {
	if t == nil {
		return nil, fmt.Errorf("database template postgres_cluster is nil")
	}
	if err := p.addClusterTemplate(t.PostgresCluster); err != nil {
		return nil, err
	}
	return p.finalize()
}

func (p *postgresPlacementBuilder) finalize() (*provision.PlacementIntent, error) {
	items, err := p.finalizeItems()
	if err != nil {
		return nil, err
	}
	connStr, err := p.resolveRuntimeConfig(items)
	if err != nil {
		return nil, err
	}
	return &provision.PlacementIntent{
		Items:            items,
		Network:          p.network,
		ConnectionString: connStr,
	}, nil
}

func (p *postgresPlacementBuilder) addInstanceTemplate(tmpl *database.Postgres_Instance_Template) error {
	if tmpl == nil {
		return fmt.Errorf("postgres instance template is nil")
	}
	item, err := p.ensureItem("postgres-master", tmpl.GetHardware())
	if err != nil {
		return err
	}
	item.Containers = append(item.Containers, p.newPostgresContainer(
		"postgres-master",
		"postgres-master",
		tmpl.GetSettings(),
		provision.Container_PostgresRuntime_ROLE_MASTER,
		0,
		false,
	))
	p.addSidecarsToItem(item, tmpl.GetSidecars())
	return nil
}

func (p *postgresPlacementBuilder) addClusterTemplate(tmpl *database.Postgres_Cluster_Template) error {
	if tmpl == nil || tmpl.GetTopology() == nil {
		return fmt.Errorf("postgres cluster topology is required")
	}
	topology := tmpl.GetTopology()
	replicasCount := int(topology.GetReplicasCount())

	masterName := "postgres-master"
	masterItem, err := p.ensureItem(masterName, topology.GetMasterHardware())
	if err != nil {
		return err
	}
	masterItem.Containers = append(masterItem.Containers, p.newPostgresContainer(
		masterName, masterName, topology.GetSettings(),
		provision.Container_PostgresRuntime_ROLE_MASTER, 0, topology.GetMonitor(),
	))
	if topology.GetMonitor() {
		p.addPostgresMonitoring(masterItem, "master")
	}

	overrideByIndex := make(map[uint32]*database.Postgres_Cluster_Template_ReplicaOverride)
	for _, o := range tmpl.GetReplicaOverrides() {
		if o != nil {
			overrideByIndex[o.GetReplicaIndex()] = o
		}
	}

	replicaNames := make([]string, 0, replicasCount)
	for i := 0; i < replicasCount; i++ {
		name := fmt.Sprintf("postgres-replica-%d", i)
		replicaNames = append(replicaNames, name)

		hardware := topology.GetReplicaHardware()
		settings := topology.GetSettings()
		if o, ok := overrideByIndex[uint32(i)]; ok {
			if o.GetHardware() != nil {
				hardware = o.GetHardware()
			}
			if o.GetSettings() != nil {
				settings = o.GetSettings()
			}
		}

		item, err := p.ensureItem(name, hardware)
		if err != nil {
			return err
		}
		item.Containers = append(item.Containers, p.newPostgresContainer(
			name, name, settings,
			provision.Container_PostgresRuntime_ROLE_REPLICA, uint32(i), topology.GetMonitor(),
		))
		if topology.GetMonitor() {
			p.addPostgresMonitoring(item, fmt.Sprintf("replica-%d", i))
		}
	}

	addons := tmpl.GetAddons()
	if _, err := p.addEtcdAddons(addons.GetDcs().GetEtcd(), masterName, replicaNames); err != nil {
		return err
	}

	if _, err := p.addPgbouncerAddons(addons.GetPooling().GetPgbouncer(), masterName, replicaNames); err != nil {
		return err
	}

	p.addBackupAddons(addons.GetBackup(), masterName, replicaNames)

	return nil
}

func (p *postgresPlacementBuilder) addEtcdAddons(
	etcd *database.Postgres_Addons_Dcs_Etcd,
	masterName string,
	replicaNames []string,
) ([]string, error) {
	if etcd == nil {
		return nil, nil
	}
	targetNames, err := p.expandPlacement(etcd.GetPlacement(), masterName, replicaNames)
	if err != nil {
		return nil, err
	}
	if len(targetNames) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(targetNames))
	for i, name := range targetNames {
		item := p.itemsByName[name]
		item.Containers = append(item.Containers, &provision.Container{
			Id:      "etcd-" + strconv.Itoa(i+1),
			Name:    "etcd-" + strconv.Itoa(i+1),
			Image:   imageEtcd,
			Monitor: etcd.GetMonitor(),
			Runtime: &provision.Container_Etcd{
				Etcd: &provision.Container_EtcdRuntime{
					ClusterSize:    etcd.GetSize(),
					NodeIndex:      uint32(i + 1),
					BaseClientPort: etcd.BaseClientPort,
				},
			},
		})
		if etcd.GetMonitor() {
			item.Containers = append(item.Containers, NewNodeExporterContainer("etcd-"+strconv.Itoa(i+1), true))
		}
		names = append(names, name)
	}
	return names, nil
}

func (p *postgresPlacementBuilder) addPgbouncerAddons(
	pgb *database.Postgres_Addons_Pooling_Pgbouncer,
	masterName string,
	replicaNames []string,
) ([]string, error) {
	if pgb == nil || !pgb.GetEnabled() {
		return nil, nil
	}
	targetNames, err := p.expandPlacement(pgb.GetPlacement(), masterName, replicaNames)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(targetNames))
	for i, name := range targetNames {
		item := p.itemsByName[name]
		containerID := "pgbouncer-" + strconv.Itoa(i+1)
		item.Containers = append(item.Containers, &provision.Container{
			Id:      containerID,
			Name:    containerID,
			Image:   imagePgbouncer,
			Monitor: pgb.GetMonitor(),
			Runtime: &provision.Container_Pgbouncer{
				Pgbouncer: &provision.Container_PgbouncerRuntime{
					Config: pgb,
				},
			},
		})
		if pgb.GetMonitor() {
			item.Containers = append(item.Containers, &provision.Container{
				Id:      "pgbouncer-exporter-" + strconv.Itoa(i+1),
				Name:    "pgbouncer-exporter-" + strconv.Itoa(i+1),
				Image:   imagePgbouncerExporter,
				Monitor: true,
				Runtime: &provision.Container_PgbouncerExporter{
					PgbouncerExporter: &provision.Container_PgbouncerExporterRuntime{
						Enabled: true,
						Port:    defaultPortPgbouncerExporter,
					},
				},
			})
		}
		names = append(names, name)
	}
	return names, nil
}

func (p *postgresPlacementBuilder) addBackupAddons(
	backup *database.Postgres_Addons_Backup,
	masterName string,
	replicaNames []string,
) []string {
	if backup == nil || !backup.GetEnabled() || backup.GetConfig() == nil {
		return nil
	}
	targets := p.expandScope(backup.GetScope(), nil, masterName, replicaNames)
	names := make([]string, 0, len(targets))
	for i, name := range targets {
		item := p.itemsByName[name]
		item.Containers = append(item.Containers, &provision.Container{
			Id:      "backup-" + strconv.Itoa(i+1),
			Name:    "backup-" + strconv.Itoa(i+1),
			Image:   imageBackup,
			Monitor: false,
			Runtime: &provision.Container_Backup{
				Backup: &provision.Container_BackupRuntime{
					Config: backup.GetConfig(),
				},
			},
		})
		names = append(names, name)
	}
	return names
}

func (p *postgresPlacementBuilder) addPostgresMonitoring(item *provision.PlacementIntent_Item, suffix string) {
	item.Containers = append(item.Containers, NewNodeExporterContainer(suffix, true))
	item.Containers = append(item.Containers, &provision.Container{
		Id:      "postgres-exporter-" + suffix,
		Name:    "postgres-exporter-" + suffix,
		Image:   imagePostgresExporter,
		Monitor: true,
		Runtime: &provision.Container_PostgresExporter{
			PostgresExporter: &provision.Container_PostgresExporterRuntime{
				Enabled: true,
				Port:    defaultPortPostgresExporter,
			},
		},
	})
}

func (p *postgresPlacementBuilder) addSidecarsToItem(item *provision.PlacementIntent_Item, sidecars []*database.Postgres_Sidecar) {
	for i, s := range sidecars {
		if s == nil {
			continue
		}
		if ne := s.GetNodeExporter(); ne != nil {
			port := ne.GetPort()
			if port == 0 {
				port = defaultPortNodeExporter
			}
			item.Containers = append(item.Containers, &provision.Container{
				Id:      "node-exporter-sidecar-" + strconv.Itoa(i),
				Name:    "node-exporter-sidecar-" + strconv.Itoa(i),
				Image:   imageNodeExporter,
				Monitor: true,
				Runtime: &provision.Container_NodeExporter{
					NodeExporter: &provision.Container_NodeExporterRuntime{
						Enabled: true,
						Port:    port,
					},
				},
			})
		}
		if pe := s.GetPostgresExporter(); pe != nil && pe.GetEnabled() {
			port := pe.GetPort()
			if port == 0 {
				port = defaultPortPostgresExporter
			}
			item.Containers = append(item.Containers, &provision.Container{
				Id:      "postgres-exporter-sidecar-" + strconv.Itoa(i),
				Name:    "postgres-exporter-sidecar-" + strconv.Itoa(i),
				Image:   imagePostgresExporter,
				Monitor: true,
				Runtime: &provision.Container_PostgresExporter{
					PostgresExporter: &provision.Container_PostgresExporterRuntime{
						Enabled:            true,
						Port:               port,
						CustomQueriesPaths: pe.GetCustomQueriesPaths(),
					},
				},
			})
		}
		if b := s.GetBackup(); b != nil {
			item.Containers = append(item.Containers, &provision.Container{
				Id:      "backup-sidecar-" + strconv.Itoa(i),
				Name:    "backup-sidecar-" + strconv.Itoa(i),
				Image:   imageBackup,
				Monitor: false,
				Runtime: &provision.Container_Backup{
					Backup: &provision.Container_BackupRuntime{
						Config: b,
					},
				},
			})
		}
	}
}

func (p *postgresPlacementBuilder) newPostgresContainer(
	id string,
	name string,
	settings *database.Postgres_Settings,
	role provision.Container_PostgresRuntime_Role,
	replicaIndex uint32,
	monitor bool,
) *provision.Container {
	return &provision.Container{
		Id:      id + "-container",
		Name:    name + "-container",
		Image:   imagePostgres,
		Monitor: monitor,
		Runtime: &provision.Container_Postgres{
			Postgres: &provision.Container_PostgresRuntime{
				Role:         role,
				Settings:     settings,
				ReplicaIndex: replicaIndex,
			},
		},
	}
}

func NewNodeExporterContainer(suffix string, monitor bool) *provision.Container {
	return &provision.Container{
		Id:      "node-exporter-" + suffix,
		Name:    "node-exporter-" + suffix,
		Image:   imageNodeExporter,
		Monitor: monitor,
		Runtime: &provision.Container_NodeExporter{
			NodeExporter: &provision.Container_NodeExporterRuntime{
				Enabled: true,
				Port:    defaultPortNodeExporter,
			},
		},
	}
}

func (p *postgresPlacementBuilder) expandPlacement(
	placement *database.Postgres_Placement,
	masterName string,
	replicaNames []string,
) ([]string, error) {
	if placement == nil {
		return nil, nil
	}
	switch mode := placement.GetMode().(type) {
	case *database.Postgres_Placement_Colocate_:
		return p.expandScope(mode.Colocate.GetScope(), mode.Colocate.ReplicaIndex, masterName, replicaNames), nil
	case *database.Postgres_Placement_Dedicated_:
		n := int(mode.Dedicated.GetInstancesCount())
		names := make([]string, 0, n)
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("dedicated-%d", len(p.order)+1)
			if _, err := p.ensureItem(name, mode.Dedicated.GetHardware()); err != nil {
				return nil, err
			}
			names = append(names, name)
		}
		return names, nil
	default:
		return nil, fmt.Errorf("placement mode is not set")
	}
}

func (p *postgresPlacementBuilder) expandScope(
	scope database.Postgres_Placement_Scope,
	replicaIndex *uint32,
	masterName string,
	replicaNames []string,
) []string {
	switch scope {
	case database.Postgres_Placement_SCOPE_MASTER:
		return []string{masterName}
	case database.Postgres_Placement_SCOPE_REPLICAS:
		return append([]string{}, replicaNames...)
	case database.Postgres_Placement_SCOPE_ALL_NODES:
		out := make([]string, 0, len(replicaNames)+1)
		out = append(out, masterName)
		out = append(out, replicaNames...)
		return out
	case database.Postgres_Placement_SCOPE_REPLICA:
		if replicaIndex == nil {
			return nil
		}
		idx := int(*replicaIndex)
		if idx < 0 || idx >= len(replicaNames) {
			return nil
		}
		return []string{replicaNames[idx]}
	default:
		return nil
	}
}

func (p *postgresPlacementBuilder) ensureItem(name string, hw *deployment.Hardware) (*provision.PlacementIntent_Item, error) {
	if hw == nil {
		return nil, fmt.Errorf("hardware is required for item %q", name)
	}
	if item, ok := p.itemsByName[name]; ok {
		return item, nil
	}
	item := &provision.PlacementIntent_Item{
		Name:       name,
		Hardware:   hw,
		Containers: make([]*provision.Container, 0),
	}
	p.itemsByName[name] = item
	p.order = append(p.order, name)
	return item, nil
}

func (p *postgresPlacementBuilder) finalizeItems() ([]*provision.PlacementIntent_Item, error) {
	items := make([]*provision.PlacementIntent_Item, 0, len(p.order))
	ips := p.network.GetIps()
	if len(ips) < len(p.order) {
		return nil, fmt.Errorf("network has %d ips, but %d items are required", len(ips), len(p.order))
	}
	for i, name := range p.order {
		item := p.itemsByName[name]
		if ips[i] == nil || ips[i].GetValue() == "" {
			return nil, fmt.Errorf("network ip at index %d is empty", i)
		}
		item.InternalIp = ips[i]
		for _, c := range item.GetContainers() {
			ensureMetadata(c)
			c.Metadata[containerMetadataDockerIPKey] = item.GetInternalIp().GetValue()
			c.Metadata[containerMetadataPlacementNodeKey] = item.GetName()
			c.Metadata[containerMetadataLogicalNameKey] = containerLogicalName(c)
		}
		items = append(items, item)
	}
	return items, nil
}

func (p *postgresPlacementBuilder) resolveRuntimeConfig(items []*provision.PlacementIntent_Item) (string, error) {
	type etcdMember struct {
		name       string
		ip         string
		clientPort uint32
		peerPort   uint32
	}
	var members []etcdMember
	var masterIP string

	for _, item := range items {
		itemIP := item.GetInternalIp().GetValue()
		for _, c := range item.GetContainers() {
			if pg := c.GetPostgres(); pg != nil && pg.GetRole() == provision.Container_PostgresRuntime_ROLE_MASTER {
				masterIP = itemIP
			}
			if e := c.GetEtcd(); e != nil {
				clientPort := e.GetBaseClientPort()
				if clientPort == 0 {
					clientPort = defaultPortEtcdClient
				}
				peerPort := e.GetPeerPort()
				if peerPort == 0 {
					peerPort = defaultPortEtcdPeer
				}
				members = append(members, etcdMember{
					name:       c.GetName(),
					ip:         itemIP,
					clientPort: clientPort,
					peerPort:   peerPort,
				})
			}
		}
	}

	etcdInitialCluster := make([]string, 0, len(members))
	etcdHosts := make([]string, 0, len(members))
	for _, m := range members {
		etcdInitialCluster = append(etcdInitialCluster, fmt.Sprintf("%s=http://%s:%d", m.name, m.ip, m.peerPort))
		etcdHosts = append(etcdHosts, fmt.Sprintf("%s:%d", m.ip, m.clientPort))
	}
	initialClusterValue := strings.Join(etcdInitialCluster, ",")
	etcdHostsValue := strings.Join(etcdHosts, ",")

	// Track the first pgbouncer endpoint for the connection string.
	var pgbouncerConnIP string
	var pgbouncerConnPort uint32

	for _, item := range items {
		itemIP := item.GetInternalIp().GetValue()
		hasPgbouncer := false
		var pgbouncerPort = defaultPortPgbouncer

		for _, c := range item.GetContainers() {
			if pb := c.GetPgbouncer(); pb != nil {
				hasPgbouncer = true
				if port := pb.GetConfig().GetPort(); port != 0 {
					pgbouncerPort = port
				}
				if pgbouncerConnIP == "" {
					pgbouncerConnIP = itemIP
					pgbouncerConnPort = pgbouncerPort
				}
			}
		}

		for _, c := range item.GetContainers() {
			switch {
			case c.GetEtcd() != nil:
				e := c.GetEtcd()
				clientPort := e.GetBaseClientPort()
				if clientPort == 0 {
					clientPort = defaultPortEtcdClient
				}
				peerPort := e.GetPeerPort()
				if peerPort == 0 {
					peerPort = defaultPortEtcdPeer
				}
				ensureEnv(c)
				c.Env["ETCD_NAME"] = c.GetName()
				c.Env["ETCD_INITIAL_CLUSTER"] = initialClusterValue
				c.Env["ETCD_INITIAL_CLUSTER_STATE"] = defaultEtcdClusterState
				c.Env["ETCD_INITIAL_CLUSTER_TOKEN"] = defaultEtcdClusterToken
				c.Env["ETCD_LISTEN_PEER_URLS"] = fmt.Sprintf("http://0.0.0.0:%d", peerPort)
				c.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = fmt.Sprintf("http://%s:%d", itemIP, peerPort)
				c.Env["ETCD_LISTEN_CLIENT_URLS"] = fmt.Sprintf("http://0.0.0.0:%d", clientPort)
				c.Env["ETCD_ADVERTISE_CLIENT_URLS"] = fmt.Sprintf("http://%s:%d", itemIP, clientPort)

			case c.GetPostgres() != nil:
				ensureEnv(c)
				c.Env["POSTGRES_USER"] = defaultPostgresUser
				c.Env["POSTGRES_PASSWORD"] = defaultPostgresPassword
				c.Env["POSTGRES_DB"] = defaultPostgresDatabase
				postgresSettings := c.GetPostgres().GetSettings()
				if postgresSettings.GetPatroni().GetEnabled() {
					if len(etcdHosts) == 0 {
						return "", fmt.Errorf("patroni is enabled but etcd endpoints are empty")
					}
					c.Env["PATRONI_NAME"] = c.GetName()
					c.Env["PATRONI_ETCD3_HOSTS"] = etcdHostsValue
					c.Env["PATRONI_POSTGRESQL_CONNECT_ADDRESS"] = fmt.Sprintf("%s:%d", itemIP, defaultPortPostgres)
					c.Env["PATRONI_RESTAPI_CONNECT_ADDRESS"] = fmt.Sprintf("%s:%d", itemIP, defaultPortPatroniAPI)
				}
				if err := applyPostgresqlConf(c, postgresSettings); err != nil {
					return "", err
				}

			case c.GetPgbouncer() != nil:
				if masterIP == "" {
					return "", fmt.Errorf("pgbouncer is configured but postgres master endpoint is not found")
				}
				ensureEnv(c)
				c.Env["PGBOUNCER_UPSTREAM_HOST"] = masterIP
				c.Env["PGBOUNCER_UPSTREAM_PORT"] = fmt.Sprintf("%d", defaultPortPostgres)

			case c.GetPostgresExporter() != nil:
				ensureEnv(c)
				target := fmt.Sprintf("%s:%d", itemIP, defaultPortPostgres)
				if masterIP != "" {
					target = fmt.Sprintf("%s:%d", masterIP, defaultPortPostgres)
				}
				c.Env["PG_EXPORTER_TARGET"] = target

			case c.GetPgbouncerExporter() != nil:
				if !hasPgbouncer {
					continue
				}
				ensureEnv(c)
				c.Env["PGBOUNCER_EXPORTER_TARGET"] = fmt.Sprintf("%s:%d", itemIP, pgbouncerPort)
			}
		}
	}

	if masterIP == "" {
		return "", fmt.Errorf("postgres master not found in placement items")
	}

	connHost := masterIP
	connPort := defaultPortPostgres
	if pgbouncerConnIP != "" {
		connHost = pgbouncerConnIP
		connPort = pgbouncerConnPort
	}
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		defaultPostgresUser, defaultPostgresPassword,
		connHost, connPort,
		defaultPostgresDatabase)

	return connStr, nil
}

func ensureEnv(c *provision.Container) {
	if c.Env == nil {
		c.Env = make(map[string]string)
	}
}

func ensureMetadata(c *provision.Container) {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
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

func applyPostgresqlConf(c *provision.Container, settings *database.Postgres_Settings) error {
	if c == nil || settings == nil || len(settings.GetPostgresqlConf()) == 0 {
		return nil
	}

	if settings.GetPatroni().GetEnabled() {
		ensureEnv(c)
		encoded, err := json.Marshal(settings.GetPostgresqlConf())
		if err != nil {
			return fmt.Errorf("encode postgresql_conf for %q: %w", c.GetName(), err)
		}
		c.Env["PATRONI_POSTGRESQL_PARAMETERS"] = string(encoded)
		return nil
	}

	c.Args = append(c.Args, postgresqlConfArgs(settings.GetPostgresqlConf())...)
	return nil
}

func postgresqlConfArgs(conf map[string]string) []string {
	keys := make([]string, 0, len(conf))
	for k := range conf {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(conf)*2)
	for _, k := range keys {
		args = append(args, "-c", fmt.Sprintf("%s=%s", k, conf[k]))
	}
	return args
}
