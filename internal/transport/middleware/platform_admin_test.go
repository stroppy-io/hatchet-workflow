package middleware_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestRequirePlatformAdmin_Allows(t *testing.T) {
	interceptor := middleware.RequirePlatformAdmin()
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return &connect.Response[struct{}]{}, nil
	})

	ctx := middleware.WithPlatformRole(context.Background(), iampb.PlatformRole_PLATFORM_ROLE_ADMIN.String())
	req := connect.NewRequest(&struct{}{})
	_, err := handler(ctx, req)
	require.NoError(t, err)
}

func TestRequirePlatformAdmin_Denies_NonAdmin(t *testing.T) {
	interceptor := middleware.RequirePlatformAdmin()
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return &connect.Response[struct{}]{}, nil
	})

	ctx := middleware.WithPlatformRole(context.Background(), "PLATFORM_ROLE_NONE")
	req := connect.NewRequest(&struct{}{})
	_, err := handler(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestRequirePlatformAdmin_Denies_NoRole(t *testing.T) {
	interceptor := middleware.RequirePlatformAdmin()
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return &connect.Response[struct{}]{}, nil
	})

	ctx := context.Background() // no role set
	req := connect.NewRequest(&struct{}{})
	_, err := handler(ctx, req)
	require.Error(t, err)
}
