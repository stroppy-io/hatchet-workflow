package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gopherex/xlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/stroppy-io/stroppy-cloud/internal/agent/opexec"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
)

// RunAgent runs the agent poll loop against the control plane: register, then
// Poll → execute the leased command's op locally via opexec → Report, forever.
// This is the production agent (one per machine/VM); the server never executes
// agent-locus nodes itself.
func RunAgent(ctx context.Context, cfg *Config, logger *xlog.Logger) error {
	log := logger.AppendName("agent")
	conn, err := grpc.NewClient(cfg.Agent.ServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(bearerUnaryInterceptor(cfg.Agent.Token)),
	)
	if err != nil {
		return fmt.Errorf("dial %s: %w", cfg.Agent.ServerAddr, err)
	}
	defer conn.Close()

	client := agentpb.NewAgentServiceClient(conn)
	tenantID := &models.TenantId{Value: cfg.Agent.TenantID}
	target := &rtagent.Target{MachineId: cfg.Agent.MachineID}
	host, _ := os.Hostname()
	opx := opexec.New()

	if _, err := client.Register(ctx, &agentpb.RegisterRequest{
		TenantId: tenantID,
		Target:   target,
		Host:     host,
		Version:  "stroppy-cloud",
		BootId:   ids.New(),
	}); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	log.Info("agent registered", xlog.String("machine", cfg.Agent.MachineID))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, perr := client.Poll(ctx, &agentpb.PollRequest{TenantId: tenantID, Target: target, Host: host})
		if perr != nil {
			log.Warn("poll failed", xlog.String("error", perr.Error()))
			sleep(ctx, cfg.Agent.PollEvery)
			continue
		}
		lease := resp.GetLease()
		if lease == nil {
			sleep(ctx, cfg.Agent.PollEvery)
			continue
		}

		cmd := lease.GetCommand()
		log.Info("executing command", xlog.String("command", cmd.GetId()))
		report := executeCommand(ctx, opx, cmd, target)
		if _, rerr := client.Report(ctx, &agentpb.ReportRequest{
			TenantId: tenantID,
			Address:  lease.GetAddress(),
			Report:   report,
		}); rerr != nil {
			log.Warn("report failed", xlog.String("error", rerr.Error()))
		}
	}
}

// executeCommand runs the command's op and maps the outcome to a Report.
func executeCommand(ctx context.Context, opx *opexec.Executor, cmd *rtagent.Command, target *rtagent.Target) *rtagent.Report {
	res, err := opx.Execute(ctx, cmd.GetOperation())
	report := &rtagent.Report{CommandId: cmd.GetId(), Target: target, Result: res}
	switch {
	case err != nil:
		report.Status = rtagent.CommandStatus_COMMAND_STATUS_FAILED
		report.Error = err.Error()
	case res.GetRunCmd() != nil && res.GetRunCmd().GetExitCode() != 0:
		report.Status = rtagent.CommandStatus_COMMAND_STATUS_FAILED
		report.Error = fmt.Sprintf("exit code %d: %s", res.GetRunCmd().GetExitCode(), string(res.GetRunCmd().GetStderr()))
	default:
		report.Status = rtagent.CommandStatus_COMMAND_STATUS_COMPLETED
	}
	return report
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// bearerUnaryInterceptor attaches the agent JWT as Authorization metadata.
func bearerUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
