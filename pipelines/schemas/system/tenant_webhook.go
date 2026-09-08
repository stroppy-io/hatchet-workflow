package system

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// TenantWebhook is tenant.webhook@1 — one outgoing webhook of a tenant: where
// run and suite events are POSTed, and the secret they are signed with.
//
// The JSON shape matches OpenAPI WebhookCreate / WebhookEvent
// (openapi/parts/30-tenant-settings.yaml).
//
// doc: STROPPY.MD §16.3 — Standard Webhooks, HMAC-SHA256 over id.ts.body.
func TenantWebhook() *schemapb.Schema {
	return schemapb.NewSchema(ids.Tenant("webhook", 1)).
		Descr("Outgoing webhook of a tenant: endpoint, subscribed events and signing secret.").
		Strict().Coerce().
		Fields(
			// Format checks that it is a URL at all; the pattern forces https,
			// because the payload carries run names and summaries.
			schemapb.Str("url").Title("Endpoint").Group("Delivery").
				Desc("HTTPS endpoint the event is POSTed to.").
				Format(schemapb.FormatURL).Pattern(`^https://`).MaxLen(2048).Required().
				Examples(schemapb.StrV("https://ci.example.com/hooks/stroppy")),

			schemapb.List("events",
				schemapb.Choice("").
					Opt(schemapb.StrV("run.started"), "Run started").
					Opt(schemapb.StrV("run.stage"), "Run changed phase").
					Opt(schemapb.StrV("run.finished"), "Run finished").
					Opt(schemapb.StrV("run.failed"), "Run failed").
					Opt(schemapb.StrV("run.cancelled"), "Run canceled").
					Opt(schemapb.StrV("suite.started"), "Suite started").
					Opt(schemapb.StrV("suite.cell_finished"), "Suite cell finished").
					Opt(schemapb.StrV("suite.finished"), "Suite finished"),
			).Title("Events").Group("Delivery").
				Desc("Which events are delivered; at least one.").
				MinItems(1).MaxItems(8).Unique().Required(),

			// doc: standardwebhooks.com — the secret signs "<id>.<timestamp>.<body>"
			// with HMAC-SHA256; the receiver needs the same value.
			schemapb.Str("secret").Title("Signing secret").Group("Delivery").
				Desc("Shared secret for the HMAC-SHA256 signature; shown once at creation.").
				MinLen(16).MaxLen(256).Secret().Required(),

			schemapb.Bool("enabled").Title("Enabled").Group("Delivery").
				Desc("Deliveries are paused while this is off; the subscription is kept.").
				Default(true),
			schemapb.Str("description").Title("Description").Group("Delivery").
				Desc("What this hook is for, for whoever finds it later.").MaxLen(512),
		).
		MustBuild()
}
