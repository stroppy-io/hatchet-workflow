package expr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// interpBraces matches ${{ ... }} interpolation markers. The body is
// matched non-greedily and does not support nesting: "${{ a }} ${{ b }}"
// yields two expressions ("a" and "b"), but a literal "}}" or "${{" inside
// an expression's own string/map/list literals is not distinguished from a
// marker boundary. Recipes needing such characters inside an expression are
// out of scope for this compiler stage.
var interpBraces = regexp.MustCompile(`\$\{\{(.*?)\}\}`)

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
	return out.Value(), nil
}
