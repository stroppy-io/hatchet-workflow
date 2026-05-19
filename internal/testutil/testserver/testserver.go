package testserver

import (
	"net/http"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	adminsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/admin"
	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	stroppysvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
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
		IAMHandler:   transportconnect.NewIAMHandler(svc, 24*time.Hour, false),
		Interceptors: interceptors,
	})
}

// AllDeps holds the services required by ForAll. Optional fields may be left
// nil — Mount tolerates unset handlers and simply omits those routes.
type AllDeps struct {
	IAM       *iam.Service
	Catalog   *catalog.Service
	Stroppy   *stroppysvc.Service
	System    *system.Service
	Templates *testingsvc.TemplateService
	Runs      *testingsvc.TestRunService
	Suites    *testingsvc.TestSuiteService

	SuiteRuns       *testingsvc.TestSuiteRunService
	SharedRuns      *testingsvc.SharedTestRunService
	SharedSuiteRuns *testingsvc.SharedSuiteRunService
	Comparison      *testingsvc.ComparisonService
	Baselines       *testingsvc.BaselineService

	Webhooks    *opssvc.WebhookService
	Quota       *opssvc.QuotaService
	BinaryCache *opssvc.BinaryCacheService

	Admin       *adminsvc.AdminService
	BinaryAdmin *adminsvc.BinaryCacheAdminService

	Agent *agentsvc.Service

	// Idempotency: tests can opt in by setting Enabled=true. Default = off
	// so the test suite doesn't pay the per-request valkey round-trip when it
	// isn't validating the middleware.
	IdempotencyEnabled bool
}

// ForAll returns an http.Handler with the full middleware chain
// (Recovery + RequestID + Auth + Tenant + ProtoValidate + Idempotency + ErrorMapper)
// and all registered service handlers — admin routes additionally enforce
// RequirePlatformAdmin. Suitable for comprehensive e2e tests via httptest.NewServer.
func ForAll(d AllDeps) http.Handler {
	// In-memory valkey backing idempotency keys. Failure here is fatal; tests
	// should treat it the same as any required infra.
	vk, err := valkey.NewInMemory()
	if err != nil {
		panic("testserver.ForAll: valkey in-memory: " + err.Error())
	}

	base := connect.WithInterceptors(
		middleware.Recovery(zap.Must(zap.NewDevelopment())),
		middleware.RequestID(),
		middleware.Auth(d.IAM, transportconnect.AuthBypass()),
		middleware.Tenant(d.IAM, transportconnect.TenantBypass()),
		// ProtoValidate intentionally omitted: a handful of generated request
		// schemas (e.g. CreateTenantRequest) require fields the server fills
		// in (Tenant.Id). Phase 2+ tests may opt-in by mounting their own
		// chain. The base chain mirrors production minus this pre-existing
		// inconsistency.
		middleware.Idempotency(vk, middleware.BuildIdempotencyRegistry(), middleware.IdempotencyConfig{
			Enabled: d.IdempotencyEnabled,
			TTL:     30 * time.Second,
		}),
		middleware.ErrorMapper(),
	)
	adminOpts := connect.WithOptions(base, connect.WithInterceptors(middleware.RequirePlatformAdmin()))

	deps := transportconnect.Deps{
		IAMHandler:        transportconnect.NewIAMHandler(d.IAM, 24*time.Hour, false),
		Interceptors:      base,
		AdminInterceptors: adminOpts,
	}
	if d.Catalog != nil {
		deps.CatalogHandler = transportconnect.NewCatalogHandler(d.Catalog)
	}
	if d.Stroppy != nil {
		deps.StroppyHandler = transportconnect.NewStroppyHandler(d.Stroppy)
	}
	if d.System != nil {
		deps.ScheduleHandler = transportconnect.NewScheduleHandler(d.System)
	}
	if d.Templates != nil && d.Runs != nil && d.Suites != nil {
		deps.TestingHandler = transportconnect.NewTestingHandler(d.Templates, d.Runs, d.Suites)
	}
	if d.SuiteRuns != nil {
		deps.TestSuiteRunHandler = transportconnect.NewTestSuiteRunHandler(d.SuiteRuns)
	}
	if d.SharedRuns != nil {
		deps.SharedTestRunHandler = transportconnect.NewSharedTestRunHandler(d.SharedRuns)
	}
	if d.SharedSuiteRuns != nil {
		deps.SharedSuiteRunHandler = transportconnect.NewSharedSuiteRunHandler(d.SharedSuiteRuns)
	}
	if d.Comparison != nil {
		deps.ComparisonHandler = transportconnect.NewComparisonHandler(d.Comparison)
	}
	if d.Baselines != nil {
		deps.BaselineHandler = transportconnect.NewBaselineHandler(d.Baselines)
	}
	if d.Webhooks != nil {
		deps.WebhookHandler = transportconnect.NewWebhookHandler(d.Webhooks)
	}
	if d.Quota != nil {
		deps.QuotaHandler = transportconnect.NewQuotaHandler(d.Quota)
	}
	if d.BinaryCache != nil {
		deps.BinaryCacheHandler = transportconnect.NewBinaryCacheHandler(d.BinaryCache)
	}
	if d.Admin != nil {
		deps.AdminHandler = transportconnect.NewAdminHandler(d.Admin)
	}
	if d.BinaryAdmin != nil {
		deps.BinaryCacheAdminHandler = transportconnect.NewBinaryCacheAdminHandler(d.BinaryAdmin)
	}
	if d.Agent != nil {
		deps.AgentHandler = transportconnect.NewAgentHandler(d.Agent)
	}
	return transportconnect.Mount(deps)
}
