package schema

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
)

func TestDeriveInputsSchema_Types(t *testing.T) {
	inputs := map[string]ast.InputSpec{
		"db_version": {Type: "string", Default: "16"},
		"threads":    {Type: "number", Default: 64},
		"ssl":        {Type: "bool", Default: false},
	}
	s, diags := DeriveInputsSchema("stroppy.workflow.tpcc", inputs)
	require.False(t, diags.HasErrors(), diags.String())
	byName := fieldsByName(s)
	require.Contains(t, byName, "db_version")
	require.Contains(t, byName, "threads")
	require.Contains(t, byName, "ssl")
}

// TestDeriveInputsSchema_Deterministic is the regression guard for the
// bare-map-range bug: two derivations of the same inputs map must produce
// fields in the same order (and therefore the same schemapb content hash),
// since schemapb.Hash / Baked hash Schema.Fields in slice order.
func TestDeriveInputsSchema_Deterministic(t *testing.T) {
	inputs := map[string]ast.InputSpec{
		"zeta":    {Type: "string", Default: "z"},
		"alpha":   {Type: "number", Default: 1},
		"mu":      {Type: "bool", Default: true},
		"beta":    {Type: "string", Default: "b"},
		"epsilon": {Type: "number", Default: 5},
	}

	s1, diags1 := DeriveInputsSchema("stroppy.workflow.tpcc", inputs)
	require.False(t, diags1.HasErrors(), diags1.String())
	s2, diags2 := DeriveInputsSchema("stroppy.workflow.tpcc", inputs)
	require.False(t, diags2.HasErrors(), diags2.String())

	names1 := fieldNames(s1)
	names2 := fieldNames(s2)
	require.Equal(t, names1, names2, "field order must be deterministic across derivations")

	require.Equal(t, schemapb.Hash(s1), schemapb.Hash(s2), "schemapb content hash must be stable across derivations")
}

// fieldNames returns a schema's top-level field names in slice order.
func fieldNames(s *schemapb.Schema) []string {
	fields := s.GetFields()
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.GetName()
	}
	return names
}

func TestDeriveInputsSchema_Kinds(t *testing.T) {
	inputs := map[string]ast.InputSpec{
		"db_version": {Type: "string", Default: "16"},
		"threads":    {Type: "number", Default: 64},
		"ssl":        {Type: "bool", Default: false},
	}
	s, diags := DeriveInputsSchema("stroppy.workflow.tpcc", inputs)
	require.False(t, diags.HasErrors(), diags.String())
	byName := fieldsByName(s)

	require.NotNil(t, byName["threads"].GetInt64(), "number type -> Int64 kind")
	require.NotNil(t, byName["ssl"].GetBool(), "bool type -> Bool kind")
	require.NotNil(t, byName["db_version"].GetString_(), "string type -> Str kind")
}

func TestDeriveInputsSchema_Default(t *testing.T) {
	inputs := map[string]ast.InputSpec{
		"db_version": {Type: "string", Default: "16"},
		"threads":    {Type: "number", Default: int64(64)},
		"ssl":        {Type: "bool", Default: true},
	}
	s, diags := DeriveInputsSchema("stroppy.workflow.tpcc", inputs)
	require.False(t, diags.HasErrors(), diags.String())
	byName := fieldsByName(s)

	require.Equal(t, "16", byName["db_version"].GetString_().GetDefault())
	require.Equal(t, int64(64), byName["threads"].GetInt64().GetDefault())
	require.True(t, byName["ssl"].GetBool().GetDefault())
}

func TestDeriveInputsSchema_UnknownType(t *testing.T) {
	inputs := map[string]ast.InputSpec{
		"weird": {Type: "bogus", Default: "whatever"},
	}
	s, diags := DeriveInputsSchema("stroppy.workflow.tpcc", inputs)
	require.False(t, diags.HasErrors(), diags.String())
	require.NotEmpty(t, diags, "unknown type should produce a warning diagnostic")

	byName := fieldsByName(s)
	require.Contains(t, byName, "weird")
	require.NotNil(t, byName["weird"].GetString_(), "unknown type degrades to Str kind")
}
