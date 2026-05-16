package iam

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type accessClaims struct {
	UserID       string `json:"sub"`
	JTI          string `json:"jti"`
	PlatformRole string `json:"platform_role,omitempty"`
	jwt.RegisteredClaims
}

func (s *Service) signAccessToken(userID, jti, platformRole string) (string, error) {
	now := time.Now()
	claims := accessClaims{
		UserID:       userID,
		JTI:          jti,
		PlatformRole: platformRole,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.jwtSecret)
}

// VerifyAccessToken validates a JWT and returns (userID, jti, platformRole, error).
func (s *Service) VerifyAccessToken(token string) (string, string, string, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected sign method: %v", t.Method)
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", "", "", err
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid {
		return "", "", "", fmt.Errorf("invalid token")
	}
	return claims.UserID, claims.JTI, claims.PlatformRole, nil
}
