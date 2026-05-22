package app

import (
	"context"
	"fmt"
	"os"

	"github.com/gopherex/xlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	agentpkg "github.com/stroppy-io/stroppy-cloud/internal/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/build"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// RunAgent dials the control plane and runs the per-machine agent loop
// (internal/agent): register → heartbeat → poll → execute on-host → report, shipping
// command logs back through the server's SendLogs RPC. The server never pushes (D16).
func RunAgent(ctx context.Context, cfg *Config, logger *xlog.Logger) error {
	conn, err := grpc.NewClient(cfg.Agent.ServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(bearerUnaryInterceptor(cfg.Agent.Token)),
	)
	if err != nil {
		return fmt.Errorf("dial %s: %w", cfg.Agent.ServerAddr, err)
	}
	defer conn.Close() //nolint:errcheck

	host, _ := os.Hostname()
	ag := agentpkg.New(logger, grpcAgentClient{c: agentpb.NewAgentServiceClient(conn)}, agentpkg.Config{
		TenantID:     cfg.Agent.TenantID,
		MachineID:    cfg.Agent.MachineID,
		Host:         host,
		Version:      build.Version,
		BootID:       ids.New(),
		PollInterval: cfg.Agent.PollEvery,
	})
	return ag.Run(ctx)
}

// grpcAgentClient adapts the generated AgentServiceClient (variadic grpc.CallOption)
// to the agent.Client interface.
type grpcAgentClient struct{ c agentpb.AgentServiceClient }

func (g grpcAgentClient) Register(ctx context.Context, req *agentpb.RegisterRequest) (*models.Agent, error) {
	return g.c.Register(ctx, req)
}

func (g grpcAgentClient) Heartbeat(ctx context.Context, req *agentpb.HeartbeatRequest) (*models.Agent, error) {
	return g.c.Heartbeat(ctx, req)
}

func (g grpcAgentClient) Poll(ctx context.Context, req *agentpb.PollRequest) (*agentpb.PollResponse, error) {
	return g.c.Poll(ctx, req)
}

func (g grpcAgentClient) Report(ctx context.Context, req *agentpb.ReportRequest) (*emptypb.Empty, error) {
	return g.c.Report(ctx, req)
}

func (g grpcAgentClient) SendLogs(ctx context.Context, req *agentpb.SendLogsRequest) (*emptypb.Empty, error) {
	return g.c.SendLogs(ctx, req)
}

// bearerUnaryInterceptor attaches the agent JWT as Authorization metadata.
func bearerUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
