package mysql

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
	mysqlEngine = "mysql"

	mysqlRolePrimary  = "primary"
	mysqlRoleReplica  = "replica"
	mysqlRoleProxysql = "proxysql"

	// mysqlPort is the MySQL client/replication port.
	mysqlPort = 3306
	// groupReplicationPort is the Group Replication communication port.
	groupReplicationPort = 33061
	// proxysqlPort is the ProxySQL client traffic port.
	proxysqlPort = 6033
)

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type Database struct{}

func (d *Database) ValidateInput(input *domain.MySqlParams) error {
	if input == nil {
		return errors.New("mysql params are required")
	}
	return input.Validate()
}

func (d *Database) BuildTopologySpec(input *domain.MySqlParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	return newMysqlSpecBuilder(input).build(), nil
}

type mysqlSpecBuilder struct {
	input   *domain.MySqlParams
	spec    *topology.TopologySpec
	dbNodes []string
}

func newMysqlSpecBuilder(input *domain.MySqlParams) *mysqlSpecBuilder {
	capacity := 1 + input.GetReplicas() + input.GetProxysql()
	return &mysqlSpecBuilder{
		input: input,
		spec: &topology.TopologySpec{
			Nodes:       make([]*topology.Node, 0, capacity),
			Components:  make([]*topology.Component, 0, capacity),
			Connections: make([]*topology.Connection, 0),
			Labels: map[string]string{
				"kind":              "database",
				"engine":            mysqlEngine,
				"replicas":          strconv.FormatUint(uint64(input.GetReplicas()), 10),
				"proxysql":          strconv.FormatUint(uint64(input.GetProxysql()), 10),
				"group_replication": strconv.FormatBool(input.GetGroupReplication()),
				"semi_sync":         strconv.FormatBool(input.GetSemiSync()),
			},
			Tags: dbspec.Tags(mysqlEngine),
		},
	}
}

func (b *mysqlSpecBuilder) build() *topology.TopologySpec {
	const primaryID = "mysql-primary"
	b.spec.Components = append(b.spec.Components, dbspec.Component(mysqlEngine, primaryID, topology.Component_KIND_DATABASE, mysqlRolePrimary, primaryID))
	b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(mysqlEngine, primaryID, mysqlRolePrimary, 0, []string{primaryID}))
	b.dbNodes = append(b.dbNodes, primaryID)

	for i := uint32(1); i <= b.input.GetReplicas(); i++ {
		id := fmt.Sprintf("mysql-replica-%d", i)
		b.spec.Components = append(b.spec.Components, dbspec.Component(mysqlEngine, id, topology.Component_KIND_REPLICA, mysqlRoleReplica, id))
		b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(mysqlEngine, id, mysqlRoleReplica, i, []string{id}))
		b.dbNodes = append(b.dbNodes, id)

		if b.input.GetGroupReplication() {
			b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
				mysqlEngine, id, primaryID,
				topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_SYNC,
				"group_replication", groupReplicationPort, false,
			))
		} else {
			mode := topology.Connection_MODE_STREAM
			if b.input.GetSemiSync() {
				mode = topology.Connection_MODE_SYNC
			}
			b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
				mysqlEngine, id, primaryID,
				topology.Connection_KIND_REPLICATION, topology.Connection_PROTOCOL_REPLICATION, mode,
				"mysql", mysqlPort, false,
			))
		}
	}

	b.addProxysqlNodes()
	return b.spec
}

func (b *mysqlSpecBuilder) addProxysqlNodes() {
	for i := uint32(1); i <= b.input.GetProxysql(); i++ {
		id := fmt.Sprintf("proxysql-%d", i)
		b.spec.Components = append(b.spec.Components, dbspec.Component(mysqlEngine, id, topology.Component_KIND_PROXY, mysqlRoleProxysql, id))
		b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(mysqlEngine, id, mysqlRoleProxysql, i, []string{id}))

		for _, target := range b.dbNodes {
			b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
				mysqlEngine, id, target,
				topology.Connection_KIND_PROXY, topology.Connection_PROTOCOL_POOL, topology.Connection_MODE_REQUEST,
				"mysql", mysqlPort, false,
			))
		}
	}
}

// mysqlServerID derives the deterministic MySQL server_id from a component id
// (primary = 1, replica-N = N+1).
func mysqlServerID(componentID string) uint32 {
	const prefix = "mysql-replica-"
	if strings.HasPrefix(componentID, prefix) {
		if n, err := strconv.ParseUint(strings.TrimPrefix(componentID, prefix), 10, 32); err == nil {
			return uint32(n) + 1
		}
	}
	return 1
}

// mysqlConfigContent renders a my.cnf [mysqld] section: deterministic defaults
// plus the role options merged on top.
func mysqlConfigContent(input *domain.MySqlParams, componentID, role string, options map[string]string, isMariaDB bool) string {
	merged := map[string]string{
		"server_id":    strconv.FormatUint(uint64(mysqlServerID(componentID)), 10),
		"datadir":      mysqlDataDir(componentID),
		"bind_address": "0.0.0.0",
		"port":         strconv.Itoa(mysqlPort),
		"log_bin":      "binlog",
		"pid_file":     "/run/mysqld/mysqld.pid",
		"socket":       "/run/mysqld/mysqld.sock",
	}
	if isMariaDB {
		merged["gtid_domain_id"] = strconv.FormatUint(uint64(mysqlServerID(componentID)), 10)
		merged["log_bin_trust_function_creators"] = "1"
	} else {
		merged["gtid_mode"] = "ON"
		merged["enforce_gtid_consistency"] = "ON"
		// Enable performance_schema memory instrumentation so the
		// mysqld_exporter --collect.perf_schema.memory_events collector
		// populates the dashboard's Internal Memory panel (memory/% instruments
		// are OFF by default in MySQL 8.0).
		merged["performance_schema"] = "ON"
		merged["performance_schema_instrument"] = "'memory/%=ON'"
	}
	if input.GetSemiSync() && !input.GetGroupReplication() {
		merged["plugin_load_add"] = "semisync_master.so;semisync_slave.so"
		merged["rpl_semi_sync_master_enabled"] = "1"
		merged["rpl_semi_sync_slave_enabled"] = "1"
	}
	if input.GetGroupReplication() {
		merged["plugin_load_add"] = "group_replication.so"
		merged["group_replication_start_on_boot"] = "OFF"
		merged["group_replication_local_address"] = fmt.Sprintf("%s:%d", componentID, groupReplicationPort)
	}
	if role == mysqlRoleReplica {
		merged["read_only"] = "ON"
	}
	for key, value := range options {
		merged[key] = value
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("[mysqld]\n")
	for _, key := range keys {
		fmt.Fprintf(&b, "%s = %s\n", key, merged[key])
	}
	return b.String()
}

func mysqlDataDir(_ string) string {
	return "/var/lib/mysql"
}
