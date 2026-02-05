package edge

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

const (
	WorkerNamePrefix = "edge-worker-"
)

func WorkerName(id ids.RunId) string {
	return WorkerNamePrefix + id.String()
}

func NewEdgeTaskId(runId ids.RunId, kind hatchet.EdgeTasks_Kind) *hatchet.EdgeTasks_Identifier {
	return &hatchet.EdgeTasks_Identifier{
		RunId:  runId.String(),
		TaskId: ids.NewUlid().Lower().String(),
		Kind:   kind,
	}
}

func EdgeTaskIdToString(task *hatchet.EdgeTasks_Identifier) string {
	return fmt.Sprintf(
		"%s-%s-%s",
		task.RunId,
		strings.ToLower(task.Kind.String()),
		task.TaskId,
	)
}

func ParseEdgeTaskId(s string) (*hatchet.EdgeTasks_Identifier, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid edge task string: %s", s)
	}
	runId, kind, taskId := parts[0], strings.ToUpper(parts[1]), parts[2]
	return &hatchet.EdgeTasks_Identifier{
		RunId:  runId,
		TaskId: taskId,
		Kind:   hatchet.EdgeTasks_Kind(hatchet.EdgeTasks_Kind_value[kind]),
	}, nil
}
