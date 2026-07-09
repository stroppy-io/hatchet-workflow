// Package ide is SP-C Task 4: per-org code-server lifecycle and git worktree
// materialization. It sits behind internal/gateway's IdeAuthorizer/
// IdeBackendResolver seams (internal/gateway/ide_auth.go,
// internal/gateway/ide_backend.go) — nothing in internal/gateway imports
// this package (avoiding the reverse dependency), this package imports
// internal/gateway only to assert its types satisfy the gateway interfaces.
package ide

import (
	"fmt"
	"strings"
)

// ScopeKind distinguishes the instance repo from an org repo in an /ide/*
// request path.
type ScopeKind int

const (
	ScopeUnknown ScopeKind = iota
	ScopeInstance
	ScopeOrg
)

// Scope is the repo an /ide/* request targets, decoded from its URL path.
// Both gateway.IdeAuthorizer and gateway.IdeBackendResolver implementations
// in this package parse the SAME way (ParseScope) so a request is
// authorized against exactly the repo it will be proxied to — never a
// mismatch between what was checked and what was served.
type Scope struct {
	Kind ScopeKind
	// OrgSlug is set only when Kind == ScopeOrg — the tenant slug segment
	// from the URL (e.g. "/ide/org/acme/..." -> "acme"). It is a slug, not a
	// tenant id: callers resolve it to an id via their own TenantRepo (the
	// URL must not leak internal ids, matching every other org-scoped route
	// in this codebase).
	OrgSlug string
	// Rest is the request path with the "/ide/instance" or "/ide/org/<slug>"
	// prefix stripped (e.g. "/" or "/some/file"), forwarded to code-server
	// unchanged.
	Rest string
}

// ParseScope decodes an /ide/* request path into a Scope. Recognized forms:
//
//	/ide/instance          -> ScopeInstance, Rest="/"
//	/ide/instance/...      -> ScopeInstance, Rest="/..."
//	/ide/org/<slug>        -> ScopeOrg,     Rest="/"
//	/ide/org/<slug>/...    -> ScopeOrg,     Rest="/..."
//
// Anything else (including a bare "/ide/" with no scope segment, or an
// empty org slug) is an error — callers MUST fail closed rather than guess
// a default scope, since a misparsed scope is a potential cross-tenant
// authorization bypass.
func ParseScope(urlPath string) (Scope, error) {
	trimmed := strings.TrimPrefix(urlPath, "/ide/")
	if trimmed == urlPath {
		return Scope{}, fmt.Errorf("ide: path %q is not under /ide/", urlPath)
	}
	segments := strings.SplitN(trimmed, "/", 3)
	switch segments[0] {
	case "instance":
		return Scope{Kind: ScopeInstance, Rest: restOf(segments, 1)}, nil
	case "org":
		if len(segments) < 2 || segments[1] == "" {
			return Scope{}, fmt.Errorf("ide: path %q missing org slug", urlPath)
		}
		return Scope{Kind: ScopeOrg, OrgSlug: segments[1], Rest: restOf(segments, 2)}, nil
	default:
		return Scope{}, fmt.Errorf("ide: path %q has unrecognized scope %q (want \"instance\" or \"org\")", urlPath, segments[0])
	}
}

// restOf reconstructs the path remainder starting at segments[from:],
// always prefixed with "/" so an empty remainder still forwards a valid
// path to code-server.
func restOf(segments []string, from int) string {
	if from >= len(segments) {
		return "/"
	}
	rest := strings.Join(segments[from:], "/")
	if rest == "" {
		return "/"
	}
	return "/" + rest
}

// Key returns the stable identifier internal/ide.Manager uses to key a
// code-server container / worktree directory for this scope — "instance"
// for the singleton instance repo, "org:<slug>" for an org's repo. Using
// the URL slug (not a resolved tenant id) here is intentional: Manager
// itself is IdeAuthorizer-agnostic and must not need a TenantRepo lookup
// just to name a container; the authorizer is what maps slug -> id and
// enforces access before EnsureRunning is ever called.
func (s Scope) Key() string {
	if s.Kind == ScopeInstance {
		return "instance"
	}
	return "org:" + s.OrgSlug
}
