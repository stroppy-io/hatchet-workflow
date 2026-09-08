package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// providerKind is the provider selector shared by the service specs.
func providerKind() *schemapb.ChoiceB {
	return schemapb.Choice("provider").Title("Provider").Group("Provider").
		Desc("Cloud the credentials belong to.").
		Opt(schemapb.StrV("yandex"), "Yandex Cloud").
		Opt(schemapb.StrV("aws"), "AWS").
		Required()
}

// ProviderVerify is spec.provider_verify@1 — the params of the
// stroppy-provider-verify pipeline, which proves a tenant provider profile
// actually works before any run is allowed to use it.
//
// doc: STROPPY.MD §16.3 — a profile is verifying → ready | failed{reason}.
func ProviderVerify() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("provider_verify", 1)).
		Descr("Params of stroppy-provider-verify: can these credentials create and destroy resources?").
		Strict().Coerce().
		Fields(
			providerKind(),
			schemapb.JSON("settings").Title("Settings").Group("Provider").
				Desc("Baked provider.<kind>.settings value being verified.").Required(),
			schemapb.Str("credentials_secret").Title("Credentials secret").Group("Provider").
				Desc("Name of the Graphene secret holding the credentials — never the value.").
				Pattern(secretNamePattern).Required(),
			schemapb.Bool("dry_run").Title("Dry run").Group("Verification").
				Desc("Only read the account (identity, permissions); do not create a probe resource.").
				Default(true),
		).
		MustBuild()
}
