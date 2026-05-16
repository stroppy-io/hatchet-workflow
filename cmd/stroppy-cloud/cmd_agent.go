package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/agent/daemon"
	"github.com/stroppy-io/stroppy-cloud/internal/agent/daemon/verbs"
	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func agentCmd() *cobra.Command {
	var (
		serverURL      string
		bootstrapToken string
		machineID      string
		stateDir       string
		roleStr        string
		agentVersion   string
		pollInterval   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run the remote-host agent daemon",
		RunE: func(c *cobra.Command, _ []string) error {
			return runAgent(c.Context(), daemon.Config{
				ServerURL:      serverURL,
				BootstrapToken: bootstrapToken,
				MachineID:      machineID,
				Role:           parseMachineRole(roleStr),
				StateDir:       stateDir,
				AgentVersion:   agentVersion,
				PollInterval:   pollInterval,
			})
		},
	}

	// Flags with env-var fallbacks.
	cmd.Flags().StringVar(&serverURL, "server-url", envOr("STROPPY_AGENT_SERVER_URL", ""), "server base URL")
	cmd.Flags().StringVar(&bootstrapToken, "bootstrap-token", envOr("STROPPY_AGENT_BOOTSTRAP_TOKEN", ""), "one-shot bootstrap token")
	cmd.Flags().StringVar(&machineID, "machine-id", envOr("STROPPY_AGENT_MACHINE_ID", ""), "stable machine identifier")
	cmd.Flags().StringVar(&stateDir, "state-dir", envOr("STROPPY_AGENT_STATE_DIR", "/var/lib/stroppy-agent"), "directory for persisted agent state")
	cmd.Flags().StringVar(&roleStr, "role", envOr("STROPPY_AGENT_ROLE", ""), "machine role (e.g. MACHINE_ROLE_DATABASE)")
	cmd.Flags().StringVar(&agentVersion, "agent-version", envOr("STROPPY_AGENT_VERSION", "dev"), "agent version string")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 2*time.Second, "poll interval")

	return cmd
}

func runAgent(ctx context.Context, cfg daemon.Config) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("--server-url / STROPPY_AGENT_SERVER_URL is required")
	}
	if cfg.MachineID == "" {
		return fmt.Errorf("--machine-id / STROPPY_AGENT_MACHINE_ID is required")
	}

	zlog := logger.NewFromConfig(&logger.Config{
		LogMod:   logger.DevelopmentMod,
		LogLevel: "info",
	})
	defer zlog.Sync() //nolint:errcheck

	d := daemon.New(cfg, zlog)
	disp := d.Dispatcher()

	// Register all 5 verbs.
	disp.Register("PutFile", verbs.PutFile)
	disp.Register("RunShell", verbs.RunShell)
	disp.Register("Systemctl", verbs.Systemctl)
	disp.Register("InstallPackage", verbs.InstallPackage)
	disp.Register("WaitFor", verbs.WaitFor)

	stopCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	zlog.Info("starting agent",
		zap.String("server_url", cfg.ServerURL),
		zap.String("machine_id", cfg.MachineID),
		zap.String("state_dir", cfg.StateDir),
	)

	if err := d.Run(stopCtx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseMachineRole(s string) catalogpb.MachineRole {
	if s == "" {
		return catalogpb.MachineRole_MACHINE_ROLE_UNSPECIFIED
	}
	if v, ok := catalogpb.MachineRole_value[s]; ok {
		return catalogpb.MachineRole(v)
	}
	return catalogpb.MachineRole_MACHINE_ROLE_UNSPECIFIED
}
