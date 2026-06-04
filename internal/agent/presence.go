package agent

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/agentconnect"
)

const defaultPresenceInterval = 15 * time.Second

type PresenceReporter struct {
	client  agentconnect.AgentRegistryServiceClient
	info    *agentpb.AgentInfo
	log     *slog.Logger
	timeout time.Duration
}

func NewPresenceReporter(serverAddr, token, machineID, runID, agentVersion string, log *slog.Logger) *PresenceReporter {
	serverAddr = strings.TrimRight(serverAddr, "/")
	machineID = strings.TrimSpace(machineID)
	if serverAddr == "" || token == "" || machineID == "" {
		return nil
	}
	if log == nil {
		log = slog.Default()
	}
	host, _ := os.Hostname()
	agentVersion = strings.TrimSpace(agentVersion)
	if agentVersion == "" {
		agentVersion = "dev"
	}
	return &PresenceReporter{
		client: agentconnect.NewAgentRegistryServiceClient(
			http.DefaultClient,
			serverAddr,
			connect.WithInterceptors(bearerInterceptor{token: token}),
		),
		info: &agentpb.AgentInfo{
			MachineId:    machineID,
			Host:         host,
			AgentVersion: agentVersion,
			RunId:        strings.TrimSpace(runID),
		},
		log:     log,
		timeout: 10 * time.Second,
	}
}

func (p *PresenceReporter) Run(ctx context.Context) {
	if p == nil || p.client == nil || p.info == nil {
		return
	}
	interval := defaultPresenceInterval
	registered := false
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if !registered {
			nextInterval, err := p.register(ctx)
			if err != nil {
				p.log.WarnContext(ctx, "agent presence register failed", slog.Any("err", err))
			} else {
				registered = true
				if nextInterval > 0 && nextInterval != interval {
					interval = nextInterval
					ticker.Reset(interval)
				}
			}
		} else if err := p.heartbeat(ctx); err != nil {
			registered = false
			p.log.WarnContext(ctx, "agent presence heartbeat failed", slog.Any("err", err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *PresenceReporter) register(ctx context.Context) (time.Duration, error) {
	callCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	resp, err := p.client.Register(callCtx, &agentpb.RegisterRequest{Info: p.info})
	if err != nil {
		return 0, err
	}
	seconds := resp.GetHeartbeatIntervalSeconds()
	if seconds == 0 {
		return defaultPresenceInterval, nil
	}
	return time.Duration(seconds) * time.Second, nil
}

func (p *PresenceReporter) heartbeat(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	_, err := p.client.Heartbeat(callCtx, &agentpb.HeartbeatRequest{MachineId: p.info.GetMachineId()})
	return err
}
