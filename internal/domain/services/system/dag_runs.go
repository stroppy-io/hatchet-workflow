package system

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/dml/set"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// StartDagRunInput carries the inputs required to launch a new DagRun.
type StartDagRunInput struct {
	Dag      *systempb.Dag
	Metadata map[string]string
}

// StartDagRun creates a DagRun row (status=RUNNING) and one NodeRun per
// node in Dag.Graph inside a single Serializable transaction.
// Nodes without deps get status=READY; all others get PENDING_DEPS.
func (s *Service) StartDagRun(ctx context.Context, in StartDagRunInput) (*systempb.DagRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "StartDagRun",
		func(ctx context.Context, _ trace.Span) (*systempb.DagRun, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*systempb.DagRun, error) {
					now := timestamppb.Now()
					startedAt := now.AsTime()

					run := &systempb.DagRun{
						Id: &systempb.DagRunId{Value: ids.New()},
						Timestamps: &commonpb.Timestamps{
							CreatedAt: now,
							UpdatedAt: now,
						},
						DagId:     in.Dag.GetId(),
						Status:    systempb.DagRunStatus_DAG_RUN_STATUS_RUNNING,
						Attempt:   1,
						StartedAt: timestamppb.New(startedAt),
						Metadata:  serializeMetadata(in.Metadata),
					}

					runScanner := run.IntoPlain()
					if _, err := s.dagRunRepo.Execute(ctx,
						systempb.DagRuns.Insert().From(runScanner.AllSetters()...),
					); err != nil {
						return nil, err
					}

					// Create one NodeRun per node in the graph.
					for _, node := range in.Dag.GetGraph().GetNodes() {
						var status systempb.NodeRunStatus
						if len(node.GetDeps()) == 0 {
							status = systempb.NodeRunStatus_NODE_RUN_STATUS_READY
						} else {
							status = systempb.NodeRunStatus_NODE_RUN_STATUS_PENDING_DEPS
						}

						nodeRun := &systempb.NodeRun{
							Id:       &systempb.NodeRunId{Value: ids.New()},
							DagRunId: run.GetId(),
							Timestamps: &commonpb.Timestamps{
								CreatedAt: now,
								UpdatedAt: now,
							},
							NodeId:   node.GetId(),
							Status:   status,
							Attempt:  0,
							Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{}},
						}

						nodeScanner := nodeRun.IntoPlain()
						// Build setters manually to avoid passing *anypb.Any (nil) for the
						// text NOT NULL output column — use empty string as default value.
						nodeSetters := nodeRunSettersWithOutput(nodeScanner, "")
						if _, err := s.nodeRunRepo.Execute(ctx,
							systempb.NodeRuns.Insert().From(nodeSetters...),
						); err != nil {
							return nil, err
						}
					}

					return run, nil
				})
		})
}

// GetDagRun fetches a DagRun by ID. Returns domainerr.NotFound when the row
// does not exist or has been soft-deleted.
func (s *Service) GetDagRun(ctx context.Context, id *systempb.DagRunId) (*systempb.DagRun, error) {
	run, err := s.dagRunRepo.QueryRow(ctx,
		systempb.DagRuns.SelectAll().Where(
			systempb.DagRuns.Id.Eq(id.GetValue()),
			systempb.DagRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("dag_run", id.GetValue()))
		}
		return nil, err
	}
	return run, nil
}

// CancelDagRun sets cancel_requested=true on the DagRun.
// Returns domainerr.NotFound if the row does not exist or is soft-deleted.
func (s *Service) CancelDagRun(ctx context.Context, id *systempb.DagRunId) error {
	return tracing.WithTraceErr(s.Tracer(), ctx, "CancelDagRun",
		func(ctx context.Context, _ trace.Span) error {
			return pgtx.WithSerializable(ctx, s.txMgr,
				func(ctx context.Context) error {
					now := time.Now()
					n, err := s.dagRunRepo.Execute(ctx,
						systempb.DagRuns.Update().
							Set(
								systempb.DagRuns.CancelRequested.Set(true),
								systempb.DagRuns.UpdatedAt.Set(now),
							).
							Where(
								systempb.DagRuns.Id.Eq(id.GetValue()),
								systempb.DagRuns.DeletedAt.IsNull(),
							),
					)
					if err != nil {
						return err
					}
					if n == 0 {
						return domainerr.NotFound(domainerr.ResourceInfo("dag_run", id.GetValue()))
					}
					return nil
				})
		})
}

// serializeMetadata converts a flat map[string]string into a *structpb.Struct
// suitable for storage in the metadata JSON column.
// A nil or empty map produces a non-nil empty Struct so the column default is
// satisfied.
func serializeMetadata(m map[string]string) *structpb.Struct {
	fields := make(map[string]*structpb.Value, len(m))
	for k, v := range m {
		fields[k] = structpb.NewStringValue(v)
	}
	return &structpb.Struct{Fields: fields}
}

// nodeRunSettersWithOutput returns all NodeRun setters with the output column
// overridden to use a plain string value. The generated AllSetters() passes
// *anypb.Any directly which pgx cannot encode into the text NOT NULL column
// when the pointer is nil. Callers that know the output string representation
// (e.g. "" for a new row, or a JSON-encoded Any for a completed row) should
// use this helper instead of AllSetters().
func nodeRunSettersWithOutput(sc *systempb.NodeRunScanner, output string) []set.ValueSetter[systempb.NodeRunColumnAlias] {
	return []set.ValueSetter[systempb.NodeRunColumnAlias]{
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnId, sc.Id),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnDagRunId, sc.DagRunId),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnCreatedAt, sc.CreatedAt),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnUpdatedAt, sc.UpdatedAt),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnDeletedAt, sc.DeletedAt),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnNodeId, sc.NodeId),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnStatus, sc.Status),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnAttempt, sc.Attempt),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnError, sc.Error),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnStartedAt, sc.StartedAt),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnFinishedAt, sc.FinishedAt),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnOutput, output),
		set.NewSetter[systempb.NodeRunColumnAlias](systempb.NodeRunColumnMetadata, sc.Metadata),
	}
}
