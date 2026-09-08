package provider

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

const saKey = `{"id":"aje0000000000000abcd","service_account_id":"ajepg0mjt06s0000abcd",` +
	`"created_at":"2026-01-01T00:00:00Z","key_algorithm":"RSA_2048",` +
	`"public_key":"-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----\n",` +
	`"private_key":"-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"}`

func TestYandexCredentials(t *testing.T) {
	schematest.Run(t, YandexCredentials(), schematest.Cases{
		Valid: []map[string]any{
			{"sa_key_json": saKey},
			{"sa_key_json": `{"id":"a","service_account_id":"b","private_key":"c","padding":"` +
				"0000000000000000000000000000000000000000000000000000000000000000" + `"}`},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{}, Code: "REQUIRED_MISSING", Path: "sa_key_json"},
			{Value: map[string]any{"sa_key_json": "{}"}, Code: "MIN_LEN_VIOLATED", Path: "sa_key_json"},
			{
				Value: map[string]any{"sa_key_json": `{"service_account_id":"b","public_key":"` +
					"0000000000000000000000000000000000000000000000000000000000000000" + `"}`},
				Code: "RULE_VIOLATED", Path: "sa_key_json",
			},
			{Value: map[string]any{"sa_key_json": saKey, "token": "x"}, Code: "UNKNOWN_FIELD", Path: "token"},
		},
	})
}
