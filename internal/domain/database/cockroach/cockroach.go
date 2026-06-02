package cockroach

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
	cockroachEngine = "cockroach"

	cockroachRoleNode = "node"

	// sqlPort is the CockroachDB SQL + node-to-node RPC port.
	sqlPort = 26257
	// httpPort is the CockroachDB admin UI / HTTP port.
	httpPort = 8080

	// flagPrefix marks an option that is a `cockroach start` startup flag
	// rather than a `SET CLUSTER SETTING`.
	flagPrefix = "flag:"
)

type Database struct{}

func (d *Database) ValidateInput(input *domain.CockroachParams) error {
	if input == nil {
		return errors.New("cockroach params are required")
	}
	return input.Validate()
}

func (d *Database) BuildTopologySpec(input *domain.CockroachParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}
	return newCockroachSpecBuilder(input).build(), nil
}

type cockroachSpecBuilder struct {
	input *domain.CockroachParams
	spec  *topology.TopologySpec
	nodes []string
}

func newCockroachSpecBuilder(input *domain.CockroachParams) *cockroachSpecBuilder {
	nodes := input.GetNodes()
	if nodes == 0 {
		nodes = 1
	}

	return &cockroachSpecBuilder{
		input: input,
		spec: &topology.TopologySpec{
			Nodes:       make([]*topology.Node, 0, nodes),
			Components:  make([]*topology.Component, 0, nodes),
			Connections: make([]*topology.Connection, 0),
			Labels: map[string]string{
				"kind":   "database",
				"engine": cockroachEngine,
				"nodes":  strconv.FormatUint(uint64(nodes), 10),
			},
			Tags: dbspec.Tags(cockroachEngine),
		},
	}
}

func (b *cockroachSpecBuilder) build() *topology.TopologySpec {
	nodes := b.input.GetNodes()
	if nodes == 0 {
		nodes = 1
	}

	for i := uint32(1); i <= nodes; i++ {
		id := fmt.Sprintf("cockroach-node-%d", i)
		b.spec.Components = append(b.spec.Components, dbspec.Component(cockroachEngine, id, topology.Component_KIND_DATABASE, cockroachRoleNode, id))
		b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(cockroachEngine, id, cockroachRoleNode, i, []string{id}))
		b.nodes = append(b.nodes, id)
	}

	// CockroachDB is homogeneous: every node after the first joins node-1.
	for _, id := range b.nodes[1:] {
		b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
			cockroachEngine, id, b.nodes[0],
			topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_SYNC,
			"rpc", sqlPort, false,
		))
	}
	return b.spec
}

// cockroachFlags renders the editable startup-flag file from `flag:`-prefixed
// options; one `--key=value` per line.
func cockroachFlags(options map[string]string) string {
	var b strings.Builder
	b.WriteString("# cockroach start startup flags (one per line)\n")
	keys := make([]string, 0, len(options))
	for key := range options {
		if strings.HasPrefix(key, flagPrefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "--%s=%s\n", strings.TrimPrefix(key, flagPrefix), options[key])
	}
	return b.String()
}

// cockroachClusterSettings renders the non-flag options as SET CLUSTER SETTING
// statements (applied post-init by the operator).
func cockroachClusterSettings(options map[string]string) string {
	keys := make([]string, 0, len(options))
	for key := range options {
		if !strings.HasPrefix(key, flagPrefix) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("-- applied after the cluster initializes\n")
	for _, key := range keys {
		fmt.Fprintf(&b, "SET CLUSTER SETTING %s = '%s';\n", key, options[key])
	}
	return b.String()
}
