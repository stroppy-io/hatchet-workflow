package spec

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestProviderVerify(t *testing.T) {
	schematest.Run(t, ProviderVerify(), schematest.Cases{
		Valid: []map[string]any{
			{
				"provider":           "yandex",
				"settings":           map[string]any{"folder_id": "b1gia87mbaomkfvsleds"},
				"credentials_secret": "yc-sa-key",
			},
			{
				"provider":           "aws",
				"settings":           map[string]any{"region": "eu-central-1"},
				"credentials_secret": "aws-access-key",
				"dry_run":            false,
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{
				"provider": "gcp", "settings": map[string]any{}, "credentials_secret": "x",
			}, Code: "CHOICE_NOT_ALLOWED", Path: "provider"},
			{Value: map[string]any{
				"provider": "aws", "credentials_secret": "x",
			}, Code: "REQUIRED_MISSING", Path: "settings"},
			{Value: map[string]any{
				"provider": "aws", "settings": map[string]any{}, "credentials_secret": "Bad Secret",
			}, Code: "PATTERN_MISMATCH", Path: "credentials_secret"},
		},
	})
}
