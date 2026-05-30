// Package workload defines the PURE stroppy WORKLOAD schema — the script /
// execution half of a test, alongside (and independent of) the database schema.
//
// It is provider-agnostic AND db-agnostic: there is nothing here about machines,
// providers, placement, or database kinds/protocols. Script↔kind and
// protocol↔kind compatibility is a CROSS-schema wizard concern and lives nowhere
// in this package. Only stroppy's own k6/runner knobs and their value ranges do.
//
// Composition note: schemapb `root` is always the top-level form. When this
// schema is embedded under another (e.g. at root.workload), its internal When
// gates must address root.workload.<field>, not root.<field>. So the builders
// take a rootPrefix ("" standalone, "workload." embedded) and all gate paths are
// built through rp(). WorkloadInputSchema() is the standalone (prefix "") form.
package workload

import (
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.WorkloadNamespace
	schemaName = types.SchemaName("stroppy")
	schemaVer  = "1.0.0"
)

// rp builds an expr path to a field of THIS schema, honoring the embed prefix:
// rp("", "k6_mode") -> "root.k6_mode"; rp("workload.", "k6_mode") ->
// "root.workload.k6_mode".
func rp(rootPrefix string, parts ...string) string {
	return "root." + rootPrefix + strings.Join(parts, ".")
}

// Validate the standalone schema at package init (MustBuild panics if malformed).
var _ = WorkloadInputSchema()

// Identity is this schema's identity (informational; a composing schema embeds
// the prefixed form via ObjectOf rather than RefID, so gates resolve correctly).
func Identity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: schemaName, Version: schemaVer}
}

// WorkloadInputSchema is the standalone, provider- and db-agnostic stroppy
// workload schema (root = the workload form). Used for run-presets / wizard.
// Discovered by `make schemas` (schemapbgen) via the name marker below.
//
//schemapbgen:name WorkloadConfig
func WorkloadInputSchema() *schemapb.Schema { return WorkloadSchema("") }

// WorkloadSchema builds the schema with a given root prefix so it can be
// embedded under a parent (a composing schema passes "workload.").
func WorkloadSchema(rootPrefix string) *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, schemaName, schemaVer).
		Descr("Stroppy workload configuration (provider- and db-agnostic): script, " +
			"k6 execution bounds (duration/iterations/vus), sizing (scale_factor/pool_size), " +
			"k6 flags, data-loading method, env overrides, run-scoped files and steps.").
		Fields(workloadFields(rootPrefix)...).
		Rules(workloadRules(rootPrefix)...).
		MustBuild()
}
