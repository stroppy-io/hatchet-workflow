package main

import (
	"log"
	"os"

	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/hatchet-dev/hatchet/pkg/cmdutils"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/build"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/stroppy"
)

func main() {
	token := os.Getenv("HATCHET_CLIENT_TOKEN")
	if token == "" {
		log.Fatalf("HATCHET_CLIENT_TOKEN is not set")
	}
	logger.NewFromEnv()
	c, err := hatchetLib.NewClient(v0Client.WithLogger(logger.Zerolog()))
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}
	provisionWorkflow, err := provision.ProvisionWorkflow(c)
	if err != nil {
		log.Fatalf("Failed to create provision workflow: %v", err)
	}
	worker, err := c.NewWorker(
		"master-worker",
		hatchetLib.WithWorkflows(
			stroppy.TestSuiteWorkflow(c),
			stroppy.TestRunWorkflow(c),
			provisionWorkflow,
		),
	)
	if err != nil {
		log.Fatalf("Failed to create Hatchet worker: %v", err)
	}

	interruptCtx, cancel := cmdutils.NewInterruptContext()
	defer cancel()
	log.Printf("Starting worker %s with ID %s", build.ServiceName, build.GlobalInstanceId)
	err = worker.StartBlocking(interruptCtx)
	if err != nil {
		log.Fatalf("Failed to start Hatchet worker: %v", err)
	}
}
