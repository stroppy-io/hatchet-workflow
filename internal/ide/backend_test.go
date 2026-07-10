package ide

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// fakeRunner records every scope it was asked to run, keyed by scope.Key(),
// so tests can assert two different orgs never got routed to the same
// running instance.
type fakeRunner struct {
	byKey map[string]string // scope.Key() -> fake address
	calls []Scope
}

func newFakeRunner() *fakeRunner { return &fakeRunner{byKey: map[string]string{}} }

func (f *fakeRunner) EnsureRunning(_ context.Context, scope Scope) (string, error) {
	f.calls = append(f.calls, scope)
	addr := "http://stroppy-ide-" + scope.Key() + ":" + codeServerPort
	f.byKey[scope.Key()] = addr
	return addr, nil
}

func TestBackendResolver_ResolvesOrgSlugToTenantIdBeforeCallingManager(t *testing.T) {
	runner := newFakeRunner()
	b := &BackendResolver{
		Manager: runner,
		Tenants: fakeTenantResolver{bySlug: map[string]*iampb.Tenant{"acme": {Id: "tenant-acme-id"}}},
	}
	addr, err := b.Backend(httptest.NewRequest(http.MethodGet, "/ide/org/acme/provider/docker/file.yaml", nil))
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected exactly 1 EnsureRunning call, got %d", len(runner.calls))
	}
	got := runner.calls[0]
	if got.Kind != ScopeOrg || got.OrgSlug != "tenant-acme-id" {
		t.Fatalf("Manager was called with the raw slug, not the resolved tenant id: %+v", got)
	}
	if addr == "" {
		t.Fatal("expected a non-empty backend address")
	}
}

func TestBackendResolver_InstanceScope_NeedsNoTenantResolution(t *testing.T) {
	runner := newFakeRunner()
	b := &BackendResolver{Manager: runner}
	if _, err := b.Backend(httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/", nil)); err != nil {
		t.Fatalf("backend: %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0].Kind != ScopeInstance {
		t.Fatalf("expected 1 ScopeInstance call, got %+v", runner.calls)
	}
}

func TestBackendResolver_UnknownOrgSlugFailsClosed(t *testing.T) {
	runner := newFakeRunner()
	b := &BackendResolver{
		Manager: runner,
		Tenants: fakeTenantResolver{bySlug: map[string]*iampb.Tenant{}},
	}
	if _, err := b.Backend(httptest.NewRequest(http.MethodGet, "/ide/org/nonexistent/provider/docker/", nil)); err == nil {
		t.Fatal("expected an error for an unknown org slug")
	}
	if len(runner.calls) != 0 {
		t.Fatal("Manager must never be called for an unresolvable org slug")
	}
}

// TestBackendResolver_TwoOrgsNeverShareAScope proves distinct orgs resolve
// to distinct Manager scopes (and therefore distinct containers/worktrees —
// see manager.go's isolation doc) even when their requests arrive on the
// same BackendResolver instance concurrently in practice.
func TestBackendResolver_TwoOrgsNeverShareAScope(t *testing.T) {
	runner := newFakeRunner()
	b := &BackendResolver{
		Manager: runner,
		Tenants: fakeTenantResolver{bySlug: map[string]*iampb.Tenant{
			"acme": {Id: "tenant-a"},
			"beta": {Id: "tenant-b"},
		}},
	}
	addrA, err := b.Backend(httptest.NewRequest(http.MethodGet, "/ide/org/acme/provider/docker/", nil))
	if err != nil {
		t.Fatalf("acme: %v", err)
	}
	addrB, err := b.Backend(httptest.NewRequest(http.MethodGet, "/ide/org/beta/provider/docker/", nil))
	if err != nil {
		t.Fatalf("beta: %v", err)
	}
	if addrA == addrB {
		t.Fatalf("two different orgs resolved to the same backend address %q", addrA)
	}
}

func TestBackendResolver_NilManagerFailsClosed(t *testing.T) {
	b := &BackendResolver{}
	if _, err := b.Backend(httptest.NewRequest(http.MethodGet, "/ide/instance/provider/docker/", nil)); err == nil {
		t.Fatal("expected an error with no Manager configured")
	}
}

// TestBackendResolver_RewritesPathToScopeRest guards the wiring: code-server
// serves from its own root, so the /ide/<scope>/… prefix must be peeled off
// before the gateway proxies. Leaving it on made every request reach the
// editor as an unknown path.
func TestBackendResolver_RewritesPathToScopeRest(t *testing.T) {
	runner := newFakeRunner()
	b := &BackendResolver{Manager: runner}

	req := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/yandex/stable/manifest.json", nil)
	if _, err := b.Backend(req); err != nil {
		t.Fatalf("backend: %v", err)
	}
	if got, want := req.URL.Path, "/stable/manifest.json"; got != want {
		t.Fatalf("proxied path = %q, want %q", got, want)
	}
}

func TestBackendResolver_RewritesBareScopePathToRoot(t *testing.T) {
	runner := newFakeRunner()
	b := &BackendResolver{Manager: runner}

	req := httptest.NewRequest(http.MethodGet, "/ide/instance/provider/yandex/", nil)
	if _, err := b.Backend(req); err != nil {
		t.Fatalf("backend: %v", err)
	}
	if got, want := req.URL.Path, "/"; got != want {
		t.Fatalf("proxied path = %q, want %q", got, want)
	}
}
