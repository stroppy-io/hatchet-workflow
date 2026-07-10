package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// CheckDiagnostics runs DslService.Check in-process over bundleFiles — the
// SAME check-mode compile pipeline (schema, decode, contract, graph,
// lowering — see internal/services/dsl/service.go's Check doc comment) the
// connect RPC and the recipe create-time Summary.compiles path both use —
// and converts every dslpb.Diagnostic it returns into an LSP
// protocol.Diagnostic, grouped by bundle-relative file path so a caller can
// publish one textDocument/publishDiagnostics notification per open
// document. No validation happens in this function; it is a pure wire
// conversion over Check's own output.
func CheckDiagnostics(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) (map[string][]protocol.Diagnostic, error) {
	resp, err := svc.Check(ctx, &dslpb.CheckRequest{Files: bundleFiles})
	if err != nil {
		// Check's own doc comment states it never returns an RPC error for a
		// bundle-content problem — an error here is a genuine transport/
		// programming failure, not a lint result, so it MUST propagate as an
		// error rather than being folded into an empty diagnostics map.
		return nil, err
	}

	out := map[string][]protocol.Diagnostic{}
	for _, d := range resp.GetDiagnostics() {
		line := clampLSPLine(d.GetLine())
		col := clampLSPLine(d.GetCol())
		out[d.GetPath()] = append(out[d.GetPath()], protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: line, Character: col},
				End:   protocol.Position{Line: line, Character: col + 1},
			},
			Severity: toLSPSeverity(d.GetSeverity()),
			Source:   d.GetModule(),
			Message:  d.GetMessage(),
		})
	}
	return out, nil
}

// toLSPSeverity maps dslpb.Severity onto protocol.DiagnosticSeverity.
// dslpb.Severity_SEVERITY_UNSPECIFIED (the enum's zero value; Check/Preview
// never actually emit it — toProtoDiagnostics in service.go always converts
// a real diag.Severity, which is either Error or Warning) maps to
// Information defensively rather than silently dropping an unrecognized
// value.
func toLSPSeverity(s dslpb.Severity) protocol.DiagnosticSeverity {
	switch s {
	case dslpb.Severity_SEVERITY_ERROR:
		return protocol.DiagnosticSeverityError
	case dslpb.Severity_SEVERITY_WARNING:
		return protocol.DiagnosticSeverityWarning
	case dslpb.Severity_SEVERITY_UNSPECIFIED:
		return protocol.DiagnosticSeverityInformation
	default:
		return protocol.DiagnosticSeverityInformation
	}
}

// clampLSPLine converts the wire's 1-based line/col (dslpb.Diagnostic's
// convention — see internal/dsl/diag.Pos's doc comment: "a 1-based
// line/column position") to LSP's 0-based Position, floored at 0.
//
// GAP, reported honestly rather than papered over: dslpb.Diagnostic.Line/Col
// are 0 for a diagnostic diag.Errorf/Warnf raised with a bare diag.Pos{}
// (e.g. service.go's "load terraform module" failure, or any
// bundle/provider-level diagnostic with no specific source line) — this
// function cannot distinguish "line 0 means unknown position" from "line 0
// means the wire literally carries 1-based line 1 minus a clamp that never
// happened" because the wire format conflates both into the same zero
// value. It resolves the ambiguity the only honest way available without
// widening the Diagnostic proto: 0 stays 0 (renders at the document's very
// first character), which is never wrong for "unknown," and is only
// One-off-wrong (LSP line 0 instead of -1, which does not exist) for a
// diagnostic that somehow legitimately meant wire line 1 rendered as
// document line 0 — which is the correct LSP answer anyway. A caller that
// needs to tell "genuinely unknown" apart from "known position at the very
// first character" would need internal/dsl/diag.Pos itself to grow an
// explicit "known" bit; that is a compiler-side change, out of this
// package's reuse-only scope, and is flagged in the T5-T7 report rather
// than worked around here with an invented heuristic.
func clampLSPLine(v uint32) uint32 {
	if v == 0 {
		return 0
	}
	return v - 1
}
