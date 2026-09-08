package provider

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// RegistryCredentials is provider.registry.credentials@1 — a docker registry
// login, so a tenant can benchmark a private build of a database. Write-only:
// forwarded to a Graphene secret, referenced by name from the RunSpec.
func RegistryCredentials() *schemapb.Schema {
	return schemapb.NewSchema(ids.Provider("registry", "credentials", 1)).
		Descr("Docker registry login for private database images (write-only).").
		Strict().Coerce().
		Fields(
			// doc: https://distribution.github.io/distribution/spec/api/ — the
			// registry host of an image reference: host[:port], no scheme, no path.
			schemapb.Str("registry").Title("Registry").Group("Registry").
				Desc("Registry host as it appears in the image reference, e.g. registry.stroppy.io.").
				Pattern(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:\d{1,5})?$`).MaxLen(255).Required().
				Examples(schemapb.StrV("registry.stroppy.io"), schemapb.StrV("ghcr.io")),
			schemapb.Str("username").Title("Username").Group("Registry").
				Desc("Registry user; for a token-only registry use the vendor's placeholder user.").
				MinLen(1).MaxLen(128).Required(),
			schemapb.Str("password").Title("Password / token").Group("Registry").
				Desc("Registry password or access token.").
				MinLen(1).MaxLen(4096).Secret().Required(),
		).
		MustBuild()
}
