package middleware

import (
	"context"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
)

const HeaderRequestID = "X-Request-Id"

func RequestID() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			id := req.Header().Get(HeaderRequestID)
			if id == "" {
				id = ids.New()
			}
			ctx = WithRequestID(ctx, id)
			resp, err := next(ctx, req)
			if resp != nil {
				resp.Header().Set(HeaderRequestID, id)
			}
			return resp, err
		}
	}
}
