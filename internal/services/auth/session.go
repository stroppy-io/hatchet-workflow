package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

const refreshSessionKeyPrefix = "auth:refresh:"

// refreshSession is the Valkey value backing a refresh token. The plaintext is
// never stored — the key is sha256(token), so a stolen DB/cache cannot mint tokens.
type refreshSession struct {
	AccountID string `json:"aid"`
	IssuedAt  int64  `json:"iat"`
}

func refreshSessionKey(tokenHash string) string {
	return refreshSessionKeyPrefix + tokenHash
}

func (s *AuthService) saveRefreshSession(ctx context.Context, tokenHash string, sess refreshSession, ttl time.Duration) error {
	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.valkey.Do(ctx,
		s.valkey.B().Set().Key(refreshSessionKey(tokenHash)).Value(string(payload)).Ex(ttl).Build(),
	).Error()
}

func (s *AuthService) getRefreshSession(ctx context.Context, tokenHash string) (*refreshSession, error) {
	raw, err := s.valkey.Do(ctx,
		s.valkey.B().Get().Key(refreshSessionKey(tokenHash)).Build(),
	).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil // unknown / expired / rotated
		}
		return nil, err
	}
	var sess refreshSession
	if err := json.Unmarshal([]byte(raw), &sess); err != nil {
		return nil, fmt.Errorf("decode refresh session: %w", err)
	}
	return &sess, nil
}

func (s *AuthService) deleteRefreshSession(ctx context.Context, tokenHash string) error {
	return s.valkey.Do(ctx,
		s.valkey.B().Del().Key(refreshSessionKey(tokenHash)).Build(),
	).Error()
}
