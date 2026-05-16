package system

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// SaveDag inserts a new Dag (assigning Id and Timestamps if absent) inside a
// Serializable transaction. Idempotent on Id: if dag.Id is already set the
// caller is responsible for ensuring uniqueness.
func (s *Service) SaveDag(ctx context.Context, dag *systempb.Dag) (*systempb.Dag, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "SaveDag",
		func(ctx context.Context, _ trace.Span) (*systempb.Dag, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*systempb.Dag, error) {
					now := timestamppb.Now()
					if dag.GetId().GetValue() == "" {
						dag.Id = &systempb.DagId{Value: ids.New()}
					}
					dag.Timestamps = &commonpb.Timestamps{
						CreatedAt: now,
						UpdatedAt: now,
					}
					scanner := dag.IntoPlain()
					if _, err := s.dagRepo.Execute(ctx,
						systempb.Dags.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return dag, nil
				})
		})
}

// GetDag fetches a single Dag by its ID. Returns domainerr.NotFound when the
// row does not exist or has been soft-deleted.
func (s *Service) GetDag(ctx context.Context, id *systempb.DagId) (*systempb.Dag, error) {
	dag, err := s.dagRepo.QueryRow(ctx,
		systempb.Dags.SelectAll().Where(
			systempb.Dags.Id.Eq(id.GetValue()),
			systempb.Dags.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("dag", id.GetValue()))
		}
		return nil, err
	}
	return dag, nil
}

// DeleteDag soft-deletes a Dag by setting deleted_at to now.
func (s *Service) DeleteDag(ctx context.Context, id *systempb.DagId) error {
	now := time.Now()
	n, err := s.dagRepo.Execute(ctx,
		systempb.Dags.Update().
			Set(systempb.Dags.DeletedAt.Set(&now)).
			Where(
				systempb.Dags.Id.Eq(id.GetValue()),
				systempb.Dags.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return domainerr.NotFound(domainerr.ResourceInfo("dag", id.GetValue()))
	}
	return nil
}
