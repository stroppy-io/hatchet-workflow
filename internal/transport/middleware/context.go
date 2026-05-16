package middleware

import "context"

type ctxKey int

const (
	keyUserID ctxKey = iota
	keyTenantID
	keyRequestID
	keyPlatformRole
)

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

func UserFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}

func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}

func TenantFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(keyTenantID).(string)
	return v
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

func RequestIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

func WithPlatformRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, keyPlatformRole, role)
}

func PlatformRoleFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(keyPlatformRole).(string)
	return v
}
