package fixture

import (
	"net/http/httptest"
	"testing"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	stroppysvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
)

// AllFixture wires every service needed for a full-stack e2e test:
// IAM + Catalog + Stroppy (mock GH) + System + Testing.
type AllFixture struct {
	*IAMFixture
	Catalog   *catalog.Service
	Stroppy   *stroppysvc.Service
	System    *system.Service
	Templates *testingsvc.TemplateService
	Runs      *testingsvc.TestRunService
	Suites    *testingsvc.TestSuiteService
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
	runs := testingsvc.NewTestRunService(exec, txMgr, events, cat, sys, b)

	return &AllFixture{
		IAMFixture: iamF,
		Catalog:    cat,
		Stroppy:    stroppy,
		System:     sys,
		Templates:  templates,
		Runs:       runs,
		Suites:     suites,
	}
}
