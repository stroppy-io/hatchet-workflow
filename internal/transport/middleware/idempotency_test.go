package middleware_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

const testProc = "/cloud.v1.testing.TestRunService/CreateTestRun"

func newIdempotencyMW(t *testing.T) (connect.UnaryFunc, *int) {
	t.Helper()
	vk, err := valkey.NewInMemory()
	require.NoError(t, err)
	registry := map[string]bool{testProc: true}
	cfg := middleware.IdempotencyConfig{Enabled: true, TTL: 30 * time.Second}
	interceptor := middleware.Idempotency(vk, registry, cfg)
	calls := 0
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		return connect.NewResponse(&struct{}{}), nil
	})
	return handler, &calls
}

func newReq(key, tenant string) connect.AnyRequest {
	r := connect.NewRequest(&struct{}{})
	if key != "" {
		r.Header().Set("X-Idempotency-Key", key)
	}
	// Procedure is read from Spec(); for AnyRequest in tests we must set it via
	// wrapping. Connect's NewRequest doesn't expose Spec, so we use a custom one.
	return &reqWithProc{AnyRequest: r, proc: testProc, tenant: tenant}
}

// reqWithProc overrides Spec() so the interceptor sees the procedure name.
type reqWithProc struct {
	connect.AnyRequest
	proc   string
	tenant string
}

func (r *reqWithProc) Spec() connect.Spec {
	return connect.Spec{Procedure: r.proc, StreamType: connect.StreamTypeUnary}
}

func ctxWithTenant(tid string) context.Context {
	return middleware.WithTenantID(context.Background(), tid)
}

// First-call passes through, second-call with same key returns
// AlreadyExists (the placeholder replay code in idempotency.go).
func TestIdempotency_DuplicateReplay(t *testing.T) {
	handler, calls := newIdempotencyMW(t)
	ctx := ctxWithTenant("tenant-1")

	_, err := handler(ctx, newReq("key-1", "tenant-1"))
	require.NoError(t, err)
	require.Equal(t, 1, *calls)

	_, err = handler(ctx, newReq("key-1", "tenant-1"))
	require.Error(t, err, "second call with same key must error (cached replay)")
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeAlreadyExists, ce.Code(), "must be AlreadyExists (cached) not Aborted (in-flight)")
	require.Equal(t, 1, *calls, "handler must NOT be invoked twice for the same key")
}

// Different keys → both calls run through.
func TestIdempotency_DifferentKeys(t *testing.T) {
	handler, calls := newIdempotencyMW(t)
	ctx := ctxWithTenant("tenant-1")

	_, err := handler(ctx, newReq("key-A", "tenant-1"))
	require.NoError(t, err)
	_, err = handler(ctx, newReq("key-B", "tenant-1"))
	require.NoError(t, err)
	require.Equal(t, 2, *calls)
}

// Same key, different tenants → keyspace is per-tenant; both pass through.
func TestIdempotency_TenantKeyspace(t *testing.T) {
	handler, calls := newIdempotencyMW(t)

	_, err := handler(ctxWithTenant("ta"), newReq("shared-key", "ta"))
	require.NoError(t, err)
	_, err = handler(ctxWithTenant("tb"), newReq("shared-key", "tb"))
	require.NoError(t, err)
	require.Equal(t, 2, *calls, "tenant-A and tenant-B with same key must both reach handler")
}

// Missing key header → middleware is a passthrough.
func TestIdempotency_NoKeyHeaderPassthrough(t *testing.T) {
	handler, calls := newIdempotencyMW(t)
	ctx := ctxWithTenant("tenant-1")

	_, err := handler(ctx, newReq("", "tenant-1"))
	require.NoError(t, err)
	_, err = handler(ctx, newReq("", "tenant-1"))
	require.NoError(t, err)
	require.Equal(t, 2, *calls)
}

// Procedure not in registry → passthrough.
func TestIdempotency_ProcedureNotRegistered(t *testing.T) {
	vk, err := valkey.NewInMemory()
	require.NoError(t, err)
	interceptor := middleware.Idempotency(vk, map[string]bool{ /* empty */ }, middleware.IdempotencyConfig{Enabled: true, TTL: time.Minute})
	calls := 0
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		return connect.NewResponse(&struct{}{}), nil
	})
	ctx := ctxWithTenant("t")
	_, err = handler(ctx, newReq("key", "t"))
	require.NoError(t, err)
	_, err = handler(ctx, newReq("key", "t"))
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

// Disabled config → passthrough.
func TestIdempotency_Disabled(t *testing.T) {
	vk, err := valkey.NewInMemory()
	require.NoError(t, err)
	registry := map[string]bool{testProc: true}
	interceptor := middleware.Idempotency(vk, registry, middleware.IdempotencyConfig{Enabled: false, TTL: time.Minute})
	calls := 0
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		return connect.NewResponse(&struct{}{}), nil
	})
	ctx := ctxWithTenant("t")
	_, err = handler(ctx, newReq("key", "t"))
	require.NoError(t, err)
	_, err = handler(ctx, newReq("key", "t"))
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

// Verify cached error responses are NOT re-cached for transient codes (the
// middleware deletes the lock so a retry can run). Use Unavailable.
func TestIdempotency_TransientErrorEvicted(t *testing.T) {
	vk, err := valkey.NewInMemory()
	require.NoError(t, err)
	registry := map[string]bool{testProc: true}
	cfg := middleware.IdempotencyConfig{Enabled: true, TTL: 30 * time.Second}
	interceptor := middleware.Idempotency(vk, registry, cfg)
	calls := 0
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		if calls == 1 {
			return nil, connect.NewError(connect.CodeUnavailable, errors.New("backend down"))
		}
		return connect.NewResponse(&struct{}{}), nil
	})

	ctx := ctxWithTenant("t")
	_, err = handler(ctx, newReq("retry-key", "t"))
	require.Error(t, err)
	// Same key + transient previous failure must allow retry to run.
	_, err = handler(ctx, newReq("retry-key", "t"))
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
