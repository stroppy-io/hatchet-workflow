package stroppy

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

// Config holds the configuration for Stroppy installation.
type Config struct {
	Version     string // e.g., "v0.9.6" or "latest"
	InstallPath string // e.g., "/usr/local/bin"
}

// Install installs the stroppy binary.
func Install(ctx context.Context, cfg Config) error {
	if cfg.InstallPath == "" {
		cfg.InstallPath = "/usr/local/bin"
	}

	goOS := runtime.GOOS
	goArch := runtime.GOARCH

	if cfg.Version == "" {
		return fmt.Errorf("version is required (e.g. v2.0.0)")
	}

	// New URL format with .tar.gz
	url := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/%s/stroppy_%s_%s.tar.gz", cfg.Version, goOS, goArch)

	tmpDir, err := os.MkdirTemp("", "stroppy-install")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "stroppy.tar.gz")

	if err := downloadFile(ctx, url, archivePath); err != nil {
		return fmt.Errorf("failed to download stroppy archive: %w", err)
	}

	// Extract the tar.gz file
	cmd := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to extract stroppy archive: %s, %w", string(out), err)
	}

	// The extracted binary is usually named 'stroppy' inside the archive
	downloadPath := filepath.Join(tmpDir, "stroppy")

	if err := os.Chmod(downloadPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod stroppy: %w", err)
	}

	destPath := filepath.Join(cfg.InstallPath, "stroppy")

	if err := copyFile(downloadPath, destPath); err != nil {
		return fmt.Errorf("failed to install stroppy to %s: %w", destPath, err)
	}

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

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// Copy permissions
	info, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, info.Mode())
	}

	return nil
}
