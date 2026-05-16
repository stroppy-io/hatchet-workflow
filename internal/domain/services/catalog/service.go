package catalog

import (
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

type Service struct {
	*tracing.Entity

	dbPresetRepo       *repository.ProtoRepository[catalogpb.DatabasePresetAlias, catalogpb.DatabasePresetColumnAlias, *catalogpb.DatabasePresetScanner, *catalogpb.DatabasePreset]
	workloadPresetRepo *repository.ProtoRepository[catalogpb.WorkloadPresetAlias, catalogpb.WorkloadPresetColumnAlias, *catalogpb.WorkloadPresetScanner, *catalogpb.WorkloadPreset]
	packageRepo        *repository.ProtoRepository[catalogpb.PackageAlias, catalogpb.PackageColumnAlias, *catalogpb.PackageScanner, *catalogpb.Package]
	settingsRepo       *repository.ProtoRepository[catalogpb.SettingsItemAlias, catalogpb.SettingsItemColumnAlias, *catalogpb.SettingsItemScanner, *catalogpb.SettingsItem]

	pkgStorage PackageStorage

	txMgr  pgtx.TxManager
	events eventing.Bus
}

func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *Service {
	return &Service{
		Entity: tracing.NewEntity("catalog.Service"),
		dbPresetRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.DatabasePresets.Table, executor),
			catalogpb.DatabasePresetConverter,
		),
		workloadPresetRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.WorkloadPresets.Table, executor),
			catalogpb.WorkloadPresetConverter,
		),
		packageRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.Packages.Table, executor),
			catalogpb.PackageConverter,
		),
		settingsRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.SettingsItems.Table, executor),
			catalogpb.SettingsItemConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}
