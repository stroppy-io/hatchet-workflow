package edge

import (
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

type workersGetter interface {
	GetWorker() *hatchet.EdgeWorker
}

func GetWorkersName[T workersGetter](container []T) []string {
	names := make([]string, 0)
	for i, worker := range container {
		names[i] = worker.GetWorker().GetWorkerName()
	}
	return names
}
