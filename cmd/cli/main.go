// Command cli is the stroppy cloud control CLI. The `serve` subcommand boots the
// demo backbone (internal/app): in-memory repos + a Temporal server worker +
// the wizard/run/overview services, and blocks until interrupted.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/stroppy-io/stroppy-cloud/internal/app"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		if err := serve(logger); err != nil {
			logger.Error("serve failed", "error", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

// serve constructs the App from the environment, starts the Temporal worker +
// services, and blocks until SIGINT/SIGTERM, then shuts down cleanly.
func serve(logger *slog.Logger) error {
	cfg := app.LoadConfig()
	// `serve --addr=:8080` overrides the agent gateway listener (the single
	// address agents are told about).
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", cfg.AgentGatewayAddr, "agent gateway listen address")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	cfg.AgentGatewayAddr = *addr
	if cfg.AgentServerAddr == "" {
		cfg.AgentServerAddr = cfg.AgentGatewayAddr
	}
	logger.Info("starting stroppy-cloud serve",
		"temporal_hostport", cfg.TemporalHostPort,
		"namespace", cfg.TemporalNamespace,
		"agent_image", cfg.AgentImage,
		"gateway_addr", cfg.AgentGatewayAddr,
		"agent_server_addr", cfg.AgentServerAddr,
		"grpc_addr", cfg.GRPCAddr,
	)

	a, err := app.New(cfg)
	if err != nil {
		return err
	}
	if err := a.Start(); err != nil {
		return err
	}
	defer a.Close()

	logger.Info("serving; press Ctrl-C to stop")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cli <command>")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  serve   boot the demo backbone (in-memory repos + temporal worker + services)")
}
