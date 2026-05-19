package system

import (
	"context"

	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// HookInvoker dispatches a registered hook by name with the schedule's
// payload. Wired by cmd_server to scheduler.HookRegistry.Lookup. TriggerNow
// calls it synchronously so callers (UI / tests) observe the side effect
// before the RPC returns instead of waiting for the next scheduler tick.
type HookInvoker func(ctx context.Context, hookName string, payload *anypb.Any) error

type Service struct {
	*tracing.Entity

	dagRepo      *repository.ProtoRepository[systempb.DagAlias, systempb.DagColumnAlias, *systempb.DagScanner, *systempb.Dag]
	dagRunRepo   *repository.ProtoRepository[systempb.DagRunAlias, systempb.DagRunColumnAlias, *systempb.DagRunScanner, *systempb.DagRun]
	nodeRunRepo  *repository.ProtoRepository[systempb.NodeRunAlias, systempb.NodeRunColumnAlias, *systempb.NodeRunScanner, *systempb.NodeRun]
	stateRepo    *repository.ProtoRepository[systempb.DagRunStateEntryAlias, systempb.DagRunStateEntryColumnAlias, *systempb.DagRunStateEntryScanner, *systempb.DagRunStateEntry]
	scheduleRepo *repository.ProtoRepository[systempb.ScheduleAlias, systempb.ScheduleColumnAlias, *systempb.ScheduleScanner, *systempb.Schedule]
	logRepo      *repository.ProtoRepository[systempb.NodeRunLogAlias, systempb.NodeRunLogColumnAlias, *systempb.NodeRunLogScanner, *systempb.NodeRunLog]

	logBus *LogBus

	db     exec.DB
	txMgr  pgtx.TxManager
	events eventing.Bus

	hookInvoker HookInvoker
}

// WithHookInvoker installs a hook dispatcher. nil = TriggerNow only updates
// next_fire_at and lets the next scheduler-worker tick fire the hook.
func (s *Service) WithHookInvoker(fn HookInvoker) *Service {
	s.hookInvoker = fn
	return s
}

func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *Service {
	return &Service{
		Entity: tracing.NewEntity("system.Service"),
		db:     executor,
		dagRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.Dags.Table, executor),
			systempb.DagConverter,
		),
		dagRunRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.DagRuns.Table, executor),
			systempb.DagRunConverter,
		),
		nodeRunRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.NodeRuns.Table, executor),
			systempb.NodeRunConverter,
		),
		stateRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.DagRunStateEntrys.Table, executor),
			systempb.DagRunStateEntryConverter,
		),
		scheduleRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.Schedules.Table, executor),
			systempb.ScheduleConverter,
		),
		logRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(systempb.NodeRunLogs.Table, executor),
			systempb.NodeRunLogConverter,
		),
		logBus: NewLogBus(),
		txMgr:  txMgr,
		events: events,
	}
}
