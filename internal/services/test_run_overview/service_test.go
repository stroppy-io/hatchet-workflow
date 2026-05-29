package test_run_overview

// service_test.go: tests for all 6 RPCs in service.go:
// GetTestRunOverview, StreamTestRunOverview, QueryLogs, StreamLogs,
// ResolveLogRef, GetRunMetrics.
//
// Each test follows the pattern from internal/services/iam:
//   - gomock controller
//   - NewMock*(ctrl) for each dependency
//   - service constructed via TestRunOverviewDeps{Tx:&MockTrm{}, ...}
//   - t.Run sub-tests for Success + every meaningful error branch
//   - assert via status.Code(err)

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// ─── fake streaming server helpers ──────────────────────────────────────────

// fakeOverviewStream implements grpc.ServerStreamingServer[monitor.Overview].
// It captures sent messages and exposes a context so the handler can be
// cancelled from the test.
type fakeOverviewStream struct {
	grpc.ServerStream
	ctx     context.Context
	cancel  context.CancelFunc
	sent    []*monitor.Overview
	sendErr error
}

func newFakeOverviewStream() *fakeOverviewStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeOverviewStream{ctx: ctx, cancel: cancel}
}

func (s *fakeOverviewStream) Context() context.Context { return s.ctx }
func (s *fakeOverviewStream) Send(ov *monitor.Overview) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, ov)
	return nil
}

// fakeLogLineStream implements grpc.ServerStreamingServer[monitor.LogLine].
type fakeLogLineStream struct {
	grpc.ServerStream
	ctx     context.Context
	cancel  context.CancelFunc
	sent    []*monitor.LogLine
	sendErr error
}

func newFakeLogLineStream() *fakeLogLineStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeLogLineStream{ctx: ctx, cancel: cancel}
}

func (s *fakeLogLineStream) Context() context.Context { return s.ctx }
func (s *fakeLogLineStream) Send(line *monitor.LogLine) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, line)
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newSvc(ctrl *gomock.Controller,
	authn *utils.MockAuthn,
	runs *MockTestRunReader,
	overview *MockOverviewReader,
	logs *MockLogReader,
	metrics *MockMetricsReader,
) *TestRunOverviewService {
	return NewTestRunOverviewService(TestRunOverviewDeps{
		Tx:       &utils.MockTrm{},
		Authn:    authn,
		Runs:     runs,
		Overview: overview,
		Logs:     logs,
		Metrics:  metrics,
	})
}

func goodClaims() *iampb.AccessClaims {
	return &iampb.AccessClaims{AccountId: "a1"}
}

// ─── GetTestRunOverview ───────────────────────────────────────────────────────

func TestGetTestRunOverview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	ov := NewMockOverviewReader(ctrl)
	svc := newSvc(ctrl, authn, runs, ov, nil, nil)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		ov.EXPECT().Get(ctx, "r1").Return(&monitor.Overview{}, nil)

		resp, err := svc.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Overview == nil {
			t.Error("expected non-nil overview")
		}
	})

	t.Run("UnauthenticatedCaller", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("EmptyTenantId", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)

		_, err := svc.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{
			TenantId: "", RunId: "r1",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("EmptyRunId", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)

		_, err := svc.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{
			TenantId: "t1", RunId: "",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("RunNotFound", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r-missing").Return(derrors.ErrNotFound)

		_, err := svc.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{
			TenantId: "t1", RunId: "r-missing",
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("RunExistsError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(errors.New("db err"))

		_, err := svc.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("OverviewGetError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		ov.EXPECT().Get(ctx, "r1").Return(nil, errors.New("store err"))

		_, err := svc.GetTestRunOverview(ctx, &api.GetTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ─── StreamTestRunOverview ────────────────────────────────────────────────────

func TestStreamTestRunOverview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	ovReader := NewMockOverviewReader(ctrl)
	svc := newSvc(ctrl, authn, runs, ovReader, nil, nil)

	t.Run("Success_ChannelClose", func(t *testing.T) {
		stream := newFakeOverviewStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		ch := make(chan *monitor.Overview, 2)
		ch <- &monitor.Overview{}
		close(ch)
		ovReader.EXPECT().Stream(ctx, "r1").Return((<-chan *monitor.Overview)(ch), nil)

		err := svc.StreamTestRunOverview(&api.StreamTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		}, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stream.sent) != 1 {
			t.Errorf("expected 1 sent message, got %d", len(stream.sent))
		}
	})

	t.Run("Success_NilMessageSkipped", func(t *testing.T) {
		stream := newFakeOverviewStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		ch := make(chan *monitor.Overview, 3)
		ch <- nil
		ch <- &monitor.Overview{}
		close(ch)
		ovReader.EXPECT().Stream(ctx, "r1").Return((<-chan *monitor.Overview)(ch), nil)

		err := svc.StreamTestRunOverview(&api.StreamTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		}, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stream.sent) != 1 {
			t.Errorf("expected 1 sent (nil skipped), got %d", len(stream.sent))
		}
	})

	t.Run("UnauthenticatedCaller", func(t *testing.T) {
		stream := newFakeOverviewStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		err := svc.StreamTestRunOverview(&api.StreamTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		}, stream)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RunNotFound", func(t *testing.T) {
		stream := newFakeOverviewStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(derrors.ErrNotFound)

		err := svc.StreamTestRunOverview(&api.StreamTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		}, stream)
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("StreamError", func(t *testing.T) {
		stream := newFakeOverviewStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		ovReader.EXPECT().Stream(ctx, "r1").Return(nil, errors.New("stream err"))

		err := svc.StreamTestRunOverview(&api.StreamTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		}, stream)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ContextCancelled", func(t *testing.T) {
		stream := newFakeOverviewStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		// Unbuffered channel that will never produce — cancel ctx instead
		ch := make(chan *monitor.Overview)
		ovReader.EXPECT().Stream(ctx, "r1").Return((<-chan *monitor.Overview)(ch), nil)

		// Cancel ctx before calling so the select hits ctx.Done immediately
		stream.cancel()

		err := svc.StreamTestRunOverview(&api.StreamTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		}, stream)
		// status.FromContextError(ctx.Err()).Err() yields Canceled code
		if status.Code(err) != codes.Canceled {
			t.Errorf("expected Canceled, got %v", err)
		}
	})

	t.Run("SendError", func(t *testing.T) {
		stream := newFakeOverviewStream()
		stream.sendErr = errors.New("send failed")
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		ch := make(chan *monitor.Overview, 1)
		ch <- &monitor.Overview{}
		ovReader.EXPECT().Stream(ctx, "r1").Return((<-chan *monitor.Overview)(ch), nil)

		err := svc.StreamTestRunOverview(&api.StreamTestRunOverviewRequest{
			TenantId: "t1", RunId: "r1",
		}, stream)
		if err == nil {
			t.Error("expected send error to propagate")
		}
	})
}

// ─── QueryLogs ───────────────────────────────────────────────────────────────

func TestQueryLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	logs := NewMockLogReader(ctrl)
	svc := newSvc(ctrl, authn, runs, nil, logs, nil)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		lines := []*monitor.LogLine{{}, {}}
		older := &monitor.LogCursor{}
		newer := &monitor.LogCursor{}
		logs.EXPECT().Query(ctx, "r1", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(lines, older, newer, nil)

		resp, err := svc.QueryLogs(ctx, &api.QueryLogsRequest{
			TenantId: "t1", RunId: "r1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Lines) != 2 {
			t.Errorf("expected 2 lines, got %d", len(resp.Lines))
		}
		if resp.Older == nil || resp.Newer == nil {
			t.Error("expected cursor pointers")
		}
	})

	t.Run("UnauthenticatedCaller", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.QueryLogs(ctx, &api.QueryLogsRequest{TenantId: "t1", RunId: "r1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("EmptyTenantId", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)

		_, err := svc.QueryLogs(ctx, &api.QueryLogsRequest{TenantId: "", RunId: "r1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("EmptyRunId", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)

		_, err := svc.QueryLogs(ctx, &api.QueryLogsRequest{TenantId: "t1", RunId: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("RunNotFound", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r-none").Return(derrors.ErrNotFound)

		_, err := svc.QueryLogs(ctx, &api.QueryLogsRequest{TenantId: "t1", RunId: "r-none"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("QueryError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		logs.EXPECT().Query(ctx, "r1", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil, nil, errors.New("log err"))

		_, err := svc.QueryLogs(ctx, &api.QueryLogsRequest{TenantId: "t1", RunId: "r1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ─── StreamLogs ──────────────────────────────────────────────────────────────

func TestStreamLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	logs := NewMockLogReader(ctrl)
	svc := newSvc(ctrl, authn, runs, nil, logs, nil)

	t.Run("Success_ChannelClose", func(t *testing.T) {
		stream := newFakeLogLineStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		ch := make(chan *monitor.LogLine, 2)
		ch <- &monitor.LogLine{}
		close(ch)
		logs.EXPECT().Stream(ctx, "r1", gomock.Any(), gomock.Any()).
			Return((<-chan *monitor.LogLine)(ch), nil)

		err := svc.StreamLogs(&api.StreamLogsRequest{TenantId: "t1", RunId: "r1"}, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stream.sent) != 1 {
			t.Errorf("expected 1 sent line, got %d", len(stream.sent))
		}
	})

	t.Run("Success_NilLineSkipped", func(t *testing.T) {
		stream := newFakeLogLineStream()
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		ch := make(chan *monitor.LogLine, 3)
		ch <- nil
		ch <- &monitor.LogLine{}
		close(ch)
		logs.EXPECT().Stream(ctx, "r1", gomock.Any(), gomock.Any()).
			Return((<-chan *monitor.LogLine)(ch), nil)

		err := svc.StreamLogs(&api.StreamLogsRequest{TenantId: "t1", RunId: "r1"}, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stream.sent) != 1 {
			t.Errorf("expected 1 sent (nil skipped), got %d", len(stream.sent))
		}
	})

	t.Run("UnauthenticatedCaller", func(t *testing.T) {
		stream := newFakeLogLineStream()
		ctx := stream.ctx
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		err := svc.StreamLogs(&api.StreamLogsRequest{TenantId: "t1", RunId: "r1"}, stream)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RunNotFound", func(t *testing.T) {
		stream := newFakeLogLineStream()
		ctx := stream.ctx
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(derrors.ErrNotFound)

		err := svc.StreamLogs(&api.StreamLogsRequest{TenantId: "t1", RunId: "r1"}, stream)
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("LogsStreamError", func(t *testing.T) {
		stream := newFakeLogLineStream()
		ctx := stream.ctx
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		logs.EXPECT().Stream(ctx, "r1", gomock.Any(), gomock.Any()).
			Return(nil, errors.New("stream err"))

		err := svc.StreamLogs(&api.StreamLogsRequest{TenantId: "t1", RunId: "r1"}, stream)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ContextCancelled", func(t *testing.T) {
		stream := newFakeLogLineStream()
		ctx := stream.ctx
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		ch := make(chan *monitor.LogLine)
		logs.EXPECT().Stream(ctx, "r1", gomock.Any(), gomock.Any()).
			Return((<-chan *monitor.LogLine)(ch), nil)

		stream.cancel()

		err := svc.StreamLogs(&api.StreamLogsRequest{TenantId: "t1", RunId: "r1"}, stream)
		if status.Code(err) != codes.Canceled {
			t.Errorf("expected Canceled, got %v", err)
		}
	})

	t.Run("SendError", func(t *testing.T) {
		stream := newFakeLogLineStream()
		stream.sendErr = errors.New("send failed")
		ctx := stream.ctx

		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)

		ch := make(chan *monitor.LogLine, 1)
		ch <- &monitor.LogLine{}
		logs.EXPECT().Stream(ctx, "r1", gomock.Any(), gomock.Any()).
			Return((<-chan *monitor.LogLine)(ch), nil)

		err := svc.StreamLogs(&api.StreamLogsRequest{TenantId: "t1", RunId: "r1"}, stream)
		if err == nil {
			t.Error("expected send error to propagate")
		}
	})
}

// ─── ResolveLogRef ────────────────────────────────────────────────────────────

func TestResolveLogRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	logs := NewMockLogReader(ctrl)
	svc := newSvc(ctrl, authn, runs, nil, logs, nil)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ref := &monitor.LogRef{RunId: "r1"}
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		logs.EXPECT().Resolve(ctx, ref).Return(&api.LogFilter{}, &monitor.LogCursor{}, nil)

		resp, err := svc.ResolveLogRef(ctx, &api.ResolveLogRefRequest{
			TenantId: "t1",
			Ref:      ref,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.RunId != "r1" {
			t.Errorf("expected run_id=r1, got %s", resp.RunId)
		}
	})

	t.Run("NilRef", func(t *testing.T) {
		_, err := svc.ResolveLogRef(ctx, &api.ResolveLogRefRequest{
			TenantId: "t1",
			Ref:      nil,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for nil ref, got %v", err)
		}
	})

	t.Run("EmptyRefRunId", func(t *testing.T) {
		_, err := svc.ResolveLogRef(ctx, &api.ResolveLogRefRequest{
			TenantId: "t1",
			Ref:      &monitor.LogRef{RunId: ""},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for empty ref.run_id, got %v", err)
		}
	})

	t.Run("UnauthenticatedCaller", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.ResolveLogRef(ctx, &api.ResolveLogRefRequest{
			TenantId: "t1",
			Ref:      &monitor.LogRef{RunId: "r1"},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("EmptyTenantId", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)

		_, err := svc.ResolveLogRef(ctx, &api.ResolveLogRefRequest{
			TenantId: "",
			Ref:      &monitor.LogRef{RunId: "r1"},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("RunNotFound", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r-none").Return(derrors.ErrNotFound)

		_, err := svc.ResolveLogRef(ctx, &api.ResolveLogRefRequest{
			TenantId: "t1",
			Ref:      &monitor.LogRef{RunId: "r-none"},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("ResolveError", func(t *testing.T) {
		ref := &monitor.LogRef{RunId: "r1"}
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		logs.EXPECT().Resolve(ctx, ref).Return(nil, nil, errors.New("resolve err"))

		_, err := svc.ResolveLogRef(ctx, &api.ResolveLogRefRequest{
			TenantId: "t1",
			Ref:      ref,
		})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ─── GetRunMetrics ────────────────────────────────────────────────────────────

func TestGetRunMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsReader(ctrl)
	svc := newSvc(ctrl, authn, runs, nil, nil, metrics)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		metrics.EXPECT().Get(ctx, "r1").Return(&monitor.RunMetrics{}, nil)

		resp, err := svc.GetRunMetrics(ctx, &api.GetRunMetricsRequest{
			TenantId: "t1", RunId: "r1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Metrics == nil {
			t.Error("expected non-nil metrics")
		}
	})

	t.Run("UnauthenticatedCaller", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.GetRunMetrics(ctx, &api.GetRunMetricsRequest{TenantId: "t1", RunId: "r1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("EmptyTenantId", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)

		_, err := svc.GetRunMetrics(ctx, &api.GetRunMetricsRequest{TenantId: "", RunId: "r1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("EmptyRunId", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)

		_, err := svc.GetRunMetrics(ctx, &api.GetRunMetricsRequest{TenantId: "t1", RunId: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("RunNotFound", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r-none").Return(derrors.ErrNotFound)

		_, err := svc.GetRunMetrics(ctx, &api.GetRunMetricsRequest{TenantId: "t1", RunId: "r-none"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("MetricsGetError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(goodClaims(), nil)
		runs.EXPECT().Exists(ctx, "t1", "r1").Return(nil)
		metrics.EXPECT().Get(ctx, "r1").Return(nil, errors.New("metrics err"))

		_, err := svc.GetRunMetrics(ctx, &api.GetRunMetricsRequest{TenantId: "t1", RunId: "r1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}
