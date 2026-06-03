package test_run_overview

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	test_run_overview owns no storage, no clock and no log/metrics backend of its
	own: every read goes through one of these ports. Implementations live
	elsewhere and are wired in via TestRunOverviewDeps. Persistence ports return
	derrors.ErrNotFound / derrors.ErrConflict so the handler can translate them
	into status codes via utils.MapErr.

	Everything here is RUNTIME, read-only insight into a run keyed by run id, and
	tenant-scoped. The annotated RESOURCE_TEST_RUN/READ permission is enforced by
	the auth interceptor — handlers do NOT re-check it. They DO: resolve the
	caller (so an unauthenticated request is rejected before any backend read),
	validate the request tenant_id, and verify the target run exists AND belongs
	to that tenant before exposing any of its observability data (no cross-tenant
	reads). Cross-tenant ids are reported by the repo as derrors.ErrNotFound and
	never leak existence.
*/

// TestRunReader verifies a run's existence and tenant ownership. Exists is the
// tenant gate every handler runs first: a run id belonging to another tenant (or
// to no run at all) MUST be reported as derrors.ErrNotFound, never confirmed, so
// observability data cannot be pulled across tenants.
type TestRunReader interface {
	// Exists reports that runID exists AND belongs to tenantID. A run of another
	// tenant, or an unknown id, returns derrors.ErrNotFound.
	Exists(ctx context.Context, tenantID, runID string) error
}

// OverviewReader serves the Overview tab. Get is a one-shot snapshot; Stream
// pushes a fresh full snapshot on every tick (the client replaces its state, no
// diff merging) until ctx is cancelled or the run reaches a terminal state, at
// which point the implementation closes the channel. Both are scoped to a run
// the caller has already been authorised to read.
//
// A snapshot bundles the persisted run record, the staged topology envelope, the
// live monitor.Overview projection and (when the run belongs to a suite) the
// parent suite-run record. The implementation is responsible for assembling all
// populated fields; the handler forwards the snapshot verbatim.
type OverviewReader interface {
	Get(ctx context.Context, runID string) (*api.TestRunOverviewSnapshot, error)
	// Stream returns a channel of full snapshots. The implementation closes the
	// channel when ctx is done or the run is terminal. It MUST honour ctx
	// cancellation promptly.
	Stream(ctx context.Context, runID string) (<-chan *api.TestRunOverviewSnapshot, error)
}

// LogReader serves the Logs tab. Query is cursor-paged history (scroll both
// directions); Stream is a bounded live tail. ResolveLogRef turns a shareable
// LogRef deep-link into a concrete (run, filter, anchor cursor) so every user
// opening the same link lands on the same line.
type LogReader interface {
	// Query returns one page of lines for the (run, filter, anchor, direction)
	// plus the cursors to page OLDER and NEWER than the returned slice.
	Query(ctx context.Context, runID string, filter *api.LogFilter, from *monitor.LogCursor, direction api.LogScrollDirection, limit uint32) (lines []*monitor.LogLine, older, newer *monitor.LogCursor, err error)
	// Stream returns a channel of live LogLine. The server MAY coalesce /
	// rate-limit under high throughput. The implementation closes the channel
	// when ctx is done. It MUST honour ctx cancellation promptly.
	Stream(ctx context.Context, runID string, filter *api.LogFilter, from *monitor.LogCursor) (<-chan *monitor.LogLine, error)
	// Resolve translates a LogRef into the concrete filter + anchor cursor used
	// to render it. The ref's run id is authoritative for the run.
	Resolve(ctx context.Context, ref *monitor.LogRef) (filter *api.LogFilter, cursor *monitor.LogCursor, err error)
}

// MetricsReader serves the Metrics tab: the run's aggregated metric summaries.
type MetricsReader interface {
	Get(ctx context.Context, runID string) (*monitor.RunMetrics, error)
}

// TestRunOverviewDeps bundles every dependency for the constructor.
type TestRunOverviewDeps struct {
	Authn    utils.Authn
	Runs     TestRunReader
	Overview OverviewReader
	Logs     LogReader
	Metrics  MetricsReader
	Tx       tx.Trm
}

type TestRunOverviewService struct {
	*api.UnimplementedTestRunOverviewServiceServer
	tx.Trm
	d TestRunOverviewDeps
}

var _ api.TestRunOverviewServiceServer = (*TestRunOverviewService)(nil)

func NewTestRunOverviewService(deps TestRunOverviewDeps) *TestRunOverviewService {
	return &TestRunOverviewService{UnimplementedTestRunOverviewServiceServer: &api.UnimplementedTestRunOverviewServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *TestRunOverviewService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *TestRunOverviewService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO. (This service is read-only; kept for parity with the
// reference package and any future transactional reads.)
func (s *TestRunOverviewService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *TestRunOverviewService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// authorizeRun is the shared per-request gate: reject an unauthenticated caller,
// require a non-empty tenant_id and run_id, and verify the run exists in that
// tenant. It returns a ready-to-return gRPC status error on failure.
func (s *TestRunOverviewService) authorizeRun(ctx context.Context, tenantID, runID string) error {
	if _, err := s.caller(ctx); err != nil {
		return err
	}
	if tenantID == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if runID == "" {
		return status.Error(codes.InvalidArgument, "run_id is required")
	}
	if err := s.d.Runs.Exists(ctx, tenantID, runID); err != nil {
		return utils.MapErr(err)
	}
	return nil
}

/*
	===== Overview tab =====
*/

// GetTestRunOverview returns a one-shot Overview snapshot for a run the caller
// can read in the request tenant. Read-only / no side effects.
func (s *TestRunOverviewService) GetTestRunOverview(ctx context.Context, req *api.GetTestRunOverviewRequest) (*api.GetTestRunOverviewResponse, error) {
	if err := s.authorizeRun(ctx, req.GetTenantId(), req.GetRunId()); err != nil {
		return nil, err
	}
	snap, err := s.d.Overview.Get(ctx, req.GetRunId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetTestRunOverviewResponse{Snapshot: snap}, nil
}

// StreamTestRunOverview pushes a fresh full snapshot on every tick until the
// stream ctx is cancelled or the run is terminal (the source channel closes).
func (s *TestRunOverviewService) StreamTestRunOverview(req *api.StreamTestRunOverviewRequest, stream grpc.ServerStreamingServer[api.TestRunOverviewSnapshot]) error {
	return s.streamOverview(stream.Context(), req.GetTenantId(), req.GetRunId(), stream.Send)
}

/*
	===== Logs tab =====
*/

// QueryLogs returns one cursor-paged page of history plus the cursors to scroll
// OLDER and NEWER. Bounded by limit so the UI never drowns. Read-only.
func (s *TestRunOverviewService) QueryLogs(ctx context.Context, req *api.QueryLogsRequest) (*api.QueryLogsResponse, error) {
	if err := s.authorizeRun(ctx, req.GetTenantId(), req.GetRunId()); err != nil {
		return nil, err
	}
	lines, older, newer, err := s.d.Logs.Query(ctx, req.GetRunId(), req.GetFilter(), req.GetFrom(), req.GetDirection(), req.GetLimit())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.QueryLogsResponse{
		Lines: lines,
		Older: older,
		Newer: newer,
	}, nil
}

// StreamLogs is a bounded live tail (follow). The server MAY coalesce /
// rate-limit; clients use QueryLogs for exact scrollback.
func (s *TestRunOverviewService) StreamLogs(req *api.StreamLogsRequest, stream grpc.ServerStreamingServer[monitor.LogLine]) error {
	return s.streamLogs(stream.Context(), req.GetTenantId(), req.GetRunId(), req.GetFilter(), req.GetFrom(), stream.Send)
}

// ResolveLogRef turns a shareable LogRef (deep-link) into a concrete run +
// filter + anchor cursor, so every user opening the same link lands on the exact
// same place. The ref carries its own run id; the request tenant_id authorizes
// access to that run. Read-only.
func (s *TestRunOverviewService) ResolveLogRef(ctx context.Context, req *api.ResolveLogRefRequest) (*api.ResolveLogRefResponse, error) {
	ref := req.GetRef()
	if ref == nil || ref.GetRunId() == "" {
		return nil, status.Error(codes.InvalidArgument, "ref with a run_id is required")
	}
	if err := s.authorizeRun(ctx, req.GetTenantId(), ref.GetRunId()); err != nil {
		return nil, err
	}
	filter, cursor, err := s.d.Logs.Resolve(ctx, ref)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ResolveLogRefResponse{
		RunId:  ref.GetRunId(),
		Filter: filter,
		Cursor: cursor,
	}, nil
}

/*
	===== Metrics tab =====
*/

// GetRunMetrics returns the run's aggregated metric summaries. Read-only.
func (s *TestRunOverviewService) GetRunMetrics(ctx context.Context, req *api.GetRunMetricsRequest) (*api.GetRunMetricsResponse, error) {
	if err := s.authorizeRun(ctx, req.GetTenantId(), req.GetRunId()); err != nil {
		return nil, err
	}
	m, err := s.d.Metrics.Get(ctx, req.GetRunId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetRunMetricsResponse{Metrics: m}, nil
}

func (s *TestRunOverviewService) streamOverview(ctx context.Context, tenantID, runID string, send func(*api.TestRunOverviewSnapshot) error) error {
	if err := s.authorizeRun(ctx, tenantID, runID); err != nil {
		return err
	}
	ch, err := s.d.Overview.Stream(ctx, runID)
	if err != nil {
		return utils.MapErr(err)
	}
	for {
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		case snap, ok := <-ch:
			if !ok {
				return nil
			}
			if snap == nil {
				continue
			}
			if err := send(snap); err != nil {
				return err
			}
		}
	}
}

func (s *TestRunOverviewService) streamLogs(
	ctx context.Context,
	tenantID string,
	runID string,
	filter *api.LogFilter,
	from *monitor.LogCursor,
	send func(*monitor.LogLine) error,
) error {
	if err := s.authorizeRun(ctx, tenantID, runID); err != nil {
		return err
	}
	ch, err := s.d.Logs.Stream(ctx, runID, filter, from)
	if err != nil {
		return utils.MapErr(err)
	}
	for {
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		case line, ok := <-ch:
			if !ok {
				return nil
			}
			if line == nil {
				continue
			}
			if err := send(line); err != nil {
				return err
			}
		}
	}
}
