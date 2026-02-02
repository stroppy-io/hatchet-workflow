package oriole_test

import (
	"testing"

	"github.com/stroppy-io/hatchet-workflow/pkg/install/testutils"
)

func TestInstallAndConfigure(t *testing.T) {
	t.Skip("Skipping OrioleDB test: OrioleDB is not available as a direct apt package and requires a different installation strategy.")
	src := `package main

import (
	"context"
	"fmt"
	"os"
	"github.com/stroppy-io/hatchet-workflow/pkg/install/postgres/oriole"
)

func main() {
	ctx := context.Background()
	cfg := oriole.Config{
		Version: "14",
		Port: 5433,
		Password: "testpassword",
	}
	if err := oriole.InstallAndConfigure(ctx, cfg); err != nil {
		fmt.Printf("Error installing oriole: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OrioleDB installed successfully")
}
`
	testutils.RunTestInContainer(t, src, []string{"/bin/bash", "-c", "PGPASSWORD=testpassword sudo -u postgres psql -p 5433 -c 'SELECT 1'"})
}
