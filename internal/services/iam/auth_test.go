package iam

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestAuthorizeLetsGetTenantReachHandlerForSlugResolution(t *testing.T) {
	auth := NewAuthInterceptor(nil, fakePermissionResolver{})
	claims := &iampb.AccessClaims{AccountId: "account-1"}
	methodAuth := &iampb.MethodAuth{
		AllOf: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TENANT, Action: iampb.Action_ACTION_READ},
		},
		TenantField: "id",
	}

	err := auth.authorize(context.Background(), verifiedCaller{claims: claims}, &api.GetTenantRequest{
		Ref: &api.GetTenantRequest_Slug{Slug: "acme"},
	}, methodAuth)
	if err != nil {
		t.Fatalf("authorize get tenant by slug: %v", err)
	}
}

func TestAuthorizeLetsAccountRequestsReachHandlerPolicy(t *testing.T) {
	auth := NewAuthInterceptor(nil, fakePermissionResolver{})
	claims := &iampb.AccessClaims{AccountId: "account-1"}
	methodAuth := &iampb.MethodAuth{
		AllOf: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_ACCOUNT, Action: iampb.Action_ACTION_UPDATE},
		},
	}

	if err := auth.authorize(context.Background(), verifiedCaller{claims: claims}, &api.GetAccountRequest{Id: "account-2"}, methodAuth); err != nil {
		t.Fatalf("authorize get account: %v", err)
	}
	if err := auth.authorize(context.Background(), verifiedCaller{claims: claims}, &api.UpdateAccountRequest{Id: "account-1"}, methodAuth); err != nil {
		t.Fatalf("authorize update account: %v", err)
	}
}

func TestAuthorizeLetsCreateTenantReachHandlerPolicy(t *testing.T) {
	auth := NewAuthInterceptor(nil, fakePermissionResolver{})
	claims := &iampb.AccessClaims{AccountId: "account-1"}
	methodAuth := &iampb.MethodAuth{
		AllOf: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TENANT, Action: iampb.Action_ACTION_CREATE},
		},
	}

	if err := auth.authorize(context.Background(), verifiedCaller{claims: claims}, &api.CreateTenantRequest{Name: "acme", Slug: "acme"}, methodAuth); err != nil {
		t.Fatalf("authorize create tenant: %v", err)
	}
}

func TestMethodAuthGeneratedDescriptors(t *testing.T) {
	publicMethods := []string{
		api.IamService_Login_FullMethodName,
		api.IamService_Refresh_FullMethodName,
		api.PublicShareService_GetSharedRun_FullMethodName,
		api.PublicRatingService_GetPublicRating_FullMethodName,
	}
	for _, method := range publicMethods {
		auth := methodAuth(method)
		if auth == nil || !auth.GetPublic() {
			t.Fatalf("%s auth = %v, want public", method, auth)
		}
	}

	auth := methodAuth(api.RecipeService_CreateRecipe_FullMethodName)
	if auth == nil || !hasAll(auth.GetAllOf(), []*iampb.Permission{
		{Resource: iampb.Resource_RESOURCE_RECIPE, Action: iampb.Action_ACTION_CREATE},
	}) {
		t.Fatalf("CreateRecipe auth = %v, want recipe create", auth)
	}
}

func TestAllCloudAPIMethodsDeclareAuth(t *testing.T) {
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "cloud.v1.api") {
			return true
		}
		for i := 0; i < fd.Services().Len(); i++ {
			svc := fd.Services().Get(i)
			for j := 0; j < svc.Methods().Len(); j++ {
				method := svc.Methods().Get(j)
				fullMethod := "/" + string(svc.FullName()) + "/" + string(method.Name())
				if methodAuth(fullMethod) == nil {
					t.Errorf("%s has no cloud.v1.iam.auth annotation", fullMethod)
				}
			}
		}
		return true
	})
}

func TestAuthorizeRestrictedTokenRequiresTokenPermission(t *testing.T) {
	auth := NewAuthInterceptor(nil, fakePermissionResolver{
		granted: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_MANAGE},
		},
	})
	methodAuth := &iampb.MethodAuth{
		AllOf: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_DELETE},
		},
		TenantField: "tenant_id",
	}

	err := auth.authorize(context.Background(), verifiedCaller{
		claims: &iampb.AccessClaims{AccountId: "account-1"},
		permissions: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_READ},
		},
		restricted: true,
	}, &api.DeleteRecipeRequest{TenantId: "tenant-1", Id: "run-1"}, methodAuth)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.PermissionDenied, err)
	}
}

func TestAuthorizeRestrictedTokenAllowsTokenPermission(t *testing.T) {
	auth := NewAuthInterceptor(nil, fakePermissionResolver{
		granted: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_MANAGE},
		},
	})
	methodAuth := &iampb.MethodAuth{
		AllOf: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_DELETE},
		},
		TenantField: "tenant_id",
	}

	err := auth.authorize(context.Background(), verifiedCaller{
		claims: &iampb.AccessClaims{AccountId: "account-1"},
		permissions: []*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_MANAGE},
		},
		restricted: true,
	}, &api.DeleteRecipeRequest{TenantId: "tenant-1", Id: "run-1"}, methodAuth)
	if err != nil {
		t.Fatalf("authorize restricted token: %v", err)
	}
}

type fakePermissionResolver struct {
	granted []*iampb.Permission
}

func (f fakePermissionResolver) EffectivePermissions(context.Context, string, string) ([]*iampb.Permission, error) {
	return f.granted, nil
}
