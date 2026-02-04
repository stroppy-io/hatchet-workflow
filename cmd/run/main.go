package main

import (
	"context"
	"fmt"
	"log"

	hatchetLib "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

func main() {
	c, err := hatchetLib.HatchetClient()
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}

	result, err := c.Run(
		context.Background(),
		"nightly-cloud-stroppy",
		&hatchet.NightlyCloudStroppyRequest{
			Cloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
			PostgresVm: &crossplane.MachineInfo{
				Cores:       2,
				Memory:      2,
				Disk:        10,
				BaseImageId: "fd82pkek8uu0ejjkh4vn",
			},
			PostgresVersion:  "17",
			PostgresSettings: map[string]string{},
			StroppyVm: &crossplane.MachineInfo{
				Cores:       2,
				Memory:      2,
				Disk:        10,
				BaseImageId: "fd82pkek8uu0ejjkh4vn",
			},
			StroppyVersion:      "v2.0.0",
			StroppyWorkloadName: "tpcc",
			StroppyEnv:          map[string]string{},
		},
	)
	if err != nil {
		log.Fatalf("Failed to run Hatchet workflow: %v", err)
	}

	fmt.Println(result)
}
