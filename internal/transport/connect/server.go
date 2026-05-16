package connect

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog/catalogconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam/iamconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy/stroppyconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system/systemconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing/testingconnect"
)

// Deps aggregates Connect handler dependencies for mounting.
type Deps struct {
	IAMHandler         *IAMHandler
	CatalogHandler     *CatalogHandler
	StroppyHandler     *StroppyHandler
	ScheduleHandler    *ScheduleHandler
	TestingHandler     *TestingHandler
	TestSuiteRunHandler *TestSuiteRunHandler
	SharedTestRunHandler *SharedTestRunHandler
	SharedSuiteRunHandler *SharedSuiteRunHandler
	ComparisonHandler  *ComparisonHandler

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

	// DatabasePreset service
	dbPresetPath, dbPresetHandler := catalogconnect.NewDatabasePresetServiceHandler(d.CatalogHandler, d.Interceptors)
	mux.Handle(dbPresetPath, dbPresetHandler)

	// WorkloadPreset service
	wlPresetPath, wlPresetHandler := catalogconnect.NewWorkloadPresetServiceHandler(d.CatalogHandler, d.Interceptors)
	mux.Handle(wlPresetPath, wlPresetHandler)

	// Package service
	pkgPath, pkgHandler := catalogconnect.NewPackageServiceHandler(d.CatalogHandler, d.Interceptors)
	mux.Handle(pkgPath, pkgHandler)

	// Settings service
	settingsPath, settingsHandler := catalogconnect.NewSettingsServiceHandler(d.CatalogHandler, d.Interceptors)
	mux.Handle(settingsPath, settingsHandler)

	// Stroppy service
	stroppyPath, stroppyHandler := stroppyconnect.NewStroppyServiceHandler(d.StroppyHandler, d.Interceptors)
	mux.Handle(stroppyPath, stroppyHandler)

	// Schedule service
	schedPath, schedHandler := systemconnect.NewScheduleServiceHandler(d.ScheduleHandler, d.Interceptors)
	mux.Handle(schedPath, schedHandler)

	// TestRunTemplate service
	tplPath, tplH := testingconnect.NewTestRunTemplateServiceHandler(d.TestingHandler, d.Interceptors)
	mux.Handle(tplPath, tplH)

	// TestRun service
	runPath, runH := testingconnect.NewTestRunServiceHandler(d.TestingHandler, d.Interceptors)
	mux.Handle(runPath, runH)

	// TestSuite service
	suitePath, suiteH := testingconnect.NewTestSuiteServiceHandler(d.TestingHandler, d.Interceptors)
	mux.Handle(suitePath, suiteH)

	// TestSuiteRun service
	if d.TestSuiteRunHandler != nil {
		suiteRunPath, suiteRunH := testingconnect.NewTestSuiteRunServiceHandler(d.TestSuiteRunHandler, d.Interceptors)
		mux.Handle(suiteRunPath, suiteRunH)
	}

	// SharedTestRun service
	if d.SharedTestRunHandler != nil {
		sharedRunPath, sharedRunH := testingconnect.NewSharedTestRunServiceHandler(d.SharedTestRunHandler, d.Interceptors)
		mux.Handle(sharedRunPath, sharedRunH)
	}

	// SharedSuiteRun service
	if d.SharedSuiteRunHandler != nil {
		sharedSuitePath, sharedSuiteH := testingconnect.NewSharedSuiteRunServiceHandler(d.SharedSuiteRunHandler, d.Interceptors)
		mux.Handle(sharedSuitePath, sharedSuiteH)
	}

	// Comparison service
	if d.ComparisonHandler != nil {
		cmpPath, cmpH := testingconnect.NewComparisonServiceHandler(d.ComparisonHandler, d.Interceptors)
		mux.Handle(cmpPath, cmpH)
	}

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
	// Stroppy version/commit listing is public catalog data — no tenant required.
	out[stroppyconnect.StroppyServiceListStroppyVersionsProcedure] = true
	out[stroppyconnect.StroppyServiceListStroppyCommitsProcedure] = true
	return out
}

// WithMiddleware wraps interceptors as a HandlerOption.
func WithMiddleware(i ...connect.Interceptor) connect.HandlerOption {
	return connect.WithInterceptors(i...)
}
