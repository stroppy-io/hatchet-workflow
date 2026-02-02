package testutils

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func RunTestInContainer(t *testing.T, mainSource string, verificationCmd []string) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	ctx := context.Background()
	// Find repo root
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("Failed to find repo root: %v", err)

	}

	// Build helper binary
	helperPath, cleanup := buildHelperBinary(t, repoRoot, mainSource)
	defer cleanup()
	// Start container
	container := startContainer(ctx, t, helperPath)
	defer container.Terminate(ctx)
	defer func() {
		if r, err := container.Logs(ctx); err == nil {
			buf := new(strings.Builder)
			io.Copy(buf, r)
			t.Logf("Container logs:\n%s", buf.String())
		}
	}()
	// Run binary
	exitCode, out, err := container.Exec(ctx, []string{"/usr/local/bin/installer_helper"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	buf := new(strings.Builder)
	if out != nil {
		io.Copy(buf, out)
	}

	if exitCode != 0 {
		t.Fatalf("Installer failed with code %d: %s", exitCode, buf.String())
	} else {
		t.Logf("Installer success: %s", buf.String())
	}
	// Run verification
	if len(verificationCmd) > 0 {
		exitCode, out, err := container.Exec(ctx, verificationCmd)
		if err != nil {
			t.Fatalf("Verification command exec failed: %v", err)
		}
		buf := new(strings.Builder)
		if out != nil {
			io.Copy(buf, out)
		}
		if exitCode != 0 {
			t.Fatalf("Verification command failed with code %d: %s", exitCode, buf.String())
		} else {
			t.Logf("Verification command success: %s", buf.String())
		}
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func buildHelperBinary(t *testing.T, repoRoot, source string) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "installer_test")
	if err != nil {
		t.Fatal(err)
	}

	// Write main.go
	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	// Write go.mod
	goMod := fmt.Sprintf(`module helper
go 1.21
require github.com/stroppy-io/hatchet-workflow v0.0.0
replace github.com/stroppy-io/hatchet-workflow => %s
`, repoRoot)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// We might need to run go mod tidy first to resolve dependencies of the imported packages
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %s, %v", string(out), err)
	}

	// Build
	helperBinPath := filepath.Join(tmpDir, "installer_helper")
	cmd := exec.Command("go", "build", "-o", helperBinPath, ".")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64")

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build helper binary: %s, %v", string(out), err)
	}

	return helperBinPath, func() { os.RemoveAll(tmpDir) }
}

func startContainer(ctx context.Context, t *testing.T, helperPath string) testcontainers.Container {
	req := testcontainers.ContainerRequest{
		Image: "ubuntu:latest",
		Cmd:   []string{"/bin/bash", "-c", "sleep infinity"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      helperPath,
				ContainerFilePath: "/usr/local/bin/installer_helper",
				FileMode:          0755,
			},
		},
		WaitingFor: wait.ForExec([]string{"/bin/true"}),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Setup dependencies
	setupCmds := [][]string{
		{"apt-get", "update"},
		{"apt-get", "install", "-y", "ca-certificates", "curl", "gnupg", "lsb-release", "sudo", "postgresql-common"},
	}

	for _, cmd := range setupCmds {
		exitCode, _, err := container.Exec(ctx, cmd)
		if err != nil || exitCode != 0 {
			t.Fatalf("Failed setup command %v: %v", cmd, err)
		}
	}

	return container
}
