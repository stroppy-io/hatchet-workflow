package execution

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/agentconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

type AgentLogIngestService struct {
	writer      *RunLogWriter
	agentTokens AgentTokenVerifier
	log         *slog.Logger
}

var _ agentconnect.AgentLogServiceHandler = (*AgentLogIngestService)(nil)

func NewAgentLogIngestService(monitoringURL, backendToken string, agentTokens AgentTokenVerifier, log *slog.Logger) *AgentLogIngestService {
	if log == nil {
		log = slog.Default()
	}
	s := &AgentLogIngestService{
		writer:      NewRunLogWriter(monitoringURL, backendToken),
		agentTokens: agentTokens,
		log:         log,
	}
	return s
}

func (s *AgentLogIngestService) ShipLogs(ctx context.Context, stream *connect.ClientStream[agentpb.LogBatch]) (*agentpb.ShipLogsAck, error) {
	claims, err := s.authorize(stream.RequestHeader().Get("Authorization"))
	if err != nil {
		return nil, err
	}

	var accepted uint64
	for stream.Receive() {
		batch := stream.Msg()
		lines, err := normalizeAgentLogLines(batch.GetLines(), claims)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if len(lines) > 0 {
			if err := s.writer.Write(ctx, lines); err != nil {
				return nil, connect.NewError(connect.CodeUnavailable, err)
			}
		}
		accepted += uint64(len(lines))
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return &agentpb.ShipLogsAck{Accepted: accepted}, nil
}

func (s *AgentLogIngestService) authorize(header string) (*agentdomain.TokenClaims, error) {
	if s.agentTokens == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("agent token verifier is not configured"))
	}
	token := agentdomain.BearerToken(header)
	if token == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing agent log token"))
	}
	claims, err := s.agentTokens.VerifyAgentToken(token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid agent log token"))
	}
	return claims, nil
}

type RunLogWriter struct {
	client *victoria.LogsClient
}

func NewRunLogWriter(monitoringURL, token string) *RunLogWriter {
	w := &RunLogWriter{}
	if base := strings.TrimRight(monitoringURL, "/"); base != "" {
		w.client = victoria.NewLogsClient(base, token)
	}
	return w
}

func (w *RunLogWriter) Write(ctx context.Context, lines []*monitor.LogLine) error {
	normalized := normalizeLogLines(lines)
	if len(normalized) == 0 || w == nil || w.client == nil {
		return nil
	}
	return w.client.Write(ctx, normalized)
}

func normalizeLogLines(lines []*monitor.LogLine) []*monitor.LogLine {
	now := timestamppb.Now()
	out := make([]*monitor.LogLine, 0, len(lines))
	for _, line := range lines {
		if line == nil || line.GetRunId() == "" || line.GetLine() == "" {
			continue
		}
		clone := *line
		if clone.ObservedAt == nil {
			clone.ObservedAt = now
		}
		if clone.Source == monitor.Source_SOURCE_UNSPECIFIED {
			clone.Source = monitor.Source_SOURCE_COMMAND
		}
		if clone.Stream == monitor.Stream_STREAM_UNSPECIFIED {
			clone.Stream = monitor.Stream_STREAM_STDOUT
		}
		out = append(out, &clone)
	}
	return out
}

func normalizeAgentLogLines(lines []*monitor.LogLine, claims *agentdomain.TokenClaims) ([]*monitor.LogLine, error) {
	if claims == nil {
		return nil, errors.New("agent log claims are required")
	}
	now := timestamppb.Now()
	out := make([]*monitor.LogLine, 0, len(lines))
	for _, line := range lines {
		if line == nil || line.GetLine() == "" {
			continue
		}
		if line.GetRunId() != "" && line.GetRunId() != claims.RunID {
			return nil, fmt.Errorf("log run_id %q does not match agent run_id %q", line.GetRunId(), claims.RunID)
		}
		if line.GetMachineId() != "" && line.GetMachineId() != claims.MachineID {
			return nil, fmt.Errorf("log machine_id %q does not match agent machine_id %q", line.GetMachineId(), claims.MachineID)
		}
		clone := *line
		clone.RunId = claims.RunID
		clone.MachineId = claims.MachineID
		if clone.ObservedAt == nil {
			clone.ObservedAt = now
		}
		if clone.Source == monitor.Source_SOURCE_UNSPECIFIED {
			clone.Source = monitor.Source_SOURCE_COMMAND
		}
		if clone.Stream == monitor.Stream_STREAM_UNSPECIFIED {
			clone.Stream = monitor.Stream_STREAM_STDOUT
		}
		out = append(out, &clone)
	}
	return out, nil
}
