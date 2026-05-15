package pgtx

import (
	"context"
	"fmt"

	trmpgx "github.com/avito-tech/go-transaction-manager/pgxv5"
	"github.com/avito-tech/go-transaction-manager/trm"
	"github.com/avito-tech/go-transaction-manager/trm/manager"
	"github.com/avito-tech/go-transaction-manager/trm/settings"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

// TxManager re-exports the trm interface so callers depend on this package
// only.
type TxManager = trm.Manager

// NewTxFlow creates a TxExecutor + Manager bound to the given pool.
func NewTxFlow(pool *pgxpool.Pool, txSettings *trmpgx.Settings, options ...sqlexec.TxExecutorOption) (*sqlexec.TxExecutor, *manager.Manager, error) {
	if pool == nil {
		return nil, nil, fmt.Errorf("pgtx: pool is nil")
	}
	if txSettings == nil {
		txSettings = ReadCommittedSettings()
	}
	mgr, err := manager.New(trmpgx.NewDefaultFactory(pool), manager.WithSettings(*txSettings))
	if err != nil {
		return nil, nil, fmt.Errorf("pgtx: build manager: %w", err)
	}
	return sqlexec.NewTxExecutor(pool, options...), mgr, nil
}

// NewSettings builds tx settings with the given isolation level.
func NewSettings(level pgx.TxIsoLevel, opts ...settings.Opt) *trmpgx.Settings {
	s := trmpgx.MustSettings(settings.Must(opts...), trmpgx.WithTxOptions(pgx.TxOptions{IsoLevel: level}))
	return &s
}

// Convenience constructors.
func SerializableSettings(opts ...settings.Opt) *trmpgx.Settings {
	return NewSettings(pgx.Serializable, opts...)
}

func RepeatableReadSettings(opts ...settings.Opt) *trmpgx.Settings {
	return NewSettings(pgx.RepeatableRead, opts...)
}

func ReadCommittedSettings(opts ...settings.Opt) *trmpgx.Settings {
	return NewSettings(pgx.ReadCommitted, opts...)
}

// WithTransactionRet runs fn inside a transaction with the given isolation,
// returning the typed result.
func WithTransactionRet[T any](
	ctx context.Context,
	mgr TxManager,
	level pgx.TxIsoLevel,
	fn func(ctx context.Context) (T, error),
	opts ...settings.Opt,
) (ret T, err error) {
	err = mgr.DoWithSettings(ctx, NewSettings(level, opts...), func(ctx context.Context) error {
		ret, err = fn(ctx)
		return err
	})
	return ret, err
}
