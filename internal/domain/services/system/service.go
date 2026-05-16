package system

import (
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

type Service struct {
	*tracing.Entity

	dagRepo      *repository.ProtoRepository[systempb.DagAlias, systempb.DagColumnAlias, *systempb.DagScanner, *systempb.Dag]
	dagRunRepo   *repository.ProtoRepository[systempb.DagRunAlias, systempb.DagRunColumnAlias, *systempb.DagRunScanner, *systempb.DagRun]
	nodeRunRepo  *repository.ProtoRepository[systempb.NodeRunAlias, systempb.NodeRunColumnAlias, *systempb.NodeRunScanner, *systempb.NodeRun]
	stateRepo    *repository.ProtoRepository[systempb.DagRunStateEntryAlias, systempb.DagRunStateEntryColumnAlias, *systempb.DagRunStateEntryScanner, *systempb.DagRunStateEntry]
	scheduleRepo *repository.ProtoRepository[systempb.ScheduleAlias, systempb.ScheduleColumnAlias, *systempb.ScheduleScanner, *systempb.Schedule]

	db     exec.DB
	txMgr  pgtx.TxManager
	events eventing.Bus
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
		txMgr:  txMgr,
		events: events,
	}
}
