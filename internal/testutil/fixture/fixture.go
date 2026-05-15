package fixture

import (
	"context"
	"testing"

	"github.com/avito-tech/go-transaction-manager/trm/manager"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/pgcontainer"
)

type F struct {
	T        *testing.T
	Ctx      context.Context
	Pool     *pgxpool.Pool
	Executor any
	TxMgr    *manager.Manager
	Events   eventing.Bus
}

func New(t *testing.T) *F {
	t.Helper()
	pool := pgcontainer.Bootstrap(t)
	exec, mgr, err := pgtx.NewTxFlow(pool, pgtx.ReadCommittedSettings())
	if err != nil {
		t.Fatalf("fixture: tx flow: %v", err)
	}
	bus := eventing.NewInMemoryBus()
	return &F{
		T:        t,
		Ctx:      context.Background(),
		Pool:     pool,
		Executor: exec,
		TxMgr:    mgr,
		Events:   bus,
	}
}
