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

	// SP-I1 (finding I3) additions: cover the gap the merge gate named --
	// Bool (inputs_schemapb.go:44), a CEL/expr rule derived from a tf
	// validation{} block (tfvars_schemapb.go's translateValidationCondition,
	// the richest Go/WASM divergence surface), and a nested-object rejection
	// case (the SP-I1 fix itself: a fixed-attribute-set object() param
	// nested under "provider" must reject an unknown key at every depth on
	// BOTH engines).
	RuleInvalidValues         map[string]any `json:"rule_invalid_values"`
	RuleInvalidExpectedField  string         `json:"rule_invalid_expected_field"`
	NestedObjectInvalidValues map[string]any `json:"nested_object_invalid_values"`
	NestedObjectExpectedField string         `json:"nested_object_expected_field"`

	// schemapb v1.6.0 Map-kind additions: the config-injection gap this task
	// closes (tfvars_schemapb.go's `case "map":` now emits a Strict-valued
	// Map instead of a permissive fieldless Object for map(object({...}))
	// terraform variables). Both engines must agree an unknown key inside a
	// map VALUE is rejected at a path naming the map key, and that an
	// arbitrary map key name is itself always accepted.
	MapValueInvalidValues map[string]any `json:"map_value_invalid_values"`
	MapValueExpectedField string         `json:"map_value_expected_field"`
}

// buildParityForm is the ONE schema-construction call shared by the fixture
// writer, TestGoBakeForm_MatchesFixtureExpectations, and (transitively, via
// the checked-in JSON) the TS parity test -- so all three provably validate
// the same schema, not three independently-drifting approximations of one.
func buildParityForm(t *testing.T) *schemapb.Schema {
	t.Helper()
	inputs := schemapb.NewSchema("stroppy.test", "inputs", "1").
		Fields(
			schemapb.Str("db_version").Default("16"),
			schemapb.Int64("threads").Gte(1).Default(4),
			// SP-I1/I3: Bool kind (inputs_schemapb.go:44 is the only production
			// site that emits it) -- must round-trip identically through Go
			// bake and the WASM engine.
			schemapb.Bool("maintenance_mode").Default(false),
		).MustBuild()
	params := schemapb.NewSchema("stroppy.test", "params", "1").
		Fields(
			schemapb.Double("replicas").Default(3),
			// SP-I1: a fixed-attribute-set object() param nested under
			// "provider" -- .Strict() is the fix this schema exercises (see
			// tfvars_schemapb.go's `case "object":`); an unknown key under it
			// must be rejected by both Go and WASM.
			schemapb.Object("network_settings", schemapb.Str("cidr").Required()).Strict(),
			// schemapb v1.6.0: a Map-kind param mirroring yandex's
			// map(object({...})) subnets/vms variables -- free, arbitrary
			// map keys, but a Strict value schema (tfvars_schemapb.go's
			// `case "map":`). Both engines must accept an arbitrary map key
			// name and reject an unknown key INSIDE a map value.
			schemapb.Map("subnets", schemapb.Str("cidr").Required()).Strict(),
		).
		// SP-I1/I3: a schema-level Rule, standing in for a tf validation{}
		// block translated via translateValidationCondition (var.NAME ->
		// root.NAME) -- the richest Go/WASM divergence surface per the merge
		// gate, since it round-trips a raw expr-lang expression string
		// through both engines' independent expr.Compile calls.
		//
		// NOTE: `this`, not `root` -- schemapb's checkObject (validate.go)
		// evaluates a nested object schema's own Rules with `this` bound to
		// THAT object's own scope (its local values) while `root` stays
		// bound to the outermost form root throughout the whole recursion
		// (see checkObject -> evalRule). DeriveProviderParamsSchemapb's
		// translateValidationCondition emits `root.NAME` (tfvars_schemapb.go),
		// which is only correct when the derived params schema is baked
		// standalone (as tfvars_schemapb_test.go's tests do) -- once
		// ComposeFormSchema nests it under "provider" via ObjectOf, `root`
		// no longer points at the params scope, so a real tf validation{}
		// rule would silently evaluate against the wrong scope post-nesting.
		// That is a genuine, separate finding outside SP-I1's fix scope
		// (see the SP-I1 report) -- `this.replicas` here is the correct
		// form for a schema-level rule regardless of nesting depth, so this
		// fixture's own Bake calls behave correctly while still exercising
		// the same expr.Compile Rule machinery on both engines.
		Rules(schemapb.Rule("this.replicas <= 10", "replicas must be at most 10")).
		MustBuild()
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
		"db_version":       "17",
		"threads":          float64(8),
		"maintenance_mode": false,
		"provider": map[string]any{
			"replicas":         float64(5),
			"network_settings": map[string]any{"cidr": "10.0.0.0/24"},
			// schemapb v1.6.0 Map kind: two arbitrary, user-chosen map keys --
			// both engines must accept the map keys themselves unconditionally.
			"subnets": map[string]any{
				"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24"},
				"my-subnet-b": map[string]any{"cidr": "10.0.2.0/24"},
			},
		},
	}
	invalidValues := map[string]any{
		"db_version":       "17",
		"threads":          float64(0), // threads < Gte(1)
		"maintenance_mode": false,
		"provider":         map[string]any{"replicas": float64(5), "network_settings": map[string]any{"cidr": "10.0.0.0/24"}},
	}
	undeclaredValues := map[string]any{
		"db_version":       "17",
		"threads":          float64(8),
		"maintenance_mode": false,
		"provider":         map[string]any{"replicas": float64(5), "network_settings": map[string]any{"cidr": "10.0.0.0/24"}},
		"extra_field":      "evil_injected_key", // root schema is Strict (commit 056becbb)
	}
	undeclaredProviderValues := map[string]any{
		"db_version":       "17",
		"threads":          float64(8),
		"maintenance_mode": false,
		"provider": map[string]any{
			"replicas":         float64(5),
			"network_settings": map[string]any{"cidr": "10.0.0.0/24"},
			"extra_nested":     "evil_injected_key", // nested "provider" object is ALSO Strict
		},
	}
	// SP-I1/I3: violates the schema-level Rule (replicas <= 10) while
	// otherwise valid -- both engines must report the same rule violation at
	// the same field path ("provider", the object field the rule is
	// attached to; see schemapb/validate.go's checkObject).
	ruleInvalidValues := map[string]any{
		"db_version":       "17",
		"threads":          float64(8),
		"maintenance_mode": false,
		"provider":         map[string]any{"replicas": float64(50), "network_settings": map[string]any{"cidr": "10.0.0.0/24"}},
	}
	// SP-I1: an unknown key nested inside the strict network_settings
	// object() param, two levels under the form root -- the exact shape the
	// SP-I1 fix (tfvars_schemapb.go's `case "object":` .Strict()) closes.
	nestedObjectInvalidValues := map[string]any{
		"db_version":       "17",
		"threads":          float64(8),
		"maintenance_mode": false,
		"provider": map[string]any{
			"replicas":         float64(5),
			"network_settings": map[string]any{"cidr": "10.0.0.0/24", "evil_key": "rm -rf /"},
		},
	}
	// schemapb v1.6.0: an unknown key inside a Map VALUE (subnets.my-subnet-a
	// .evil_key) -- the exact config-injection shape this task's fix closes
	// (tfvars_schemapb.go's `case "map":` now emits a Strict-valued Map for
	// map(object({...})) terraform variables). The map KEY (my-subnet-a)
	// itself must never be flagged; only the unknown field inside its value.
	mapValueInvalidValues := map[string]any{
		"db_version":       "17",
		"threads":          float64(8),
		"maintenance_mode": false,
		"provider": map[string]any{
			"replicas":         float64(5),
			"network_settings": map[string]any{"cidr": "10.0.0.0/24"},
			"subnets": map[string]any{
				"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24", "evil_key": "rm -rf /"},
			},
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
		RuleInvalidValues:               ruleInvalidValues,
		RuleInvalidExpectedField:        "provider",
		NestedObjectInvalidValues:       nestedObjectInvalidValues,
		NestedObjectExpectedField:       "provider.network_settings.evil_key",
		MapValueInvalidValues:           mapValueInvalidValues,
		MapValueExpectedField:           "provider.subnets.my-subnet-a.evil_key",
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

	t.Run("rule_invalid_values fails the schema-level CEL/expr rule (replicas <= 10)", func(t *testing.T) {
		baked, ferrs, err := BakeForm(form, fixture.RuleInvalidValues)
		require.NoError(t, err)
		require.NotEmpty(t, ferrs, "Go: rule_invalid_values must fail the replicas<=10 rule")
		require.Nil(t, baked)
		require.True(t, hasFieldPath(ferrs, fixture.RuleInvalidExpectedField), "expected a FieldError at %q, got %+v", fixture.RuleInvalidExpectedField, ferrs)
	})

	t.Run("nested_object_invalid_values rejects an unknown key inside the strict nested network_settings object (SP-I1)", func(t *testing.T) {
		baked, ferrs, err := BakeForm(form, fixture.NestedObjectInvalidValues)
		require.NoError(t, err)
		require.NotEmpty(t, ferrs, "Go: unknown key inside a fixed-attribute nested object() must be rejected")
		require.Nil(t, baked)
		require.True(t, hasFieldPath(ferrs, fixture.NestedObjectExpectedField), "expected a FieldError at %q, got %+v", fixture.NestedObjectExpectedField, ferrs)
	})

	t.Run("map_value_invalid_values rejects an unknown key inside the strict Map value schema (schemapb v1.6.0)", func(t *testing.T) {
		baked, ferrs, err := BakeForm(form, fixture.MapValueInvalidValues)
		require.NoError(t, err)
		require.NotEmpty(t, ferrs, "Go: unknown key inside a Map value must be rejected")
		require.Nil(t, baked)
		require.True(t, hasFieldPath(ferrs, fixture.MapValueExpectedField), "expected a FieldError at %q, got %+v", fixture.MapValueExpectedField, ferrs)
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
