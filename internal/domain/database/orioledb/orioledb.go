package orioledb

import (
	"errors"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	orioledbEngine     = "orioledb"
	orioledbRoleMaster = "master"

	// pgPort is the PostgreSQL wire port exposed by the OrioleDB container.
	pgPort = 5432

	// defaultImage is used when OrioledbParams.image is empty.
	defaultImage = "orioledb/orioledb:latest-pg17"
)

type Database struct{}

func (d *Database) ValidateInput(input *domain.OrioledbParams) error {
	if input == nil {
		return errors.New("orioledb params are required")
	}
	return input.Validate()
}

// BuildTopologySpec returns a single-node topology: one OrioleDB container.
// OrioleDB has no upstream HA yet, so there are no replicas or coordinators.
func (d *Database) BuildTopologySpec(input *domain.OrioledbParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	id := "orioledb-master-1"
	spec := &topology.TopologySpec{
		Nodes: []*topology.Node{
			dbspec.Node(orioledbEngine, id, orioledbRoleMaster, 1, []string{id}),
		},
		Components: []*topology.Component{
			dbspec.Component(orioledbEngine, id, topology.Component_KIND_DATABASE, orioledbRoleMaster, id),
		},
		Connections: []*topology.Connection{},
		Labels: map[string]string{
			"kind":   "database",
			"engine": orioledbEngine,
			"nodes":  "1",
		},
		Tags: dbspec.Tags(orioledbEngine),
	}
	return spec, nil
}
