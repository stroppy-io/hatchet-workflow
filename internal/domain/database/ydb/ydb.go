package ydb

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	ydbEngine = "ydb"

	ydbRoleStorage  = "storage"
	ydbRoleDatabase = "database"
	ydbRoleHaproxy  = "haproxy"

	// grpcPort is the YDB client gRPC port.
	grpcPort = 2136
	// storageGrpcPort is the static-node gRPC/node-broker port. Keep it
	// separate from the client gRPC port so combined single-node deployments can
	// run static and dynamic daemons on the same host.
	storageGrpcPort = 2135
	// icPort is the interconnect (node-to-node) port.
	icPort         = 19001
	databaseICPort = 19002
	// monPort is the monitoring HTTP port.
	monPort         = 8765
	databaseMonPort = 8766

	defaultDatabasePath = "/Root/testdb"
	ydbPDiskDir         = "/var/lib/stroppy-cloud"
	ydbPDiskPath        = ydbPDiskDir + "/ydb-pdisk.data"
)

type Database struct{}

func (d *Database) ValidateInput(input *domain.YdbParams) error {
	if input == nil {
		return errors.New("ydb params are required")
	}
	return input.Validate()
}

func (d *Database) BuildTopologySpec(input *domain.YdbParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	return newYdbSpecBuilder(input).build(), nil
}

type ydbSpecBuilder struct {
	input    *domain.YdbParams
	spec     *topology.TopologySpec
	storage  []string
	database []string
}

func newYdbSpecBuilder(input *domain.YdbParams) *ydbSpecBuilder {
	storageNodes := input.GetStorageNodes()
	if storageNodes == 0 {
		storageNodes = 1
	}
	capacity := storageNodes + input.GetDatabaseNodes() + input.GetHaproxy()

	return &ydbSpecBuilder{
		input: input,
		spec: &topology.TopologySpec{
			Nodes:       make([]*topology.Node, 0, capacity),
			Components:  make([]*topology.Component, 0, capacity),
			Connections: make([]*topology.Connection, 0),
			Labels: map[string]string{
				"kind":            "database",
				"engine":          ydbEngine,
				"storage_nodes":   strconv.FormatUint(uint64(storageNodes), 10),
				"database_nodes":  strconv.FormatUint(uint64(input.GetDatabaseNodes()), 10),
				"haproxy":         strconv.FormatUint(uint64(input.GetHaproxy()), 10),
				"fault_tolerance": ydbFaultTolerance(input),
				"combined":        strconv.FormatBool(input.GetDatabaseNodes() == 0),
			},
			Tags: dbspec.Tags(ydbEngine),
		},
	}
}

func (b *ydbSpecBuilder) build() *topology.TopologySpec {
	storageNodes := b.input.GetStorageNodes()
	if storageNodes == 0 {
		storageNodes = 1
	}

	for i := uint32(1); i <= storageNodes; i++ {
		id := fmt.Sprintf("ydb-storage-%d", i)
		b.spec.Components = append(b.spec.Components, dbspec.Component(ydbEngine, id, topology.Component_KIND_DATABASE, ydbRoleStorage, id))
		b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(ydbEngine, id, ydbRoleStorage, i, []string{id}))
		b.storage = append(b.storage, id)
	}

	// Storage nodes form the cluster over the interconnect port; later nodes
	// register against the first storage node.
	for _, id := range b.storage[1:] {
		b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
			ydbEngine, id, b.storage[0],
			topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_GRPC, topology.Connection_MODE_SYNC,
			"ic", icPort, false,
		))
	}

	for i := uint32(1); i <= b.input.GetDatabaseNodes(); i++ {
		id := fmt.Sprintf("ydb-database-%d", i)
		b.spec.Components = append(b.spec.Components, dbspec.Component(ydbEngine, id, topology.Component_KIND_DATABASE, ydbRoleDatabase, id))
		b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(ydbEngine, id, ydbRoleDatabase, i, []string{id}))
		b.database = append(b.database, id)

		for _, storageID := range b.storage {
			b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
				ydbEngine, id, storageID,
				topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_GRPC, topology.Connection_MODE_REQUEST,
				"node-broker", storageGrpcPort, false,
			))
		}
	}

	b.addHaproxyNodes()
	return b.spec
}

func (b *ydbSpecBuilder) addHaproxyNodes() {
	// HAProxy fronts the compute tier; in combined mode that is the storage nodes.
	targets := b.database
	if len(targets) == 0 {
		targets = b.storage
	}

	for i := uint32(1); i <= b.input.GetHaproxy(); i++ {
		haproxyID := fmt.Sprintf("haproxy-%d", i)
		b.spec.Components = append(b.spec.Components, dbspec.Component(ydbEngine, haproxyID, topology.Component_KIND_PROXY, ydbRoleHaproxy, haproxyID))
		b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(ydbEngine, haproxyID, ydbRoleHaproxy, i, []string{haproxyID}))

		for _, target := range targets {
			b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
				ydbEngine, haproxyID, target,
				topology.Connection_KIND_PROXY, topology.Connection_PROTOCOL_GRPC, topology.Connection_MODE_REQUEST,
				"grpc", grpcPort, false,
			))
		}
	}
}

func ydbFaultTolerance(input *domain.YdbParams) string {
	switch input.GetFaultTolerance() {
	case domain.YdbParams_FAULT_TOLERANCE_BLOCK_4_2:
		return "block-4-2"
	case domain.YdbParams_FAULT_TOLERANCE_MIRROR_3_DC:
		return "mirror-3-dc"
	default:
		return "none"
	}
}

func ydbDiskType(input *domain.YdbParams) string {
	switch input.GetDefaultDiskType() {
	case domain.YdbParams_DISK_TYPE_NVME:
		return "nvme"
	case domain.YdbParams_DISK_TYPE_ROT:
		return "rot"
	default:
		return "ssd"
	}
}

func ydbDatabasePath(input *domain.YdbParams) string {
	if path := input.GetDatabasePath(); path != "" {
		return path
	}
	return defaultDatabasePath
}

// ydbConfigContent renders a compact ydb config.yaml for the given node type
// ("STORAGE" or "COMPUTE"), with the matching *_options merged flat under a
// passthrough block. hosts is the runtime-resolved static (storage) node
// address list; empty in preview (infrastructure not provisioned yet).
func ydbConfigContent(input *domain.YdbParams, nodeType string, options map[string]string, hosts []string, budget ydbNodeBudget) string {
	hostCount := len(hosts)
	if hostCount == 0 {
		hostCount = int(input.GetStorageNodes())
		if hostCount == 0 {
			hostCount = 1
		}
	}
	zones := []string{"zone-a", "zone-b", "zone-c"}
	diskType := strings.ToUpper(ydbDiskType(input))
	storagePoolKind := ydbDiskType(input)

	var b strings.Builder
	fmt.Fprintf(&b, "static_erasure: %s\n", ydbFaultTolerance(input))
	b.WriteString("host_configs:\n")
	b.WriteString("- drive:\n")
	fmt.Fprintf(&b, "  - path: %s\n", ydbPDiskPath)
	fmt.Fprintf(&b, "    type: %s\n", diskType)
	b.WriteString("  host_config_id: 1\n")
	b.WriteString("hosts:\n")
	for i := 0; i < hostCount; i++ {
		host := fmt.Sprintf("__YDB_HOST_%d__", i)
		if i < len(hosts) && hosts[i] != "" {
			host = hosts[i]
		}
		fmt.Fprintf(&b, "- host: %s\n", host)
		b.WriteString("  host_config_id: 1\n")
		b.WriteString("  walle_location:\n")
		fmt.Fprintf(&b, "    body: %d\n", i+1)
		fmt.Fprintf(&b, "    data_center: '%s'\n", zones[i%len(zones)])
		fmt.Fprintf(&b, "    rack: '%d'\n", i+1)
	}
	fmt.Fprintf(&b, "domains_config:\n")
	fmt.Fprintf(&b, "  domain:\n")
	fmt.Fprintf(&b, "  - name: Root\n")
	fmt.Fprintf(&b, "    storage_pool_types:\n")
	fmt.Fprintf(&b, "    - kind: %s\n", storagePoolKind)
	fmt.Fprintf(&b, "      pool_config:\n")
	fmt.Fprintf(&b, "        box_id: 1\n")
	fmt.Fprintf(&b, "        erasure_species: %s\n", ydbFaultTolerance(input))
	fmt.Fprintf(&b, "        kind: %s\n", storagePoolKind)
	fmt.Fprintf(&b, "        pdisk_filter:\n")
	fmt.Fprintf(&b, "        - property:\n")
	fmt.Fprintf(&b, "          - type: %s\n", diskType)
	fmt.Fprintf(&b, "        vdisk_kind: Default\n")
	fmt.Fprintf(&b, "  state_storage:\n")
	fmt.Fprintf(&b, "  - ring:\n")
	fmt.Fprintf(&b, "      node: [%s]\n", ydbStateStorageNodeList(input, hosts))
	fmt.Fprintf(&b, "      nto_select: %d\n", ydbStateStorageNToSelect(input, hosts))
	fmt.Fprintf(&b, "    ssid: 1\n")
	b.WriteString("table_service_config:\n")
	b.WriteString("  sql_version: 1\n")
	b.WriteString("actor_system_config:\n")
	b.WriteString("  use_auto_config: true\n")
	if nodeType != "" {
		fmt.Fprintf(&b, "  node_type: %s\n", nodeType)
	}
	if budget.cpuCores > 0 {
		fmt.Fprintf(&b, "  cpu_count: %d\n", budget.cpuCores)
	}
	fmt.Fprintf(&b, "blob_storage_config:\n")
	fmt.Fprintf(&b, "  service_set:\n")
	fmt.Fprintf(&b, "    groups:\n")
	fmt.Fprintf(&b, "    - erasure_species: %s\n", ydbFaultTolerance(input))
	fmt.Fprintf(&b, "      rings:\n")
	b.WriteString("      - fail_domains:\n")
	for i := 0; i < hostCount; i++ {
		b.WriteString("        - vdisk_locations:\n")
		fmt.Fprintf(&b, "          - node_id: %d\n", i+1)
		fmt.Fprintf(&b, "            pdisk_category: %s\n", diskType)
		fmt.Fprintf(&b, "            path: %s\n", ydbPDiskPath)
	}
	b.WriteString("channel_profile_config:\n")
	b.WriteString("  profile:\n")
	b.WriteString("  - channel:\n")
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&b, "    - erasure_species: %s\n", ydbFaultTolerance(input))
		fmt.Fprintf(&b, "      pdisk_category: 1\n")
		fmt.Fprintf(&b, "      storage_pool_kind: %s\n", storagePoolKind)
	}
	b.WriteString("    profile_id: 0\n")
	b.WriteString("grpc_config:\n")
	fmt.Fprintf(&b, "  port: %d\n", grpcPort)
	if len(options) > 0 {
		b.WriteString("config_passthrough:\n")
		keys := make([]string, 0, len(options))
		for key := range options {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "  %s: %s\n", key, options[key])
		}
	}
	return b.String()
}

func ydbStateStorageNodeList(input *domain.YdbParams, hosts []string) string {
	count := len(hosts)
	if count == 0 {
		count = int(input.GetStorageNodes())
		if count == 0 {
			count = 1
		}
	}
	nto := ydbStateStorageNToSelect(input, hosts)
	nodes := make([]string, 0, nto)
	for i := 1; i <= nto; i++ {
		nodes = append(nodes, strconv.Itoa(i))
	}
	return strings.Join(nodes, ", ")
}

func ydbStateStorageNToSelect(input *domain.YdbParams, hosts []string) int {
	count := len(hosts)
	if count == 0 {
		count = int(input.GetStorageNodes())
		if count == 0 {
			count = 1
		}
	}
	switch ydbFaultTolerance(input) {
	case "mirror-3-dc":
		if count >= 9 {
			return 9
		}
	case "block-4-2":
		if count >= 5 {
			return 5
		}
	}
	if count >= 3 {
		return 3
	}
	return 1
}
