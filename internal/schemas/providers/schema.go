// Package providers defines the provider (deployment-backend) schemas.
//
// Each provider is its OWN registered schema, provider-agnostic databases know
// nothing about them. The provider schema owns exactly the facts a database is
// agnostic about: credentials, region/network, the disk-type catalog, compute
// platforms/shapes, zones and quota limits. The top-level (cluster) schema picks
// a provider, references it, and writes the machine/quota/placement rules that
// need BOTH the database and provider subtrees.
//
// These schemas are intentionally EMBED-SAFE: per-field constraints only, NO
// schema-level/cross-field `root.*` rules. (Under composition `root` re-points to
// the top form, so any internal `root.x` rule would misfire — all cross-field
// logic lives in the cluster schema.)
package providers

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

const (
	schemaNs   = types.ProviderNamespace
	schemaVer  = "1.0.0"
	nameYandex = types.SchemaName("yandex")
	nameDocker = types.SchemaName("docker")
)

// Validate both schemas at package init (MustBuild panics on a malformed descriptor).
var (
	_ = YandexProviderSchema()
	_ = DockerProviderSchema()
)

// YandexIdentity / DockerIdentity expose the provider schema identities for
// composition (RefID) by the cluster schema.
func YandexIdentity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: nameYandex, Version: schemaVer}
}

func DockerIdentity() *schemapb.SchemaIdentity {
	return &schemapb.SchemaIdentity{Namespace: schemaNs, Name: nameDocker, Version: schemaVer}
}
