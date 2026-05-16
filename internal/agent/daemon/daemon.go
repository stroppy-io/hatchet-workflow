package daemon

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/agentconnect"
)

const defaultPollInterval = 2 * time.Second

// Config holds all daemon startup parameters.
type Config struct {
	ServerURL      string
	BootstrapToken string
	MachineID      string
	Role           catalogpb.MachineRole
	InternalIP     string
	AgentVersion   string
	StateDir       string
	PollInterval   time.Duration
}

// Daemon connects to the server, registers, and drives the poll loop.
type Daemon struct {
	cfg        Config
	log        *zap.Logger
	dispatcher *Dispatcher
	cache      *reportCache

	// set after registration
	client     agentconnect.AgentServiceClient
	state      *State
}

// New creates a Daemon with the given config and an empty Dispatcher.
func New(cfg Config, log *zap.Logger) *Daemon {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	return &Daemon{
		cfg:        cfg,
		log:        log,
		dispatcher: NewDispatcher(),
		cache:      newReportCache(defaultCacheSize),
	}
}

// Dispatcher returns the daemon's Dispatcher so callers can register verbs.
func (d *Daemon) Dispatcher() *Dispatcher { return d.dispatcher }

// Run performs register-or-resume then the poll loop.
// Blocks until ctx is cancelled, then calls Deregister.
func (d *Daemon) Run(ctx context.Context) error {
	if err := d.init(ctx); err != nil {
		return fmt.Errorf("daemon init: %w", err)
	}

	defer func() {
		deregCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req := connect.NewRequest(&agentpb.AgentId{Value: d.state.AgentID})
		req.Header().Set("Authorization", "Bearer "+d.state.PollToken)
		if _, err := d.client.Deregister(deregCtx, req); err != nil {
			d.log.Warn("deregister failed", zap.Error(err))
		} else {
			d.log.Info("deregistered")
		}
	}()

	return d.pollLoop(ctx)
}

// init loads or creates state, then builds the ConnectRPC client.
func (d *Daemon) init(ctx context.Context) error {
	st, err := LoadState(d.cfg.StateDir)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	d.client = agentconnect.NewAgentServiceClient(http.DefaultClient, d.cfg.ServerURL)

	if st != nil {
		d.state = st
		d.log.Info("resuming with existing agent state",
			zap.String("agent_id", st.AgentID))
		return nil
	}

	return d.register(ctx)
}

// register calls the Register RPC using the bootstrap token and persists state.
func (d *Daemon) register(ctx context.Context) error {
	req := connect.NewRequest(&agentpb.RegisterRequest{
		BootstrapToken: d.cfg.BootstrapToken,
		MachineId:      d.cfg.MachineID,
		Role:           d.cfg.Role,
		InternalIp:     d.cfg.InternalIP,
		AgentVersion:   d.cfg.AgentVersion,
		Capabilities:   []string{"systemd", "apt"},
	})

	resp, err := d.client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	st := &State{
		AgentID:   resp.Msg.GetAgent().GetId().GetValue(),
		PollToken: resp.Msg.GetPollToken(),
		MachineID: d.cfg.MachineID,
	}
	if err := st.Save(d.cfg.StateDir); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	d.state = st
	d.log.Info("registered", zap.String("agent_id", st.AgentID))
	return nil
}

// pollLoop ticks every PollInterval: drain reports, POST Poll, dispatch commands.
func (d *Daemon) pollLoop(ctx context.Context) error {
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			d.poll(ctx)
		}
	}
}

func (d *Daemon) poll(ctx context.Context) {
	agentReport := d.dispatcher.DrainReports()
	agentReport.Heartbeat = &agentpb.Heartbeat{
		MachineId:    d.cfg.MachineID,
		Ts:           timestamppb.Now(),
		AgentVersion: d.cfg.AgentVersion,
		Capabilities: []string{"systemd", "apt"},
	}

	req := connect.NewRequest(&agentpb.PollRequest{
		AgentId: &agentpb.AgentId{Value: d.state.AgentID},
		Report:  agentReport,
	})
	req.Header().Set("Authorization", "Bearer "+d.state.PollToken)

	resp, err := d.client.Poll(ctx, req)
	if err != nil {
		d.log.Warn("poll error", zap.Error(err))
		return
	}

	for _, cmd := range resp.Msg.GetCommands() {
		if cached := d.cache.Lookup(cmd.GetId()); cached != nil {
			// Already executed — requeue the cached report.
			d.dispatcher.QueueReport(cached)
			continue
		}
		go func(c *agentpb.Command) {
			rep := d.dispatcher.Dispatch(ctx, c)
			d.cache.Store(c.GetId(), rep)
		}(cmd)
	}
}
