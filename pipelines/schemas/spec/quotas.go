package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Quotas is spec.quotas@1 — the params of the stroppy-quotas pipeline, which
// reads the provider's own limits and usage with the tenant's credentials.
//
// doc: STROPPY.MD §16.3 — the result is cached by the server and consulted
// before a launch; a cache older than a minute triggers a fresh run.
func Quotas() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("quotas", 1)).
		Descr("Params of stroppy-quotas: read the provider limits and usage of one profile.").
		Strict().Coerce().
		Fields(
			providerKind(),
			schemapb.JSON("settings").Title("Settings").Group("Provider").
				Desc("Baked provider.<kind>.settings value; picks the folder or account to read.").Required(),
			schemapb.Str("credentials_secret").Title("Credentials secret").Group("Provider").
				Desc("Name of the Graphene secret holding the credentials — never the value.").
				Pattern(secretNamePattern).Required(),
			schemapb.Str("location").Title("Location").Group("Provider").
				Desc("Zone (yandex) or region (aws) to read zone-scoped quotas for; empty reads all.").
				MaxLen(64),
		).
		MustBuild()
}
