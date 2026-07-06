package workflows

import (
	"strings"

	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

const stageUpdateStatusPrefix = "workflow_stage_"

func rootStage(name string, order uint32) *workflowpb.Stage {
	return &workflowpb.Stage{
		NodeExecutionId: deploymentbuilder.StageExecutionID(name),
		Name:            name,
		Status:          common.Status_STATUS_PENDING,
		Attempt:         1,
		Order:           order,
		Phase:           name,
		Worker:          masterWorker(),
		StatusReason:    stageStatusReason(common.Status_STATUS_PENDING),
	}
}

func startRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_RUNNING
	if stage.StartedAt == nil {
		stage.StartedAt = timestamppb.New(workflow.Now(ctx))
	}
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func completeRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_COMPLETED
	stage.FinishedAt = timestamppb.New(workflow.Now(ctx))
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func failRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_FAILED
	stage.FinishedAt = timestamppb.New(workflow.Now(ctx))
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func cancelRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_CANCELLED
	stage.FinishedAt = timestamppb.New(workflow.Now(ctx))
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func stageStatusReason(status common.Status) string {
	if status == common.Status_STATUS_UNSPECIFIED {
		status = common.Status_STATUS_PENDING
	}
	return stageUpdateStatusPrefix + strings.ToLower(status.String())
}

func masterWorker() *domainpb.Worker {
	return &domainpb.Worker{Id: "master", Kind: domainpb.Worker_KIND_MASTER}
}
