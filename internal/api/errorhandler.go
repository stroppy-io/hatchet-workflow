package api

import (
	"context"
	"net/http"

	"github.com/go-faster/jx"

	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ErrorHandler answers ogen's own failures (bad path, undecodable body,
// missing bearer) with the same Problem shape handlers produce.
func ErrorHandler(h *Handler) oas.ErrorHandler {
	return func(ctx context.Context, w http.ResponseWriter, _ *http.Request, err error) {
		p := h.NewError(ctx, err)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(p.StatusCode)
		e := jx.GetEncoder()
		defer jx.PutEncoder(e)
		p.Response.Encode(e)
		_, _ = w.Write(e.Bytes()) //nolint:errcheck // client went away
	}
}
