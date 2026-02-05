package main

import (
	"log"

	"github.com/hatchet-dev/hatchet/pkg/cmdutils"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/build"
	"github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/stroppy"
)

func main() {
	c, err := hatchet_ext.HatchetClient()
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}
	provisionWorkflow, err := provision.ProvisionWorkflow(c)
	if err != nil {
		log.Fatalf("Failed to create provision workflow: %v", err)
	}
	worker, err := c.NewWorker(
		"deployment-worker",
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
