package system

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
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
							NodeId:  node.GetId(),
							Status:  status,
							Attempt: 0,
						}

						nodeScanner := nodeRun.IntoPlain()
						if _, err := s.nodeRunRepo.Execute(ctx,
							systempb.NodeRuns.Insert().From(nodeScanner.AllSetters()...),
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

// CancelDagRun signals cancellation for a running DagRun.
//
// TODO: requires DagRun.cancel_requested column (proto change). Workers will
// check this field once it is added. Until then the method is a stub.
func (s *Service) CancelDagRun(_ context.Context, _ *systempb.DagRunId) error {
	return errors.New("cancel not implemented")
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
