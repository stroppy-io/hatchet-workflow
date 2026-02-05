package test

import (
	"errors"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

var (
	ErrUnsupportedDatabaseTopology = errors.New("unsupported database topology")
	ErrClusterNotImplemented       = errors.New("cluster database topology not implemented")
)

const (
	DefaultStroppyInstallPath = "/usr/local/bin/stroppy"
)

const (
	metadataRoleKey = "role"
	metadataTypeKey = "type"
)

func newStroppyWorker(
	runId ids.RunId,
	test *stroppy.Test,
) *hatchet.EdgeWorker {
	return &hatchet.EdgeWorker{
		WorkerName: edge.WorkerName(runId),
		AcceptableTasks: []*hatchet.EdgeTasks_Identifier{
			edge.NewEdgeTaskId(runId, hatchet.EdgeTasks_SETUP_SOFTWARE),
			edge.NewEdgeTaskId(runId, hatchet.EdgeTasks_RUN_STROPPY),
		},
		Hardware: test.GetStroppyHardware(),
		Software: []*hatchet.Software{
			{
				SetupStrategy: hatchet.Software_SETUP_STRATEGY_INSTALL,
				Software: &hatchet.Software_Stroppy{
					Stroppy: &hatchet.StroppyInstallation{
						Version:    test.GetStroppyVersion(),
						BinaryPath: DefaultStroppyInstallPath,
					},
				},
			},
		},
		Metadata: map[string]string{
			metadataRoleKey: "stroppy",
		},
	}
}

var ErrEmptyPostgresInstance = errors.New("empty postgres instance")

func newDatabaseWorkers(
	runId ids.RunId,
	db *database.Database,
) ([]*hatchet.EdgeWorker, error) {
	if db.GetStandalone().GetPostgres() == nil {
		return nil, ErrEmptyPostgresInstance
	}
	err := db.GetStandalone().GetPostgres().Validate()
	if err != nil {
		return nil, err
	}
	switch db.GetTopology().(type) {
	case *database.Database_Standalone:
		return []*hatchet.EdgeWorker{
			{
				WorkerName: edge.WorkerName(runId),
				AcceptableTasks: []*hatchet.EdgeTasks_Identifier{
					edge.NewEdgeTaskId(runId, hatchet.EdgeTasks_SETUP_SOFTWARE),
				},
				Hardware: db.GetStandalone().GetHardware(),
				Software: []*hatchet.Software{
					{
						SetupStrategy: hatchet.Software_SETUP_STRATEGY_INSTALL_AND_START,
						Software: &hatchet.Software_Postgres{
							Postgres: &database.Postgres_Instance{
								Version:  db.GetStandalone().GetPostgres().GetVersion(),
								Settings: db.GetStandalone().GetPostgres().GetSettings(),
							},
						},
					},
				},
				Metadata: map[string]string{
					metadataRoleKey: "database",
					metadataTypeKey: "postgres",
				},
			},
		}, nil
	case *database.Database_Cluster:
		return nil, errors.Join(ErrUnsupportedDatabaseTopology, ErrClusterNotImplemented)
	default:
		return nil, ErrUnsupportedDatabaseTopology
	}
}

func NewTestWorkers(
	runId ids.RunId,
	test *stroppy.Test,
) (*hatchet.EdgeWorkersSet, error) {
	databaseWorkers, err := newDatabaseWorkers(runId, test.GetDatabase())
	if err != nil {
		return nil, err
	}
	return &hatchet.EdgeWorkersSet{
		EdgeWorkers: append(databaseWorkers, newStroppyWorker(runId, test)),
	}, nil

}
