package ide

import (
	"context"
	"fmt"
	"net/http"

	"github.com/stroppy-io/stroppy-cloud/internal/gateway"
)

// runner is the subset of *Manager BackendResolver needs — a narrow
// interface so tests can fake container/worktree lifecycle without a live
// docker daemon or Gitea. *Manager satisfies it structurally.
type runner interface {
	EnsureRunning(ctx context.Context, scope Scope) (string, error)
}

// BackendResolver implements gateway.IdeBackendResolver over a Manager: it
// parses the request's Scope (ParseScope), resolves an org scope's URL slug
// to a real tenant id via Tenants (the same lookup Authorizer.CanAuthor
// already performed to authorize this same request — gateway.go only calls
// Backend AFTER CanAuthor returned true, see serveHTTP), and delegates to
// Manager.EnsureRunning, which starts (or reuses) that scope's — and ONLY
// that scope's — container and worktree (see manager.go's isolation doc).
//
// Using the resolved tenant id (not the raw slug) as the scope Manager
// operates on matters: two orgs could in principle pick colliding slugs
// across a slug rename race, but ids are immutable — pinning Manager's
// container/worktree naming to the id, not the slug, is what keeps a
// worktree from ever being reused across a slug handoff.
type BackendResolver struct {
	Manager runner
	Tenants TenantResolver
}

var _ gateway.IdeBackendResolver = (*BackendResolver)(nil)

// Backend implements gateway.IdeBackendResolver.
func (b *BackendResolver) Backend(r *http.Request) (string, error) {
	if b == nil || b.Manager == nil {
		return "", fmt.Errorf("ide: backend resolver not configured")
	}
	scope, err := ParseScope(r.URL.Path)
	if err != nil {
		return "", err
	}
	if scope.Kind == ScopeOrg {
		if b.Tenants == nil {
			return "", fmt.Errorf("ide: backend resolver has no TenantResolver, cannot resolve org slug %q", scope.OrgSlug)
		}
		tenant, err := b.Tenants.GetBySlug(r.Context(), scope.OrgSlug)
		if err != nil || tenant.GetId() == "" {
			return "", fmt.Errorf("ide: unknown org slug %q", scope.OrgSlug)
		}
		scope.OrgSlug = tenant.GetId()
	}
	target, err := b.Manager.EnsureRunning(r.Context(), scope)
	if err != nil {
		return "", err
	}
	// code-server serves from its own root, not from /ide/<scope>/…. Rewrite
	// the request onto the scope-relative remainder ParseScope already peeled
	// off, or every proxied request reaches the editor as an unknown path.
	r.URL.Path = scope.Rest
	return target, nil
}
