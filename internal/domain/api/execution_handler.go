package api

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"

	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	apiv1 "github.com/stroppy-io/hatchet-workflow/internal/proto/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type executionHandler struct {
	hatchet *hatchetLib.Client
}

func NewExecutionHandler(hatchet *hatchetLib.Client) *executionHandler {
	return &executionHandler{hatchet: hatchet}
}

func (h *executionHandler) StreamWorkflowGraph(ctx context.Context, req *connect.Request[apiv1.StreamWorkflowGraphRequest], stream *connect.ServerStream[apiv1.StreamWorkflowGraphResponse]) error {
	runID := req.Msg.GetRunId()
	if runID == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		details, err := h.hatchet.Runs().Get(ctx, runID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("get run: %w", err))
		}

		graph := buildWorkflowGraph(details)
		if err := stream.Send(&apiv1.StreamWorkflowGraphResponse{Graph: graph}); err != nil {
			return err
		}

		// Stop streaming when workflow is terminal
		switch details.Run.Status {
		case rest.V1TaskStatusCOMPLETED, rest.V1TaskStatusFAILED, rest.V1TaskStatusCANCELLED:
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func buildWorkflowGraph(details *rest.V1WorkflowRunDetails) *apiv1.WorkflowGraph {
	// Build task lookup by step ID
	taskByStep := make(map[string]*rest.V1TaskSummary, len(details.Tasks))
	for i := range details.Tasks {
		task := &details.Tasks[i]
		if task.StepId != nil {
			taskByStep[task.StepId.String()] = task
		}
	}

	var nodes []*apiv1.WorkflowNode
	var edges []*apiv1.WorkflowEdge

	for _, shapeItem := range details.Shape {
		node := &apiv1.WorkflowNode{
			Id:   shapeItem.TaskExternalId.String(),
			Name: shapeItem.TaskName,
		}

		// Find matching task for status/timing
		if task, ok := taskByStep[shapeItem.StepId.String()]; ok {
			node.Status = mapNodeStatus(task.Status)
			if task.StartedAt != nil {
				node.StartedAt = timestamppb.New(*task.StartedAt)
			}
			if task.FinishedAt != nil {
				node.CompletedAt = timestamppb.New(*task.FinishedAt)
			}
			if task.ErrorMessage != nil {
				node.Error = *task.ErrorMessage
			}
		}

		nodes = append(nodes, node)

		// Build edges from children
		for _, childStepID := range shapeItem.ChildrenStepIds {
			// Find the child shape item to get its task external ID
			for _, child := range details.Shape {
				if child.StepId == childStepID {
					edges = append(edges, &apiv1.WorkflowEdge{
						FromNodeId: shapeItem.TaskExternalId.String(),
						ToNodeId:   child.TaskExternalId.String(),
					})
					break
				}
			}
		}
	}

	return &apiv1.WorkflowGraph{
		Nodes:  nodes,
		Edges:  edges,
		Status: mapWorkflowStatus(details.Run.Status),
	}
}

func mapNodeStatus(s rest.V1TaskStatus) apiv1.WorkflowNodeStatus {
	switch s {
	case rest.V1TaskStatusQUEUED:
		return apiv1.WorkflowNodeStatus_WORKFLOW_NODE_STATUS_PENDING
	case rest.V1TaskStatusRUNNING:
		return apiv1.WorkflowNodeStatus_WORKFLOW_NODE_STATUS_RUNNING
	case rest.V1TaskStatusCOMPLETED:
		return apiv1.WorkflowNodeStatus_WORKFLOW_NODE_STATUS_COMPLETED
	case rest.V1TaskStatusFAILED:
		return apiv1.WorkflowNodeStatus_WORKFLOW_NODE_STATUS_FAILED
	case rest.V1TaskStatusCANCELLED:
		return apiv1.WorkflowNodeStatus_WORKFLOW_NODE_STATUS_CANCELLED
	default:
		return apiv1.WorkflowNodeStatus_WORKFLOW_NODE_STATUS_NONE
	}
}

func mapWorkflowStatus(s rest.V1TaskStatus) apiv1.WorkflowStatus {
	switch s {
	case rest.V1TaskStatusQUEUED:
		return apiv1.WorkflowStatus_WORKFLOW_STATUS_PENDING
	case rest.V1TaskStatusRUNNING:
		return apiv1.WorkflowStatus_WORKFLOW_STATUS_RUNNING
	case rest.V1TaskStatusCOMPLETED:
		return apiv1.WorkflowStatus_WORKFLOW_STATUS_COMPLETED
	case rest.V1TaskStatusFAILED:
		return apiv1.WorkflowStatus_WORKFLOW_STATUS_FAILED
	case rest.V1TaskStatusCANCELLED:
		return apiv1.WorkflowStatus_WORKFLOW_STATUS_CANCELLED
	default:
		return apiv1.WorkflowStatus_WORKFLOW_STATUS_NONE
	}
}
