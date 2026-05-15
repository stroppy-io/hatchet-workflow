package connect

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam/iamconnect"
)

// Deps aggregates Connect handler dependencies for mounting.
type Deps struct {
	IAMHandler *IAMHandler

	Interceptors connect.Option
}

// Mount returns an http.Handler aggregating all Connect handlers.
func Mount(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Auth service
	authPath, authHandler := iamconnect.NewAuthServiceHandler(d.IAMHandler, d.Interceptors)
	mux.Handle(authPath, authHandler)

	// User service
	userPath, userHandler := iamconnect.NewUserServiceHandler(d.IAMHandler, d.Interceptors)
	mux.Handle(userPath, userHandler)

	// Tenant service
	tenantPath, tenantHandler := iamconnect.NewTenantServiceHandler(d.IAMHandler, d.Interceptors)
	mux.Handle(tenantPath, tenantHandler)

	return mux
}

// AuthBypass returns the set of fully-qualified procedure paths that skip
// JWT enforcement.
func AuthBypass() map[string]bool {
	return map[string]bool{
		iamconnect.AuthServiceLoginProcedure:         true,
		iamconnect.AuthServiceRefreshTokensProcedure: true,
		iamconnect.AuthServiceLogoutProcedure:        true,
	}
}

// TenantBypass returns procedures that skip tenant resolution.
func TenantBypass() map[string]bool {
	out := AuthBypass()
	out[iamconnect.UserServiceCreateUserProcedure] = true
	out[iamconnect.UserServiceMeProcedure] = true
	out[iamconnect.TenantServiceCreateTenantProcedure] = true
	out[iamconnect.TenantServiceListMyTenantsProcedure] = true
	return out
}

// WithMiddleware wraps interceptors as a HandlerOption.
func WithMiddleware(i ...connect.Interceptor) connect.HandlerOption {
	return connect.WithInterceptors(i...)
}
