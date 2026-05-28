package iam

import (
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// selfOrAdmin reports whether the caller is the named account or a platform admin.
func selfOrAdmin(c *iam.AccessClaims, accountID string) bool {
	return c.GetIsAdmin() || c.GetAccountId() == accountID
}

// hasAll reports whether granted satisfies every required permission, honoring
// ACTION_MANAGE as a wildcard implying every action on the same resource.
func hasAll(granted, required []*iam.Permission) bool {
	manage := make(map[iam.Resource]bool)
	exact := make(map[[2]int32]bool)
	for _, g := range granted {
		if g.GetAction() == iam.Action_ACTION_MANAGE {
			manage[g.GetResource()] = true
		}
		exact[[2]int32{int32(g.GetResource()), int32(g.GetAction())}] = true
	}
	for _, r := range required {
		if manage[r.GetResource()] {
			continue
		}
		if exact[[2]int32{int32(r.GetResource()), int32(r.GetAction())}] {
			continue
		}
		return false
	}
	return true
}

// emailDomain returns the lowercased domain part of an email, or "".
func emailDomain(email string) string {
	if i := strings.LastIndex(email, "@"); i >= 0 {
		return strings.ToLower(email[i+1:])
	}
	return ""
}

// domainAllowed reports whether email's domain is in allowed; an empty allow-list
// permits any domain.
func domainAllowed(email string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	d := emailDomain(email)
	for _, a := range allowed {
		if strings.EqualFold(strings.TrimSpace(a), d) {
			return true
		}
	}
	return false
}

// sanitizeNickname derives a handle-safe nickname from an email for JIT signup.
func sanitizeNickname(email, fallbackID string) string {
	local := email
	if before, _, found := strings.Cut(email, "@"); found {
		local = before
	}
	var b strings.Builder
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		}
	}
	nick := b.String()
	if len(nick) < 3 {
		nick = "user-" + fallbackID
	}
	if len(nick) > 64 {
		nick = nick[:64]
	}
	return nick
}
