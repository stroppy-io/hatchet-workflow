// Package picodata defines the PURE Picodata database schema.
//
// It is provider-agnostic and machine-agnostic: topology is LOGICAL (instance
// counts + replication + tiers + sharding + feature flags), and there is nothing
// here about providers, machines, disks, bind addresses or placement. Those
// belong to the provider schema and the top-level cluster schema, which composes
// this schema and owns every cross-field / machine / quota / placement rule.
//
// Composition note: schemapb `root` is always the top-level form. When this
// schema is embedded under another (e.g. at root.database), its internal When
// gates must address root.database.<field>, not root.<field>. So the builders
// take a rootPrefix ("" standalone, "database." embedded) and all gate paths are
// built through rp(). PicodataInputSchema() is the standalone (prefix "") form.
package picodata

import (
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.DatabaseNamespace
	schemaName = types.SchemaName("picodata")
	schemaVer  = "1.0.0"
)

// rp builds an expr path to a field of THIS schema, honoring the embed prefix:
// rp("", "topology") -> "root.topology"; rp("database.", "config","pg_enabled")
// -> "root.database.config.pg_enabled".
func rp(rootPrefix string, parts ...string) string {
	return "root." + rootPrefix + strings.Join(parts, ".")
}

// Validate the standalone schema at package init (MustBuild panics if malformed).
var _ = PicodataInputSchema()

// Identity is this schema's identity (informational; the cluster schema embeds
// the prefixed form via ObjectOf rather than RefID, so gates resolve correctly).
func Identity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: schemaName, Version: schemaVer}
}

// PicodataInputSchema is the standalone, provider-agnostic Picodata schema
// (root = the picodata form). Used for domain.Database.params / presets.
// Discovered by `make schemas` (schemapbgen) via the name marker below.
//
//schemapbgen:name PicodataConfig
func PicodataInputSchema() *schemapb.Schema { return PicodataSchema("") }

// PicodataSchema builds the schema with a given root prefix so it can be embedded
// under a parent (the cluster schema passes "database.").
func PicodataSchema(rootPrefix string) *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, schemaName, schemaVer).
		Descr("Picodata database configuration (provider-agnostic): logical topology, "+
			"replication, tiers, sharding and instance (picodata.yaml) tuning.").
		Fields(topologyFields()...).
		Fields(
			shardingSection(rootPrefix),
			tiersSection(rootPrefix),
			configSection(rootPrefix),
		).
		Rules(dbRules(rootPrefix)...).
		MustBuild()
}
