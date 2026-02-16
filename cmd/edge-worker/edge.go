package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/build"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/core/shutdown"
	domainEdge "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/edge/containers"
	workflowsEdge "github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
)

// TODO: Add health check endpoint to validate container health

func makeQuitSignal() chan os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	return quit
}

func main() {
	token := os.Getenv("HATCHET_CLIENT_TOKEN")
	if token == "" {
		log.Fatalf("HATCHET_CLIENT_TOKEN is not set")
	}
	serverUrl := os.Getenv("HATCHET_CLIENT_SERVER_URL")
	hostPort := os.Getenv("HATCHET_CLIENT_HOST_PORT")
	if serverUrl == "" && hostPort == "" {
		log.Fatalf("HATCHET_CLIENT_SERVER_URL or HATCHET_CLIENT_HOST_PORT is not set")
	}
	if serverUrl != "" {
		log.Printf("Using HATCHET_CLIENT_SERVER_URL=%s", serverUrl)
	} else {
		log.Printf("Using HATCHET_CLIENT_HOST_PORT=%s", hostPort)
	}
	logger.NewFromEnv()
	c, err := hatchetLib.NewClient(v0Client.WithLogger(logger.Zerolog()))
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}

	workerName := os.Getenv(domainEdge.WorkerNameEnvKey)
	if workerName == "" {
		log.Fatalf("HATCHET_EDGE_WORKER_NAME is not set")
	}
	acceptableTasks := os.Getenv(domainEdge.WorkerAcceptableTasksEnvKey)
	if acceptableTasks == "" {
		log.Fatalf("HATCHET_EDGE_ACCEPTABLE_TASKS is not set")
	}
	parsedTasksIds, err := domainEdge.ParseTaskIds(acceptableTasks)
	if err != nil {
		log.Fatalf("Failed to parse acceptable tasks: %v", err)
	}
	for _, task := range parsedTasksIds {
		log.Printf("Acceptable task: %s", domainEdge.TaskIdToString(task))
	}
	runIDs := make([]string, 0, len(parsedTasksIds))
	seenRunIDs := make(map[string]struct{}, len(parsedTasksIds))
	for _, task := range parsedTasksIds {
		runID := task.GetRunId()
		if runID == "" {
			continue
		}
		if _, ok := seenRunIDs[runID]; ok {
			continue
		}
		seenRunIDs[runID] = struct{}{}
		runIDs = append(runIDs, runID)
	}

	tasks := make([]hatchetLib.WorkflowBase, 0)
	for _, task := range parsedTasksIds {
		switch task.GetKind() {
		case edge.Task_KIND_INSTALL_STROPPY:
			tasks = append(tasks, workflowsEdge.InstallStroppy(c, task))
		case edge.Task_KIND_RUN_STROPPY:
			tasks = append(tasks, workflowsEdge.RunStroppyTask(c, task))
		case edge.Task_KIND_SETUP_CONTAINERS:
			tasks = append(tasks, workflowsEdge.SetupContainersTask(c, task))
		default:
			log.Fatalf("Unexpected task kind: %s", task.GetKind().String())
		}
	}
	worker, err := c.NewWorker(
		workerName,
		hatchetLib.WithWorkflows(tasks...),
	)
	if err != nil {
		log.Fatalf("Failed to create Hatchet worker: %v", err)
	}

	log.Printf("Starting edge worker %s with ID %s", build.ServiceName, build.GlobalInstanceId)
	cancelWrk, err := worker.Start()
	if err != nil {
		log.Fatalf("Failed to start Hatchet worker: %v", err)
	}
	shutdown.RegisterFn(func() {
		log.Printf("Stopping edge worker %s with ID %s", build.ServiceName, build.GlobalInstanceId)
		if err := cancelWrk(); err != nil {
			log.Printf("Failed to stop Hatchet worker: %v", err)
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if cleanupErr := containers.Cleanup(cleanupCtx); cleanupErr != nil {
			log.Printf("Failed to cleanup tracked containers: %v", cleanupErr)
		}
	})
	shutdown.WaitSignal(makeQuitSignal())
}
