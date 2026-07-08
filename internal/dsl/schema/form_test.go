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
