package iam

import (
	"context"
	"time"

	"github.com/avito-tech/go-transaction-manager/trm"
)

// MockTrm is a dummy transaction manager that simply executes the closure
// without initiating an actual database transaction. This is useful for unit
// testing service logic that relies on doTx.
type MockTrm struct{}

func (m *MockTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *MockTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type FakeClock struct{}

func (FakeClock) Now() time.Time { return time.Now() }
