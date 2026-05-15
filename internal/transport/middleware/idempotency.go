package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
)

// IdempotencyConfig knobs.
type IdempotencyConfig struct {
	Enabled bool
	TTL     time.Duration
}

// cachedResp is what we store.
type cachedResp struct {
	Code    int32
	Message string
	Body    []byte
}

const (
	headerKey = "X-Idempotency-Key"
	prefix    = "idemp:"
)

func Idempotency(store *valkey.Client, registry map[string]bool, cfg IdempotencyConfig) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !cfg.Enabled || !registry[req.Spec().Procedure] {
				return next(ctx, req)
			}
			key := req.Header().Get(headerKey)
			if key == "" {
				return next(ctx, req)
			}
			tenantID := TenantFromCtx(ctx)
			if tenantID == "" {
				return next(ctx, req)
			}
			storeKey := prefix + tenantID + ":" + req.Spec().Procedure + ":" + key

			acquired, err := store.SetNX(ctx, storeKey, "pending", cfg.TTL)
			if err != nil || !acquired {
				// Look up existing state
				raw, err := store.Get(ctx, storeKey)
				if err == nil && raw != "" && raw != "pending" {
					var c cachedResp
					if err := json.Unmarshal([]byte(raw), &c); err == nil {
						return replay(req, c)
					}
				}
				return nil, connect.NewError(connect.CodeAborted, errors.New("duplicate request in flight"))
			}

			resp, herr := next(ctx, req)
			cache(ctx, store, storeKey, cfg.TTL, resp, herr)
			return resp, herr
		}
	}
}

func cache(ctx context.Context, store *valkey.Client, key string, ttl time.Duration, resp connect.AnyResponse, herr error) {
	c := cachedResp{}
	if herr != nil {
		var ce *connect.Error
		if errors.As(herr, &ce) {
			c.Code = int32(ce.Code())
			c.Message = ce.Message()
		}
		// Don't cache transient codes
		switch connect.Code(c.Code) {
		case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeResourceExhausted:
			_ = store.Del(ctx, key)
			return
		}
	}
	if resp != nil {
		if m, ok := resp.Any().(proto.Message); ok {
			body, _ := proto.Marshal(m)
			c.Body = body
		}
	}
	data, _ := json.Marshal(c)
	_ = store.Set(ctx, key, string(data), ttl)
}

func replay(_ connect.AnyRequest, c cachedResp) (connect.AnyResponse, error) {
	if c.Code != 0 {
		return nil, connect.NewError(connect.Code(c.Code), errors.New(c.Message))
	}
	// Returning a raw bytes envelope is non-trivial without the response type.
	// For Phase 02 we accept that cached responses cannot replay the body —
	// idempotent ops re-run on cache miss; the value of the cache is the
	// duplicate-in-flight protection. Refine post Phase 02 when the response
	// type can be resolved from the procedure registry.
	return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("idempotent replay placeholder"))
}
