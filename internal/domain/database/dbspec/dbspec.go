// Package dbspec holds the small, engine-agnostic constructors for building a
// cloud.v1.topology.TopologySpec: components, nodes, connections, tags, and a
// stable option-map renderer. Per-engine topology builders (picodata, ydb,
// cockroach, mysql, ...) compose these so every engine emits the same label and
// tag shape.
package dbspec

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// Tags wraps a string list into a common.Tags message.
func Tags(values ...string) *common.Tags {
	return &common.Tags{Tags: values}
}

// Component builds a topology component owned by the named engine.
func Component(engine, id string, kind topology.Component_Kind, role, nodeID string) *topology.Component {
	return &topology.Component{
		Id:     id,
		Kind:   kind,
		Engine: engine,
		Role:   role,
		Labels: map[string]string{
			"engine":  engine,
			"role":    role,
			"node_id": nodeID,
		},
		Tags: Tags(engine, role),
	}
}

// Node builds a topology node hosting the given component ids.
func Node(engine, id, role string, ordinal uint32, componentIDs []string) *topology.Node {
	return &topology.Node{
		Id:           id,
		ComponentIds: componentIDs,
		Labels: map[string]string{
			"engine":  engine,
			"role":    role,
			"ordinal": strconv.FormatUint(uint64(ordinal), 10),
		},
		Tags: Tags(engine, role),
	}
}

// Connection builds a topology connection between two components.
func Connection(
	engine, from, to string,
	kind topology.Connection_Kind,
	protocol topology.Connection_Protocol,
	mode topology.Connection_Mode,
	endpoint string,
	port uint32,
	colocated bool,
) *topology.Connection {
	return &topology.Connection{
		FromComponentId: from,
		ToComponentId:   to,
		Kind:            kind,
		Protocol:        protocol,
		Mode:            mode,
		EndpointName:    endpoint,
		Port:            &port,
		Colocated:       colocated,
		Tags:            Tags(engine),
	}
}

// ComponentID joins a node id and a component name into a stable component id.
func ComponentID(nodeID, componentName string) string {
	return fmt.Sprintf("%s-%s", nodeID, componentName)
}

// RenderOptions renders a sorted "key<separator>value" block; used for flat
// config-file passthroughs (e.g. my.cnf, haproxy.cfg).
func RenderOptions(options map[string]string, separator string) string {
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
