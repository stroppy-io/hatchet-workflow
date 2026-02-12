package main

import (
	"context"
	"log"
	"os"

	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/hatchet-dev/hatchet/pkg/cmdutils"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/build"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	edge2 "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

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

	workerName := os.Getenv(edge2.WorkerNameEnvKey)
	if workerName == "" {
		log.Fatalf("HATCHET_EDGE_WORKER_NAME is not set")
	}
	acceptableTasks := os.Getenv(edge2.WorkerAcceptableTasksEnvKey)
	if acceptableTasks == "" {
		log.Fatalf("HATCHET_EDGE_ACCEPTABLE_TASKS is not set")
	}

	parsedTasksIds, err := edge2.ParseTaskIds(acceptableTasks)
	if err != nil {
		log.Fatalf("Failed to parse acceptable tasks: %v", err)
	}

	tracker, err := edge.NewContainerTracker()
	if err != nil {
		log.Printf("Warning: failed to create container tracker: %v", err)
	}

	tasks := make([]hatchetLib.WorkflowBase, 0)
	for _, task := range parsedTasksIds {
		switch task.GetKind() {
		case hatchet.EdgeTasks_SETUP_SOFTWARE:
			tasks = append(tasks, edge.InstallSoftwareTask(c, task, tracker))
		case hatchet.EdgeTasks_RUN_STROPPY:
			tasks = append(tasks, edge.RunStroppyTask(c, task))
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

	interruptCtx, cancel := cmdutils.NewInterruptContext()
	defer cancel()

	log.Printf("Starting edge worker %s with ID %s", build.ServiceName, build.GlobalInstanceId)
	err = worker.StartBlocking(interruptCtx)
	if err != nil {
		log.Fatalf("Failed to start Hatchet worker: %v", err)
	}

	if tracker != nil {
		log.Printf("Cleaning up containers...")
		tracker.Cleanup(context.Background())
	}
}
