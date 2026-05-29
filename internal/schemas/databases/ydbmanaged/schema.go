// Package ydbmanaged defines the Yandex Managed Service for YDB database schema.
//
// It models the DATABASE-UNDER-TEST configuration the user chooses for a managed
// YDB offering: the serverless/dedicated flavor, RCU throttling, the dedicated
// resource preset, the fixed/auto scale policy and the storage config. Although
// Managed YDB is inherently a Yandex product, this schema is still a pure
// DATABASE schema — the client runner VM, terraform credentials, network/SA
// plumbing and runtime-filled endpoint/database_path are provider/cluster
// concerns and live in those schemas, which compose this one.
//
// Composition note: schemapb `root` is always the top-level form. When this
// schema is embedded under another (e.g. at root.database), its internal When
// gates must address root.database.<field>, not root.<field>. So the builders
// take a rootPrefix ("" standalone, "database." embedded) and all gate paths are
// built through rp(). YDBManagedInputSchema() is the standalone (prefix "") form.
package ydbmanaged

import (
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.DatabaseNamespace
	schemaName = types.SchemaName("ydb-managed")
	schemaVer  = "1.0.0"
)

// rp builds an expr path to a field of THIS schema, honoring the embed prefix:
// rp("", "type") -> "root.type"; rp("database.", "dedicated","scale") ->
// "root.database.dedicated.scale".
func rp(rootPrefix string, parts ...string) string {
	return "root." + rootPrefix + strings.Join(parts, ".")
}

// Validate the standalone schema at package init (MustBuild panics if malformed).
var _ = YDBManagedInputSchema()

// Identity is this schema's identity (informational; the cluster schema embeds
// the prefixed form via ObjectOf rather than RefID, so gates resolve correctly).
func Identity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: schemaName, Version: schemaVer}
}

// YDBManagedInputSchema is the standalone managed-YDB schema (root = the
// managed-YDB form). Used for domain.Database.params / presets. Discovered by
// `make schemas` (schemapbgen) via the name marker below.
//
//schemapbgen:name YDBManagedConfig
func YDBManagedInputSchema() *schemapb.Schema { return YDBManagedSchema("") }

// YDBManagedSchema builds the schema with a given root prefix so it can be
// embedded under a parent (the cluster schema passes "database.").
func YDBManagedSchema(rootPrefix string) *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, schemaName, schemaVer).
		Descr("Yandex Managed Service for YDB database configuration: serverless vs "+
			"dedicated flavor, RCU throttling, resource preset, fixed/auto scale policy "+
			"and storage config. Provider/cluster concerns (client VM, creds, network) "+
			"are owned by the composing schemas.").
		Fields(topologyFields()...).
		Fields(
			serverlessSection(rootPrefix),
			dedicatedSection(rootPrefix),
		).
		MustBuild()
}
