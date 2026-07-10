package gitrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// InstanceOrg is the fixed Gitea org every LEVEL_INSTANCE catalog entry's own
// repo is created under — one repo per (kind, slug) pair inside it (see
// EntryRepoName), never the tenant-facing org-per-tenant namespace TenantOrg
// builds. Fixed and never derived from user input, so it can never collide
// with (or be spoofed by) a hostile tenant slug.
const InstanceOrg = "stroppy-instance"

// tenantOrgPrefix namespaces every tenant's Gitea org away from InstanceOrg
// and from each other.
const tenantOrgPrefix = "tenant-"

// slugSanitizeRe matches every byte NOT allowed in the sanitized segment of a
// Gitea repo/org name this package mints (lower-case alnum, dot, underscore,
// hyphen) — anything else (path separators, whitespace, unicode, shell
// metacharacters, "..") collapses to a single "-".
var slugSanitizeRe = regexp.MustCompile(`[^a-z0-9._-]+`)

// maxSanitizedSlugLen bounds the human-readable portion of a minted repo/org
// name so the (sanitized-slug + "-" + 8-hex-hash) result stays comfortably
// under Gitea's own repo-name length limit regardless of how long a
// user-supplied slug is.
const maxSanitizedSlugLen = 60

// SanitizeSlug lower-cases raw and replaces every character outside
// [a-z0-9._-] with "-", trims leading/trailing separator noise, and falls
// back to "entry" for a raw slug that sanitizes to nothing (e.g. all
// symbols/unicode) — the caller (EntryRepoName/TenantOrg) always appends a
// content hash after this, so a degenerate/collided sanitized form never
// causes two different raw inputs to name the same Gitea object.
func SanitizeSlug(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	cleaned := slugSanitizeRe.ReplaceAllString(lower, "-")
	cleaned = strings.Trim(cleaned, "-._")
	if cleaned == "" {
		cleaned = "entry"
	}
	if len(cleaned) > maxSanitizedSlugLen {
		cleaned = strings.Trim(cleaned[:maxSanitizedSlugLen], "-._")
	}
	return cleaned
}

// shortHash returns the first 8 hex characters of sha256(raw) — appended to
// every sanitized name this file mints so that (a) two raw inputs whose
// sanitized forms collide (e.g. "Foo/Bar" and "foo-bar") never name the same
// Gitea org/repo, and (b) the mapping is a pure function of raw, so no
// lookup table is needed to reproduce a name deterministically (required for
// the idempotent migration path: a name can always be recomputed from a
// catalog_entries row's (level, tenant_id, kind, slug) alone).
func shortHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:8]
}

// TenantOrg returns the Gitea org every one of tenantID's own catalog-entry
// repos (and its IDE org-catalog worktrees, one repo per entry now — see
// internal/ide) is created under. tenantID is expected to be an internal
// UUID (never a user-supplied slug — callers must resolve a URL-facing org
// slug to a tenant id BEFORE calling this, exactly like every other
// org-scoped code path in this codebase), but SanitizeSlug+shortHash are
// still applied defensively: even a maximally hostile tenantID can only ever
// widen or shrink ITS OWN org name, never produce InstanceOrg's fixed string
// or collide with another tenant's org (the hash is keyed on the exact
// tenantID string).
func TenantOrg(tenantID string) string {
	return tenantOrgPrefix + SanitizeSlug(tenantID) + "-" + shortHash(tenantID)
}

// EntryRepoName returns the repo name for one catalog entry "item" — a
// (kindStr, slug) pair scoped within whatever org OwnerFor resolves for its
// (level, tenantID) — where kindStr is "provider" or "workflow" (a plain
// string so this low-level package never needs to import catalogpb; callers
// pass catalogpb.Kind's lower-cased String() suffix). All versions of the
// same catalog item (same level/tenant/kind/slug) share ONE repo — a version
// is a commit in that repo's history, never a new repo — so this name must
// be a pure, stable function of (kindStr, slug) alone, recomputable without
// any lookup (the same property TenantOrg documents, needed for idempotent
// migration/repair).
func EntryRepoName(kindStr, slug string) string {
	return kindStr + "-" + SanitizeSlug(slug) + "-" + shortHash(slug)
}

// OwnerFor returns the Gitea org an entry's repo lives under: InstanceOrg for
// a LEVEL_INSTANCE entry (tenantID must be "" — same invariant
// catalog.requireLevel already enforces one layer up), TenantOrg(tenantID)
// for a LEVEL_ORG entry.
func OwnerFor(isInstanceLevel bool, tenantID string) string {
	if isInstanceLevel {
		return InstanceOrg
	}
	return TenantOrg(tenantID)
}
