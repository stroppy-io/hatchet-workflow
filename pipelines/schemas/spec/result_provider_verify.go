package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// ResultProviderVerify is spec.result.provider_verify@1 — what the
// verification pipeline returns; the server turns it into the profile status.
func ResultProviderVerify() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("result.provider_verify", 1)).
		Descr("Result of stroppy-provider-verify: whether the profile is usable, and why not.").
		Strict().Coerce().
		Fields(
			schemapb.Bool("ok").Title("Usable").Group("Result").
				Desc("True when the credentials authenticate and carry the permissions a run needs.").
				Required(),
			schemapb.Str("account_id").Title("Account").Group("Result").
				Desc("Service account id (yandex) or IAM ARN / account id (aws) the credentials resolve to.").
				MaxLen(256),
			schemapb.Str("scope").Title("Scope").Group("Result").
				Desc("Folder id (yandex) or region/project the check ran against.").
				MaxLen(128),
			schemapb.List("permissions",
				schemapb.Object("",
					schemapb.Str("name").Title("Permission").
						Desc("Provider permission or IAM action that was probed.").
						MinLen(1).MaxLen(128).Required(),
					schemapb.Bool("granted").Title("Granted").Required(),
				).Strict(),
			).Title("Permissions").Group("Result").
				Desc("Per-permission verdict; a missing one is why ok is false.").
				MaxItems(64),
			schemapb.Str("error").Title("Error").Group("Result").
				Desc("Provider error text when ok is false; shown as the profile status reason.").
				MaxLen(4096),
		).
		Rules(schemapb.Rule(
			`!("ok" in root) || root.ok || ("error" in root)`,
			"a failed verification must say why",
		).ID("failure-has-reason")).
		MustBuild()
}
