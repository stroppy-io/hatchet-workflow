package main

import (
	"log"
	"os"

	"github.com/hatchet-dev/hatchet/pkg/cmdutils"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/hatchet"
	stroppynightly "github.com/stroppy-io/hatchet-workflow/internal/workflows/stroppy-nightly"
)

const RunIdEnvVar = "RUN_ID"

func main() {
	c, err := hatchet.HatchetClient()
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}

	runId := os.Getenv(RunIdEnvVar)
	if runId == "" {
		log.Fatalf("RUN_ID environment variable is not set")
	}

	worker, err := c.NewWorker(
		"deployment-worker",
		hatchetLib.WithWorkflows(
			stroppynightly.NightlyCloudStroppyRunWorkflow(runId, c),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create Hatchet worker: %v", err)
	}

	interruptCtx, cancel := cmdutils.NewInterruptContext()
	defer cancel()

	err = worker.StartBlocking(interruptCtx)
	if err != nil {
		log.Fatalf("Failed to start Hatchet worker: %v", err)
	}
}
