package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/scheduler"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestSchedulerAdvancesNextFireAt(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()

	past := time.Now().UTC().Add(-time.Minute)
	sched := &systempb.Schedule{
		CronExpr:   "* * * * *",
		HookName:   "stub",
		NextFireAt: timestamppb.New(past),
		Enabled:    true,
	}
	created, err := f.System.CreateSchedule(ctx, sched)
	require.NoError(t, err)

	srv := scheduler.New(f.F.Pool, f.System, scheduler.Config{Tick: 100 * time.Millisecond}, zap.NewNop())
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	go srv.Run(runCtx)

	deadline := time.Now().Add(4 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("scheduler did not advance next_fire_at within 4s")
		}
		got, err := f.System.GetSchedule(ctx, created.GetId())
		require.NoError(t, err)
		if got.GetNextFireAt() != nil && got.GetNextFireAt().AsTime().After(time.Now()) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
