package agent

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/agentconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

type LogSink interface {
	Ship(context.Context, []*monitor.LogLine) error
}

type ConnectLogSink struct {
	client agentconnect.AgentLogServiceClient
}

func NewConnectLogSink(serverAddr, token string) *ConnectLogSink {
	serverAddr = strings.TrimRight(serverAddr, "/")
	if serverAddr == "" {
		return nil
	}
	opts := []connect.ClientOption{}
	if token != "" {
		opts = append(opts, connect.WithInterceptors(bearerInterceptor{token: token}))
	}
	return &ConnectLogSink{
		client: agentconnect.NewAgentLogServiceClient(http.DefaultClient, serverAddr, opts...),
	}
}

func (s *ConnectLogSink) Ship(ctx context.Context, lines []*monitor.LogLine) error {
	if s == nil || s.client == nil || len(lines) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stream, err := s.client.ShipLogs(ctx)
	if err != nil {
		return err
	}
	if err := stream.Send(&agentpb.LogBatch{Lines: lines}); err != nil {
		_, _ = stream.CloseAndReceive()
		return err
	}
	_, err = stream.CloseAndReceive()
	return err
}

type bearerInterceptor struct {
	token string
}

func (i bearerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", "Bearer "+i.token)
		return next(ctx, req)
	}
}

func (i bearerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", "Bearer "+i.token)
		return conn
	}
}

func (i bearerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

type logSinkWriter struct {
	ctx     context.Context
	logger  *slog.Logger
	sink    LogSink
	context commandLogContext
	stream  monitor.Stream
	mu      sync.Mutex
	pending string
}

func (w *logSinkWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	text := w.pending + string(p)
	if text == "" {
		return len(p), nil
	}
	complete := strings.HasSuffix(text, "\n")
	parts := strings.Split(text, "\n")
	if complete {
		w.pending = ""
	} else {
		w.pending = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}
	lines := logLinesFromParts(w.context, w.stream, parts)
	if len(lines) == 0 || w.sink == nil {
		return len(p), nil
	}
	if err := w.sink.Ship(w.ctx, lines); err != nil && w.logger != nil {
		w.logger.DebugContext(w.ctx, "ship command logs failed", "err", err)
	}
	return len(p), nil
}

func (w *logSinkWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending == "" {
		return
	}
	lines := logLinesFromParts(w.context, w.stream, []string{w.pending})
	w.pending = ""
	if len(lines) == 0 || w.sink == nil {
		return
	}
	if err := w.sink.Ship(w.ctx, lines); err != nil && w.logger != nil {
		w.logger.DebugContext(w.ctx, "ship command logs failed", "err", err)
	}
}

type streamLogWriter struct {
	writers []io.Writer
}

func (w *streamLogWriter) Write(p []byte) (int, error) {
	for _, writer := range w.writers {
		if _, err := writer.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *streamLogWriter) Flush() {
	if w == nil {
		return
	}
	for _, writer := range w.writers {
		if flushable, ok := writer.(interface{ Flush() }); ok {
			flushable.Flush()
		}
	}
}

type commandLogContext struct {
	runID                 string
	nodeExecutionID       string
	parentNodeExecutionID string
	phase                 string
	stageName             string
	componentID           string
	machineID             string
	stepID                string
	action                string
	mentions              []string
	unit                  string
}

func commandLogContextFromEnv(env map[string]string) commandLogContext {
	unit := env[deploymentbuilder.EnvAction]
	if unit == "" {
		unit = "command"
	}
	machineID := env[deploymentbuilder.EnvNodeID]
	if machineID == "" {
		machineID = env["STROPPY_MACHINE_ID"]
	}
	return commandLogContext{
		runID:                 env[deploymentbuilder.EnvRunID],
		nodeExecutionID:       env[deploymentbuilder.EnvNodeExecutionID],
		parentNodeExecutionID: env[deploymentbuilder.EnvParentNodeExecutionID],
		phase:                 env[deploymentbuilder.EnvPhase],
		stageName:             env[deploymentbuilder.EnvStageName],
		componentID:           env[deploymentbuilder.EnvComponentID],
		machineID:             machineID,
		stepID:                env[deploymentbuilder.EnvStepID],
		action:                env[deploymentbuilder.EnvAction],
		mentions:              splitMentions(env[deploymentbuilder.EnvOperationMentions]),
		unit:                  unit,
	}
}

func logLinesFromChunk(ctx commandLogContext, stream monitor.Stream, chunk []byte) []*monitor.LogLine {
	text := strings.TrimRight(string(chunk), "\n")
	if strings.TrimSpace(text) == "" || ctx.runID == "" {
		return nil
	}
	return logLinesFromParts(ctx, stream, strings.Split(text, "\n"))
}

func logLinesFromParts(ctx commandLogContext, stream monitor.Stream, parts []string) []*monitor.LogLine {
	if ctx.runID == "" {
		return nil
	}
	lines := make([]*monitor.LogLine, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSuffix(part, "\r")
		if strings.TrimSpace(part) == "" {
			continue
		}
		lines = append(lines, &monitor.LogLine{
			ObservedAt:            timestamppb.Now(),
			RunId:                 ctx.runID,
			NodeExecutionId:       ctx.nodeExecutionID,
			ParentNodeExecutionId: ctx.parentNodeExecutionID,
			Phase:                 ctx.phase,
			StageName:             ctx.stageName,
			ComponentId:           ctx.componentID,
			MachineId:             ctx.machineID,
			StepId:                ctx.stepID,
			Action:                ctx.action,
			Mentions:              ctx.mentions,
			Source:                monitor.Source_SOURCE_COMMAND,
			Unit:                  ctx.unit,
			Stream:                stream,
			Line:                  part,
		})
	}
	return lines
}

func splitMentions(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
