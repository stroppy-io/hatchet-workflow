package postgres

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	postgresEngine = "postgres"

	postgresRoleMaster    = "master"
	postgresRoleReplica   = "replica"
	postgresRoleHaproxy   = "haproxy"
	postgresRolePgbouncer = "pgbouncer"
	postgresRolePatroni   = "patroni"
	postgresRoleEtcd      = "etcd"
)

type Database struct{}

func (p *Database) ValidateInput(input *domain.PostgresParams) error {
	if input == nil {
		return errors.New("postgres params are required")
	}
	if err := input.Validate(); err != nil {
		return err
	}
	if input.GetSyncReplicas() > input.GetReplicas() {
		return fmt.Errorf("sync_replicas=%d exceeds replicas=%d", input.GetSyncReplicas(), input.GetReplicas())
	}

	return nil
}

func (p *Database) BuildTopologySpec(input *domain.PostgresParams) (*topology.TopologySpec, error) {
	if err := p.ValidateInput(input); err != nil {
		return nil, err
	}

	builder := newPostgresSpecBuilder(input)
	return builder.build(), nil
}

type postgresSpecBuilder struct {
	input *domain.PostgresParams
	spec  *topology.TopologySpec

	pgComponentIDs    []string
	patroniComponents []string
	etcdComponents    []string
}

func newPostgresSpecBuilder(input *domain.PostgresParams) *postgresSpecBuilder {
	hasPatroni := input.GetPatroni() || input.GetSyncReplicas() > 0
	hasEtcd := input.GetEtcd() || hasPatroni

	return &postgresSpecBuilder{
		input: input,
		spec: &topology.TopologySpec{
			Nodes:       make([]*topology.Node, 0, 1+input.GetReplicas()+input.GetHaproxy()),
			Components:  make([]*topology.Component, 0, postgresComponentCapacity(input, hasPatroni, hasEtcd)),
			Connections: make([]*topology.Connection, 0),
			Labels: map[string]string{
				"kind":          "database",
				"engine":        postgresEngine,
				"replicas":      strconv.FormatUint(uint64(input.GetReplicas()), 10),
				"haproxy":       strconv.FormatUint(uint64(input.GetHaproxy()), 10),
				"pgbouncer":     strconv.FormatBool(input.GetPgbouncer()),
				"patroni":       strconv.FormatBool(hasPatroni),
				"etcd":          strconv.FormatBool(hasEtcd),
				"sync_replicas": strconv.FormatUint(uint64(input.GetSyncReplicas()), 10),
			},
			Tags: tags(postgresEngine),
		},
	}
}

func (b *postgresSpecBuilder) build() *topology.TopologySpec {
	b.addPostgresNode("postgres-master", postgresRoleMaster, 0)

	for i := uint32(1); i <= b.input.GetReplicas(); i++ {
		replicaID := fmt.Sprintf("postgres-replica-%d", i)
		b.addPostgresNode(replicaID, postgresRoleReplica, i)
		b.addConnection(replicaID, "postgres-master", topology.Connection_KIND_REPLICATION, topology.Connection_PROTOCOL_REPLICATION, topology.Connection_MODE_STREAM, "postgres", 5432, false)
	}

	b.addPatroniConnections()
	b.addEtcdPeerConnections()
	b.addHaproxyNodes()

	return b.spec
}

func (b *postgresSpecBuilder) addPostgresNode(nodeID, role string, ordinal uint32) {
	componentIDs := []string{nodeID}

	b.spec.Components = append(b.spec.Components, component(
		nodeID,
		postgresComponentKind(role),
		role,
		nodeID,
	))
	b.pgComponentIDs = append(b.pgComponentIDs, nodeID)

	if b.input.GetPgbouncer() {
		pgbouncerID := componentID(nodeID, postgresRolePgbouncer)
		componentIDs = append(componentIDs, pgbouncerID)
		b.spec.Components = append(b.spec.Components, component(
			pgbouncerID,
			topology.Component_KIND_PROXY,
			postgresRolePgbouncer,
			nodeID,
		))
		b.addConnection(pgbouncerID, nodeID, topology.Connection_KIND_PROXY, topology.Connection_PROTOCOL_POOL, topology.Connection_MODE_REQUEST, "postgres", 5432, true)
	}

	if b.hasPatroni() {
		patroniID := componentID(nodeID, postgresRolePatroni)
		componentIDs = append(componentIDs, patroniID)
		b.patroniComponents = append(b.patroniComponents, patroniID)
		b.spec.Components = append(b.spec.Components, component(
			patroniID,
			topology.Component_KIND_COORDINATOR,
			postgresRolePatroni,
			nodeID,
		))
		b.addConnection(patroniID, nodeID, topology.Connection_KIND_SUPPORT, topology.Connection_PROTOCOL_CONTROL, topology.Connection_MODE_SYNC, "patroni_rest", 8008, true)
	}

	if b.hasEtcd() && len(b.etcdComponents) < 3 {
		etcdID := componentID(nodeID, postgresRoleEtcd)
		componentIDs = append(componentIDs, etcdID)
		b.etcdComponents = append(b.etcdComponents, etcdID)
		b.spec.Components = append(b.spec.Components, component(
			etcdID,
			topology.Component_KIND_COORDINATOR,
			postgresRoleEtcd,
			nodeID,
		))
	}

	b.spec.Nodes = append(b.spec.Nodes, node(nodeID, role, ordinal, componentIDs))
}

func (b *postgresSpecBuilder) addPatroniConnections() {
	if len(b.patroniComponents) == 0 || len(b.etcdComponents) == 0 {
		return
	}

	for _, patroniID := range b.patroniComponents {
		for _, etcdID := range b.etcdComponents {
			b.addConnection(
				patroniID,
				etcdID,
				topology.Connection_KIND_COORDINATION,
				topology.Connection_PROTOCOL_CONTROL,
				topology.Connection_MODE_HEARTBEAT,
				"etcd_client",
				2379,
				componentNodeID(patroniID) == componentNodeID(etcdID),
			)
		}
	}
}

func (b *postgresSpecBuilder) addEtcdPeerConnections() {
	for i, from := range b.etcdComponents {
		for _, to := range b.etcdComponents[i+1:] {
			b.addConnection(from, to, topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_CONTROL, topology.Connection_MODE_SYNC, "etcd_peer", 2380, false)
			b.addConnection(to, from, topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_CONTROL, topology.Connection_MODE_SYNC, "etcd_peer", 2380, false)
		}
	}
}

func (b *postgresSpecBuilder) addHaproxyNodes() {
	for i := uint32(1); i <= b.input.GetHaproxy(); i++ {
		haproxyID := fmt.Sprintf("haproxy-%d", i)
		b.spec.Components = append(b.spec.Components, component(
			haproxyID,
			topology.Component_KIND_PROXY,
			postgresRoleHaproxy,
			haproxyID,
		))
		b.spec.Nodes = append(b.spec.Nodes, node(haproxyID, postgresRoleHaproxy, i, []string{haproxyID}))

		for _, pgComponentID := range b.pgComponentIDs {
			targetID := pgComponentID
			protocol := topology.Connection_PROTOCOL_TCP
			endpoint := "postgres"
			port := uint32(5432)
			if b.input.GetPgbouncer() {
				targetID = componentID(pgComponentID, postgresRolePgbouncer)
				protocol = topology.Connection_PROTOCOL_POOL
				endpoint = "pgbouncer"
				port = 6432
			}
			b.addConnection(haproxyID, targetID, topology.Connection_KIND_PROXY, protocol, topology.Connection_MODE_REQUEST, endpoint, port, false)
		}
	}
}

func (b *postgresSpecBuilder) addConnection(from, to string, kind topology.Connection_Kind, protocol topology.Connection_Protocol, mode topology.Connection_Mode, endpoint string, port uint32, colocated bool) {
	b.spec.Connections = append(b.spec.Connections, connection(from, to, kind, protocol, mode, endpoint, port, colocated))
}

func (b *postgresSpecBuilder) hasPatroni() bool {
	return b.input.GetPatroni() || b.input.GetSyncReplicas() > 0
}

func (b *postgresSpecBuilder) hasEtcd() bool {
	return b.input.GetEtcd() || b.hasPatroni()
}

func postgresComponentCapacity(input *domain.PostgresParams, hasPatroni, hasEtcd bool) uint32 {
	pgNodes := uint32(1) + input.GetReplicas()
	perPGNode := uint32(1)
	if input.GetPgbouncer() {
		perPGNode++
	}
	if hasPatroni {
		perPGNode++
	}

	etcdComponents := uint32(0)
	if hasEtcd {
		etcdComponents = minUint32(pgNodes, 3)
	}

	return pgNodes*perPGNode + etcdComponents + input.GetHaproxy()
}

func postgresComponentKind(role string) topology.Component_Kind {
	if role == postgresRoleReplica {
		return topology.Component_KIND_REPLICA
	}
	return topology.Component_KIND_DATABASE
}

func component(id string, kind topology.Component_Kind, role, nodeID string) *topology.Component {
	return &topology.Component{
		Id:     id,
		Kind:   kind,
		Engine: postgresEngine,
		Role:   role,
		Labels: map[string]string{
			"engine":  postgresEngine,
			"role":    role,
			"node_id": nodeID,
		},
		Tags: tags(postgresEngine, role),
	}
}

func node(id, role string, ordinal uint32, componentIDs []string) *topology.Node {
	return &topology.Node{
		Id:           id,
		ComponentIds: componentIDs,
		Labels: map[string]string{
			"engine":  postgresEngine,
			"role":    role,
			"ordinal": strconv.FormatUint(uint64(ordinal), 10),
		},
		Tags: tags(postgresEngine, role),
	}
}

func connection(from, to string, kind topology.Connection_Kind, protocol topology.Connection_Protocol, mode topology.Connection_Mode, endpoint string, port uint32, colocated bool) *topology.Connection {
	return &topology.Connection{
		FromComponentId: from,
		ToComponentId:   to,
		Kind:            kind,
		Protocol:        protocol,
		Mode:            mode,
		EndpointName:    endpoint,
		Port:            &port,
		Colocated:       colocated,
		Tags:            tags(postgresEngine),
	}
}

func configPath(componentID, role string) string {
	return "/etc/stroppy-cloud/" + componentID + "/" + configFileName(role)
}

func configFileName(role string) string {
	switch role {
	case postgresRoleHaproxy:
		return "haproxy.cfg"
	case postgresRolePgbouncer:
		return "pgbouncer.ini"
	case postgresRolePatroni:
		return "patroni.yml"
	case postgresRoleEtcd:
		return "etcd.conf"
	default:
		return "postgresql.conf"
	}
}

func configContent(role string, options map[string]string) string {
	switch role {
	case postgresRolePgbouncer:
		return "[pgbouncer]\n" + renderOptions(options, " = ")
	case postgresRolePatroni:
		return renderOptions(options, ": ")
	default:
		return renderOptions(options, " = ")
	}
}

func renderOptions(options map[string]string, separator string) string {
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s%s%s\n", key, separator, options[key])
	}
	return b.String()
}

func componentID(nodeID, componentName string) string {
	return fmt.Sprintf("%s-%s", nodeID, componentName)
}

func componentNodeID(componentID string) string {
	for _, suffix := range []string{
		"-" + postgresRolePgbouncer,
		"-" + postgresRolePatroni,
		"-" + postgresRoleEtcd,
	} {
		if strings.HasSuffix(componentID, suffix) {
			return strings.TrimSuffix(componentID, suffix)
		}
	}
	return componentID
}

func tags(values ...string) *common.Tags {
	return &common.Tags{Tags: values}
}

func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
