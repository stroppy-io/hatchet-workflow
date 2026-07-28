package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Run-scope cookie: an authenticated in-app viewer's equivalent of a public
// share token. A share link scopes embedded Grafana via a share token; a
// logged-in user viewing their own run has no share, so the run detail page
// exchanges its authenticated session (server-side, access-checked) for one of
// these — a signed run id the gateway trusts to scope /public/metrics without a
// share record. It grants nothing but reading that one run's series.
//
// Signed with an HMAC secret shared by the minter (the authenticated
// GrafanaSession RPC) and this verifier. The token is NOT a bearer: possessing
// it lets you read one run's metrics, which the holder was already authorised
// for when it was minted.
const runScopePrefix = "run."

// SignRunScope produces the opaque cookie value for runID: "run.<runID>.<sig>".
// runID is carried in the clear (it is not a secret); the signature is what the
// gateway checks. Returns "" if unusable so callers can treat it as "no scope".
func SignRunScope(secret, runID string) string {
	if secret == "" || runID == "" || strings.ContainsAny(runID, ".") {
		return ""
	}
	return runScopePrefix + runID + "." + runScopeSig(secret, runID)
}

// verifyRunScope returns the runID a token attests to, or ok=false. A tampered
// runID changes the payload and fails the constant-time signature check.
func verifyRunScope(secret, token string) (string, bool) {
	if secret == "" || !strings.HasPrefix(token, runScopePrefix) {
		return "", false
	}
	body := strings.TrimPrefix(token, runScopePrefix)
	dot := strings.LastIndexByte(body, '.')
	if dot <= 0 {
		return "", false
	}
	runID, sig := body[:dot], body[dot+1:]
	if !hmac.Equal([]byte(sig), []byte(runScopeSig(secret, runID))) {
		return "", false
	}
	return runID, true
}

func runScopeSig(secret, runID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(runID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
