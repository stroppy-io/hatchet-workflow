package webhookworker

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// hmacSign returns hex(HMAC-SHA256(secret, body)).
func hmacSign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
