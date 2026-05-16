package system

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// PutState upserts (dag_run_id, key) → value with an optimistic version bump.
// On hit: UPDATE value + version+1 + updated_at.
// On miss: INSERT with version=1.
func (s *Service) PutState(ctx context.Context, dagRunID *systempb.DagRunId, key string, value *anypb.Any) (*systempb.DagRunStateEntry, error) {
	existing, err := s.stateRepo.QueryRow(ctx,
		systempb.DagRunStateEntrys.SelectAll().Where(
			systempb.DagRunStateEntrys.DagRunId.Eq(dagRunID.GetValue()),
			systempb.DagRunStateEntrys.Key.Eq(key),
			systempb.DagRunStateEntrys.DeletedAt.IsNull(),
		),
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	now := time.Now()

	if existing != nil {
		// Update existing entry.
		existing.Value = value
		existing.Version++
		if existing.Timestamps == nil {
			existing.Timestamps = &commonpb.Timestamps{}
		}
		existing.Timestamps.UpdatedAt = timestamppb.New(now)

		sc := existing.IntoPlain()
		if _, err := s.stateRepo.Execute(ctx,
			systempb.DagRunStateEntrys.Update().
				Set(
					sc.GetSetter(systempb.DagRunStateEntryColumnValue)(),
					sc.GetSetter(systempb.DagRunStateEntryColumnVersion)(),
					sc.GetSetter(systempb.DagRunStateEntryColumnUpdatedAt)(),
				).
				Where(
					systempb.DagRunStateEntrys.Id.Eq(existing.GetId().GetValue()),
				),
		); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Insert new entry.
	nowPb := timestamppb.New(now)
	entry := &systempb.DagRunStateEntry{
		Id:       &systempb.DagRunStateEntryId{Value: ids.New()},
		DagRunId: dagRunID,
		Timestamps: &commonpb.Timestamps{
			CreatedAt: nowPb,
			UpdatedAt: nowPb,
		},
		Key:     key,
		Value:   value,
		Version: 1,
	}
	sc := entry.IntoPlain()
	if _, err := s.stateRepo.Execute(ctx,
		systempb.DagRunStateEntrys.Insert().From(sc.AllSetters()...),
	); err != nil {
		return nil, err
	}
	return entry, nil
}

// GetState retrieves the value stored under (dag_run_id, key).
// Returns (value, true, nil) on hit, (nil, false, nil) on miss, (nil, false, err) on error.
func (s *Service) GetState(ctx context.Context, dagRunID *systempb.DagRunId, key string) (*anypb.Any, bool, error) {
	entry, err := s.stateRepo.QueryRow(ctx,
		systempb.DagRunStateEntrys.SelectAll().Where(
			systempb.DagRunStateEntrys.DagRunId.Eq(dagRunID.GetValue()),
			systempb.DagRunStateEntrys.Key.Eq(key),
			systempb.DagRunStateEntrys.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return entry.Value, true, nil
}
