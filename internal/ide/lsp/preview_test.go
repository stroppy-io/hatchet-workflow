package lsp

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

func TestPreview_ReturnsCompiledPlanForValidBundle(t *testing.T) {
	svc := dsl.NewDslService()
	result, err := Preview(context.Background(), svc, loadBundle(t, postgresHADir))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Plan == nil {
		t.Fatalf("expected a non-nil compiled plan for a valid bundle, diagnostics: %+v", result.Diagnostics)
	}
}

func TestPreview_ReturnsNilPlanWithDiagnosticsForBrokenBundle(t *testing.T) {
	svc := dsl.NewDslService()
	bundle := map[string][]byte{"cluster.yaml": []byte(": not valid yaml")}
	result, err := Preview(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Plan != nil {
		t.Fatal("expected nil plan for a broken bundle")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic for a broken bundle")
	}
}
