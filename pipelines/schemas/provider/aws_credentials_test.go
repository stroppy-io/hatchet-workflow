package provider

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestAwsCredentials(t *testing.T) {
	schematest.Run(t, AwsCredentials(), schematest.Cases{
		Valid: []map[string]any{
			{
				"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
				"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			{
				"access_key_id":     "ASIAIOSFODNN7EXAMPLE",
				"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"session_token":     "FwoGZXIvYXdzEExampleSessionToken",
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{
				"access_key_id":     "AKIASHORT",
				"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			}, Code: "PATTERN_MISMATCH", Path: "access_key_id"},
			{Value: map[string]any{
				"access_key_id": "AKIAIOSFODNN7EXAMPLE",
			}, Code: "REQUIRED_MISSING", Path: "secret_access_key"},
			{Value: map[string]any{
				"access_key_id":     "ASIAIOSFODNN7EXAMPLE",
				"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			}, Code: "RULE_VIOLATED", Path: ""},
			{Value: map[string]any{
				"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
				"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"session_token":     "FwoGZXIvYXdzEExampleSessionToken",
			}, Code: "RULE_VIOLATED", Path: ""},
		},
	})
}
