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

// ScopeKind distinguishes the instance repo namespace from an org repo
// namespace in an /ide/* request path.
type ScopeKind int

const (
	ScopeUnknown ScopeKind = iota
	ScopeInstance
	ScopeOrg
)

// EntryKindProvider/EntryKindWorkflow are the only two values an /ide/*
// path's kind segment may name — the same "provider"/"workflow" strings
// internal/services/catalog.BundleIdentity.KindStr and
// internal/gitrepo.EntryRepoName use, so a Scope's (EntryKind, EntrySlug)
// pair names EXACTLY the same repo the catalog storage layer would compute
// for the equivalent CatalogEntry — the IDE opens the identical physical
// repo a GetOrgProviderFiles/GetOrgWorkflowFiles call would read.
const (
	EntryKindProvider = "provider"
	EntryKindWorkflow = "workflow"
)

// Scope is the single catalog-entry repo an /ide/* request targets, decoded
// from its URL path. One catalog entry (one (level, [tenant], kind, slug)
// item) is one full Gitea repo (spec: "каждый провайдер или воркфлоу
// подготовленый это отдельный репозиторий полностью") — a Scope therefore
// names an ENTRY, not an org-wide directory the way the pre-redesign
// version of this type did (see git history: that version's Scope covered
// the tenant's whole org-catalog monorepo). Both gateway.IdeAuthorizer and
// gateway.IdeBackendResolver implementations in this package parse the SAME
// way (ParseScope) so a request is authorized against exactly the repo it
// will be proxied to — never a mismatch between what was checked and what
// was served.
type Scope struct {
	Kind ScopeKind
	// OrgSlug is set only when Kind == ScopeOrg — the tenant slug segment
	// from the URL (e.g. "/ide/org/acme/..." -> "acme"). It is a slug, not a
	// tenant id: callers resolve it to an id via their own TenantRepo (the
	// URL must not leak internal ids, matching every other org-scoped route
	// in this codebase). BackendResolver overwrites this field with the
	// resolved tenant id before calling Manager.EnsureRunning — see its doc.
	OrgSlug string
	// EntryKind is EntryKindProvider or EntryKindWorkflow — which catalog
	// repo namespace (within the instance org, or this OrgSlug's tenant org)
	// EntrySlug is looked up in.
	EntryKind string
	// EntrySlug is the catalog item's slug — together with EntryKind (and,
	// for ScopeOrg, the resolved tenant id) this is exactly the
	// BundleIdentity a catalog Create/Update call for the SAME entry would
	// build, so Manager can recompute the identical repo name
	// (gitrepo.EntryRepoName) without any catalog lookup.
	EntrySlug string
	// Rest is the request path with the
	// "/ide/instance/<kind>/<slug>" or "/ide/org/<orgSlug>/<kind>/<slug>"
	// prefix stripped (e.g. "/" or "/some/file"), forwarded to code-server
	// unchanged.
	Rest string
}

// ParseScope decodes an /ide/* request path into a Scope. Recognized forms:
//
//	/ide/instance/<kind>/<slug>          -> ScopeInstance, Rest="/"
//	/ide/instance/<kind>/<slug>/...      -> ScopeInstance, Rest="/..."
//	/ide/org/<orgSlug>/<kind>/<slug>     -> ScopeOrg,     Rest="/"
//	/ide/org/<orgSlug>/<kind>/<slug>/... -> ScopeOrg,     Rest="/..."
//
// <kind> MUST be exactly "provider" or "workflow" (EntryKindProvider/
// EntryKindWorkflow) — anything else is rejected rather than passed through,
// since it is used verbatim (after Manager recomputes the deterministic repo
// name from it) to pick a physical Gitea repo; accepting an arbitrary string
// here would let a hostile kind segment probe for repos this scheme never
// intended to name. Anything not matching one of the two forms above
// (missing org slug, missing kind, missing entry slug, or an unrecognized
// top-level segment) is an error — callers MUST fail closed rather than
// guess a default scope, since a misparsed scope is a potential
// cross-tenant/cross-entry authorization bypass.
func ParseScope(urlPath string) (Scope, error) {
	trimmed := strings.TrimPrefix(urlPath, "/ide/")
	if trimmed == urlPath {
		return Scope{}, fmt.Errorf("ide: path %q is not under /ide/", urlPath)
	}
	switch {
	case trimmed == "instance" || strings.HasPrefix(trimmed, "instance/"):
		rest := strings.TrimPrefix(strings.TrimPrefix(trimmed, "instance"), "/")
		kind, slug, tail, err := splitEntryPath(rest)
		if err != nil {
			return Scope{}, fmt.Errorf("ide: instance path %q: %w", urlPath, err)
		}
		return Scope{Kind: ScopeInstance, EntryKind: kind, EntrySlug: slug, Rest: tail}, nil
	case trimmed == "org" || strings.HasPrefix(trimmed, "org/"):
		afterOrg := strings.TrimPrefix(strings.TrimPrefix(trimmed, "org"), "/")
		segments := strings.SplitN(afterOrg, "/", 2)
		orgSlug := segments[0]
		if orgSlug == "" {
			return Scope{}, fmt.Errorf("ide: path %q missing org slug", urlPath)
		}
		rest := ""
		if len(segments) == 2 {
			rest = segments[1]
		}
		kind, slug, tail, err := splitEntryPath(rest)
		if err != nil {
			return Scope{}, fmt.Errorf("ide: org %q path %q: %w", orgSlug, urlPath, err)
		}
		return Scope{Kind: ScopeOrg, OrgSlug: orgSlug, EntryKind: kind, EntrySlug: slug, Rest: tail}, nil
	default:
		segment := strings.SplitN(trimmed, "/", 2)[0]
		return Scope{}, fmt.Errorf("ide: path %q has unrecognized scope %q (want \"instance\" or \"org\")", urlPath, segment)
	}
}

// splitEntryPath parses "<kind>/<slug>[/rest...]" (rest MAY be the
// remainder after the org slug for ScopeOrg, or everything after
// "instance/" for ScopeInstance) into (kind, slug, restPath). kind must be
// EntryKindProvider or EntryKindWorkflow; slug must be non-empty.
func splitEntryPath(rest string) (kind, slug, restPath string, err error) {
	segments := strings.SplitN(rest, "/", 3)
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return "", "", "", fmt.Errorf("missing entry kind/slug (want \"<provider|workflow>/<slug>\")")
	}
	kind = segments[0]
	if kind != EntryKindProvider && kind != EntryKindWorkflow {
		return "", "", "", fmt.Errorf("unrecognized entry kind %q (want %q or %q)", kind, EntryKindProvider, EntryKindWorkflow)
	}
	slug = segments[1]
	restPath = restOf(segments, 2)
	return kind, slug, restPath, nil
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
// code-server container / worktree directory for this scope —
// "instance:<kind>:<slug>" for a LEVEL_INSTANCE entry,
// "org:<slug>:<kind>:<entrySlug>" for a LEVEL_ORG entry. Using the URL slug
// (not a resolved tenant id) here is intentional: Manager itself is
// IdeAuthorizer-agnostic and must not need a TenantRepo lookup just to name
// a container; the authorizer/backend resolver is what maps slug -> id and
// enforces access before EnsureRunning is ever called (BackendResolver
// overwrites OrgSlug with the resolved tenant id before that call, so the
// key actually used to name the container/worktree is id-based in
// practice — see BackendResolver's doc).
func (s Scope) Key() string {
	if s.Kind == ScopeInstance {
		return "instance:" + s.EntryKind + ":" + s.EntrySlug
	}
	return "org:" + s.OrgSlug + ":" + s.EntryKind + ":" + s.EntrySlug
}
