//go:build e2e

package e2e_test

import (
	"context"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

func TestE2E_Schedule_Create_With_Cron_HookName(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "sched")
	cli := env.WithTenant(tnt.GetValue())
	resp, err := cli.Schedule.CreateSchedule(ctx, connect.NewRequest(&systempb.CreateScheduleRequest{
		Schedule: &systempb.Schedule{
			CronExpr: "*/5 * * * *", HookName: "launch_test_run", Enabled: true,
		},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetId().GetValue())
	require.Equal(t, "*/5 * * * *", resp.Msg.GetCronExpr())
	require.Equal(t, "launch_test_run", resp.Msg.GetHookName())
}

func TestE2E_Schedule_Get_List_Update_EnableDisable_Delete(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "sched-flow")
	cli := env.WithTenant(tnt.GetValue())

	created, err := cli.Schedule.CreateSchedule(ctx, connect.NewRequest(&systempb.CreateScheduleRequest{
		Schedule: &systempb.Schedule{CronExpr: "0 * * * *", HookName: "ping", Enabled: true},
	}))
	require.NoError(t, err)
	id := created.Msg.GetId()

	got, err := cli.Schedule.GetSchedule(ctx, connect.NewRequest(id))
	require.NoError(t, err)
	require.Equal(t, "ping", got.Msg.GetHookName())

	lst, err := cli.Schedule.ListSchedules(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.NotEmpty(t, lst.Msg.GetSchedules())

	upd, err := cli.Schedule.UpdateSchedule(ctx, connect.NewRequest(&systempb.UpdateScheduleRequest{
		Schedule: &systempb.Schedule{Id: id, CronExpr: "30 * * * *", HookName: "ping", Enabled: true},
	}))
	require.NoError(t, err)
	require.Equal(t, "30 * * * *", upd.Msg.GetCronExpr())

	dis, err := cli.Schedule.DisableSchedule(ctx, connect.NewRequest(id))
	require.NoError(t, err)
	require.False(t, dis.Msg.GetEnabled())
	en, err := cli.Schedule.EnableSchedule(ctx, connect.NewRequest(id))
	require.NoError(t, err)
	require.True(t, en.Msg.GetEnabled())

	_, err = cli.Schedule.DeleteSchedule(ctx, connect.NewRequest(id))
	require.NoError(t, err)
}

// TestE2E_Schedule_TriggerNow_Fires_Hook verifies TriggerNow invokes the
// registered hook synchronously and returns only after the hook completes.
// system.Service.WithHookInvoker is wired with a flag-flipping hook so we can
// observe the side effect without a running scheduler worker.
func TestE2E_Schedule_TriggerNow_Fires_Hook(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()

	var fired atomic.Bool
	env.F.System.WithHookInvoker(func(_ context.Context, name string, _ *anypb.Any) error {
		if name == "test_hook" {
			fired.Store(true)
		}
		return nil
	})

	tnt := env.SeedTenant(t, "sched-trig")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.Schedule.CreateSchedule(ctx, connect.NewRequest(&systempb.CreateScheduleRequest{
		Schedule: &systempb.Schedule{CronExpr: "0 * * * *", HookName: "test_hook", Enabled: true},
	}))
	require.NoError(t, err)
	resp, err := cli.Schedule.TriggerNow(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.GetNextFireAt(), "TriggerNow should set next_fire_at = now()")
	require.True(t, fired.Load(), "hook must fire synchronously inside TriggerNow")
}
