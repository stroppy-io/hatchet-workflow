package fixture

import (
	"testing"
	"time"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker/handlers"
)

type SystemFixture struct {
	*IAMFixture
	System *system.Service
	Worker *nodeworker.Worker
}

func NewSystem(t *testing.T) *SystemFixture {
	t.Helper()
	iam := NewIAM(t)
	exec := iam.F.Executor.(*sqlexec.TxExecutor)
	sys := system.New(exec, iam.F.TxMgr, iam.F.Events)
	reg := nodeworker.NewRegistry()
	reg.Register(handlers.NewMockHandler())
	w := nodeworker.New(iam.F.Pool, sys, reg, nodeworker.Config{Workers: 1, Tick: 50 * time.Millisecond}, zap.NewNop())
	return &SystemFixture{IAMFixture: iam, System: sys, Worker: w}
}
