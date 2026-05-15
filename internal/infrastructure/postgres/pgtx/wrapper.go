package pgtx

import (
	"context"

	"github.com/avito-tech/go-transaction-manager/trm/settings"
	"github.com/jackc/pgx/v5"
)

// WithSerializable runs fn inside a SERIALIZABLE transaction.
func WithSerializable(ctx context.Context, mgr TxManager, fn func(ctx context.Context) error, opts ...settings.Opt) error {
	return mgr.DoWithSettings(ctx, NewSettings(pgx.Serializable, opts...), fn)
}

// WithSerializableRet returns a typed value from a SERIALIZABLE tx.
func WithSerializableRet[T any](ctx context.Context, mgr TxManager, fn func(ctx context.Context) (T, error), opts ...settings.Opt) (T, error) {
	return WithTransactionRet(ctx, mgr, pgx.Serializable, fn, opts...)
}

// WithRepeatableRead runs fn inside a REPEATABLE READ transaction.
func WithRepeatableRead(ctx context.Context, mgr TxManager, fn func(ctx context.Context) error, opts ...settings.Opt) error {
	return mgr.DoWithSettings(ctx, NewSettings(pgx.RepeatableRead, opts...), fn)
}

// WithRepeatableReadRet returns a typed value from a REPEATABLE READ tx.
func WithRepeatableReadRet[T any](ctx context.Context, mgr TxManager, fn func(ctx context.Context) (T, error), opts ...settings.Opt) (T, error) {
	return WithTransactionRet(ctx, mgr, pgx.RepeatableRead, fn, opts...)
}

// WithReadCommitted runs fn inside a READ COMMITTED transaction.
func WithReadCommitted(ctx context.Context, mgr TxManager, fn func(ctx context.Context) error, opts ...settings.Opt) error {
	return mgr.DoWithSettings(ctx, NewSettings(pgx.ReadCommitted, opts...), fn)
}

// WithReadCommittedRet returns a typed value from a READ COMMITTED tx.
func WithReadCommittedRet[T any](ctx context.Context, mgr TxManager, fn func(ctx context.Context) (T, error), opts ...settings.Opt) (T, error) {
	return WithTransactionRet(ctx, mgr, pgx.ReadCommitted, fn, opts...)
}
