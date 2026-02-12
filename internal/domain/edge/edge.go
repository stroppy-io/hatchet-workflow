package edge

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
)

const (
	WorkerNamePrefix = "edge-wrk"

	WorkerNameEnvKey            = "HATCHET_EDGE_WORKER_NAME"
	WorkerAcceptableTasksEnvKey = "HATCHET_EDGE_ACCEPTABLE_TASKS"
)

func NewWorkerName(id ids.RunId, role string) string {
	return fmt.Sprintf(
		"%s-%s-%s",
		WorkerNamePrefix,
		role,
		id.String(),
	)
}

func NewTaskId(runId ids.RunId, kind edge.Task_Kind) *edge.Task_Identifier {
	return &edge.Task_Identifier{
		RunId:  runId.String(),
		TaskId: ids.NewUlid().Lower().String(),
		Kind:   kind,
	}
}

func TaskIdToString(task *edge.Task_Identifier) string {
	return fmt.Sprintf(
		"%s-%s-%s",
		task.RunId,
		strings.ToLower(task.Kind.String()),
		task.TaskId,
	)
}

func TaskIdListToString(tasks []*edge.Task_Identifier) string {
	var result []string
	for _, task := range tasks {
		result = append(result, TaskIdToString(task))
	}
	return strings.Join(result, ";")
}

func ParseTaskId(s string) (*edge.Task_Identifier, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid edge task string: %s", s)
	}
	runId, kind, taskId := parts[0], strings.ToUpper(parts[1]), parts[2]
	return &edge.Task_Identifier{
		RunId:  runId,
		TaskId: taskId,
		Kind:   edge.Task_Kind(edge.Task_Kind_value[kind]),
	}, nil
}

func ParseTaskIds(s string) ([]*edge.Task_Identifier, error) {
	if s == "" {
		return nil, nil
	}
	var result []*edge.Task_Identifier
	for _, task := range strings.Split(s, ";") {
		id, err := ParseTaskId(task)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}
