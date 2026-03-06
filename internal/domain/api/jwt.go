package api

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

type JWTManager struct {
	secret []byte
}

type TokenClaims struct {
	Username  string `json:"username"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{secret: []byte(secret)}
}

func (m *JWTManager) GenerateTokens(username string) (accessToken, refreshToken string, expiresAt time.Time, err error) {
	expiresAt = time.Now().Add(accessTokenTTL)
	accessToken, err = m.generateToken(username, "access", expiresAt)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate access token: %w", err)
	}

	refreshExpiresAt := time.Now().Add(refreshTokenTTL)
	refreshToken, err = m.generateToken(username, "refresh", refreshExpiresAt)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}

	return accessToken, refreshToken, expiresAt, nil
}

func (m *JWTManager) ValidateAccessToken(tokenStr string) (*TokenClaims, error) {
	claims, err := m.parseToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "access" {
		return nil, fmt.Errorf("expected access token, got %s", claims.TokenType)
	}
	return claims, nil
}

func (m *JWTManager) ValidateRefreshToken(tokenStr string) (*TokenClaims, error) {
	claims, err := m.parseToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("expected refresh token, got %s", claims.TokenType)
	}
	return claims, nil
}

func (m *JWTManager) generateToken(username, tokenType string, expiresAt time.Time) (string, error) {
	claims := TokenClaims{
		Username:  username,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) parseToken(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
