// Package cockroach defines the PURE CockroachDB database schema.
//
// It is provider-agnostic and machine-agnostic: topology is LOGICAL (shape +
// node count), and there is nothing here about providers, machines, disks,
// ports or placement. Those belong to the provider schema and the top-level
// cluster schema, which composes this schema and owns every cross-field /
// machine / quota / placement rule.
//
// CockroachDB is a homogeneous, symmetric distributed SQL store: every node is
// identical (no master/replica roles), nodes form a cluster via gossip --join,
// and durability/availability come from per-range replication governed by the
// default zone replication factor. The engine surface here is therefore small:
// version, node count, replication factor, the two memory budgets
// (--cache / --max-sql-memory), security posture, clock skew, and a free-form
// `SET CLUSTER SETTING` escape hatch.
//
// Composition note: schemapb `root` is always the top-level form. When this
// schema is embedded under another (e.g. at root.database), its internal When
// gates must address root.database.<field>, not root.<field>. So the builders
// take a rootPrefix ("" standalone, "database." embedded) and all gate paths are
// built through rp(). CockroachInputSchema() is the standalone (prefix "") form.
package cockroach

import (
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.DatabaseNamespace
	schemaName = types.SchemaName("cockroach")
	schemaVer  = "1.0.0"
)

// rp builds an expr path to a field of THIS schema, honoring the embed prefix:
// rp("", "topology") -> "root.topology"; rp("database.", "cluster","cache") ->
// "root.database.cluster.cache".
func rp(rootPrefix string, parts ...string) string {
	return "root." + rootPrefix + strings.Join(parts, ".")
}

// Validate the standalone schema at package init (MustBuild panics if malformed).
var _ = CockroachInputSchema()

// Identity is this schema's identity (informational; the cluster schema embeds
// the prefixed form via ObjectOf rather than RefID, so gates resolve correctly).
func Identity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: schemaName, Version: schemaVer}
}

// CockroachInputSchema is the standalone, provider-agnostic CockroachDB schema
// (root = the cockroach form). Used for domain.Database.params / presets.
// Discovered by `make schemas` (schemapbgen) via the name marker below.
//
//schemapbgen:name CockroachConfig
func CockroachInputSchema() *schemapb.Schema { return CockroachSchema("") }

// CockroachSchema builds the schema with a given root prefix so it can be
// embedded under a parent (the cluster schema passes "database.").
func CockroachSchema(rootPrefix string) *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, schemaName, schemaVer).
		Descr("CockroachDB database configuration (provider-agnostic): logical topology " +
			"(homogeneous N nodes, no master/replica), default replication factor, memory " +
			"budgets (cache / max_sql_memory), security posture and a SET CLUSTER SETTING " +
			"escape hatch.").
		Fields(topologyFields()...).
		Fields(
			clusterSection(rootPrefix),
		).
		Rules(dbRules(rootPrefix)...).
		MustBuild()
}
