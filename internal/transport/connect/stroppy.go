package connect

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	stroppysvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

// StroppyHandler implements StroppyServiceHandler.
type StroppyHandler struct{ svc *stroppysvc.Service }

// NewStroppyHandler constructs a StroppyHandler.
func NewStroppyHandler(svc *stroppysvc.Service) *StroppyHandler { return &StroppyHandler{svc: svc} }

func (h *StroppyHandler) ListStroppyVersions(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[stroppypb.StroppyVersionList], error) {
	result, err := h.svc.ListStroppyVersions(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *StroppyHandler) ListStroppyCommits(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[stroppypb.StroppyCommitList], error) {
	result, err := h.svc.ListStroppyCommits(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *StroppyHandler) ProbeStroppyConfig(ctx context.Context, req *connect.Request[stroppypb.ProbeStroppyConfigRequest]) (*connect.Response[stroppypb.ProbeStroppyConfigResponse], error) {
	result, err := h.svc.ProbeStroppyConfig(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *StroppyHandler) PreviewStroppyConfig(ctx context.Context, req *connect.Request[stroppypb.PreviewStroppyConfigRequest]) (*connect.Response[stroppypb.PreviewStroppyConfigResponse], error) {
	result, err := h.svc.PreviewStroppyConfig(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}
