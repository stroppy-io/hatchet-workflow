package golang

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Config holds the configuration for the Go installation.
type Config struct {
	Version     string // e.g., "1.21.5"
	InstallPath string // e.g., "/usr/local"
}

// Install installs Go on the machine.
func Install(ctx context.Context, cfg Config) error {
	if cfg.Version == "" {
		return fmt.Errorf("go version is required")
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = "/usr/local"
	}

	// Construct download URL
	goOS := runtime.GOOS
	goArch := runtime.GOARCH
	fileName := fmt.Sprintf("go%s.%s-%s.tar.gz", cfg.Version, goOS, goArch)
	url := fmt.Sprintf("https://go.dev/dl/%s", fileName)

	// Create temporary directory for download
	tmpDir, err := os.MkdirTemp("", "go-install")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, fileName)

	// Download the tarball
	if err := downloadFile(ctx, url, tarPath); err != nil {
		return fmt.Errorf("failed to download go: %w", err)
	}

	// Remove existing installation if any
	goInstallDir := filepath.Join(cfg.InstallPath, "go")
	if err := os.RemoveAll(goInstallDir); err != nil {
		return fmt.Errorf("failed to remove existing go installation: %w", err)
	}

	// Extract the tarball
	if err := extractTarGz(ctx, tarPath, cfg.InstallPath); err != nil {
		return fmt.Errorf("failed to extract go: %w", err)
	}

	// Setup environment variables (optional, but helpful)
	// This usually requires modifying shell profiles which might be intrusive.
	// For now, we just log where it is installed.
	// Users might need to add export PATH=$PATH:/usr/local/go/bin to their profile.

	return nil
}

func downloadFile(ctx context.Context, url string, filepath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarGz(ctx context.Context, tarPath, destDir string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Use tar command to extract
	cmd := exec.CommandContext(ctx, "tar", "-C", destDir, "-xzf", tarPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar extraction failed: %s, %w", string(out), err)
	}
	return nil
}
