package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/gopherex/xlog"
	"github.com/spf13/cobra"

	"github.com/stroppy-io/stroppy-cloud/internal/app"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the control plane: grpc API + dag processor (config from env)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			logger := xlog.Default()
			cfg := app.LoadConfig()

			srv, err := app.BuildServer(ctx, cfg, logger)
			if err != nil {
				return err
			}
			defer srv.Close()
			return srv.Run(ctx)
		},
	}
}

func agentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent",
		Short: "Run the per-machine agent: register + poll/execute/report (config from env)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			logger := xlog.Default()
			cfg := app.LoadConfig()
			if err := app.RunAgent(ctx, cfg, logger); err != nil && err != context.Canceled {
				return err
			}
			return nil
		},
	}
}
