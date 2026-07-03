package diag_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

func TestListCollectsAndDetectsErrors(t *testing.T) {
	var l diag.List
	if l.HasErrors() {
		t.Fatal("empty list must not have errors")
	}
	l.Add(diag.Diagnostic{Severity: diag.Warning, Path: "cluster.yaml", Message: "w"})
	if l.HasErrors() {
		t.Fatal("warning is not an error")
	}
	l.Errorf("cluster.yaml", diag.Pos{Line: 3, Col: 5}, "bad field %q", "cpu")
	if !l.HasErrors() {
		t.Fatal("expected error after Errorf")
	}
	if got := l[1].Message; got != `bad field "cpu"` {
		t.Fatalf("unexpected message: %s", got)
	}
}
