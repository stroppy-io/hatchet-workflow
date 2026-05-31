package app

import (
	"context"

	"github.com/avito-tech/go-transaction-manager/trm"
	"github.com/gopherex/pgtx/pkg/tx"
)

// noopTrm is a no-op transaction manager satisfying tx.Trm (trm.Manager) for the
// in-memory demo: there is no real database, so every "transaction" is just the
// callback run with the ambient context. The services call
// tx.DoSerializable/DoSerializableRet which delegate to DoWithSettings, so this
// is all they need.
type noopTrm struct{}

var _ tx.Trm = (*noopTrm)(nil)

// Do runs fn with the given context, no transaction.
func (noopTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// DoWithSettings runs fn with the given context, ignoring the settings (no real
// transaction to configure).
func (noopTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
