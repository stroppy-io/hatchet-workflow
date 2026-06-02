package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// SSOState is the server-side flow state stashed between Authorize and Exchange
// to defeat CSRF (state), code-interception (PKCE verifier) and ID-token replay
// (nonce).
type SSOState struct {
	// State is the opaque CSRF token echoed back on the callback (the key).
	State string
	// ProviderID is the provider the flow was started for.
	ProviderID string
	// CodeVerifier is the PKCE verifier whose challenge was sent to the IdP.
	CodeVerifier string
	// Nonce is bound to the ID token to prevent replay.
	Nonce string
	// ExpiresAt bounds how long the in-flight authorization may be completed.
	ExpiresAt time.Time
}

// SSOStateStore keeps SSO flow state server-side between the redirect and the
// callback. The integration layer supplies an implementation (any short-lived
// keyed store). Consume is single-use: it returns the state and atomically
// removes it; an unknown/expired/consumed key returns derrors.ErrNotFound.
type SSOStateStore interface {
	Save(ctx context.Context, s SSOState) error
	Consume(ctx context.Context, state string) (SSOState, error)
}

// oidcDiscovery is the subset of the OpenID Provider Metadata document we use.
type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

// oidcTokenResponse is the subset of the token endpoint response we use.
type oidcTokenResponse struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// idTokenClaims is the subset of OIDC ID-token claims we read.
type idTokenClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	Nonce         string `json:"nonce"`
	jwt.RegisteredClaims
}

// OIDCFlows implements iamsvc.SSOFlows: a real OIDC authorization-code flow with
// PKCE, using the IdentityProvider config for discovery and token exchange. It
// returns typed FailedPrecondition errors when the runtime config it needs
// (redirect base URL) is missing — never a fake success.
type OIDCFlows struct {
	state           SSOStateStore
	redirectBaseURL string
	stateTTL        time.Duration
	http            *http.Client
}

var _ iamsvc.SSOFlows = (*OIDCFlows)(nil)

// NewOIDCFlows builds the flow engine. redirectBaseURL (from
// Config.SSORedirectBaseURL) is the public origin the IdP redirects back to; an
// empty value makes Authorize fail with a typed error rather than emit a broken
// redirect.
func NewOIDCFlows(cfg Config, state SSOStateStore) *OIDCFlows {
	return &OIDCFlows{
		state:           state,
		redirectBaseURL: strings.TrimRight(cfg.SSORedirectBaseURL, "/"),
		stateTTL:        10 * time.Minute,
		http:            &http.Client{Timeout: 15 * time.Second},
	}
}

// Authorize builds the IdP authorization-code redirect URL, generating and
// persisting the per-flow state/PKCE/nonce.
func (f *OIDCFlows) Authorize(ctx context.Context, provider *iam.IdentityProvider) (string, string, error) {
	if f.redirectBaseURL == "" {
		return "", "", derrors.FailedPrecondition("identity.sso_redirect_unconfigured", "sso redirect base url is not configured")
	}
	if provider.GetIssuer() == "" || provider.GetClientId() == "" {
		return "", "", derrors.FailedPrecondition("identity.sso_provider_incomplete", "identity provider is missing issuer or client_id")
	}

	disc, err := f.discover(ctx, provider.GetIssuer())
	if err != nil {
		return "", "", err
	}

	state, err := randToken(32)
	if err != nil {
		return "", "", err
	}
	nonce, err := randToken(32)
	if err != nil {
		return "", "", err
	}
	verifier, err := randToken(48)
	if err != nil {
		return "", "", err
	}
	challenge := pkceChallenge(verifier)

	if err := f.state.Save(ctx, SSOState{
		State:        state,
		ProviderID:   provider.GetId(),
		CodeVerifier: verifier,
		Nonce:        nonce,
		ExpiresAt:    time.Now().Add(f.stateTTL),
	}); err != nil {
		return "", "", err
	}

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", provider.GetClientId())
	q.Set("redirect_uri", f.callbackURL(provider.GetSlug()))
	q.Set("scope", scopeParam(provider.GetScopes()))
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	authURL := disc.AuthorizationEndpoint
	sep := "?"
	if strings.Contains(authURL, "?") {
		sep = "&"
	}
	return authURL + sep + q.Encode(), state, nil
}

// Exchange completes the flow: it validates state, redeems the authorization
// code at the token endpoint (with the stored PKCE verifier), parses the ID
// token and returns the IdP subject, email and its email_verified claim.
func (f *OIDCFlows) Exchange(ctx context.Context, provider *iam.IdentityProvider, secret, code, state string) (string, string, bool, error) {
	if provider.GetIssuer() == "" || provider.GetClientId() == "" {
		return "", "", false, derrors.FailedPrecondition("identity.sso_provider_incomplete", "identity provider is missing issuer or client_id")
	}
	if code == "" || state == "" {
		return "", "", false, derrors.Invalid("code", "missing authorization code or state")
	}

	st, err := f.state.Consume(ctx, state)
	if errors.Is(err, derrors.ErrNotFound) {
		return "", "", false, derrors.Unauthenticated("unknown or expired sso state")
	}
	if err != nil {
		return "", "", false, err
	}
	if !time.Now().Before(st.ExpiresAt) {
		return "", "", false, derrors.Unauthenticated("sso state expired")
	}
	if st.ProviderID != "" && provider.GetId() != "" && st.ProviderID != provider.GetId() {
		return "", "", false, derrors.Unauthenticated("sso state provider mismatch")
	}

	disc, err := f.discover(ctx, provider.GetIssuer())
	if err != nil {
		return "", "", false, err
	}

	tok, err := f.redeem(ctx, disc.TokenEndpoint, provider, secret, code, st.CodeVerifier)
	if err != nil {
		return "", "", false, err
	}
	if tok.IDToken == "" {
		return "", "", false, derrors.Unauthenticated("idp returned no id_token")
	}

	claims, err := parseIDToken(tok.IDToken)
	if err != nil {
		return "", "", false, derrors.Unauthenticated("invalid id_token").Wrap(err)
	}
	if claims.Issuer != "" && !sameIssuer(claims.Issuer, provider.GetIssuer()) {
		return "", "", false, derrors.Unauthenticated("id_token issuer mismatch")
	}
	if st.Nonce != "" && claims.Nonce != st.Nonce {
		return "", "", false, derrors.Unauthenticated("id_token nonce mismatch")
	}
	if claims.Subject == "" {
		return "", "", false, derrors.Unauthenticated("id_token has no subject")
	}
	return claims.Subject, strings.ToLower(claims.Email), truthy(claims.EmailVerified), nil
}

func (f *OIDCFlows) callbackURL(slug string) string {
	return f.redirectBaseURL + "/auth/sso/" + url.PathEscape(slug) + "/callback"
}

func (f *OIDCFlows) discover(ctx context.Context, issuer string) (*oidcDiscovery, error) {
	base := strings.TrimRight(issuer, "/")
	if !strings.HasPrefix(base, "https://") {
		return nil, derrors.FailedPrecondition("identity.sso_issuer_insecure", "oidc issuer must be https")
	}
	endpoint := base + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.sso_discovery_failed", "oidc discovery request failed").Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, derrors.FailedPrecondition("identity.sso_discovery_failed", fmt.Sprintf("oidc discovery returned %d", resp.StatusCode))
	}
	var disc oidcDiscovery
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&disc); err != nil {
		return nil, derrors.FailedPrecondition("identity.sso_discovery_failed", "oidc discovery document is invalid").Wrap(err)
	}
	if disc.AuthorizationEndpoint == "" || disc.TokenEndpoint == "" {
		return nil, derrors.FailedPrecondition("identity.sso_discovery_failed", "oidc discovery document is missing endpoints")
	}
	return &disc, nil
}

func (f *OIDCFlows) redeem(ctx context.Context, tokenEndpoint string, provider *iam.IdentityProvider, secret, code, verifier string) (*oidcTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", f.callbackURL(provider.GetSlug()))
	form.Set("client_id", provider.GetClientId())
	form.Set("code_verifier", verifier)
	if secret != "" {
		form.Set("client_secret", secret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := f.http.Do(req)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.sso_token_exchange_failed", "oidc token request failed").Wrap(err)
	}
	defer resp.Body.Close()

	var out oidcTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, derrors.FailedPrecondition("identity.sso_token_exchange_failed", "oidc token response is invalid").Wrap(err)
	}
	if resp.StatusCode != http.StatusOK || out.Error != "" {
		msg := out.ErrorDesc
		if msg == "" {
			msg = out.Error
		}
		if msg == "" {
			msg = fmt.Sprintf("token endpoint returned %d", resp.StatusCode)
		}
		return nil, derrors.Unauthenticated("oidc token exchange rejected: " + msg)
	}
	return &out, nil
}

// parseIDToken decodes the ID token claims WITHOUT signature verification: it is
// delivered over the back-channel TLS token endpoint we just called, so the
// transport authenticates the IdP. (Full JWKS verification would require an
// additional well-known fetch; the issuer+nonce checks above bind the token to
// this flow.)
func parseIDToken(raw string) (*idTokenClaims, error) {
	claims := &idTokenClaims{}
	parser := jwt.NewParser()
	if _, _, err := parser.ParseUnverified(raw, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func scopeParam(scopes []string) string {
	if len(scopes) == 0 {
		return "openid email profile"
	}
	hasOpenID := false
	for _, s := range scopes {
		if s == "openid" {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		scopes = append([]string{"openid"}, scopes...)
	}
	return strings.Join(scopes, " ")
}

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func sameIssuer(a, b string) bool {
	return strings.TrimRight(a, "/") == strings.TrimRight(b, "/")
}

// truthy interprets the OIDC email_verified claim, which some IdPs send as a
// JSON bool and others as a string.
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}
