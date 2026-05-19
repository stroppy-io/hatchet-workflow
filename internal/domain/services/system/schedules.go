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

const triggerNowQuery = `
UPDATE schedules
SET next_fire_at     = now(),
    lease_owner      = '',
    lease_expires_at = NULL,
    updated_at       = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, created_at, updated_at, deleted_at,
          cron_expr, hook_name, payload, enabled, catchup, misfire_policy,
          max_misfire_age_seconds, last_fired_at, next_fire_at,
          lease_owner, lease_expires_at, consecutive_failures,
          last_failure_error, metadata
`

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

// UpdateSchedule patches the mutable fields of an existing schedule inside a
// Serializable transaction. Fields patched: CronExpr, HookName, Payload,
// Enabled, Catchup, MisfirePolicy, MaxMisfireAgeSeconds, Metadata.
func (s *Service) UpdateSchedule(ctx context.Context, sched *systempb.Schedule) (*systempb.Schedule, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "UpdateSchedule",
		func(ctx context.Context, _ trace.Span) (*systempb.Schedule, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*systempb.Schedule, error) {
					existing, err := s.GetSchedule(ctx, sched.GetId())
					if err != nil {
						return nil, err
					}
					if sched.CronExpr != "" {
						existing.CronExpr = sched.CronExpr
					}
					if sched.HookName != "" {
						existing.HookName = sched.HookName
					}
					if sched.Payload != nil {
						existing.Payload = sched.Payload
					}
					existing.Enabled = sched.Enabled
					if sched.Catchup != systempb.CatchupMode_CATCHUP_MODE_UNSPECIFIED {
						existing.Catchup = sched.Catchup
					}
					if sched.MisfirePolicy != systempb.MisfirePolicy_MISFIRE_POLICY_UNSPECIFIED {
						existing.MisfirePolicy = sched.MisfirePolicy
					}
					if sched.MaxMisfireAgeSeconds != 0 {
						existing.MaxMisfireAgeSeconds = sched.MaxMisfireAgeSeconds
					}
					if sched.Metadata != nil {
						existing.Metadata = sched.Metadata
					}
					existing.Timestamps.UpdatedAt = timestamppb.Now()
					scanner := existing.IntoPlain()
					if _, err := s.scheduleRepo.Execute(ctx,
						systempb.Schedules.Update().
							Set(
								scanner.GetSetter(systempb.ScheduleColumnCronExpr)(),
								scanner.GetSetter(systempb.ScheduleColumnHookName)(),
								scanner.GetSetter(systempb.ScheduleColumnPayload)(),
								scanner.GetSetter(systempb.ScheduleColumnEnabled)(),
								scanner.GetSetter(systempb.ScheduleColumnCatchup)(),
								scanner.GetSetter(systempb.ScheduleColumnMisfirePolicy)(),
								scanner.GetSetter(systempb.ScheduleColumnMaxMisfireAgeSeconds)(),
								scanner.GetSetter(systempb.ScheduleColumnMetadata)(),
								scanner.GetSetter(systempb.ScheduleColumnUpdatedAt)(),
							).
							Where(
								systempb.Schedules.Id.Eq(existing.GetId().GetValue()),
							),
					); err != nil {
						return nil, err
					}
					return existing, nil
				})
		})
}

// EnableSchedule sets enabled = true on a schedule.
func (s *Service) EnableSchedule(ctx context.Context, id *systempb.ScheduleId) (*systempb.Schedule, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "EnableSchedule",
		func(ctx context.Context, _ trace.Span) (*systempb.Schedule, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*systempb.Schedule, error) {
					now := time.Now()
					n, err := s.scheduleRepo.Execute(ctx,
						systempb.Schedules.Update().
							Set(
								systempb.Schedules.Enabled.Set(true),
								systempb.Schedules.UpdatedAt.Set(now),
							).
							Where(
								systempb.Schedules.Id.Eq(id.GetValue()),
								systempb.Schedules.DeletedAt.IsNull(),
							),
					)
					if err != nil {
						return nil, err
					}
					if n == 0 {
						return nil, domainerr.NotFound(domainerr.ResourceInfo("schedule", id.GetValue()))
					}
					return s.GetSchedule(ctx, id)
				})
		})
}

// DisableSchedule sets enabled = false on a schedule.
func (s *Service) DisableSchedule(ctx context.Context, id *systempb.ScheduleId) (*systempb.Schedule, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "DisableSchedule",
		func(ctx context.Context, _ trace.Span) (*systempb.Schedule, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*systempb.Schedule, error) {
					now := time.Now()
					n, err := s.scheduleRepo.Execute(ctx,
						systempb.Schedules.Update().
							Set(
								systempb.Schedules.Enabled.Set(false),
								systempb.Schedules.UpdatedAt.Set(now),
							).
							Where(
								systempb.Schedules.Id.Eq(id.GetValue()),
								systempb.Schedules.DeletedAt.IsNull(),
							),
					)
					if err != nil {
						return nil, err
					}
					if n == 0 {
						return nil, domainerr.NotFound(domainerr.ResourceInfo("schedule", id.GetValue()))
					}
					return s.GetSchedule(ctx, id)
				})
		})
}

// TriggerNow forces the schedule to fire on the next tick by setting
// next_fire_at = now(), clearing the lease fields. Returns the updated schedule.
func (s *Service) TriggerNow(ctx context.Context, id *systempb.ScheduleId) (*systempb.Schedule, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "TriggerNow",
		func(ctx context.Context, _ trace.Span) (*systempb.Schedule, error) {
			scanner := &systempb.ScheduleScanner{}
			rows, err := s.db.Query(ctx, triggerNowQuery, id.GetValue())
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			found := false
			for rows.Next() {
				found = true
				targets := []any{
					&scanner.Id, &scanner.CreatedAt, &scanner.UpdatedAt, &scanner.DeletedAt,
					&scanner.CronExpr, &scanner.HookName, &scanner.Payload, &scanner.Enabled,
					&scanner.Catchup, &scanner.MisfirePolicy, &scanner.MaxMisfireAgeSeconds,
					&scanner.LastFiredAt, &scanner.NextFireAt,
					&scanner.LeaseOwner, &scanner.LeaseExpiresAt,
					&scanner.ConsecutiveFailures, &scanner.LastFailureError, &scanner.Metadata,
				}
				if err := rows.Scan(targets...); err != nil {
					return nil, err
				}
			}
			if err := rows.Err(); err != nil {
				return nil, err
			}
			if !found {
				return nil, domainerr.NotFound(domainerr.ResourceInfo("schedule", id.GetValue()))
			}
			sched := scanner.IntoPb()
			// Fire the hook synchronously so the caller (UI TriggerNow button,
			// integration test) sees the side effect before the RPC returns.
			// On hook failure we still return the updated schedule — the
			// scheduler-worker tick will retry via recordFailure semantics.
			if s.hookInvoker != nil && sched.GetHookName() != "" {
				_ = s.hookInvoker(ctx, sched.GetHookName(), sched.GetPayload())
			}
			return sched, nil
		})
}
