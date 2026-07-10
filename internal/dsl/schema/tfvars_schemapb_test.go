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

// TestDeriveProviderParamsSchemapb_ExcludesProviderContractNodesVar locks
// in the fix for the yandex-module bug: a terraform module's standard
// provider-contract `stroppy_nodes` variable (internal/infrastructure/
// provider/terraform.go's Provision always computes and overwrites it from
// MachineGroups -- never user-supplied) must not leak into the derived
// provider.params form schema the same way stroppy_machine_ext doesn't,
// since a launch-form user has no correct value to give it.
func TestDeriveProviderParamsSchemapb_ExcludesProviderContractNodesVar(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "zone" {
  type = string
}
variable "stroppy_nodes" {
  type = list(object({
    id     = string
    group  = string
    cpu    = number
    ram_gb = number
    disk = optional(object({
      size_gb = number
      type    = string
    }))
    ext = optional(any, {})
  }))
  default = []
}
`)

	params, ext, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())
	require.NotNil(t, params)
	require.Nil(t, ext, "no stroppy_machine_ext declared, ext schema stays nil")

	byName := fieldsByName(params)
	require.Contains(t, byName, "zone")
	require.NotContains(t, byName, "stroppy_nodes", "provider-contract nodes var must never be a user-facing param")
}

// TestDeriveProviderParamsSchemapb_YandexModule exercises schema derivation
// against the real yandex terraform module (deployments/terraform/yandex),
// pinning: (1) its per-VM sizing/networking knobs (compute, network,
// managed_ydb) surface as ordinary provider.params, (2) its standard
// stroppy_nodes contract input is excluded from provider.params (a user
// never fills it in -- terraform.go computes it from MachineGroups), and
// (3) its stroppy_machine_ext variable produces a non-nil ext schema
// (cluster.yaml's machines.<name>.yandex: {...} block).
func TestDeriveProviderParamsSchemapb_YandexModule(t *testing.T) {
	moduleDir := filepath.Join("..", "..", "..", "deployments", "terraform", "yandex")
	if _, err := os.Stat(moduleDir); err != nil {
		t.Skipf("yandex module not found at %s: %v", moduleDir, err)
	}

	params, ext, diags := DeriveProviderParamsSchemapb(moduleDir, "yandex")
	require.NotNil(t, params)
	t.Logf("diags: %s", diags.String())

	byName := fieldsByName(params)
	require.Contains(t, byName, "compute")
	require.Contains(t, byName, "network")
	require.Contains(t, byName, "managed_ydb")
	require.NotContains(t, byName, "stroppy_nodes", "provider-contract nodes var must never be a user-facing param")
	require.NotContains(t, byName, "stroppy_machine_ext", "ext var must be split out, not a param")

	require.NotNil(t, ext, "yandex module declares stroppy_machine_ext, ext schema must be derived")
	extFields := fieldsByName(ext)
	require.Contains(t, extFields, "platform_id")
	require.Contains(t, extFields, "network_acceleration")
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

// TestDeriveProviderParamsSchemapb_MapOfObjectValueRejectsUnknownKey used to
// be the KNOWN-GAP case named in the SP-I1 brief: schemapb had no
// map-with-typed-values kind, so an unknown key inside a
// map(object({...}))'s VALUE object (subnets.my-subnet-a.evil_key) flowed
// through unrejected. schemapb v1.6.0 closed that gap with a Map kind (free
// keys, typed+validated values -- see schemapb/new.go's Map builder), and
// tfvars_schemapb.go's `case "map":` now emits a Map with a Strict value
// schema whenever the terraform map's value type is object({...}). This
// test is inverted from its known-gap form: it now asserts the CORRECT
// (rejecting) behavior.
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
	require.NotEmpty(t, ferrs, "unknown key inside a map(object({...})) value must be rejected (schemapb v1.6.0 Map kind, Strict value schema)")
	require.Nil(t, baked)
	require.True(t, hasFieldPath(ferrs, "subnets.my-subnet-a.evil_key"), "expected a FieldError at %q, got %+v", "subnets.my-subnet-a.evil_key", ferrs)
}

// TestMapKind_RuleOnValueScopesThisToTheMapValue verifies commit d98c09a7's
// `this`-not-`root` scoping fix (translateValidationCondition) extends
// correctly to the Map kind: a .Rule(...) attached to a Map's value schema
// (MapB.Rule, schemapb/new.go) must evaluate with `this` bound to the
// INDIVIDUAL map value object it's currently validating, not the map itself
// or the outer root -- mirroring ObjectB.Rule's scoping (checkObject) via
// checkMap's own evalRule call (schemapb/validate.go). This is a
// schemapb-builder-level test (not routed through
// DeriveProviderParamsSchemapb, which doesn't yet derive tf validation{}
// blocks for map(object({...})) value attributes) exercising the same
// MapB.Rule building block tfvars_schemapb.go's `case "map":` could use if a
// future tf module needs a per-map-value validation rule.
func TestMapKind_RuleOnValueScopesThisToTheMapValue(t *testing.T) {
	schema := schemapb.NewSchema("stroppy.test", "map_rule", "1").
		Fields(
			schemapb.Map("subnets",
				schemapb.Str("cidr").Required(),
				schemapb.Int64("size").Required(),
			).Strict().Rule(schemapb.Rule("this.size <= 24", "size must be at most 24")),
		).MustBuild()

	t.Run("a map value violating the rule is rejected at that value's own path", func(t *testing.T) {
		baked, ferrs := schema.Bake(map[string]any{
			"subnets": map[string]any{
				"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24", "size": float64(28)},
			},
		})
		require.NotEmpty(t, ferrs, "this.size <= 24 rule must fail for size=28")
		require.Nil(t, baked)
		require.True(t, hasFieldPath(ferrs, "subnets.my-subnet-a"), "expected the rule error at the map value's own path, got %+v", ferrs)
	})

	t.Run("a compliant map value bakes clean", func(t *testing.T) {
		baked, ferrs := schema.Bake(map[string]any{
			"subnets": map[string]any{
				"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24", "size": float64(20)},
			},
		})
		require.Empty(t, ferrs)
		require.NotNil(t, baked)
	})
}
