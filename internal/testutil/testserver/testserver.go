package testserver

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	stroppysvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	transportconnect "github.com/stroppy-io/stroppy-cloud/internal/transport/connect"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// ForIAM returns an http.Handler that wires IAM handlers with Auth + ErrorMapper
// middleware, suitable for httptest.NewServer.
func ForIAM(svc *iam.Service) http.Handler {
	interceptors := connect.WithInterceptors(
		middleware.Auth(svc, transportconnect.AuthBypass()),
		middleware.ErrorMapper(),
	)
	return transportconnect.Mount(transportconnect.Deps{
		IAMHandler:   transportconnect.NewIAMHandler(svc),
		Interceptors: interceptors,
	})
}

// AllDeps holds the services required by ForAll.
type AllDeps struct {
	IAM      *iam.Service
	Catalog  *catalog.Service
	Stroppy  *stroppysvc.Service
	System   *system.Service
	Templates *testingsvc.TemplateService
	Runs     *testingsvc.TestRunService
	Suites   *testingsvc.TestSuiteService
}

// ForAll returns an http.Handler with the full middleware chain
// (Auth + Tenant + ErrorMapper) and all registered service handlers.
// Suitable for comprehensive e2e tests via httptest.NewServer.
func ForAll(d AllDeps) http.Handler {
	interceptors := connect.WithInterceptors(
		middleware.Auth(d.IAM, transportconnect.AuthBypass()),
		middleware.Tenant(d.IAM, transportconnect.TenantBypass()),
		middleware.ErrorMapper(),
	)
	deps := transportconnect.Deps{
		IAMHandler:      transportconnect.NewIAMHandler(d.IAM),
		CatalogHandler:  transportconnect.NewCatalogHandler(d.Catalog),
		StroppyHandler:  transportconnect.NewStroppyHandler(d.Stroppy),
		ScheduleHandler: transportconnect.NewScheduleHandler(d.System),
		TestingHandler:  transportconnect.NewTestingHandler(d.Templates, d.Runs, d.Suites),
		Interceptors:    interceptors,
	}
	return transportconnect.Mount(deps)
}
