// Package ydb defines the PURE self-hosted YDB database schema.
//
// It is provider-agnostic and machine-agnostic: topology is LOGICAL (combined vs
// split + node counts + erasure/feature flags), and there is nothing here about
// providers, machines, disk HARDWARE (disk type catalog + sizing), VM sizing, or
// placement. Those belong to the provider schema and the top-level cluster
// schema, which composes this schema and owns every cross-field / machine /
// quota / placement / zone rule (e.g. mirror-3-dc => >=3 storage nodes across 3
// data centers, and pdisk SIZE auto-calculation).
//
// Composition note: schemapb `root` is always the top-level form. When this
// schema is embedded under another (e.g. at root.database), its internal When
// gates must address root.database.<field>, not root.<field>. So the builders
// take a rootPrefix ("" standalone, "database." embedded) and all gate paths are
// built through rp(). YDBInputSchema() is the standalone (prefix "") form.
package ydb

import (
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.DatabaseNamespace
	schemaName = types.SchemaName("ydb")
	schemaVer  = "1.0.0"
)

// rp builds an expr path to a field of THIS schema, honoring the embed prefix:
// rp("", "topology") -> "root.topology"; rp("database.", "storage","storage_groups")
// -> "root.database.storage.storage_groups".
func rp(rootPrefix string, parts ...string) string {
	return "root." + rootPrefix + strings.Join(parts, ".")
}

// Validate the standalone schema at package init (MustBuild panics if malformed).
var _ = YDBInputSchema()

// Identity is this schema's identity (informational; the cluster schema embeds
// the prefixed form via ObjectOf rather than RefID, so gates resolve correctly).
func Identity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: schemaName, Version: schemaVer}
}

// YDBInputSchema is the standalone, provider-agnostic self-hosted YDB schema
// (root = the ydb form). Used for domain.Database.params / presets.
// Discovered by `make schemas` (schemapbgen) via the name marker below.
//
//schemapbgen:name YDBConfig
func YDBInputSchema() *schemapb.Schema { return YDBSchema("") }

// YDBSchema builds the schema with a given root prefix so it can be embedded
// under a parent (the cluster schema passes "database.").
func YDBSchema(rootPrefix string) *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, schemaName, schemaVer).
		Descr("Self-hosted YDB database configuration (provider-agnostic): logical "+
			"topology (combined vs split), blob-storage layout & fault tolerance, "+
			"protocol/port surface, and server config (memory, security, actor system).").
		Fields(topologyFields(rootPrefix)...).
		Fields(
			storageSection(),
			protocolsSection(rootPrefix),
			configSection(),
		).
		Rules(dbRules(rootPrefix)...).
		MustBuild()
}
