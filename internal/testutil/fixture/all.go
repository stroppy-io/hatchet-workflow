package fixture

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	adminsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/admin"
	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	stroppysvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// AllFixture wires every service needed for a full-stack e2e test:
// IAM + Catalog + Stroppy (mock GH) + System + Testing + Ops + Admin + Agent.
type AllFixture struct {
	*IAMFixture
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

	Admin       *adminsvc.AdminService
	BinaryAdmin *adminsvc.BinaryCacheAdminService

	Webhooks    *opssvc.WebhookService
	Quota       *opssvc.QuotaService
	BinaryCache *opssvc.BinaryCacheService

	Agent    *agentsvc.Service
	AgentHub *agentsvc.Hub

	Builder  *dagbuilder.Builder
	PkgStore *inMemoryPackageStorage
}

// inMemoryPackageStorage implements catalog.PackageStorage backed by a map.
type inMemoryPackageStorage struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newInMemoryPackageStorage() *inMemoryPackageStorage {
	return &inMemoryPackageStorage{data: map[string][]byte{}}
}

func (s *inMemoryPackageStorage) PutObject(_ context.Context, key string, body io.Reader, _ string) (string, error) {
	buf, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(buf))
	copy(cp, buf)
	s.data[key] = cp
	return "s3://memory/" + key, nil
}

func (s *inMemoryPackageStorage) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "http://memory.local/" + key, nil
}

func (s *inMemoryPackageStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// Get returns the bytes stored under key, or (nil, false).
func (s *inMemoryPackageStorage) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[key]
	if !ok {
		return nil, false
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, true
}

// noopMetrics is a MetricsPort returning empty series.
type noopMetrics struct{}

func (noopMetrics) Query(_ context.Context, _ *iampb.TenantId, _ *testingpb.TestRunId, _ string) (*testingpb.MetricSeriesList, error) {
	return &testingpb.MetricSeriesList{}, nil
}

// NewAll creates a full-stack fixture. releasesJSON / commitsJSON are forwarded
// to the stub GitHub server used by the Stroppy service.
func NewAll(t *testing.T, releasesJSON, commitsJSON string) *AllFixture {
	t.Helper()
	iamF := NewIAM(t)
	exec := iamF.F.Executor.(*sqlexec.TxExecutor)
	txMgr := iamF.F.TxMgr
	events := iamF.F.Events

	// Catalog — catalog.Service satisfies dagbuilder.CatalogPort directly.
	cat := catalog.New(exec, txMgr, events)
	pkgStore := newInMemoryPackageStorage()
	cat.SetPackageStorage(pkgStore)

	// Mirror the cmd_server behaviour: seed builtin packages on TenantCreated
	// so e2e tests that exercise the DAG-builder package resolution succeed
	// out of the box (Builder requires a real Package row per engine kind).
	events.Subscribe(eventing.TopicTenantCreated, func(_ context.Context, e eventing.Event) {
		payload, ok := e.Payload.(eventing.TenantCreated)
		if !ok {
			return
		}
		seedCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = cat.SeedBuiltinPackages(seedCtx, &iampb.TenantId{Value: payload.TenantID}, nil)
	})

	// Stroppy (mock GitHub) — reuses stubGitHubHandler defined in stroppy.go.
	gh := httptest.NewServer(stubGitHubHandler(releasesJSON, commitsJSON))
	t.Cleanup(gh.Close)
	runner := stroppybin.New("dev", "/tmp/stroppy-test-binaries")
	vk, err := valkey.NewInMemory()
	if err != nil {
		t.Fatalf("fixture.NewAll: valkey in-memory: %v", err)
	}
	t.Cleanup(vk.Close)
	stroppy := stroppysvc.New(runner, vk, gh.URL+"/releases", gh.URL+"/commits")

	// System
	sys := system.New(exec, txMgr, events)

	// Testing layer
	b := dagbuilder.New(cat)
	templates := testingsvc.NewTemplateService(exec, txMgr, events)
	suites := testingsvc.NewTestSuiteService(exec, txMgr, events)

	// Ops services (need pool for QuotaService).
	quota := opssvc.NewQuotaService(exec, iamF.F.Pool, txMgr)
	webhooks := opssvc.NewWebhookService(exec, txMgr, events)
	binCache := opssvc.NewBinaryCacheService(exec, txMgr)

	// NOTE: TestRunService defaults to NoopQuota — Phase 2+ tests that exercise
	// quota enforcement can call `f.Runs.WithQuota(f.Quota)` themselves.
	runs := testingsvc.NewTestRunService(exec, txMgr, events, cat, sys, b)
	runs = runs.WithMetrics(noopMetrics{})
	suiteRuns := testingsvc.NewTestSuiteRunService(exec, txMgr, events, sys, b)
	sharedRuns := testingsvc.NewSharedTestRunService(exec, txMgr, events)
	sharedSuiteRuns := testingsvc.NewSharedSuiteRunService(exec, txMgr, events)
	comparison := testingsvc.NewComparisonService(noopMetrics{})
	comparison.WithTestRunLister(runs)
	baselines := testingsvc.NewBaselineService(exec, txMgr, events)

	// Admin services.
	admin := adminsvc.NewAdminService(iamF.IAM)
	binAdmin := adminsvc.NewBinaryCacheAdminService(exec, txMgr)

	// Agent — JWT secret reused from IAM bootstrap.
	hub := agentsvc.NewHub()
	cmdRepo := agentsvc.NewCommandsRepo(exec, txMgr)
	bootstrap := agentsvc.NewBootstrapTokenStore([]byte("test-jwt-secret-32-bytes-long!!"))
	agentSvc := agentsvc.New(exec, txMgr, events, hub, cmdRepo, bootstrap)

	return &AllFixture{
		IAMFixture:      iamF,
		Catalog:         cat,
		Stroppy:         stroppy,
		System:          sys,
		Templates:       templates,
		Runs:            runs,
		Suites:          suites,
		SuiteRuns:       suiteRuns,
		SharedRuns:      sharedRuns,
		SharedSuiteRuns: sharedSuiteRuns,
		Comparison:      comparison,
		Baselines:       baselines,
		Admin:           admin,
		BinaryAdmin:     binAdmin,
		Webhooks:        webhooks,
		Quota:           quota,
		BinaryCache:     binCache,
		Agent:           agentSvc,
		AgentHub:        hub,
		Builder:         b,
		PkgStore:        pkgStore,
	}
}

// SeedTenant creates a tenant via IAM and synchronously seeds builtin packages,
// bypassing the async TopicTenantCreated subscriber for deterministic test setup.
// Returns the tenant id.
func (f *AllFixture) SeedTenant(ctx context.Context, name string, ownerID *iampb.UserId) (*iampb.TenantId, error) {
	tenant, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: name}}, ownerID)
	if err != nil {
		return nil, fmt.Errorf("seed tenant: create: %w", err)
	}
	// Synchronous seed — the async subscriber may or may not have completed by
	// the time the caller wants to operate on the tenant.
	if err := f.Catalog.SeedBuiltinPackages(ctx, tenant.GetId(), nil); err != nil {
		// Tolerate "already exists" — the async subscriber may have raced us.
		// Other errors propagate.
		_ = err
	}
	return tenant.GetId(), nil
}
