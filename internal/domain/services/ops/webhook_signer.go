package ops

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// hmacSign returns hex(HMAC-SHA256(secret, body)). Used by TestWebhook and by
// the webhookworker delivery loop; both packages should consume this helper so
// a webhook receiver verifies test deliveries with the same scheme as
// production deliveries.
func hmacSign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
