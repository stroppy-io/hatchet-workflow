package identity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Browser-auth handshake for the embedded IDE (SP-C): the SPA already holds
// a Bearer access token; a browser navigation to /ide/<scope>/... cannot
// carry a custom header, so it cannot present that token directly. The
// SPA instead calls an authenticated endpoint with its Bearer token to mint
// a ticket, then navigates with the ticket as a query parameter; the
// gateway exchanges the ticket for an httpOnly, scope-bound session cookie
// and redirects to the clean URL. See internal/ide/ticket.go for the
// gateway-side consumer of these two token shapes.
//
// Two distinct JWT "purpose" claims are minted from the SAME signing
// secret, deliberately never interchangeable:
//   - ide_ticket: ~30s TTL, single-use (tracked server-side by jti, see
//     used below), exchanged exactly once for a session cookie. It is the
//     ONLY thing that ever appears in a URL/query string/browser history.
//   - ide_session: longer TTL, reusable until expiry, never leaves an
//     httpOnly cookie. It is deliberately NOT the same token as the SPA's
//     long-lived access JWT (see accessJWTClaims) — it carries no
//     platform-admin bit, no permission grants, nothing beyond
//     (accountID, one scope key). A leaked ide_session cookie authorizes
//     opening exactly one already-authorized catalog-entry repo's IDE and
//     nothing else on the platform (not the connect API, not any other
//     scope) — the blast radius of a stolen cookie is bounded to what the
//     cookie's own Path scoping already limits it to, not the account's
//     full privilege set the access JWT would carry.
const (
	ideTicketPurpose  = "ide_ticket"
	ideSessionPurpose = "ide_session"

	// IdeTicketTTL is the ticket's lifetime — long enough for the browser to
	// receive the mint response and issue the navigation, short enough that
	// a ticket sitting in a proxy access log/browser history is worthless
	// within well under a minute.
	IdeTicketTTL = 30 * time.Second
	// IdeSessionTTL is the session cookie's lifetime. It is intentionally
	// shorter than the refresh-token lifetime but long enough to cover a
	// realistic editing session without forcing a fresh "Open in IDE" click
	// (which is the only way to renew it — there is no session refresh
	// endpoint, matching the ticket's single-purpose, capability-token
	// design).
	IdeSessionTTL = 12 * time.Hour
)

// ideTokenClaims is the shared claim shape for both the ticket and the
// session cookie token — only Purpose and Scope differ per use, everything
// else (subject, expiry, jti) reuses jwt.RegisteredClaims exactly like
// accessJWTClaims/refresh claims elsewhere in this file.
type ideTokenClaims struct {
	Purpose string `json:"purpose"`
	Scope   string `json:"scope"`
	jwt.RegisteredClaims
}

// IdeTicketService mints and verifies the ide_ticket / ide_session token
// pair described above. It reuses JWTTokenService's signing secret/issuer
// (Config.SigningSecret) — no separate key is introduced.
type IdeTicketService struct {
	secret []byte
	issuer string

	mu   sync.Mutex
	used map[string]time.Time // ticket jti -> expiry, pruned lazily on Consume
}

// NewIdeTicketService builds the service. Like NewJWTTokenService, it
// refuses an empty signing secret rather than silently minting
// unverifiable tokens.
func NewIdeTicketService(cfg Config) (*IdeTicketService, error) {
	if cfg.SigningSecret == "" {
		return nil, errors.New("identity: ide ticket signing secret is not configured")
	}
	return &IdeTicketService{
		secret: []byte(cfg.SigningSecret),
		issuer: cfg.issuer(),
		used:   make(map[string]time.Time),
	}, nil
}

// MintTicket issues a fresh single-use ticket bound to (accountID, scope).
// Callers MUST have already authorized accountID against scope (see
// internal/ide.TicketIssuer, which runs the full Authorizer.CanAuthor check
// using the caller's Bearer token before ever calling this) — MintTicket
// itself performs no authorization, only signs the claim.
func (s *IdeTicketService) MintTicket(accountID, scope string) (string, error) {
	now := time.Now()
	claims := ideTokenClaims{
		Purpose: ideTicketPurpose,
		Scope:   scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   accountID,
			Issuer:    s.issuer,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(IdeTicketTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// ConsumeTicket validates ticket (signature, issuer, expiry, purpose) and
// enforces single-use: a jti seen once is rejected on every subsequent
// call, including a second call with the SAME ticket string (replay). It
// returns the (accountID, scope) the ticket was minted for so the caller
// can mint a session bound to the identical pair.
func (s *IdeTicketService) ConsumeTicket(_ context.Context, ticket string) (accountID, scope string, err error) {
	claims, err := s.parse(ticket, ideTicketPurpose)
	if err != nil {
		return "", "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for jti, exp := range s.used {
		if exp.Before(now) {
			delete(s.used, jti)
		}
	}
	if _, seen := s.used[claims.ID]; seen {
		return "", "", errors.New("identity: ide ticket already used")
	}
	s.used[claims.ID] = claims.ExpiresAt.Time
	return claims.Subject, claims.Scope, nil
}

// MintSession issues the longer-lived, reusable-until-expiry session token
// the gateway sets as the httpOnly IDE cookie. Callers MUST only call this
// as the direct result of a successful ConsumeTicket for the SAME
// (accountID, scope) pair — see internal/ide.TicketExchanger.
func (s *IdeTicketService) MintSession(accountID, scope string) (string, error) {
	now := time.Now()
	claims := ideTokenClaims{
		Purpose: ideSessionPurpose,
		Scope:   scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   accountID,
			Issuer:    s.issuer,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(IdeSessionTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// VerifySession validates a session cookie token (signature, issuer,
// expiry, purpose). Unlike ConsumeTicket it is NOT single-use — the same
// cookie authorizes every request for the scope it names until it expires.
func (s *IdeTicketService) VerifySession(_ context.Context, token string) (accountID, scope string, err error) {
	claims, err := s.parse(token, ideSessionPurpose)
	if err != nil {
		return "", "", err
	}
	return claims.Subject, claims.Scope, nil
}

func (s *IdeTicketService) parse(token, wantPurpose string) (*ideTokenClaims, error) {
	claims := &ideTokenClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, s.keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid {
		return nil, errors.New("identity: invalid ide token")
	}
	if claims.Purpose != wantPurpose {
		// A ticket presented where a session is expected (or vice versa) is
		// rejected outright — the two token shapes are never interchangeable
		// even though they share a signing key.
		return nil, fmt.Errorf("identity: ide token has purpose %q, want %q", claims.Purpose, wantPurpose)
	}
	if claims.Subject == "" || claims.Scope == "" || claims.ID == "" {
		return nil, errors.New("identity: ide token missing subject/scope/id")
	}
	if claims.ExpiresAt == nil {
		return nil, errors.New("identity: ide token missing expiry")
	}
	return claims, nil
}

func (s *IdeTicketService) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
	}
	return s.secret, nil
}
