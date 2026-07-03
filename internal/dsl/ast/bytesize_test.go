package ast_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
)

func TestParseByteSize(t *testing.T) {
	cases := []struct {
		in   string
		want ast.ByteSize
	}{
		{"512", 512},
		{"512k", 512 << 10},
		{"100m", 100 << 20},
		{"32g", 32 << 30},
		{"32G", 32 << 30},
		{" 32g ", 32 << 30},
	}
	for _, c := range cases {
		got, err := ast.ParseByteSize(c.in)
		if err != nil {
			t.Fatalf("ParseByteSize(%q): unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParseByteSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseByteSizeErrors(t *testing.T) {
	for _, in := range []string{"", "notasize", "g", "12x", "-5g"} {
		if _, err := ast.ParseByteSize(in); err == nil {
			t.Fatalf("ParseByteSize(%q): expected error, got nil", in)
		}
	}
}

func TestParseDuration(t *testing.T) {
	got, err := ast.ParseDuration("120s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ast.Duration(120e9) {
		t.Fatalf("ParseDuration(120s) = %v, want 120s", got)
	}
}

func TestParseDurationError(t *testing.T) {
	if _, err := ast.ParseDuration("notaduration"); err == nil {
		t.Fatal("expected error for bad duration string")
	}
}
