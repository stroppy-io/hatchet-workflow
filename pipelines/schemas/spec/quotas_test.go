package spec

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestQuotas(t *testing.T) {
	schematest.Run(t, Quotas(), schematest.Cases{
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
				"location":           "eu-central-1a",
			},
		},
		Invalid: []schematest.Invalid{
			{
				Value: map[string]any{"settings": map[string]any{}, "credentials_secret": "x"},
				Code:  "REQUIRED_MISSING", Path: "provider",
			},
			{
				Value: map[string]any{"provider": "azure", "settings": map[string]any{}, "credentials_secret": "x"},
				Code:  "CHOICE_NOT_ALLOWED", Path: "provider",
			},
			{Value: map[string]any{
				"provider": "aws", "settings": map[string]any{}, "credentials_secret": "x", "region": "eu",
			}, Code: "UNKNOWN_FIELD", Path: "region"},
		},
	})
}
