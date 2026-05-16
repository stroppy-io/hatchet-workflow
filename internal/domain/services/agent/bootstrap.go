package agent

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const bootstrapTTL = 10 * time.Minute

// BootstrapClaims holds the data encoded in a bootstrap token.
type BootstrapClaims struct {
	TenantID  string `json:"tenant_id"`
	DagRunID  string `json:"dag_run_id"`
	MachineID string `json:"machine_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// BootstrapTokenStore issues and verifies short-lived HS256 JWTs used as
// one-shot bootstrap tokens for agent registration.
type BootstrapTokenStore struct {
	secret []byte
}

// NewBootstrapTokenStore constructs a store using the given HMAC secret.
func NewBootstrapTokenStore(secret []byte) *BootstrapTokenStore {
	return &BootstrapTokenStore{secret: secret}
}

// Issue signs a new HS256 JWT with bootstrapTTL expiry.
func (s *BootstrapTokenStore) Issue(tenantID, dagRunID, machineID, role string) (string, error) {
	now := time.Now()
	claims := BootstrapClaims{
		TenantID:  tenantID,
		DagRunID:  dagRunID,
		MachineID: machineID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(bootstrapTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

// Verify parses and validates the token. Returns error on expiry or bad signature.
func (s *BootstrapTokenStore) Verify(token string) (*BootstrapClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &BootstrapClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("bootstrap: unexpected sign method %v", t.Method)
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*BootstrapClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("bootstrap: invalid token claims")
	}
	return claims, nil
}
