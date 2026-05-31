// Command agent is the stroppy cloud agent. It runs inside a deployed container
// (which emulates a cloud VM 1:1), connects to Temporal as a worker, and
// executes the AgentCommandService activities locally on the host.
package main

import (
	"log/slog"
	"net/url"
	"os"

	"go.temporal.io/sdk/client"
	temporallog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"

	"github.com/stroppy-io/stroppy-cloud/internal/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// grpcHostPort strips a URL scheme (and any path) from the server address so the
// Temporal gRPC client gets a bare host:port. "http://host:8080" -> "host:8080";
// a value already in host:port form is returned unchanged.
func grpcHostPort(serverAddr string) string {
	if u, err := url.Parse(serverAddr); err == nil && u.Host != "" {
		return u.Host
	}
	return serverAddr
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// The agent is told ONLY the server address (a full URL like
	// http://host:port). Temporal is reached through the gateway's transparent
	// gRPC proxy at that same address, so dialing the server IS dialing Temporal —
	// but the gRPC client wants a bare host:port, so strip the scheme. Namespace +
	// task queue default locally.
	var (
		serverAddr = getenv("STROPPY_SERVER_ADDR", "http://127.0.0.1:8080")
		namespace  = getenv("TEMPORAL_NAMESPACE", "default")
		taskQueue  = getenv("AGENT_TASK_QUEUE", "stroppy-agent")
	)
	hostPort := grpcHostPort(serverAddr)

	logger.Info("starting stroppy agent",
		"server_addr", serverAddr,
		"temporal_hostport", hostPort,
		"namespace", namespace,
		"task_queue", taskQueue,
	)

	c, err := client.Dial(client.Options{
		HostPort:  hostPort,
		Namespace: namespace,
		Logger:    temporallog.NewStructuredLogger(logger),
	})
	if err != nil {
		logger.Error("failed to dial temporal", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	// EnableSessionWorker lets the orchestrating TestWorkflow pin a sequence of
	// activities (create temp dir, fetch the stroppy binary, write configs, run
	// the load) to THIS one agent worker via a Temporal worker session.
	w := worker.New(c, taskQueue, worker.Options{
		EnableSessionWorker: true,
	})

	impl := agent.NewActivities(agent.WithLogger(logger))
	workflow.RegisterAgentCommandServiceActivities(w, impl)

	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}
