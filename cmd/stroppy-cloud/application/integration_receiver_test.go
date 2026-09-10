//go:build integration

package application

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// received is one signed delivery as the consumer saw it.
type received struct {
	id, ts, sig string
	body        []byte
}

// hookReceiver is the webhook consumer under test: records the signed
// request and answers 200, or 500 while fail is set.
type hookReceiver struct {
	*httptest.Server
	fail bool
}

func newReceiver(t *testing.T, out chan<- received) *hookReceiver {
	t.Helper()
	r := &hookReceiver{}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		out <- received{req.Header.Get("Webhook-Id"), req.Header.Get("Webhook-Timestamp"), req.Header.Get("Webhook-Signature"), body}
		if r.fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.Close)
	return r
}
