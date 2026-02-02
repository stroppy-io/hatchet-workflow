package vanilla_test

import (
	"testing"

	"github.com/stroppy-io/hatchet-workflow/pkg/install/testutils"
)

func TestInstallAndConfigure(t *testing.T) {
	src := `package main

import (
	"context"
	"fmt"
	"os"
	"github.com/stroppy-io/hatchet-workflow/pkg/install/postgres/vanilla"
)

func main() {
	ctx := context.Background()
	cfg := vanilla.Config{
		Version: "14",
		Port: 5432,
		Password: "testpassword",
	}
	if err := vanilla.InstallAndConfigure(ctx, cfg); err != nil {
		fmt.Printf("Error installing vanilla: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Vanilla Postgres installed successfully")
}
`
	testutils.RunTestInContainer(t, src, []string{"/bin/bash", "-c", "PGPASSWORD=testpassword sudo -u postgres psql -p 5432 -c 'SELECT 1'"})
}
