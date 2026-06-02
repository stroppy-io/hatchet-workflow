package postgres

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestPostgresBuildTopologySpec(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&domain.PostgresParams{
		Replicas:  2,
		Haproxy:   1,
		Pgbouncer: true,
		Patroni:   true,
		Etcd:      true,
		MasterOptions: map[string]string{
			"max_connections": "200",
		},
		PgbouncerOptions: map[string]string{
			"pool_mode": "transaction",
		},
	})
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("spec is invalid: %v", err)
	}

	if got, want := len(spec.GetNodes()), 4; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}

	wantComponents := map[string]struct {
		kind topology.Component_Kind
		role string
	}{
		"postgres-master":              {topology.Component_KIND_DATABASE, "master"},
		"postgres-master-pgbouncer":    {topology.Component_KIND_PROXY, "pgbouncer"},
		"postgres-master-patroni":      {topology.Component_KIND_COORDINATOR, "patroni"},
		"postgres-master-etcd":         {topology.Component_KIND_COORDINATOR, "etcd"},
		"postgres-replica-1":           {topology.Component_KIND_REPLICA, "replica"},
		"postgres-replica-1-pgbouncer": {topology.Component_KIND_PROXY, "pgbouncer"},
		"postgres-replica-1-patroni":   {topology.Component_KIND_COORDINATOR, "patroni"},
		"postgres-replica-1-etcd":      {topology.Component_KIND_COORDINATOR, "etcd"},
		"postgres-replica-2":           {topology.Component_KIND_REPLICA, "replica"},
		"postgres-replica-2-pgbouncer": {topology.Component_KIND_PROXY, "pgbouncer"},
		"postgres-replica-2-patroni":   {topology.Component_KIND_COORDINATOR, "patroni"},
		"postgres-replica-2-etcd":      {topology.Component_KIND_COORDINATOR, "etcd"},
		"haproxy-1":                    {topology.Component_KIND_PROXY, "haproxy"},
	}

	components := componentsByID(spec)
	for id, want := range wantComponents {
		got, ok := components[id]
		if !ok {
			t.Fatalf("component %q is missing", id)
		}
		if got.GetKind() != want.kind {
			t.Fatalf("component %q kind = %s, want %s", id, got.GetKind(), want.kind)
		}
		if got.GetEngine() != "postgres" {
			t.Fatalf("component %q engine = %q, want postgres", id, got.GetEngine())
		}
		if got.GetRole() != want.role {
			t.Fatalf("component %q role = %q, want %q", id, got.GetRole(), want.role)
		}
	}

	assertNodeComponents(t, spec, "postgres-master", []string{
		"postgres-master",
		"postgres-master-pgbouncer",
		"postgres-master-patroni",
		"postgres-master-etcd",
	})

	assertConnection(t, spec, "postgres-replica-1", "postgres-master", topology.Connection_KIND_REPLICATION, topology.Connection_PROTOCOL_REPLICATION, topology.Connection_MODE_STREAM, "postgres", 5432, false)
	assertConnection(t, spec, "postgres-master-pgbouncer", "postgres-master", topology.Connection_KIND_PROXY, topology.Connection_PROTOCOL_POOL, topology.Connection_MODE_REQUEST, "postgres", 5432, true)
	assertConnection(t, spec, "postgres-master-patroni", "postgres-master-etcd", topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_CONTROL, topology.Connection_MODE_HEARTBEAT, "etcd_client", 2379, true)
	assertConnection(t, spec, "haproxy-1", "postgres-master-pgbouncer", topology.Connection_KIND_PROXY, topology.Connection_PROTOCOL_POOL, topology.Connection_MODE_REQUEST, "pgbouncer", 6432, false)
}

func TestPostgresBuildTopologySpecDefaultSingleNode(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&domain.PostgresParams{})
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("spec is invalid: %v", err)
	}

	if got, want := len(spec.GetNodes()), 1; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	if got, want := len(spec.GetComponents()), 1; got != want {
		t.Fatalf("components = %d, want %d", got, want)
	}
	if got := len(spec.GetConnections()); got != 0 {
		t.Fatalf("connections = %d, want 0", got)
	}
}

func TestPostgresBuildTopologySpecSyncReplicasEnablePatroniAndEtcd(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&domain.PostgresParams{
		Replicas:     1,
		SyncReplicas: 1,
	})
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	components := componentsByID(spec)
	for _, id := range []string{
		"postgres-master-patroni",
		"postgres-master-etcd",
		"postgres-replica-1-patroni",
		"postgres-replica-1-etcd",
	} {
		if _, ok := components[id]; !ok {
			t.Fatalf("component %q is missing", id)
		}
	}
}

func TestPostgresValidateInputRejectsInvalidSyncReplicas(t *testing.T) {
	err := (&Database{}).ValidateInput(&domain.PostgresParams{
		Replicas:     1,
		SyncReplicas: 2,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func componentsByID(spec *topology.TopologySpec) map[string]*topology.Component {
	components := make(map[string]*topology.Component, len(spec.GetComponents()))
	for _, component := range spec.GetComponents() {
		components[component.GetId()] = component
	}
	return components
}

func assertNodeComponents(t *testing.T, spec *topology.TopologySpec, nodeID string, want []string) {
	t.Helper()

	for _, node := range spec.GetNodes() {
		if node.GetId() != nodeID {
			continue
		}
		if got := node.GetComponentIds(); !sameStrings(got, want) {
			t.Fatalf("node %q component ids = %v, want %v", nodeID, got, want)
		}
		return
	}

	t.Fatalf("node %q is missing", nodeID)
}

func assertConnection(
	t *testing.T,
	spec *topology.TopologySpec,
	from string,
	to string,
	kind topology.Connection_Kind,
	protocol topology.Connection_Protocol,
	mode topology.Connection_Mode,
	endpoint string,
	port uint32,
	colocated bool,
) {
	t.Helper()

	for _, connection := range spec.GetConnections() {
		if connection.GetFromComponentId() != from || connection.GetToComponentId() != to {
			continue
		}
		if connection.GetKind() != kind {
			t.Fatalf("connection %s -> %s kind = %s, want %s", from, to, connection.GetKind(), kind)
		}
		if connection.GetProtocol() != protocol {
			t.Fatalf("connection %s -> %s protocol = %s, want %s", from, to, connection.GetProtocol(), protocol)
		}
		if connection.GetMode() != mode {
			t.Fatalf("connection %s -> %s mode = %s, want %s", from, to, connection.GetMode(), mode)
		}
		if connection.GetEndpointName() != endpoint {
			t.Fatalf("connection %s -> %s endpoint = %q, want %q", from, to, connection.GetEndpointName(), endpoint)
		}
		if connection.GetPort() != port {
			t.Fatalf("connection %s -> %s port = %d, want %d", from, to, connection.GetPort(), port)
		}
		if connection.GetColocated() != colocated {
			t.Fatalf("connection %s -> %s colocated = %t, want %t", from, to, connection.GetColocated(), colocated)
		}
		return
	}

	t.Fatalf("connection %s -> %s is missing", from, to)
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
