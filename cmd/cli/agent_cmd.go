package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	temporalclient "go.temporal.io/sdk/client"
	temporallog "go.temporal.io/sdk/log"
	temporalworker "go.temporal.io/sdk/worker"

	agentworker "github.com/stroppy-io/stroppy-cloud/internal/agent"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// agentCmd runs the node-side Temporal worker. The agent is told ONLY the server
// address (a URL); Temporal is reached through the gateway's transparent gRPC
// proxy at that same address, so dialing the server IS dialing Temporal — strip
// the scheme for the gRPC client. Activities run on a per-machine task queue so
// the workflow can pin a machine's steps to exactly this agent.
func agentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent",
		Short: "Run in agent mode (a Temporal worker reached via the server's gRPC proxy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverAddr := envOrAgent("STROPPY_SERVER_ADDR", "http://127.0.0.1:8080")
			namespace := envOrAgent("TEMPORAL_NAMESPACE", "default")
			agentToken := os.Getenv("STROPPY_AGENT_TOKEN")
			if agentToken == "" {
				return fmt.Errorf("agent: STROPPY_AGENT_TOKEN is required")
			}
			taskQueue := os.Getenv("AGENT_TASK_QUEUE")
			if taskQueue == "" {
				return fmt.Errorf("agent: AGENT_TASK_QUEUE is required")
			}
			machineID := os.Getenv("STROPPY_MACHINE_ID")
			if machineID == "" {
				machineID = os.Getenv("AGENT_MACHINE_ID")
			}
			if machineID == "" {
				return fmt.Errorf("agent: STROPPY_MACHINE_ID is required")
			}
			hostPort, tlsConfig := agentGRPCTarget(serverAddr)

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			logger.Info("starting stroppy agent",
				"server_addr", serverAddr, "temporal_hostport", hostPort,
				"temporal_tls", tlsConfig != nil,
				"namespace", namespace, "task_queue", taskQueue, "machine_id", machineID)

			clientOptions := temporalclient.Options{
				HostPort:  hostPort,
				Namespace: namespace,
				Logger:    temporallog.NewStructuredLogger(logger),
			}
			if tlsConfig != nil {
				clientOptions.ConnectionOptions.TLS = tlsConfig
			}
			clientOptions.HeadersProvider = staticHeadersProvider{
				"authorization": "Bearer " + agentToken,
			}
			c, err := temporalclient.Dial(clientOptions)
			if err != nil {
				return fmt.Errorf("agent: dial temporal: %w", err)
			}
			defer c.Close()

			// EnableSessionWorker lets the orchestrating workflow pin a sequence of
			// activities (write configs, fetch binaries, run the load) to THIS agent.
			w := temporalworker.New(c, taskQueue, temporalworker.Options{EnableSessionWorker: true})
			impl := agentworker.NewActivities(
				agentworker.WithLogger(logger),
				agentworker.WithLogSink(agentworker.NewConnectLogSink(serverAddr, agentToken)),
			)
			workflowpb.RegisterAgentCommandServiceActivities(w, impl)

			agentCtx, cancelAgent := context.WithCancel(cmd.Context())
			defer cancelAgent()
			go agentworker.NewPresenceReporter(
				serverAddr,
				agentToken,
				machineID,
				os.Getenv("STROPPY_RUN_ID"),
				os.Getenv("STROPPY_AGENT_VERSION"),
				logger,
			).Run(agentCtx)

			return w.Run(temporalworker.InterruptCh())
		},
	}
}

type staticHeadersProvider map[string]string

func (p staticHeadersProvider) GetHeaders(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(p))
	for key, value := range p {
		out[key] = value
	}
	return out, nil
}

func envOrAgent(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// agentGRPCTarget strips a URL scheme/path so the Temporal gRPC client gets a
// bare host:port. HTTPS server addresses dial the gateway with TLS through Caddy.
func agentGRPCTarget(serverAddr string) (string, *tls.Config) {
	if u, err := url.Parse(serverAddr); err == nil && u.Host != "" {
		target := u.Host
		host := u.Hostname()
		port := u.Port()
		if port == "" && host != "" {
			switch u.Scheme {
			case "https":
				port = "443"
			case "http":
				port = "80"
			}
			if port != "" {
				target = net.JoinHostPort(host, port)
			}
		}
		if u.Scheme == "https" {
			return target, &tls.Config{ServerName: host}
		}
		return target, nil
	}
	return serverAddr, nil
}
