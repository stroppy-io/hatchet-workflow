package agent

import (
	"context"
	"time"

	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

// CommandsRepo wraps the AgentCommands table with state-machine helpers.
type CommandsRepo struct {
	repo  *repository.ProtoRepository[agentpb.AgentCommandAlias, agentpb.AgentCommandColumnAlias, *agentpb.AgentCommandScanner, *agentpb.AgentCommand]
	txMgr pgtx.TxManager
}

func NewCommandsRepo(executor exec.DB, txMgr pgtx.TxManager) *CommandsRepo {
	return &CommandsRepo{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(agentpb.AgentCommands.Table, executor),
			agentpb.AgentCommandConverter,
		),
		txMgr: txMgr,
	}
}

// Insert persists a new AgentCommand row with state=PENDING.
// id is the hub command ID; pass "" to generate a new one.
func (r *CommandsRepo) Insert(ctx context.Context, id, agentID, nodeRunID string, commandPayload []byte) (*agentpb.AgentCommand, error) {
	if id == "" {
		id = ids.New()
	}
	now := time.Now()
	nowTs := timestamppb.New(now)
	cmd := &agentpb.AgentCommand{
		Id:             &agentpb.AgentCommandId{Value: id},
		AgentId:        &agentpb.AgentId{Value: agentID},
		NodeRunId:      nodeRunID,
		CommandPayload: commandPayload,
		State:          agentpb.AgentCommandState_AGENT_COMMAND_STATE_PENDING,
		Attempt:        0,
		ResultPayload:  []byte{},
		Timestamps: &commonpb.Timestamps{
			CreatedAt: nowTs,
			UpdatedAt: nowTs,
		},
	}
	scanner := cmd.IntoPlain()
	if _, err := r.repo.Execute(ctx, agentpb.AgentCommands.Insert().From(scanner.AllSetters()...)); err != nil {
		return nil, err
	}
	return cmd, nil
}

// MarkDelivered transitions a command to DELIVERED.
func (r *CommandsRepo) MarkDelivered(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.repo.Execute(ctx,
		agentpb.AgentCommands.Update().
			Set(
				agentpb.AgentCommands.State.Set(agentpb.AgentCommandState_AGENT_COMMAND_STATE_DELIVERED.String()),
				agentpb.AgentCommands.DeliveredAt.Set(&now),
				agentpb.AgentCommands.UpdatedAt.Set(now),
			).
			Where(agentpb.AgentCommands.Id.Eq(id)),
	)
	return err
}

// MarkReported transitions a command to REPORTED and stores result_payload.
func (r *CommandsRepo) MarkReported(ctx context.Context, id string, resultPayload []byte) error {
	now := time.Now()
	if resultPayload == nil {
		resultPayload = []byte{}
	}
	_, err := r.repo.Execute(ctx,
		agentpb.AgentCommands.Update().
			Set(
				agentpb.AgentCommands.State.Set(agentpb.AgentCommandState_AGENT_COMMAND_STATE_REPORTED.String()),
				agentpb.AgentCommands.ReportedAt.Set(&now),
				agentpb.AgentCommands.ResultPayload.Set(resultPayload),
				agentpb.AgentCommands.UpdatedAt.Set(now),
			).
			Where(agentpb.AgentCommands.Id.Eq(id)),
	)
	return err
}

// FindByNodeRun returns all agent_commands rows for a node_run_id.
func (r *CommandsRepo) FindByNodeRun(ctx context.Context, nodeRunID string) ([]*agentpb.AgentCommand, error) {
	return r.repo.Query(ctx,
		agentpb.AgentCommands.SelectAll().Where(agentpb.AgentCommands.NodeRunId.Eq(nodeRunID)),
	)
}

// FindByID returns a single AgentCommand by id. Returns pgx.ErrNoRows if absent.
func (r *CommandsRepo) FindByID(ctx context.Context, id string) (*agentpb.AgentCommand, error) {
	return r.repo.QueryRow(ctx,
		agentpb.AgentCommands.SelectAll().Where(agentpb.AgentCommands.Id.Eq(id)),
	)
}

// FindReportedForNodeRun returns the first REPORTED command for a node_run_id.
func (r *CommandsRepo) FindReportedForNodeRun(ctx context.Context, nodeRunID string) (*agentpb.AgentCommand, error) {
	cmds, err := r.FindByNodeRun(ctx, nodeRunID)
	if err != nil {
		return nil, err
	}
	for _, c := range cmds {
		if c.GetState() == agentpb.AgentCommandState_AGENT_COMMAND_STATE_REPORTED {
			return c, nil
		}
	}
	return nil, nil
}

// FindReportedForID returns a REPORTED command row by its ID, or nil if not found or not REPORTED.
func (r *CommandsRepo) FindReportedForID(ctx context.Context, id string) (*agentpb.AgentCommand, error) {
	cmd, err := r.repo.QueryRow(ctx,
		agentpb.AgentCommands.SelectAll().Where(
			agentpb.AgentCommands.Id.Eq(id),
			agentpb.AgentCommands.State.Eq(agentpb.AgentCommandState_AGENT_COMMAND_STATE_REPORTED.String()),
		),
	)
	if err != nil {
		// QueryRow returns pgx.ErrNoRows if not found — treat as nil.
		return nil, nil
	}
	return cmd, nil
}
