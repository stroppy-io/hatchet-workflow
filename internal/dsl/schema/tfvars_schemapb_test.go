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

// TestDeriveProviderParamsSchemapb_FixedObjectRejectsUnknownKey is the SP-I1
// regression: commit 056becbb strict-ified ONLY the top-level composed form
// (form.go's ComposeFormSchema), so a fixed-attribute-set `object({...})`
// terraform variable (e.g. yandex's `network_settings`/`vms`-adjacent
// objects, NOT the free-key map(object(...)) shapes) survived Bake with an
// injected unknown key underneath it, because DeriveProviderParamsSchemapb's
// `case "object":` branch (tfvars_schemapb.go) built a non-strict
// schemapb.Object. Before the fix this test's Bake call must succeed with
// the evil key silently accepted (fails as written below); after the fix it
// must be rejected.
func TestDeriveProviderParamsSchemapb_FixedObjectRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "network_settings" {
  type = object({
    cidr = string
  })
}
`)
	params, _, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())
	require.NotNil(t, params)

	baked, ferrs := params.Bake(map[string]any{
		"network_settings": map[string]any{
			"cidr":      "10.0.0.0/24",
			"evil_key":  "rm -rf /",
			"__proto__": "polluted",
		},
	})
	require.NotEmpty(t, ferrs, "unknown key inside a fixed-attribute object() must be rejected")
	require.Nil(t, baked)

	found := false
	for _, e := range ferrs {
		if e.GetField() == "network_settings.evil_key" {
			found = true
		}
	}
	require.True(t, found, "expected a FieldError at network_settings.evil_key, got %+v", ferrs)
}

// TestDeriveProviderParamsSchemapb_DeepNestedObjectRejectsUnknownKey checks
// strictness is applied recursively at every depth, not just one level
// under the top-level object() variable.
func TestDeriveProviderParamsSchemapb_DeepNestedObjectRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "topology" {
  type = object({
    zone = object({
      name = string
    })
  })
}
`)
	params, _, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())
	require.NotNil(t, params)

	baked, ferrs := params.Bake(map[string]any{
		"topology": map[string]any{
			"zone": map[string]any{
				"name":     "ru-central1-a",
				"evil_key": "rm -rf /",
			},
		},
	})
	require.NotEmpty(t, ferrs, "unknown key at the deepest nested object level must be rejected")
	require.Nil(t, baked)

	found := false
	for _, e := range ferrs {
		if e.GetField() == "topology.zone.evil_key" {
			found = true
		}
	}
	require.True(t, found, "expected a FieldError at topology.zone.evil_key, got %+v", ferrs)
}

// TestDeriveProviderParamsSchemapb_FixedObjectAcceptsDeclaredKeys is the
// green control for the strict-object fix: legitimate declared attributes
// (including nested ones) must still bake cleanly.
func TestDeriveProviderParamsSchemapb_FixedObjectAcceptsDeclaredKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "network_settings" {
  type = object({
    cidr = string
    zone = optional(string, "ru-central1-a")
  })
}
`)
	params, _, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())

	baked, ferrs := params.Bake(map[string]any{
		"network_settings": map[string]any{
			"cidr": "10.0.0.0/24",
			"zone": "ru-central1-b",
		},
	})
	require.Empty(t, ferrs)
	require.NotNil(t, baked)
}

// TestDeriveProviderParamsSchemapb_MapOfObjectAcceptsArbitraryKeys documents
// the CRITICAL nuance this fix must NOT break: `map(object({...}))` (e.g.
// yandex's `subnets`/`vms` variables) has free, user-chosen keys (subnet
// names, VM names) by design. Marking that shape strict would wrongly
// reject legitimate configs, so it must stay permissive at the key level.
func TestDeriveProviderParamsSchemapb_MapOfObjectAcceptsArbitraryKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "subnets" {
  type = map(object({
    cidr = string
  }))
}
`)
	params, _, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())

	baked, ferrs := params.Bake(map[string]any{
		"subnets": map[string]any{
			"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24"},
			"my-subnet-b": map[string]any{"cidr": "10.0.2.0/24"},
		},
	})
	require.Empty(t, ferrs, "arbitrary map keys are legitimate and must be accepted")
	require.NotNil(t, baked)
}

// TestDeriveProviderParamsSchemapb_MapOfObjectValueRejectsUnknownKey is the
// KNOWN-GAP case named in the SP-I1 brief: an unknown key inside a
// map(object({...}))'s VALUE object (subnets.my-subnet-a.evil_key) should be
// rejected, but schemapb has no map-with-typed-values kind (only List,
// Object, OneOf, Ref, scalars, Computed -- see schemapb/schema.pb.go's
// Schema_Filed oneof and schema.proto), so DeriveProviderParamsSchemapb's
// map(T) fallback (tfvars_schemapb.go's `case "map":`) has nowhere to attach
// a value schema at all: it degrades to an empty, non-strict Object() with
// zero declared fields, and there is no schemapb-native way to validate
// "free keys, strict values" without inventing a fake per-key rule engine
// (explicitly out of scope -- see SP-I1 report). This test is written to
// show the gap, not to pass: it currently (and will continue to, until
// schemapb ships a Map kind) accept the evil key rather than reject it.
func TestDeriveProviderParamsSchemapb_MapOfObjectValueRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "subnets" {
  type = map(object({
    cidr = string
  }))
}
`)
	params, _, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())

	baked, ferrs := params.Bake(map[string]any{
		"subnets": map[string]any{
			"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24", "evil_key": "rm -rf /"},
		},
	})
	// KNOWN GAP (see SP-I1 report): schemapb cannot express "free keys,
	// strict values", so this currently bakes clean instead of rejecting.
	// If this assertion ever starts failing, schemapb gained a Map kind --
	// wire it up in tfvars_schemapb.go's `case "map":` and flip this test to
	// require.NotEmpty(t, ferrs).
	require.Empty(t, ferrs, "KNOWN GAP: schemapb has no map-with-typed-values kind; document, do not fake")
	require.NotNil(t, baked)
}
