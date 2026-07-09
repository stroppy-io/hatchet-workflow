package schema

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/schemapb/schemapb"
)

func TestComposeFormSchema_NestsProvider(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()
	params := schemapb.NewSchema("ns", "params", "1").
		Fields(schemapb.Double("replicas").Default(3)).MustBuild()

	form, err := ComposeFormSchema("stroppy.form.tpcc", inputs, params)
	require.NoError(t, err)
	byName := fieldsByName(form)
	require.Contains(t, byName, "db_version", "inputs hoisted to top level")
	require.Contains(t, byName, "provider", "params nested under provider")
}

// TestComposeFormSchema_StrictRejectsUnknownTopLevelKey is the bake-time
// half of the SP-D T6 strict-fix (baked_apply.go's ApplyBakedInputs is the
// apply-time half): before this fix ComposeFormSchema never called
// .Strict(), so Bake silently accepted any extra top-level key alongside
// the declared inputs -- BakeForm returned a non-nil Baked with the bogus
// key still in Values, which SplitBakedValues/ApplyBakedInputs would have
// forwarded on. A strict form rejects it as a FieldError instead.
func TestComposeFormSchema_StrictRejectsUnknownTopLevelKey(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()

	form, err := ComposeFormSchema("stroppy.form.tpcc", inputs, nil)
	require.NoError(t, err)

	baked, ferrs, err := BakeForm(form, map[string]any{
		"db_version":        "17",
		"evil_injected_key": "rm -rf /",
	})
	require.NoError(t, err)
	require.NotEmpty(t, ferrs, "undeclared top-level key must be rejected, not silently passed")
	require.Nil(t, baked, "blocking FieldError leaves baked nil")
}

// TestComposeFormSchema_StrictRejectsUnknownNestedProviderKey covers the
// nested "provider" object ComposeFormSchema builds via
// schemapb.ObjectOf(...) -- it must also be strict, independent of the root
// schema's own strictness, since ObjectOf clones params into its own nested
// Schema with its own Strict flag.
func TestComposeFormSchema_StrictRejectsUnknownNestedProviderKey(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()
	params := schemapb.NewSchema("ns", "params", "1").
		Fields(schemapb.Double("replicas").Default(3)).MustBuild()

	form, err := ComposeFormSchema("stroppy.form.tpcc", inputs, params)
	require.NoError(t, err)

	baked, ferrs, err := BakeForm(form, map[string]any{
		"db_version": "17",
		"provider": map[string]any{
			"replicas":          float64(5),
			"evil_injected_key": "rm -rf /",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, ferrs, "undeclared nested provider key must be rejected")
	require.Nil(t, baked)
}

// TestComposeFormSchema_StrictAcceptsDeclaredValues is the green control for
// the two rejection tests above: legitimate values -- declared inputs plus
// declared provider params -- must still bake cleanly once the schema is
// strict.
func TestComposeFormSchema_StrictAcceptsDeclaredValues(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()
	params := schemapb.NewSchema("ns", "params", "1").
		Fields(schemapb.Double("replicas").Default(3)).MustBuild()

	form, err := ComposeFormSchema("stroppy.form.tpcc", inputs, params)
	require.NoError(t, err)

	baked, ferrs, err := BakeForm(form, map[string]any{
		"db_version": "17",
		"provider":   map[string]any{"replicas": float64(5)},
	})
	require.NoError(t, err)
	require.Empty(t, ferrs)
	require.NotNil(t, baked)
}

// TestComposeFormSchema_StrictAcceptsDockerNoProviderCase covers the nil
// params case (e.g. the docker builtin, which has no tf module and so no
// provider params schema at all): strict on the root schema must not
// require a "provider" key that was never declared, and legitimate
// inputs-only values still bake cleanly.
func TestComposeFormSchema_StrictAcceptsDockerNoProviderCase(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()

	form, err := ComposeFormSchema("stroppy.form.tpcc", inputs, nil)
	require.NoError(t, err)

	baked, ferrs, err := BakeForm(form, map[string]any{"db_version": "17"})
	require.NoError(t, err)
	require.Empty(t, ferrs)
	require.NotNil(t, baked)
}

func TestBakeForm_ValidAndInvalid(t *testing.T) {
	form := schemapb.NewSchema("ns", "form", "1").
		Fields(schemapb.Int64("replicas").Gte(1).Default(3)).MustBuild()

	// NOTE (deviation from brief): schemapb.Schema.Bake -> Validate checks
	// numeric fields via a direct `val.(float64)` type assertion (see
	// schemapb/validate.go numericCheck) -- it does not accept a raw Go
	// int64, even for an Int64-kind field. Real schemapb tests confirm this
	// (schemapb_test.go: `bad.Bake(map[string]any{"n": float64(5)})`). Using
	// int64(5)/int64(0) as the brief literally shows makes even the "valid"
	// case fail with a type FieldError ("expected number"), so values are
	// passed as float64 here to match actual runtime behavior.
	baked, ferrs, err := BakeForm(form, map[string]any{"replicas": float64(5)})
	require.NoError(t, err)
	require.Empty(t, ferrs)
	require.NotNil(t, baked)

	_, ferrs2, err2 := BakeForm(form, map[string]any{"replicas": float64(0)})
	require.NoError(t, err2)
	require.NotEmpty(t, ferrs2, "value below Gte(1) → FieldError")
}
