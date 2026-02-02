package stroppy_test

import (
	"testing"

	"github.com/stroppy-io/hatchet-workflow/pkg/install/testutils"
)

func TestInstall(t *testing.T) {
	src := `package main

import (
	"context"
	"fmt"
	"os"
	"github.com/stroppy-io/hatchet-workflow/pkg/install/stroppy"
)

func main() {
	ctx := context.Background()
	cfg := stroppy.Config{
		Version: "v2.0.0",
		InstallPath: "/usr/local/bin",
	}
	if err := stroppy.Install(ctx, cfg); err != nil {
		fmt.Printf("Error installing stroppy: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Stroppy installed successfully")
}
`
	testutils.RunTestInContainer(t, src, []string{"/usr/local/bin/stroppy", "version"})
}
