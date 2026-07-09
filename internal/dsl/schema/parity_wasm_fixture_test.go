package schema

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/schemapb/schemapb"
)

// parityFixture is the shared oracle both this package's tests and
// web/src/components/launch-form/parity.test.ts read: one schema, plus four
// value sets (valid / Gte-invalid / two undeclared-key variants) and the
// Go-computed expectations each side must reproduce. Keeping the struct tags
// exact matters -- the TS side imports the same JSON file by path and reads
// these exact keys.
type parityFixture struct {
	Schema                          json.RawMessage `json:"schema"`
	ValidValues                     map[string]any  `json:"valid_values"`
	InvalidValues                   map[string]any  `json:"invalid_values"`
	UndeclaredValues                map[string]any  `json:"undeclared_values"`
	UndeclaredProviderValues        map[string]any  `json:"undeclared_provider_values"`
	InvalidExpectedField            string          `json:"invalid_expected_field"`
	UndeclaredExpectedField         string          `json:"undeclared_expected_field"`
	UndeclaredProviderExpectedField string          `json:"undeclared_provider_expected_field"`
	ValidBakedHash                  string          `json:"valid_baked_hash"`
}

// buildParityForm is the ONE schema-construction call shared by the fixture
// writer, TestGoBakeForm_MatchesFixtureExpectations, and (transitively, via
// the checked-in JSON) the TS parity test -- so all three provably validate
// the same schema, not three independently-drifting approximations of one.
func buildParityForm(t *testing.T) *schemapb.Schema {
	t.Helper()
	inputs := schemapb.NewSchema("stroppy.test", "inputs", "1").
		Fields(schemapb.Str("db_version").Default("16"), schemapb.Int64("threads").Gte(1).Default(4)).MustBuild()
	params := schemapb.NewSchema("stroppy.test", "params", "1").
		Fields(schemapb.Double("replicas").Default(3)).MustBuild()
	form, err := ComposeFormSchema("stroppy.test.form", inputs, params)
	require.NoError(t, err)
	return form
}

// TestWriteParityFixture regenerates testdata/parity_form.json from
// buildParityForm, so the Go test and the TS parity test
// (web/src/components/launch-form/parity.test.ts) share one source of truth
// for "what does a real launch form look like, and what must both engines
// do with it". Run with -run TestWriteParityFixture to regenerate after a
// schema shape change; every other run just re-validates the checked-in
// fixture is fresh (byte-identical) rather than silently drifting.
func TestWriteParityFixture(t *testing.T) {
	form := buildParityForm(t)
	schemaJSON, err := protojson.Marshal(form)
	require.NoError(t, err)

	validValues := map[string]any{
		"db_version": "17",
		"threads":    float64(8),
		"provider":   map[string]any{"replicas": float64(5)},
	}
	invalidValues := map[string]any{
		"db_version": "17",
		"threads":    float64(0), // threads < Gte(1)
		"provider":   map[string]any{"replicas": float64(5)},
	}
	undeclaredValues := map[string]any{
		"db_version":  "17",
		"threads":     float64(8),
		"provider":    map[string]any{"replicas": float64(5)},
		"extra_field": "evil_injected_key", // root schema is Strict (commit 056becbb)
	}
	undeclaredProviderValues := map[string]any{
		"db_version": "17",
		"threads":    float64(8),
		"provider": map[string]any{
			"replicas":     float64(5),
			"extra_nested": "evil_injected_key", // nested "provider" object is ALSO Strict
		},
	}

	baked, ferrs := form.Bake(validValues)
	require.Empty(t, ferrs, "valid_values must be the clean control case")
	require.NotNil(t, baked)
	validBakedHash := schemapb.Hash(baked)

	fixture := parityFixture{
		Schema:                          schemaJSON,
		ValidValues:                     validValues,
		InvalidValues:                   invalidValues,
		UndeclaredValues:                undeclaredValues,
		UndeclaredProviderValues:        undeclaredProviderValues,
		InvalidExpectedField:            "threads",
		UndeclaredExpectedField:         "extra_field",
		UndeclaredProviderExpectedField: "provider.extra_nested",
		ValidBakedHash:                  hex.EncodeToString(validBakedHash[:]),
	}
	out, err := json.MarshalIndent(fixture, "", "  ")
	require.NoError(t, err)

	existing, readErr := os.ReadFile("testdata/parity_form.json")
	if readErr == nil {
		require.JSONEq(t, string(existing), string(out), "testdata/parity_form.json is stale -- regenerate with -run TestWriteParityFixture")
		return
	}
	require.NoError(t, os.WriteFile("testdata/parity_form.json", out, 0o644))
}

// readParityFixture loads the checked-in fixture. Every test below re-reads
// it fresh rather than sharing state with TestWriteParityFixture, so a run
// with -run TestGoBakeForm... alone (no TestWriteParityFixture) still
// exercises the real on-disk file the TS side imports.
func readParityFixture(t *testing.T) parityFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/parity_form.json")
	require.NoError(t, err, "testdata/parity_form.json missing -- run: go test ./internal/dsl/schema/... -run TestWriteParityFixture -v")
	var fixture parityFixture
	require.NoError(t, json.Unmarshal(raw, &fixture))
	return fixture
}

func mustParitySchema(t *testing.T, fixture parityFixture) *schemapb.Schema {
	t.Helper()
	var form schemapb.Schema
	require.NoError(t, protojson.Unmarshal(fixture.Schema, &form))
	return &form
}

// TestGoBakeForm_MatchesFixtureExpectations is this package's half of the
// Go/WASM parity lock: it independently re-bakes the checked-in fixture
// (not the in-memory buildParityForm result) and checks the outcomes match
// what the fixture claims, so a fixture edited by hand (or a schemapb
// upgrade that silently changes behavior) fails loudly here too, not just
// on the TS side.
func TestGoBakeForm_MatchesFixtureExpectations(t *testing.T) {
	fixture := readParityFixture(t)
	form := mustParitySchema(t, fixture)

	t.Run("valid_values bakes clean and hashes to the fixture's recorded hash", func(t *testing.T) {
		baked, ferrs, err := BakeForm(form, fixture.ValidValues)
		require.NoError(t, err)
		require.Empty(t, ferrs, "Go: valid_values must bake clean")
		require.NotNil(t, baked)
		got := schemapb.Hash(baked)
		require.Equal(t, fixture.ValidBakedHash, hex.EncodeToString(got[:]), "Go schemapb.Hash(baked) must match the fixture's recorded hash")
	})

	t.Run("invalid_values fails Gte(1) on threads", func(t *testing.T) {
		baked, ferrs, err := BakeForm(form, fixture.InvalidValues)
		require.NoError(t, err)
		require.NotEmpty(t, ferrs, "Go: invalid_values must fail Gte(1) on threads")
		require.Nil(t, baked)
		require.True(t, hasFieldPath(ferrs, fixture.InvalidExpectedField), "expected a FieldError at %q, got %+v", fixture.InvalidExpectedField, ferrs)
	})

	t.Run("undeclared top-level key is rejected by the strict root schema", func(t *testing.T) {
		baked, ferrs, err := BakeForm(form, fixture.UndeclaredValues)
		require.NoError(t, err)
		require.NotEmpty(t, ferrs, "Go: undeclared top-level key must be rejected (strict root, commit 056becbb)")
		require.Nil(t, baked)
		require.True(t, hasFieldPath(ferrs, fixture.UndeclaredExpectedField), "expected a FieldError at %q, got %+v", fixture.UndeclaredExpectedField, ferrs)
	})

	t.Run("undeclared nested provider key is rejected by the strict provider object", func(t *testing.T) {
		baked, ferrs, err := BakeForm(form, fixture.UndeclaredProviderValues)
		require.NoError(t, err)
		require.NotEmpty(t, ferrs, "Go: undeclared nested provider key must be rejected (strict nested provider, commit 056becbb)")
		require.Nil(t, baked)
		require.True(t, hasFieldPath(ferrs, fixture.UndeclaredProviderExpectedField), "expected a FieldError at %q, got %+v", fixture.UndeclaredProviderExpectedField, ferrs)
	})
}

func hasFieldPath(ferrs []*schemapb.FieldError, field string) bool {
	for _, e := range ferrs {
		if e.GetField() == field {
			return true
		}
	}
	return false
}
