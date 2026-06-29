package pgnoop

import (
	"errors"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	pgnoopEngine   = "pgnoop"
	pgnoopRoleNode = "node"

	// pgPort is the PostgreSQL wire port the pg-noop blackhole listens on.
	pgPort = 5432

	nodeID = "pgnoop-1"

	// defaultPgNoopVersion is the pinned pg-noop release used when
	// DatabaseParams.version is empty. Bump alongside the binary cache upstream.
	defaultPgNoopVersion = "0.1.2"
	// downloadAsset is the static musl tarball published per release (cargo-dist).
	downloadAsset = "pg-noop-x86_64-unknown-linux-musl.tar.xz"
)

type Database struct{}

func (d *Database) ValidateInput(input *domain.PgNoopParams) error {
	if input == nil {
		return errors.New("pg_noop params are required")
	}
	return input.Validate()
}

// BuildTopologySpec builds the single blackhole node. pg-noop has no replication,
// HA or persistence: it is one node fronting nothing, speaking plain pg-wire.
func (d *Database) BuildTopologySpec(input *domain.PgNoopParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	return &topology.TopologySpec{
		Nodes:      []*topology.Node{dbspec.Node(pgnoopEngine, nodeID, pgnoopRoleNode, 1, []string{nodeID})},
		Components: []*topology.Component{dbspec.Component(pgnoopEngine, nodeID, topology.Component_KIND_DATABASE, pgnoopRoleNode, nodeID)},
		Labels: map[string]string{
			"kind":   "database",
			"engine": pgnoopEngine,
			"nodes":  "1",
		},
		Tags: dbspec.Tags(pgnoopEngine),
	}, nil
}
