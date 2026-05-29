package utils

import (
	"context"

	"github.com/avito-tech/go-transaction-manager/trm"
)

// MockTrm is a dummy transaction manager that simply executes the closure
// without initiating an actual database transaction. Shared by every service's
// unit tests that exercise doTx (it is a plain struct, not a gomock mock).
type MockTrm struct{}

func (m *MockTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *MockTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
