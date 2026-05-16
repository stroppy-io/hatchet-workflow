package handlers

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

type MockHandler struct{}

func NewMockHandler() *MockHandler { return &MockHandler{} }
func (h *MockHandler) Kind() string { return "Mock" }
func (h *MockHandler) Execute(_ context.Context, _ *systempb.NodeRun, _ *anypb.Any, _ nodeworker.StateStore) (*anypb.Any, error) {
	return nil, nil
}
