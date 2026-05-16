package fixture

import (
	"testing"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
)

type CatalogFixture struct {
	*IAMFixture
	Catalog *catalog.Service
}

func NewCatalog(t *testing.T) *CatalogFixture {
	t.Helper()
	iam := NewIAM(t)
	exec := iam.F.Executor.(*sqlexec.TxExecutor)
	cat := catalog.New(exec, iam.F.TxMgr, iam.F.Events)
	return &CatalogFixture{IAMFixture: iam, Catalog: cat}
}
