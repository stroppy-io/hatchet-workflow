package main

import (
	"context"
	"fmt"
	"log/slog"
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
			hostPort := agentGRPCHostPort(serverAddr)

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			logger.Info("starting stroppy agent",
				"server_addr", serverAddr, "temporal_hostport", hostPort,
				"namespace", namespace, "task_queue", taskQueue)

			clientOptions := temporalclient.Options{
				HostPort:  hostPort,
				Namespace: namespace,
				Logger:    temporallog.NewStructuredLogger(logger),
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

// agentGRPCHostPort strips a URL scheme/path so the Temporal gRPC client gets a
// bare host:port. "http://host:8080" -> "host:8080".
func agentGRPCHostPort(serverAddr string) string {
	if u, err := url.Parse(serverAddr); err == nil && u.Host != "" {
		return u.Host
	}
	return serverAddr
}
