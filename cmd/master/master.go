package master

import (
	"log"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/stroppy-nightly"
)

func main() {
	c, err := hatchet.HatchetClient()
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}

	worker, err := c.NewWorker(
		"deployment-worker",
		//hatchet.WithWorkflows(workflows.FirstWorkflow(c)),
		hatchetLib.WithWorkflows(stroppy_nightly.NightlyCloudStroppyFn(c)),
		//hatchet.WithSlots(100),
	)
	if err != nil {
		log.Fatalf("Failed to create Hatchet worker: %v", err)
	}

}
