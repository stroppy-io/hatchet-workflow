package postgres

import (
	"context"

	"github.com/gopherex/pgtx/pkg/tx"
)

// Transactor is the domain's Transactor over pgtx: read-committed, no
// retry (callers repeat side effects themselves).
type Transactor struct{ trm tx.Trm }

// NewTransactor builds it.
func NewTransactor(trm tx.Trm) Transactor { return Transactor{trm: trm} }

// Do runs fn in one transaction; repositories built over TxDB join it.
func (t Transactor) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoReadCommitted(ctx, t.trm, fn)
}
