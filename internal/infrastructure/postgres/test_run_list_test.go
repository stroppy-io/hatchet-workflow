package postgres

import (
	"strings"
	"testing"
)

func TestTestRunSearchClauseCoversNameAndDescription(t *testing.T) {
	b := &argBuilder{}
	clause := testRunSearchClause(b, "Needle")

	if !strings.Contains(clause, "name") {
		t.Fatalf("search clause %q does not include name", clause)
	}
	if !strings.Contains(clause, "description") {
		t.Fatalf("search clause %q does not include description", clause)
	}
	if len(b.args) != 1 {
		t.Fatalf("args len = %d, want 1", len(b.args))
	}
	if got, want := b.args[0], "%needle%"; got != want {
		t.Fatalf("search arg = %q, want %q", got, want)
	}
}
