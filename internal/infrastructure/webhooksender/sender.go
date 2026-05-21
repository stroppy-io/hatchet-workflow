// Package webhooksender delivers webhook payloads over HTTP with an HMAC-SHA256
// signature header and bounded retry/backoff.
package webhooksender

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

// SignatureHeader carries the hex HMAC-SHA256 of the body, keyed by the webhook secret.
const SignatureHeader = "X-Stroppy-Signature"

// Sender posts signed webhook payloads.
type Sender struct {
	http     *http.Client
	attempts int
	backoff  time.Duration
}

// New builds a Sender.
func New() *Sender {
	return &Sender{
		http:     &http.Client{Timeout: 10 * time.Second},
		attempts: 3,
		backoff:  500 * time.Millisecond,
	}
}

// Send POSTs payload to url, signing it with secret (when set). It retries on
// transport errors and >=500 responses; a >=400 client response is final.
func (s *Sender) Send(ctx context.Context, url, secret string, payload []byte) error {
	var lastErr error
	for attempt := 0; attempt < s.attempts; attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, s.backoff*time.Duration(attempt)); err != nil {
				return err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("webhook: build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if secret != "" {
			req.Header.Set(SignatureHeader, sign(secret, payload))
		}
		resp, err := s.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()
		switch {
		case resp.StatusCode < 300:
			return nil
		case resp.StatusCode < 500:
			return fmt.Errorf("webhook: rejected with status %d", resp.StatusCode)
		default:
			lastErr = fmt.Errorf("webhook: status %d", resp.StatusCode)
		}
	}
	return lastErr
}

func sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
