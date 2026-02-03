package main

import (
	"log"
	"os"

	"github.com/hatchet-dev/hatchet/pkg/cmdutils"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/stroppy-nightly"
	valkeygo "github.com/valkey-io/valkey-go"
)

const (
	K8SConfigPath = "K8S_CONFIG_PATH"
	ValkeyUrl     = "VALKEY_URL"
)

func main() {
	k8sConfigPath := os.Getenv(K8SConfigPath)
	if k8sConfigPath == "" {
		log.Fatalf("K8S_CONFIG_PATH environment variable is not set")
	}
	valkeyClient, err := valkeygo.NewClient(valkeygo.MustParseURL(os.Getenv(ValkeyUrl)))
	if err != nil {
		log.Fatalf("Failed to create Valkey client: %v", err)
	}
	c, err := hatchet.HatchetClient()
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}

	provisionWorkflow, err := stroppy_nightly.NightlyCloudStroppyProvisionWorkflow(c, valkeyClient, k8sConfigPath)
	if err != nil {
		log.Fatalf("Failed to create provision workflow: %v", err)
	}

	worker, err := c.NewWorker(
		"deployment-worker",
		hatchetLib.WithWorkflows(provisionWorkflow),
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
