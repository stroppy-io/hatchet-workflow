// Package mariadb defines the PURE MariaDB database schema.
//
// It is provider-agnostic and machine-agnostic: topology is LOGICAL (roles +
// counts + feature flags), and there is nothing here about providers, machines,
// disks, ports or placement. Those belong to the provider schema and the
// top-level cluster schema, which composes this schema and owns every
// cross-field / machine / quota / placement rule (e.g. the galera-XOR-semisync
// constraint, server-id allocation, wsrep cluster addresses).
//
// It mirrors the MySQL schema where the my.cnf surface is shared and diverges
// where MariaDB is genuinely different: Galera (wsrep) instead of InnoDB Group
// Replication, and no caching_sha2_password default auth plugin.
//
// Composition note: schemapb `root` is always the top-level form. When this
// schema is embedded under another (e.g. at root.database), its internal When
// gates must address root.database.<field>, not root.<field>. So the builders
// take a rootPrefix ("" standalone, "database." embedded) and all gate paths are
// built through rp(). MariaDBInputSchema() is the standalone (prefix "") form.
package mariadb

import (
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.DatabaseNamespace
	schemaName = types.SchemaName("mariadb")
	schemaVer  = "1.0.0"
)

// rp builds an expr path to a field of THIS schema, honoring the embed prefix:
// rp("", "topology") -> "root.topology"; rp("database.", "replication","mode") ->
// "root.database.replication.mode".
func rp(rootPrefix string, parts ...string) string {
	return "root." + rootPrefix + strings.Join(parts, ".")
}

// Validate the standalone schema at package init (MustBuild panics if malformed).
var _ = MariaDBInputSchema()

// Identity is this schema's identity (informational; the cluster schema embeds
// the prefixed form via ObjectOf rather than RefID, so gates resolve correctly).
func Identity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: schemaName, Version: schemaVer}
}

// MariaDBInputSchema is the standalone, provider-agnostic MariaDB schema
// (root = the mariadb form). Used for domain.Database.params / presets.
// Discovered by `make schemas` (schemapbgen) via the name marker below.
//
//schemapbgen:name MariaDBConfig
func MariaDBInputSchema() *schemapb.Schema { return MariaDBSchema("") }

// MariaDBSchema builds the schema with a given root prefix so it can be embedded
// under a parent (the cluster schema passes "database.").
func MariaDBSchema(rootPrefix string) *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, schemaName, schemaVer).
		Descr("MariaDB database configuration (provider-agnostic): logical topology, "+
			"replication (async / semi-sync / Galera), ProxySQL routing intent, "+
			"my.cnf server tuning, and authentication.").
		Fields(topologyFields()...).
		Fields(
			replicationSection(rootPrefix),
			proxySection(rootPrefix),
			tuningSection(),
			authSection(rootPrefix),
			routingSection(rootPrefix),
		).
		Rules(dbRules(rootPrefix)...).
		MustBuild()
}
