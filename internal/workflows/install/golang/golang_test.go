package golang_test

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
	"github.com/stroppy-io/hatchet-workflow/pkg/install/golang"
)

func main() {
	ctx := context.Background()
	cfg := golang.Config{
		Version: "1.21.5",
		InstallPath: "/usr/local",
	}
	if err := golang.Install(ctx, cfg); err != nil {
		fmt.Printf("Error installing golang: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Golang installed successfully")
}
`
	testutils.RunTestInContainer(t, src, []string{"/usr/local/go/bin/go", "version"})
}
