package provider

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// YandexCredentials is provider.yandex.credentials@1 — the authorized key of
// the tenant's service account. Write-only: the server forwards it straight to
// a Graphene secret and never stores or returns it.
func YandexCredentials() *schemapb.Schema {
	return schemapb.NewSchema(ids.Provider("yandex", "credentials", 1)).
		Descr("Yandex Cloud service account authorized key (write-only).").
		Strict().Coerce().
		Fields(
			// doc: https://yandex.cloud/en/docs/iam/operations/authentication/manage-authorized-keys
			// — the key file has exactly six top-level fields: id,
			// service_account_id, created_at, key_algorithm, public_key,
			// private_key. Kept as a string so the JSON reaches the secret byte
			// for byte; the rule only checks that the three load-bearing keys are
			// present.
			schemapb.Str("sa_key_json").Title("Authorized key (JSON)").Group("Credentials").
				Desc("Contents of the authorized-key file created with `yc iam key create --output key.json`.").
				MinLen(64).MaxLen(1 << 16).Secret().Required().
				Rules(schemapb.Rule(
					`this.contains("service_account_id") && this.contains("private_key") && this.contains("\"id\"")`,
					"not a Yandex Cloud authorized key: id, service_account_id and private_key are required",
				).ID("sa-key-shape")),
		).
		MustBuild()
}
