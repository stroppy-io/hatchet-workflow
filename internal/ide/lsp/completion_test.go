package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// TestCompletionItems_ListsTopLevelSchemaProperties reuses the postgres-ha
// golden bundle (same fixture diagnostics_test.go uses, and the same one
// internal/services/dsl/service_test.go's TestComposedSchemaContainsProviderParamField
// asserts a "zone" provider param field against) rather than fabricating a
// new bundle — CompletionItems must not silently drift from what
// ComposedSchema actually derives.
func TestCompletionItems_ListsTopLevelSchemaProperties(t *testing.T) {
	svc := dsl.NewDslService()
	items, err := CompletionItems(context.Background(), svc, loadBundle(t, postgresHADir))
	if err != nil {
		t.Fatalf("completion items: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one completion item from the composed schema")
	}

	byLabel := map[string]protocol.CompletionItem{}
	for _, it := range items {
		byLabel[it.Label] = it
	}

	// "provider" itself must appear as the top-level object field
	// schema.ComposeFormSchema nests provider params under (form.go: "the
	// composed root ... nested 'provider' object field").
	if _, ok := byLabel["provider"]; !ok {
		t.Fatalf("expected a top-level %q completion item, got %+v", "provider", labels(items))
	}
	// Nested provider params surface as dotted "provider.<name>" items —
	// "zone" is the yandex provider param service_test.go's
	// TestComposedSchemaContainsProviderParamField already asserts exists in
	// this exact fixture's schema_json.
	if _, ok := byLabel["provider.zone"]; !ok {
		t.Fatalf("expected a %q completion item for the nested provider param, got %+v", "provider.zone", labels(items))
	}
}

func TestCompletionItems_DockerBuiltinHasWorkflowInputsNoProviderField(t *testing.T) {
	svc := dsl.NewDslService()
	bundle := map[string][]byte{
		"cluster.yaml": []byte("version: 1\n" +
			"provider:\n  use: docker\n" +
			"machines:\n  db:\n    count: 1\n    resources: { cpu: 2, ram: 2g, disk: { size: 10g, type: ssd } }\n" +
			"services:\n  postgres:\n    on: db\n    image: postgres:17\n    network: host\n"),
		"workflow.yaml": []byte("inputs:\n  iterations: int\n" +
			"jobs:\n  postgres:\n    service: postgres\n"),
	}

	items, err := CompletionItems(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("completion items: %v", err)
	}
	byLabel := map[string]protocol.CompletionItem{}
	for _, it := range items {
		byLabel[it.Label] = it
	}
	if _, ok := byLabel["iterations"]; !ok {
		t.Fatalf("expected the workflow input %q hoisted to top level, got %+v", "iterations", labels(items))
	}
	if _, ok := byLabel["provider"]; ok {
		t.Fatalf("docker builtin has no tf module: expected no %q field, got %+v", "provider", labels(items))
	}
}

func labels(items []protocol.CompletionItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Label)
	}
	return out
}
