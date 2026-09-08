package provider

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// AwsCredentials is provider.aws.credentials@1 — an IAM access key of the
// tenant's account. Write-only: the server forwards it straight to a Graphene
// secret and never stores or returns it.
func AwsCredentials() *schemapb.Schema {
	return schemapb.NewSchema(ids.Provider("aws", "credentials", 1)).
		Descr("AWS IAM access key of the tenant account (write-only).").
		Strict().Coerce().
		Fields(
			// doc: https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_identifiers.html
			// — AKIA is a long-term access key, ASIA a temporary STS one. AWS
			// documents the prefixes but not the length; 20 characters is the
			// long-standing convention.
			schemapb.Str("access_key_id").Title("Access key id").Group("Credentials").
				Desc("IAM access key id; AKIA… for a long-term key, ASIA… for temporary STS credentials.").
				Pattern(`^(AKIA|ASIA)[A-Z0-9]{16}$`).Required().
				Examples(schemapb.StrV("AKIAIOSFODNN7EXAMPLE")),
			schemapb.Str("secret_access_key").Title("Secret access key").Group("Credentials").
				Desc("Secret half of the access key (40 base64 characters).").
				MinLen(16).MaxLen(128).Secret().Required(),
			// doc: same page — a session token accompanies ASIA… credentials only.
			schemapb.Str("session_token").Title("Session token").Group("Credentials").
				Desc("STS session token; required with temporary (ASIA…) credentials, absent otherwise.").
				MinLen(16).MaxLen(4096).Secret(),
		).
		RequiredWhen("session_token", `("access_key_id" in root) && root.access_key_id.startsWith("ASIA")`).
		Rules(schemapb.Rule(
			`!("session_token" in root) || (("access_key_id" in root) && root.access_key_id.startsWith("ASIA"))`,
			"a session token belongs to temporary (ASIA…) credentials only",
		).ID("session-token-only-temporary")).
		MustBuild()
}
