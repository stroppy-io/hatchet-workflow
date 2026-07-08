package schema

import (
	"sort"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// DeriveInputsSchema maps a workflow/component's declared "inputs:" (see
// ast.InputSpec) to a schemapb schema, mirroring
// DeriveProviderParamsSchemapb's best-effort posture: an input whose
// declared type isn't one of the known scalar kinds degrades to a
// permissive string field (with a warning diagnostic) rather than failing
// the whole derivation.
//
// NOTE on real builder API vs. the plan's sketch: as with
// DeriveProviderParamsSchemapb, schemapb v1.4.4's fluent builders are named
// Str/Int64/Bool (not String), matching the plan's Step 3 sketch for those
// three kinds -- confirmed against schemapb/new.go.
func DeriveInputsSchema(namespace string, inputs map[string]ast.InputSpec) (*schemapb.Schema, diag.List) {
	var diags diag.List

	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	sort.Strings(names)

	var fields []schemapb.FieldDef
	for _, name := range names {
		spec := inputs[name]
		switch spec.Type {
		case "string", "version", "":
			b := schemapb.Str(name)
			if s, ok := spec.Default.(string); ok {
				b.Default(s)
			}
			fields = append(fields, b)
		case "number", "int":
			b := schemapb.Int64(name)
			if d, ok := toInt64(spec.Default); ok {
				b.Default(d)
			}
			fields = append(fields, b)
		case "bool":
			b := schemapb.Bool(name)
			if v, ok := spec.Default.(bool); ok {
				b.Default(v)
			}
			fields = append(fields, b)
		default:
			diags.Warnf("", diag.Pos{}, "input %q: unknown type %q, treating as string", name, spec.Type)
			fields = append(fields, schemapb.Str(name))
		}
	}

	s, err := schemapb.NewSchema(namespace, "inputs", "1").Fields(fields...).Build()
	if err != nil {
		diags.Errorf("", diag.Pos{}, "build inputs schema: %v", err)
	}
	return s, diags
}

// toInt64 converts a numeric default value (which may have been decoded
// from YAML as int or from JSON-shaped sources as float64) to an int64.
// Any other shape (type mismatch) reports ok=false so the caller skips the
// default rather than guessing -- same pattern as tfvars_schemapb.go's
// toFloat64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}
