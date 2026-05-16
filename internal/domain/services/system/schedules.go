package system

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// CreateSchedule inserts a new Schedule (assigning Id and Timestamps) inside a
// Serializable transaction. No tenant_id column exists on the schedules table.
func (s *Service) CreateSchedule(ctx context.Context, sched *systempb.Schedule) (*systempb.Schedule, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateSchedule",
		func(ctx context.Context, _ trace.Span) (*systempb.Schedule, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*systempb.Schedule, error) {
					now := timestamppb.Now()
					if sched.GetId().GetValue() == "" {
						sched.Id = &systempb.ScheduleId{Value: ids.New()}
					}
					sched.Timestamps = &commonpb.Timestamps{
						CreatedAt: now,
						UpdatedAt: now,
					}
					if sched.Payload == nil {
						sched.Payload = &anypb.Any{}
					}
					if sched.Metadata == nil {
						sched.Metadata = &structpb.Struct{Fields: map[string]*structpb.Value{}}
					}
					scanner := sched.IntoPlain()
					if _, err := s.scheduleRepo.Execute(ctx,
						systempb.Schedules.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return sched, nil
				})
		})
}

// GetSchedule fetches a single Schedule by its ID. Returns domainerr.NotFound when the
// row does not exist or has been soft-deleted.
func (s *Service) GetSchedule(ctx context.Context, id *systempb.ScheduleId) (*systempb.Schedule, error) {
	sched, err := s.scheduleRepo.QueryRow(ctx,
		systempb.Schedules.SelectAll().Where(
			systempb.Schedules.Id.Eq(id.GetValue()),
			systempb.Schedules.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("schedule", id.GetValue()))
		}
		return nil, err
	}
	return sched, nil
}

// ListSchedules returns all non-deleted schedules. There is no tenant_id column
// on the schedules table so no tenant filter is applied.
func (s *Service) ListSchedules(ctx context.Context) ([]*systempb.Schedule, error) {
	return s.scheduleRepo.Query(ctx,
		systempb.Schedules.SelectAll().Where(
			systempb.Schedules.DeletedAt.IsNull(),
		),
	)
}

// DeleteSchedule soft-deletes a Schedule by setting deleted_at to now.
func (s *Service) DeleteSchedule(ctx context.Context, id *systempb.ScheduleId) error {
	now := time.Now()
	n, err := s.scheduleRepo.Execute(ctx,
		systempb.Schedules.Update().
			Set(systempb.Schedules.DeletedAt.Set(&now)).
			Where(
				systempb.Schedules.Id.Eq(id.GetValue()),
				systempb.Schedules.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return domainerr.NotFound(domainerr.ResourceInfo("schedule", id.GetValue()))
	}
	return nil
}
