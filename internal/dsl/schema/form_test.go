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
