package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token kinds carried in the "knd" claim. The access token is a short-lived,
// stateless JWT; the principal kind selects which extra claims are meaningful.
const (
	KindAccount = "account"
	KindAgent   = "agent"
)

// ErrInvalidToken is returned for any malformed, forged, or expired token.
var ErrInvalidToken = errors.New("invalid token")

// Claims is the access-token payload. Account tokens set Subject(=account id)+
// IsAdmin; agent machine tokens set AgentID+TenantID+MachineID.
type Claims struct {
	jwt.RegisteredClaims
	Kind      string `json:"knd"`
	IsAdmin   bool   `json:"adm,omitempty"`
	AgentID   string `json:"agt,omitempty"`
	TenantID  string `json:"tnt,omitempty"`
	MachineID string `json:"mch,omitempty"`
}

// Signer signs and parses HS256 access tokens with a shared secret.
type Signer struct {
	secret []byte
}

// NewSigner builds a Signer over the given HMAC secret.
func NewSigner(secret []byte) *Signer {
	return &Signer{secret: secret}
}

func (s *Signer) sign(c *Claims, ttl time.Duration) (string, error) {
	now := time.Now()
	c.IssuedAt = jwt.NewNumericDate(now)
	// ttl <= 0 means a non-expiring token: omit the exp claim entirely so the
	// validator never rejects it (jwt/v5 treats a missing exp as "no expiry").
	// Agent machine tokens use this so a long benchmark never loses auth mid-run.
	if ttl > 0 {
		c.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
}

// SignAccount issues an access token for a human account.
func (s *Signer) SignAccount(accountID string, isAdmin bool, ttl time.Duration) (string, error) {
	return s.sign(&Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: accountID},
		Kind:             KindAccount,
		IsAdmin:          isAdmin,
	}, ttl)
}

// SignAgent issues a machine token for an agent (cloud-init bootstrap, D18).
func (s *Signer) SignAgent(agentID, tenantID, machineID string, ttl time.Duration) (string, error) {
	return s.sign(&Claims{
		Kind:      KindAgent,
		AgentID:   agentID,
		TenantID:  tenantID,
		MachineID: machineID,
	}, ttl)
}

// Parse validates the token signature and expiry and returns its claims.
func (s *Signer) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return s.secret, nil
	}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return claims, nil
}
