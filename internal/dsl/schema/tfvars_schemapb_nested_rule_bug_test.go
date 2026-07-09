package schema

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/schemapb/schemapb"
)

// The tests in this file characterize and lock in the fix for the
// translateValidationCondition scope bug documented in
// tfvars_schemapb.go's doc comment and .superpowers/sdd/
// spd-celscope-report.md: a tf `validation { condition = var.X ... }` block
// is translated into a schemapb Rule attached to the derived params
// schema's own Rules (a schema-level rule). That schema is ALWAYS nested
// under a "provider" object field in every real production Bake call
// (form.go's ComposeFormSchema -> BakeForm); schemapb/validate.go's
// checkObject evaluates a nested object's own schema-level Rules with
// `this` bound to that object's own local scope at every nesting depth,
// while `root` stays bound to the outermost form root for the whole
// recursion. So the emitted rule must reference `this.NAME`, not
// `root.NAME` -- see the fix in tfvars_schemapb.go's
// translateValidationCondition.

// TestDeriveProviderParamsSchemapb_ValidationRule_NestedViaFormRejectsInvalid
// is the real-world production shape: the params schema (carrying a
// this.replicas>=1 rule translated from a tf validation{} block) nested
// under "provider" by ComposeFormSchema, exactly like
// ComposeLaunchFormSchema builds a real launch form. An invalid value
// (replicas=0) must be rejected with the tf validation block's own error
// message, at the "provider" field path (the object the rule is attached
// to -- see schemapb/validate.go's checkObject).
func TestDeriveProviderParamsSchemapb_ValidationRule_NestedViaFormRejectsInvalid(t *testing.T) {
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
	require.NotEmpty(t, params.GetRules())
	require.Equal(t, "this.replicas >= 1", params.GetRules()[0].GetExpr(), "translated rule must be this-relative, not root-relative")

	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()
	form, err := ComposeFormSchema("stroppy.form.yandex", inputs, params)
	require.NoError(t, err)

	baked, ferrs, err := BakeForm(form, map[string]any{
		"db_version": "16",
		"provider": map[string]any{
			"replicas": 0.0, // invalid: < 1
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, ferrs, "invalid provider.replicas=0 must be rejected by the tf validation block's translated rule")
	require.Nil(t, baked)

	found := false
	for _, e := range ferrs {
		if e.GetField() == "provider" && e.GetMessage() == "at least 1 replica" {
			found = true
		}
	}
	require.True(t, found, "expected the tf validation block's own error message at field \"provider\", got %+v", ferrs)
}

// TestDeriveProviderParamsSchemapb_ValidationRule_NestedViaFormAcceptsValid
// is the green control: a legitimately valid value (replicas=5) must bake
// clean through the same nested path.
func TestDeriveProviderParamsSchemapb_ValidationRule_NestedViaFormAcceptsValid(t *testing.T) {
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

	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()
	form, err := ComposeFormSchema("stroppy.form.yandex", inputs, params)
	require.NoError(t, err)

	baked, ferrs, err := BakeForm(form, map[string]any{
		"db_version": "16",
		"provider": map[string]any{
			"replicas": 5.0, // valid: >= 1
		},
	})
	require.NoError(t, err)
	require.Empty(t, ferrs, "valid provider.replicas=5 must bake clean")
	require.NotNil(t, baked)
}

// TestDeriveProviderParamsSchemapb_ValidationRule_StandaloneBakeIsNotAProductionPath
// documents the known, deliberate limitation of the this.NAME fix: schemapb
// binds `this` to nil (not the schema's own values) when a schema-level Rule
// belongs to the literal Bake root (schemapb/validate.go's top-level
// `validate` calls evalRule with this=nil for the root schema's own Rules;
// only a NESTED object's own schema-level Rules get this bound to their
// local scope -- see checkObject). So Bake-ing the derived params schema
// directly, with no wrapping object, makes a this.NAME rule error instead of
// evaluating -- this is safe only because grep confirms no production
// caller ever does that (the sole production Bake call is form.go's
// BakeForm, always on the ComposeFormSchema-composed form; see
// spd-celscope-report.md). This test locks in that this IS still the
// documented, expected (if surprising) behavior, so a future standalone
// Bake caller fails loudly instead of silently misvalidating.
func TestDeriveProviderParamsSchemapb_ValidationRule_StandaloneBakeIsNotAProductionPath(t *testing.T) {
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

	// Even a VALID value errors when Bake'd standalone (this == nil at the
	// literal root) -- fail-closed-loud, not silently wrong, and not a path
	// any production code takes.
	baked, ferrs := params.Bake(map[string]any{"replicas": 5.0})
	require.NotEmpty(t, ferrs, "standalone Bake of a this.NAME-ruled schema must fail loudly (this is nil at the literal root), not silently misvalidate")
	require.Nil(t, baked)
}
