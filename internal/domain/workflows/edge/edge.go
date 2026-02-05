package edge

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

const (
	WorkerNamePrefix = "edge-worker-"

	WorkerNameEnvKey            = "HATCHET_EDGE_WORKER_NAME"
	WorkerAcceptableTasksEnvKey = "HATCHET_EDGE_ACCEPTABLE_TASKS"
)

func WorkerName(id ids.RunId) string {
	return WorkerNamePrefix + id.String()
}

func NewTaskId(runId ids.RunId, kind hatchet.EdgeTasks_Kind) *hatchet.EdgeTasks_Identifier {
	return &hatchet.EdgeTasks_Identifier{
		RunId:  runId.String(),
		TaskId: ids.NewUlid().Lower().String(),
		Kind:   kind,
	}
}

func TaskIdToString(task *hatchet.EdgeTasks_Identifier) string {
	return fmt.Sprintf(
		"%s-%s-%s",
		task.RunId,
		strings.ToLower(task.Kind.String()),
		task.TaskId,
	)
}

func TaskIdListToString(tasks []*hatchet.EdgeTasks_Identifier) string {
	var result []string
	for _, task := range tasks {
		result = append(result, TaskIdToString(task))
	}
	return strings.Join(result, ";")
}

func ParseTaskId(s string) (*hatchet.EdgeTasks_Identifier, error) {
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

func ParseTaskIds(s string) ([]*hatchet.EdgeTasks_Identifier, error) {
	if s == "" {
		return nil, nil
	}
	var result []*hatchet.EdgeTasks_Identifier
	for _, task := range strings.Split(s, ";") {
		id, err := ParseTaskId(task)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}
