package expr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// interpBraces matches ${{ ... }} interpolation markers. The body is
// matched non-greedily and does not support nesting: "${{ a }} ${{ b }}"
// yields two expressions ("a" and "b"), but a literal "}}" or "${{" inside
// an expression's own string/map/list literals is not distinguished from a
// marker boundary. Recipes needing such characters inside an expression are
// out of scope for this compiler stage. The (?s) flag makes "." match
// newlines too, so a marker whose body spans multiple lines (e.g. a
// multi-line CEL expression written for readability in YAML) is still
// captured whole instead of silently dropped.
var interpBraces = regexp.MustCompile(`(?s)\$\{\{(.*?)\}\}`)

// Extract returns the CEL expressions embedded in s via ${{ ... }}
// interpolation markers, in order of appearance, with surrounding
// whitespace trimmed. It returns nil if s contains no markers. An
// unterminated marker (a "${{" with no matching "}}") is not matched and
// contributes nothing to the result.
func Extract(s string) []string {
	matches := interpBraces.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

// Check typechecks expr against env and reports any compile errors as
// diagnostics anchored at (path, pos) — the location of the string that
// hosts the expression, since CEL's own in-expression column offsets are
// not meaningful to a recipe author working in YAML. Diagnostic.Module is
// left empty for the caller (contract-check) to fill in with the
// component/provider that authored the rule.
func Check(env *cel.Env, expr, path string, pos diag.Pos) diag.List {
	var list diag.List
	_, iss := env.Compile(expr)
	if iss == nil {
		return list
	}
	for _, e := range iss.Errors() {
		list.Add(diag.Diagnostic{
			Severity: diag.Error,
			Path:     path,
			Pos:      pos,
			Message:  e.Message,
		})
	}
	return list
}

// Eval compiles and evaluates expr against env with the given variable
// bindings, returning the result as a native Go value. It is used both for
// contract-check-time evaluation (e.g. constant-folding constraints) and at
// runtime by the recipe executor.
//
// The result is normalized before being returned (see normalizeEvalResult):
// any protobuf well-known-type wrapper reachable from ProviderMachineView.Ext
// (*structpb.Struct, *structpb.ListValue, *structpb.Value — see env.go's
// ProviderMachineView doc for why Ext is shaped that way) is converted to
// its native Go equivalent, recursively, so callers never see a proto type
// leak out of this package. One consequence of that conversion: numbers
// that originate from ext data come back as float64 (structpb.Value only
// carries JSON number semantics), while CEL-native integers (MachineView/
// ProviderMachineView int64 fields, integer literals, ...) come back as
// int64. Callers must not assume both sides of a comparison built from
// Eval's result share the same Go numeric type.
func Eval(env *cel.Env, expr string, vars map[string]any) (any, error) {
	ast, iss := env.Compile(expr)
	if iss != nil && iss.Err() != nil {
		return nil, fmt.Errorf("compile %q: %w", expr, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program %q: %w", expr, err)
	}
	out, _, err := prg.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("eval %q: %w", expr, err)
	}
	return normalizeEvalResult(out.Value()), nil
}

// normalizeEvalResult recursively converts protobuf well-known-type wrapper
// values into native Go types: *structpb.Struct becomes map[string]any (via
// AsMap), *structpb.ListValue becomes []any (via AsSlice), and *structpb.Value
// becomes whatever AsInterface resolves it to. Nested map[string]any and
// []any values are walked too, in case a proto wrapper is embedded inside a
// CEL-constructed map or list rather than being the top-level result. CEL's
// own native types (int64, string, bool, ...) pass through unchanged.
func normalizeEvalResult(v any) any {
	switch t := v.(type) {
	case *structpb.Struct:
		return normalizeEvalResult(t.AsMap())
	case *structpb.ListValue:
		return normalizeEvalResult(t.AsSlice())
	case *structpb.Value:
		return normalizeEvalResult(t.AsInterface())
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeEvalResult(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeEvalResult(val)
		}
		return out
	default:
		return v
	}
}
