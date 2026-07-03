package expr_test

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/expr"
)

func TestVisibilityRule(t *testing.T) {
	compEnv, err := expr.ComponentEnv()
	if err != nil {
		t.Fatalf("ComponentEnv: %v", err)
	}
	provEnv, err := expr.ProviderEnv()
	if err != nil {
		t.Fatalf("ProviderEnv: %v", err)
	}
	// компонент видит домен
	if d := expr.Check(compEnv, "inputs.nodes.count >= 3", "c.yaml", diag.Pos{}); d.HasErrors() {
		t.Fatalf("domain expr must typecheck: %+v", d)
	}
	// компонент НЕ видит ext — ключевой негативный тест спеки §10
	if d := expr.Check(compEnv, "machine.ext.platform_id == 'x'", "c.yaml", diag.Pos{}); !d.HasErrors() {
		t.Fatal("component must not see ext")
	}
	// провайдер видит ext
	if d := expr.Check(provEnv, "machine.ext.core_fraction == 100 ? machine.cpu >= 8 : true", "m.yaml", diag.Pos{}); d.HasErrors() {
		t.Fatalf("provider must see ext: %+v", d)
	}
}

func TestExtractAndEval(t *testing.T) {
	exprs := expr.Extract(`http://${{ machines.db.machines[0].ip }}:8008/${{ matrix.workload }}`)
	if len(exprs) != 2 {
		t.Fatalf("extract: %v", exprs)
	}
	env, err := expr.ComponentEnv()
	if err != nil {
		t.Fatalf("ComponentEnv: %v", err)
	}
	out, err := expr.Eval(env, "machines.db.count * 2", map[string]any{
		"machines": map[string]any{"db": expr.MachineGroupView{Count: 3}},
	})
	if err != nil || out != int64(6) {
		t.Fatalf("eval: %v %v", out, err)
	}
}

// --- edge cases beyond the brief's two locked tests ---

func TestCheckInvalidSyntaxReportsPos(t *testing.T) {
	env, err := expr.ComponentEnv()
	if err != nil {
		t.Fatalf("ComponentEnv: %v", err)
	}
	pos := diag.Pos{Line: 12, Col: 7}
	d := expr.Check(env, "inputs.nodes.count >=", "c.yaml", pos)
	if !d.HasErrors() {
		t.Fatal("syntactically invalid CEL must produce a diagnostic")
	}
	if len(d) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	got := d[0]
	if got.Path != "c.yaml" || got.Pos != pos {
		t.Fatalf("diagnostic anchored at wrong location: %+v", got)
	}
	if got.Message == "" {
		t.Fatal("diagnostic message must not be empty")
	}
}

func TestCheckValidExprNoDiagnostics(t *testing.T) {
	env, err := expr.ComponentEnv()
	if err != nil {
		t.Fatalf("ComponentEnv: %v", err)
	}
	if d := expr.Check(env, "1 + 1 == 2", "c.yaml", diag.Pos{}); len(d) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", d)
	}
}

func TestEvalTypeMismatchErrors(t *testing.T) {
	env, err := expr.ComponentEnv()
	if err != nil {
		t.Fatalf("ComponentEnv: %v", err)
	}
	// matrix is declared map[string]string; comparing it to an int is a
	// compile-time type error, which Eval must surface as a Go error.
	_, err = expr.Eval(env, "matrix == 5", nil)
	if err == nil {
		t.Fatal("expected type-mismatch error, got nil")
	}
}

func TestEvalRuntimeErrorSurfaces(t *testing.T) {
	env, err := expr.ComponentEnv()
	if err != nil {
		t.Fatalf("ComponentEnv: %v", err)
	}
	// inputs is dyn, so this typechecks but fails at runtime: dividing by
	// zero on a value that resolves to an int at eval time.
	_, err = expr.Eval(env, "10 / inputs.zero", map[string]any{
		"inputs": map[string]any{"zero": 0},
	})
	if err == nil {
		t.Fatal("expected runtime division-by-zero error, got nil")
	}
}

func TestExtractNoMarkers(t *testing.T) {
	if got := expr.Extract("plain string, no markers"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestExtractMultipleMarkers(t *testing.T) {
	got := expr.Extract("${{ a }}-${{ b }}-${{ c }}")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("extract: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extract[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractUnterminated(t *testing.T) {
	if got := expr.Extract("prefix ${{ a.b.c without a closer"); got != nil {
		t.Fatalf("unterminated marker must not match, got %v", got)
	}
}

func TestExtractTrimsWhitespace(t *testing.T) {
	got := expr.Extract("${{   spaced.out   }}")
	if len(got) != 1 || got[0] != "spaced.out" {
		t.Fatalf("extract did not trim whitespace: %v", got)
	}
}

func TestExtractExprsAreCELParseable(t *testing.T) {
	// Sanity: Extract's output is meant to be fed straight into Check/Eval.
	exprs := expr.Extract(`${{ machines.db.machines.map(m, m.ip).join(",") }}`)
	if len(exprs) != 1 {
		t.Fatalf("extract: %v", exprs)
	}
	if strings.Contains(exprs[0], "${{") || strings.Contains(exprs[0], "}}") {
		t.Fatalf("extracted expression retains marker syntax: %q", exprs[0])
	}
	env, err := expr.ComponentEnv()
	if err != nil {
		t.Fatalf("ComponentEnv: %v", err)
	}
	if d := expr.Check(env, exprs[0], "c.yaml", diag.Pos{}); d.HasErrors() {
		t.Fatalf("extracted expr must typecheck: %+v", d)
	}
}
