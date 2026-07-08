package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/schemapb/schemapb"
)

// writeFile writes content to name inside dir, failing the test on error.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

// fieldsByName indexes a schema's top-level fields by name.
func fieldsByName(s *schemapb.Schema) map[string]*schemapb.Schema_Filed {
	out := make(map[string]*schemapb.Schema_Filed, len(s.GetFields()))
	for _, f := range s.GetFields() {
		out[f.GetName()] = f
	}
	return out
}

func TestDeriveProviderParamsSchemapb_ScalarsAndExt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "image" {
  type        = string
  description = "container image"
  default     = "postgres:16"
}
variable "replicas" {
  type    = number
  default = 3
}
variable "token" {
  type      = string
  sensitive = true
}
variable "stroppy_machine_ext" {
  type = object({ zone = string, disk_gb = optional(number, 100) })
}
`)

	params, ext, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())
	require.NotNil(t, params)
	require.NotNil(t, ext)

	byName := fieldsByName(params) // helper: map[string]*schemapb.Schema_Filed
	require.Contains(t, byName, "image")
	require.Contains(t, byName, "replicas")
	require.Contains(t, byName, "token")
	require.NotContains(t, byName, "stroppy_machine_ext", "ext var must be split out")

	require.Equal(t, "container image", byName["image"].GetDescription())
	require.True(t, byName["token"].GetSecret(), "sensitive var → secret")

	extFields := fieldsByName(ext)
	require.Contains(t, extFields, "zone")
	require.Contains(t, extFields, "disk_gb")
	require.True(t, extFields["zone"].GetRequired(), "bare object() attribute is required")
	require.False(t, extFields["disk_gb"].GetRequired(), "optional(...) object() attribute is not required")
}

func TestDeriveProviderParamsSchemapb_ValidationToRule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "replicas" {
  type = number
  validation {
    condition     = var.replicas >= 1
    error_message = "at least 1 replica"
  }
}
`)
	params, _, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors())
	require.NotEmpty(t, params.GetRules(), "tf validation block → schemapb rule")
	require.Contains(t, params.GetRules()[0].GetMessage(), "at least 1 replica")
}
