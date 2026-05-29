// Package postgres defines the PURE PostgreSQL database schema.
//
// It is provider-agnostic and machine-agnostic: topology is LOGICAL (roles +
// counts + feature flags), and there is nothing here about providers, machines,
// disks, ports or placement. Those belong to the provider schema and the
// top-level cluster schema, which composes this schema and owns every
// cross-field / machine / quota / placement rule.
//
// Composition note: schemapb `root` is always the top-level form. When this
// schema is embedded under another (e.g. at root.database), its internal When
// gates must address root.database.<field>, not root.<field>. So the builders
// take a rootPrefix ("" standalone, "database." embedded) and all gate paths are
// built through rp(). PostgresInputSchema() is the standalone (prefix "") form.
package postgres

import (
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.DatabaseNamespace
	schemaName = types.SchemaName("postgres")
	schemaVer  = "1.0.0"
)

// rp builds an expr path to a field of THIS schema, honoring the embed prefix:
// rp("", "topology") -> "root.topology"; rp("database.", "ha","dcs") ->
// "root.database.ha.dcs".
func rp(rootPrefix string, parts ...string) string {
	return "root." + rootPrefix + strings.Join(parts, ".")
}

// Validate the standalone schema at package init (MustBuild panics if malformed).
var _ = PostgresInputSchema()

// Identity is this schema's identity (informational; the cluster schema embeds
// the prefixed form via ObjectOf rather than RefID, so gates resolve correctly).
func Identity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: schemaName, Version: schemaVer}
}

// PostgresInputSchema is the standalone, provider-agnostic PostgreSQL schema
// (root = the postgres form). Used for domain.Database.params / presets.
// Discovered by `make schemas` (schemapbgen) via the name marker below.
//
//schemapbgen:name PostgresConfig
func PostgresInputSchema() *schemapb.Schema { return PostgresSchema("") }

// PostgresSchema builds the schema with a given root prefix so it can be embedded
// under a parent (the cluster schema passes "database.").
func PostgresSchema(rootPrefix string) *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, schemaName, schemaVer).
		Descr("PostgreSQL database configuration (provider-agnostic): logical topology, "+
			"HA, replication, pooling, routing intent, server tuning, auth, extensions.").
		Fields(topologyFields()...).
		Fields(
			haSection(rootPrefix),
			replicationSection(rootPrefix),
			poolingSection(rootPrefix),
			routingSection(rootPrefix),
			tuningSection(),
			authSection(),
			extensionsSection(),
		).
		Rules(dbRules(rootPrefix)...).
		MustBuild()
}
