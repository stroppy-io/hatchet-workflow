package lsp

import (
	"context"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// PreviewResult is the stroppy/previewBundle custom LSP command's response
// payload: the compiled plan (nil if the bundle failed to compile) plus
// every diagnostic, mirroring dslpb.PreviewResponse's own shape 1:1 so a
// webview client can reuse RecipeEditor.tsx's existing plan-rendering logic
// without a second translation layer.
type PreviewResult struct {
	Plan        *dslpb.CompiledPlan `json:"plan"`
	Diagnostics []*dslpb.Diagnostic `json:"diagnostics"`
}

// Preview runs DslService.Preview in-process over bundleFiles — the exact
// "what will be provisioned" surface RecipeEditor.tsx's own preview button
// already calls over connect-rpc, reused here in-process rather than
// re-implemented. Like Preview itself, this never returns a Go error for a
// problem in the bundle's content (that comes back as Diagnostics with a
// nil Plan); an error return here means a genuine transport/programming
// failure.
func Preview(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) (*PreviewResult, error) {
	resp, err := svc.Preview(ctx, &dslpb.PreviewRequest{Files: bundleFiles})
	if err != nil {
		return nil, err
	}
	return &PreviewResult{Plan: resp.GetPlan(), Diagnostics: resp.GetDiagnostics()}, nil
}
