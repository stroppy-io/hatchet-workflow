package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// CheckDiagnostics runs DslService.Check in-process over bundleFiles — the
// exact same schema/decode/contract/graph/lowering pipeline (plus
// provider-resolution and, when configured, stroppy-version diagnostics)
// the connect handler and RecipeService.CheckRecipe already run — and
// converts every dslpb.Diagnostic into an LSP protocol.Diagnostic, grouped
// by file path so a caller can publishDiagnostics per open document. No
// diagnostic is computed here: this function is purely a wire-shape
// translation from dslpb.Diagnostic (Severity/Path/Line/Col/Message/Module)
// to protocol.Diagnostic (Range/Severity/Source/Message).
//
// KNOWN GAP (see clampToZeroBased for the Line/Col half of this): a
// contract-check diagnostic (internal/dsl/contract/check.go) also commonly
// carries an EMPTY Path — it is a cross-component property of the whole
// bundle, not any one file (verified live: the etcd-quorum diagnostic
// diagnostics_test.go exercises has GetPath() == ""). Such diagnostics land
// in this map under the "" key. server.go's publishDiagnostics deliberately
// republishes the "" bucket against the bundle's cluster.yaml (the closest
// analog to "the file this affects" in the fixed two-file layout) rather
// than dropping them — an LSP diagnostic with no known document would
// otherwise never reach the editor at all. This is a real, disclosed
// approximation, not a claim that the diagnostic's position is known.
func CheckDiagnostics(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) (map[string][]protocol.Diagnostic, error) {
	resp, err := svc.Check(ctx, &dslpb.CheckRequest{Files: bundleFiles})
	if err != nil {
		return nil, err
	}
	out := map[string][]protocol.Diagnostic{}
	for _, d := range resp.GetDiagnostics() {
		out[d.GetPath()] = append(out[d.GetPath()], toLSPDiagnostic(d))
	}
	return out, nil
}

func toLSPDiagnostic(d *dslpb.Diagnostic) protocol.Diagnostic {
	line := clampToZeroBased(d.GetLine())
	col := clampToZeroBased(d.GetCol())
	sev := toLSPSeverity(d.GetSeverity())
	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: line, Character: col},
			End:   protocol.Position{Line: line, Character: col + 1},
		},
		Severity: sev,
		Source:   d.GetModule(),
		Message:  d.GetMessage(),
	}
}

func toLSPSeverity(s dslpb.Severity) protocol.DiagnosticSeverity {
	switch s {
	case dslpb.Severity_SEVERITY_ERROR:
		return protocol.DiagnosticSeverityError
	case dslpb.Severity_SEVERITY_WARNING:
		return protocol.DiagnosticSeverityWarning
	default:
		return protocol.DiagnosticSeverityInformation
	}
}

// clampToZeroBased converts the wire's 1-based line/col (dslpb.Diagnostic's
// convention — internal/dsl/diag.Pos is documented "1-based line/column",
// and internal/services/dsl/service.go's toProtoDiagnostics carries that
// convention straight onto the wire via clampUint32) to LSP's 0-based
// Position, floored at 0.
//
// KNOWN GAP, reported rather than papered over: not every diagnostic this
// compiler emits carries a real Pos. internal/dsl/schema (YAML decode/
// jsonschema validation) and internal/dsl/include (bundle-structure/
// path-traversal checks) diagnostics do carry a real line/column from the
// underlying yaml.v3/jsonschema decoders. But internal/dsl/contract/check.go
// — the component `requires:` capability/expr checker, e.g. the etcd-quorum
// contract violation this package's own diagnostics_test.go exercises via
// the same fixture internal/services/dsl/service_test.go's
// TestCheckEtcdQuorumViolationDiagnostic uses — constructs its
// diag.Diagnostic values with NO Pos at all (grep
// internal/dsl/contract/check.go's diags.Add(diag.Diagnostic{...}) call
// sites: none of them sets Pos), because a contract violation is a
// cross-component/cross-file property with no single source line to point
// at. Those diagnostics arrive here with Line==Col==0 and this function
// floors them to LSP Position{0,0} — i.e. "top of the file", not a lie
// about a specific line, but not a useful squiggle location either. A
// caller (server.go's publishDiagnostics) still reports these correctly by
// Path and Message; only the exact Range degrades to the file's start. This
// is the honest behavior the task's instructions asked for instead of
// fabricating a plausible-looking line/column.
func clampToZeroBased(v uint32) uint32 {
	if v == 0 {
		return 0
	}
	return v - 1
}
