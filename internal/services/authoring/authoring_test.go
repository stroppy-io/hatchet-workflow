package authoring

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherex/xlog"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
)

// stubResolver is a fake VersionResolver for the version-augmentation tests.
type stubResolver struct {
	versions []string
	err      error
}

func (s stubResolver) Versions(context.Context) ([]string, error) { return s.versions, s.err }
func (s stubResolver) Latest(context.Context) (string, error) {
	if len(s.versions) == 0 {
		return "", s.err
	}
	return s.versions[0], s.err
}

// svc builds an AuthoringService wired only with a logger + resolver. The
// augmentation path under test never touches authz, so a nil gate is fine.
func svc(r VersionResolver) *AuthoringService {
	return New(xlog.Default(), nil, r)
}

func hasVersionWarning(diags []*uipb.Diagnostic) bool {
	for _, d := range diags {
		if d.GetSeverity() == uipb.Severity_SEVERITY_WARNING &&
			d.GetCode() == uipb.DiagnosticCode_DIAGNOSTIC_CODE_STROPPY_VERSION_BELOW_MIN {
			return true
		}
	}
	return false
}

func TestAugment_WarnsWhenVersionNotPublished(t *testing.T) {
	s := svc(stubResolver{versions: []string{"v4.3.0", "v4.2.1"}})
	diags := s.augmentVersionDiagnostics(context.Background(), "v4.9.9", nil)
	if !hasVersionWarning(diags) {
		t.Fatalf("expected an unpublished-version WARNING, got %+v", diags)
	}
}

func TestAugment_NoWarnWhenPublished(t *testing.T) {
	s := svc(stubResolver{versions: []string{"v4.3.0", "v4.2.1"}})
	// "4.3.0" without the v prefix must still match "v4.3.0".
	diags := s.augmentVersionDiagnostics(context.Background(), "4.3.0", nil)
	if hasVersionWarning(diags) {
		t.Fatalf("did not expect a warning for a published version, got %+v", diags)
	}
}

func TestAugment_ResolverErrorDegradesToStaticCheck(t *testing.T) {
	// Network down: keep the diagnostics we were handed, add nothing.
	pre := []*uipb.Diagnostic{{
		Severity: uipb.Severity_SEVERITY_ERROR,
		Code:     uipb.DiagnosticCode_DIAGNOSTIC_CODE_STROPPY_VERSION_BELOW_MIN,
	}}
	s := svc(stubResolver{err: errors.New("github unreachable")})
	out := s.augmentVersionDiagnostics(context.Background(), "v3.0.0", pre)
	if len(out) != len(pre) {
		t.Fatalf("resolver error must not add diagnostics; got %d want %d", len(out), len(pre))
	}
}

func TestAugment_SkipsCommitAndEmptyVersions(t *testing.T) {
	s := svc(stubResolver{versions: []string{"v4.3.0"}})
	if d := s.augmentVersionDiagnostics(context.Background(), "commit:deadbeef", nil); hasVersionWarning(d) {
		t.Fatal("commit:<sha> dev version must not warn")
	}
	if d := s.augmentVersionDiagnostics(context.Background(), "", nil); hasVersionWarning(d) {
		t.Fatal("empty version must not warn")
	}
}

func TestAugment_NilResolverIsNoop(t *testing.T) {
	s := svc(nil)
	if d := s.augmentVersionDiagnostics(context.Background(), "v9.9.9", nil); len(d) != 0 {
		t.Fatalf("nil resolver must be a no-op, got %+v", d)
	}
}

func TestAugment_EmptyResolvedSetDoesNotWarn(t *testing.T) {
	// Cold cache + soft failure can yield an empty set without an error; do not
	// warn that every version is "unpublished" in that case.
	s := svc(stubResolver{versions: nil})
	if d := s.augmentVersionDiagnostics(context.Background(), "v4.3.0", nil); hasVersionWarning(d) {
		t.Fatal("empty resolved set must not warn")
	}
}
