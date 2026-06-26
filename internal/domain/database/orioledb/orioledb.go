package orioledb

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	orioledbEngine      = "orioledb"
	orioledbRoleMaster  = "master"
	orioledbRoleReplica = "replica"
	orioledbRoleHaproxy = "haproxy"

	// pgPort is the PostgreSQL wire port exposed by the OrioleDB container.
	pgPort = 5432
	// haproxyWritePort routes to the primary; haproxyReadPort to the replicas.
	haproxyWritePort = 5432
	haproxyReadPort  = 5433
	// healthPort serves the per-node primary/replica check HAProxy probes.
	healthPort = 8008

	// defaultImage is used when OrioledbParams.image is empty.
	defaultImage = "orioledb/orioledb:latest-pg17"

	masterID  = "orioledb-master-1"
	haproxyID = "orioledb-haproxy-1"
)

type Database struct{}

func (d *Database) ValidateInput(input *domain.OrioledbParams) error {
	if input == nil {
		return errors.New("orioledb params are required")
	}
	return input.Validate()
}

// BuildTopologySpec builds a master + N streaming replicas, optionally fronted
// by one HAProxy. OrioleDB has no upstream auto-failover yet (Patroni is phase
// 2); replicas are read-scale streaming standbys.
func (d *Database) BuildTopologySpec(input *domain.OrioledbParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	replicas := input.GetReplicas()
	withHAProxy := input.GetHaproxy() > 0

	nodes := []*topology.Node{dbspec.Node(orioledbEngine, masterID, orioledbRoleMaster, 1, []string{masterID})}
	comps := []*topology.Component{dbspec.Component(orioledbEngine, masterID, topology.Component_KIND_DATABASE, orioledbRoleMaster, masterID)}
	var conns []*topology.Connection

	for i := uint32(1); i <= replicas; i++ {
		id := fmt.Sprintf("orioledb-replica-%d", i)
		nodes = append(nodes, dbspec.Node(orioledbEngine, id, orioledbRoleReplica, 1+i, []string{id}))
		comps = append(comps, dbspec.Component(orioledbEngine, id, topology.Component_KIND_DATABASE, orioledbRoleReplica, id))
		// replica streams FROM the master.
		conns = append(conns, dbspec.Connection(orioledbEngine, id, masterID,
			topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_STREAM,
			"rep", pgPort, false))
	}

	if withHAProxy {
		nodes = append(nodes, dbspec.Node(orioledbEngine, haproxyID, orioledbRoleHaproxy, 100, []string{haproxyID}))
		comps = append(comps, dbspec.Component(orioledbEngine, haproxyID, topology.Component_KIND_DATABASE, orioledbRoleHaproxy, haproxyID))
		conns = append(conns, dbspec.Connection(orioledbEngine, haproxyID, masterID,
			topology.Connection_KIND_FLOW, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_SYNC,
			"lb", pgPort, false))
		for i := uint32(1); i <= replicas; i++ {
			conns = append(conns, dbspec.Connection(orioledbEngine, haproxyID, fmt.Sprintf("orioledb-replica-%d", i),
				topology.Connection_KIND_FLOW, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_SYNC,
				"lb", pgPort, false))
		}
	}

	return &topology.TopologySpec{
		Nodes:       nodes,
		Components:  comps,
		Connections: conns,
		Labels: map[string]string{
			"kind":   "database",
			"engine": orioledbEngine,
			"nodes":  strconv.FormatInt(int64(len(nodes)), 10),
		},
		Tags: dbspec.Tags(orioledbEngine),
	}, nil
}
