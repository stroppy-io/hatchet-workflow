package iam

// auth_test.go: tests for the auth interceptor and related helper functions in auth.go.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestContextClaims(t *testing.T) {
	claims := &iam.AccessClaims{AccountId: "user1"}
	ctx := ContextWithClaims(context.Background(), claims)
	got, ok := ClaimsFromContext(ctx)
	if !ok || got.AccountId != claims.AccountId {
		t.Errorf("claims not recovered correctly")
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		md      metadata.MD
		want    string
		wantErr codes.Code
	}{
		{
			name: "valid token",
			md:   metadata.Pairs("authorization", "Bearer valid-token"),
			want: "valid-token",
		},
		{
			name: "case insensitive bearer",
			md:   metadata.Pairs("authorization", "bearer valid-token"),
			want: "valid-token",
		},
		{
			name:    "missing md",
			md:      nil,
			wantErr: codes.Unauthenticated,
		},
		{
			name:    "missing header",
			md:      metadata.Pairs("other", "val"),
			wantErr: codes.Unauthenticated,
		},
		{
			name:    "not bearer",
			md:      metadata.Pairs("authorization", "Basic abc"),
			wantErr: codes.Unauthenticated,
		},
		{
			name:    "empty token",
			md:      metadata.Pairs("authorization", "Bearer "),
			wantErr: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}
			got, err := bearerToken(ctx)
			if tt.wantErr != 0 {
				if status.Code(err) != tt.wantErr {
					t.Errorf("got error code %v, want %v", status.Code(err), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got token %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitFullMethod(t *testing.T) {
	svc, meth, ok := splitFullMethod("/pkg.Svc/Method")
	if !ok || svc != "pkg.Svc" || meth != "Method" {
		t.Error("failed basic split")
	}
	_, _, ok = splitFullMethod("invalid")
	if ok {
		t.Error("should fail on invalid")
	}
	_, _, ok = splitFullMethod("/no-slash")
	if ok {
		t.Error("should fail when no second slash")
	}
}

func TestTenantIDFromRequest(t *testing.T) {
	req := &api.GetAccountRequest{Id: "acc1"}
	req2 := &api.CreateRoleRequest{TenantId: "ten1"}

	if got := tenantIDFromRequest(req2, "tenant_id"); got != "ten1" {
		t.Errorf("expected ten1, got %q", got)
	}
	if got := tenantIDFromRequest(req2, ""); got != "ten1" {
		t.Errorf("expected ten1 (default field), got %q", got)
	}
	if got := tenantIDFromRequest(req, "non_existent"); got != "" {
		t.Errorf("expected empty string for non-existent field")
	}
	// Non-proto message
	if got := tenantIDFromRequest("not-a-proto", "tenant_id"); got != "" {
		t.Errorf("expected empty string for non-proto type")
	}
}

func TestAuthInterceptor_Unary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokens := NewMockTokenVerifier(ctrl)
	mockPerms := NewMockPermissionResolver(ctrl)
	interceptor := NewAuthInterceptor(mockTokens, mockPerms).Unary()

	ctx := context.Background()
	req := &api.GetMyAccountRequest{}
	info := &grpc.UnaryServerInfo{FullMethod: "/cloud.v1.api.IamService/GetMyAccount"}

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	t.Run("Unauthenticated_NoToken", func(t *testing.T) {
		_, err := interceptor(ctx, req, info, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		ctxWithToken := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer invalid"))
		mockTokens.EXPECT().Verify(gomock.Any(), "invalid").Return(nil, errors.New("invalid"))
		_, err := interceptor(ctxWithToken, req, info, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("AuthenticatedSuccess", func(t *testing.T) {
		ctxWithToken := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer valid"))
		claims := &iam.AccessClaims{AccountId: "user1"}
		mockTokens.EXPECT().Verify(gomock.Any(), "valid").Return(claims, nil)
		resp, err := interceptor(ctxWithToken, req, info, handler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("expected ok, got %v", resp)
		}
	})

	t.Run("AdminOnly_Denied_NonAdmin", func(t *testing.T) {
		infoAdmin := &grpc.UnaryServerInfo{FullMethod: "/cloud.v1.api.IamService/NonExistent"}
		ctxWithToken := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer valid"))
		claims := &iam.AccessClaims{AccountId: "user1", IsAdmin: false}
		mockTokens.EXPECT().Verify(gomock.Any(), "valid").Return(claims, nil)
		_, err := interceptor(ctxWithToken, req, infoAdmin, handler)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("AdminOnly_Success_Admin", func(t *testing.T) {
		infoAdmin := &grpc.UnaryServerInfo{FullMethod: "/cloud.v1.api.IamService/NonExistent"}
		ctxWithToken := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer valid"))
		claims := &iam.AccessClaims{AccountId: "user1", IsAdmin: true}
		mockTokens.EXPECT().Verify(gomock.Any(), "valid").Return(claims, nil)
		resp, err := interceptor(ctxWithToken, req, infoAdmin, handler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("expected ok, got %v", resp)
		}
	})

	t.Run("AllOf_Success", func(t *testing.T) {
		infoList := &grpc.UnaryServerInfo{FullMethod: "/cloud.v1.api.IamService/ListAccounts"}
		ctxWithToken := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer valid"))
		claims := &iam.AccessClaims{AccountId: "user1"}
		mockTokens.EXPECT().Verify(gomock.Any(), "valid").Return(claims, nil)
		mockPerms.EXPECT().EffectivePermissions(gomock.Any(), "user1", "").Return([]*iam.Permission{
			{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_LIST},
		}, nil)
		resp, err := interceptor(ctxWithToken, req, infoList, handler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("expected ok, got %v", resp)
		}
	})

	t.Run("AllOf_Denied", func(t *testing.T) {
		infoList := &grpc.UnaryServerInfo{FullMethod: "/cloud.v1.api.IamService/ListAccounts"}
		ctxWithToken := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer valid"))
		claims := &iam.AccessClaims{AccountId: "user1"}
		mockTokens.EXPECT().Verify(gomock.Any(), "valid").Return(claims, nil)
		mockPerms.EXPECT().EffectivePermissions(gomock.Any(), "user1", "").Return([]*iam.Permission{
			{Resource: iam.Resource_RESOURCE_TENANT, Action: iam.Action_ACTION_READ},
		}, nil)
		_, err := interceptor(ctxWithToken, req, infoList, handler)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})
}

func TestAuthorize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := NewMockPermissionResolver(ctrl)
	interceptor := NewAuthInterceptor(nil, perms)
	ctx := context.Background()

	t.Run("PermissionsError", func(t *testing.T) {
		perms.EXPECT().EffectivePermissions(ctx, "a1", "").Return(nil, errors.New("db err"))
		err := interceptor.authorize(ctx, &iam.AccessClaims{AccountId: "a1"}, nil, &iam.MethodAuth{})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("MissingRequiredPermission", func(t *testing.T) {
		perms.EXPECT().EffectivePermissions(ctx, "a1", "").Return([]*iam.Permission{}, nil)
		err := interceptor.authorize(ctx, &iam.AccessClaims{AccountId: "a1"}, nil, &iam.MethodAuth{
			AllOf: []*iam.Permission{{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_READ}},
		})
		if err == nil {
			t.Error("expected perm denied error")
		}
	})

	t.Run("AdminShortCircuit", func(t *testing.T) {
		// Admin bypasses permission check entirely
		err := interceptor.authorize(ctx, &iam.AccessClaims{AccountId: "admin", IsAdmin: true}, nil, &iam.MethodAuth{
			AllOf: []*iam.Permission{{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_READ}},
		})
		if err != nil {
			t.Errorf("admin should always pass: %v", err)
		}
	})
}

func TestClaimsAuthn(t *testing.T) {
	authn := ClaimsAuthn{}
	ctx := context.Background()

	_, err := authn.Caller(ctx)
	if !errors.Is(err, errNoClaims) {
		t.Errorf("expected errNoClaims, got %v", err)
	}

	claims := &iam.AccessClaims{AccountId: "user1"}
	ctx = ContextWithClaims(ctx, claims)
	got, err := authn.Caller(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got != claims {
		t.Errorf("got different claims")
	}
}
