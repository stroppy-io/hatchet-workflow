package schema

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/schemapb/schemapb"
)

// hashBaked returns schemapb's content hash of a *Baked snapshot. *Baked
// implements HashPB (schema_hashpb.pb.go), so it satisfies the interface
// schemapb.Hash expects directly -- same public entrypoint already used for
// Schema determinism in inputs_schemapb_test.go.
func hashBaked(t *testing.T, b *schemapb.Baked) [32]byte {
	t.Helper()
	require.NotNil(t, b)
	return schemapb.Hash(b)
}

// TestFormBake_StableHash locks the invariant that baking the same composed
// form schema with the same values twice produces identical Baked hashes --
// the foundation SP-D's Go-vs-WASM parity guard builds on.
func TestFormBake_StableHash(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16")).MustBuild()
	params := schemapb.NewSchema("ns", "params", "1").
		Fields(schemapb.Double("replicas").Default(3)).MustBuild()
	form, err := ComposeFormSchema("ns.form", inputs, params)
	require.NoError(t, err)

	// numeric values MUST be float64, not int64 -- schemapb's numericCheck
	// hard-asserts float64 (see form_test.go note on the same contract).
	vals := map[string]any{"db_version": "17", "provider": map[string]any{"replicas": float64(5)}}

	b1, e1, err1 := BakeForm(form, vals)
	b2, e2, err2 := BakeForm(form, vals)
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Empty(t, e1)
	require.Empty(t, e2)
	require.NotNil(t, b1)
	require.NotNil(t, b2)
	require.Equal(t, hashBaked(t, b1), hashBaked(t, b2), "same schema+values → same Baked hash")
}
