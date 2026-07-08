package schema

import (
	"testing"

	"github.com/stretchr/testify/require"

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
