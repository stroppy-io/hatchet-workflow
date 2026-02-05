package stroppy

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/edge"
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
	DefaultOptStroppyWorkdir  = "/opt/stroppy"
)

const (
	MetadataRoleKey = "role"
	MetadataTypeKey = "type"
)

const (
	WorkerRoleStroppy         = "stroppy"
	WorkerRoleDatabase        = "database"
	WorkerTypePostgresMaster  = "postgres-master"
	WorkerTypePostgresReplica = "postgres-replica"
)

func newStroppyWorker(
	runId ids.RunId,
	test *stroppy.Test,
) *hatchet.EdgeWorker {
	return &hatchet.EdgeWorker{
		WorkerName: edge.WorkerName(runId),
		AcceptableTasks: []*hatchet.EdgeTasks_Identifier{
			edge.NewTaskId(runId, hatchet.EdgeTasks_SETUP_SOFTWARE),
			edge.NewTaskId(runId, hatchet.EdgeTasks_RUN_STROPPY),
		},
		Hardware: test.GetStroppyHardware(),
		Software: []*hatchet.Software{
			{
				SetupStrategy: hatchet.Software_SETUP_STRATEGY_INSTALL,
				Software: &hatchet.Software_Stroppy{
					Stroppy: &stroppy.StroppyCli{
						Version: test.GetStroppyCli().GetVersion(),
						BinaryPath: defaults.StringPtrOrDefaultPtr(
							test.GetStroppyCli().BinaryPath,
							DefaultStroppyInstallPath,
						),
						// NOTE: We dont need to set this vars here cause they doesn't matter for installation
						//Workload:   test.GetStroppyCli().GetWorkload(),
						//StroppyEnv: test.GetStroppyCli().GetStroppyEnv(),
					},
				},
			},
		},
		Metadata: map[string]string{
			MetadataRoleKey: WorkerRoleStroppy,
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
					edge.NewTaskId(runId, hatchet.EdgeTasks_SETUP_SOFTWARE),
				},
				Hardware: db.GetStandalone().GetHardware(),
				Software: []*hatchet.Software{
					{
						SetupStrategy: hatchet.Software_SETUP_STRATEGY_INSTALL_AND_START,
						Software: &hatchet.Software_Postgres{
							Postgres: db.GetStandalone().GetPostgres(),
						},
					},
				},
				Metadata: map[string]string{
					MetadataRoleKey: WorkerRoleDatabase,
					MetadataTypeKey: WorkerTypePostgresMaster,
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

func SelectDeployedEdgeWorker(
	workers []*hatchet.DeployedEdgeWorker,
	metadata map[string]string,
) (*hatchet.DeployedEdgeWorker, error) {
	foundWorker, ok := lo.Find(workers, func(w *hatchet.DeployedEdgeWorker) bool {
		equals := 0
		for key, value := range metadata {
			if w.GetWorker().GetMetadata()[key] == value {
				equals++
			}
		}
		return equals == len(metadata)
	})
	if !ok {
		return nil, fmt.Errorf("failed to find worker with metadata %v", metadata)
	}
	return foundWorker, nil
}

func GetWorkerTask(worker *hatchet.DeployedEdgeWorker, kind hatchet.EdgeTasks_Kind) (*hatchet.EdgeTasks_Identifier, error) {
	task, ok := lo.Find(worker.GetWorker().GetAcceptableTasks(), func(t *hatchet.EdgeTasks_Identifier) bool {
		return t.GetKind() == kind
	})
	if !ok {
		return nil, fmt.Errorf("failed to find task with kind %s", kind.String())
	}
	return task, nil
}
