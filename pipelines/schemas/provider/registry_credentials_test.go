package provider

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestRegistryCredentials(t *testing.T) {
	schematest.Run(t, RegistryCredentials(), schematest.Cases{
		Valid: []map[string]any{
			{"registry": "ghcr.io", "username": "stroppy", "password": "ghp_token"},
			{"registry": "registry.stroppy.io:5000", "username": "robot$ci", "password": "s3cret"},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{
				"registry": "https://ghcr.io", "username": "u", "password": "p",
			}, Code: "PATTERN_MISMATCH", Path: "registry"},
			{
				Value: map[string]any{"registry": "ghcr.io", "password": "p"},
				Code:  "REQUIRED_MISSING", Path: "username",
			},
			{
				Value: map[string]any{"registry": "ghcr.io", "username": "u"},
				Code:  "REQUIRED_MISSING", Path: "password",
			},
			{Value: map[string]any{
				"registry": "ghcr.io", "username": "u", "password": "p", "email": "x@y.z",
			}, Code: "UNKNOWN_FIELD", Path: "email"},
		},
	})
}
