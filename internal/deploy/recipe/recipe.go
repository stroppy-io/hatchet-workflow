// Package recipe is the per-DB deploy engine: it turns a baked database config
// plus a logical role into the per-component deploy recipe — the configuration
// files to lay down and the install/config commands to run — and wires the
// component connections.
//
// This is the rebuild of the old task_postgres/dbconfig, simplified for a
// single-node docker demo: one postgres node (topology=single) deployed inside
// one privileged agent container. The agent runs the emitted commands via
// Temporal activities. Runtime IPs/ports are not known at recipe-build time, so
// commands and files use placeholders (PlaceholderDBHost, PlaceholderDBPort)
// that the workflow substitutes from the deployed topology before execution.
package recipe

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// Placeholders substituted by the workflow once runtime IPs/ports are known.
const (
	PlaceholderDBHost = "__DB_HOST__"
	PlaceholderDBPort = "__DB_PORT__"
)

// Refs are runtime references passed into the recipe: artifacts the recipe must
// point at but does not itself produce (e.g. the stroppy binary served from the
// server/minio).
type Refs struct {
	// StroppyBinaryURL is the URL the workload component downloads the stroppy
	// runner binary from (server/minio).
	StroppyBinaryURL string
	// StroppyChecksum optionally verifies the downloaded stroppy binary.
	StroppyChecksum string
}

// Recipe is the per-DB deploy engine. BuildComponent produces the deploy
// strategy (config files + commands) for one component given its role; Wire
// produces the connections between deployed components.
type Recipe interface {
	// BuildComponent returns the deploy strategy for the component playing the
	// given role. db is the baked database config (schema values, as a map);
	// wl is the baked workload config (may be nil); refs are runtime references.
	BuildComponent(role string, db map[string]any, wl map[string]any, refs Refs) *topology.Component_Strategy
	// Wire returns the connections between the deployed components, keyed by the
	// component ids grouped per role.
	Wire(componentIDsByRole map[string][]string) []*topology.Connection
}

// recipes is the per-kind registry (mirrors schemas/expand). Add new engines by
// registering them here.
var recipes = map[string]Recipe{
	"postgres":  pgRecipe{},
	"cockroach": crRecipe{},
	"mysql":     myRecipe{maria: false},
	"mariadb":   myRecipe{maria: true},
	"picodata":  pdRecipe{},
	"ydb":       ydbRecipe{},
}

// Build dispatches to the recipe for the given database kind and builds the
// component strategy for role. Returns nil when no recipe is registered for the
// kind.
func Build(kind, role string, db map[string]any, wl map[string]any, refs Refs) *topology.Component_Strategy {
	r, ok := recipes[kind]
	if !ok {
		return nil
	}
	return r.BuildComponent(role, db, wl, refs)
}

// Wire dispatches to the recipe for the given database kind and wires the
// component connections. Returns nil when no recipe is registered for the kind.
func Wire(kind string, componentIDsByRole map[string][]string) []*topology.Connection {
	r, ok := recipes[kind]
	if !ok {
		return nil
	}
	return r.Wire(componentIDsByRole)
}
