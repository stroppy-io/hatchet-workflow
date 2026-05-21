package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultStroppyBinaryCacheDir = "/tmp/stroppy-cloud/stroppy-binaries"

var stroppyBinaryMu sync.Mutex

func (s *Server) resolveStroppyBinary(ctx context.Context, version string) (string, error) {
	if override := strings.TrimSpace(os.Getenv("STROPPY_PROBE_BIN")); override != "" {
		return override, nil
	}

	version = strings.TrimSpace(version)
	if version == "" {
		if path, err := exec.LookPath("stroppy"); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("stroppy version is required and no stroppy binary was found in PATH")
	}

	stroppyBinaryMu.Lock()
	defer stroppyBinaryMu.Unlock()

	if sha, ok := strings.CutPrefix(version, "commit:"); ok {
		return ensureCommitStroppyBinary(ctx, sha)
	}
	return ensureReleaseStroppyBinary(ctx, version)
}

func stroppyBinaryCacheDir() string {
	if dir := strings.TrimSpace(os.Getenv("STROPPY_BINARY_CACHE_DIR")); dir != "" {
		return dir
	}
	return defaultStroppyBinaryCacheDir
}

func ensureReleaseStroppyBinary(ctx context.Context, version string) (string, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return "", fmt.Errorf("stroppy release version is empty")
	}
	dir := filepath.Join(stroppyBinaryCacheDir(), safeCacheName("v"+version))
	binPath := filepath.Join(dir, "stroppy")
	if fileExists(binPath) {
		return binPath, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create stroppy cache dir: %w", err)
	}
	url := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/v%s/stroppy_linux_amd64.tar.gz", version)
	if err := downloadStroppyTarball(ctx, url, binPath); err != nil {
		return "", err
	}
	return binPath, nil
}

func ensureCommitStroppyBinary(ctx context.Context, sha string) (string, error) {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if len(sha) < 7 {
		return "", fmt.Errorf("commit-pinned stroppy version needs at least 7 hex characters")
	}
	for _, r := range sha {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", fmt.Errorf("invalid commit sha %q", sha)
		}
	}
	dir := filepath.Join(stroppyBinaryCacheDir(), "commit-"+sha)
	binPath := filepath.Join(dir, "stroppy")
	if fileExists(binPath) {
		return binPath, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create stroppy cache dir: %w", err)
	}
	url := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/nightly-%s/stroppy", sha)
	if err := downloadRawExecutable(ctx, url, binPath); err != nil {
		return "", err
	}
	return binPath, nil
}

func downloadStroppyTarball(ctx context.Context, url, dest string) error {
	resp, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("read stroppy tarball: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	tmp := dest + ".tmp"
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read stroppy tarball: %w", err)
		}
		if h.FileInfo().IsDir() || filepath.Base(h.Name) != "stroppy" {
			continue
		}
		if err := writeExecutable(tmp, tr); err != nil {
			return err
		}
		return os.Rename(tmp, dest)
	}
	return fmt.Errorf("stroppy binary not found in release tarball")
}

func downloadRawExecutable(ctx context.Context, url, dest string) error {
	resp, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	tmp := dest + ".tmp"
	if err := writeExecutable(tmp, resp.Body); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// stroppyHTTPClient is shared across binary downloads. Default
// http.DefaultClient has no timeout knobs; GitHub's release-assets CDN
// occasionally hangs on TLS handshake or first byte, which surfaces as
// "TLS handshake timeout" in probe responses. Explicit Transport with
// generous-but-bounded timeouts + a few retries make the path reliable
// enough that a single transient flake doesn't fail the whole UI flow.
var stroppyHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	},
}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	const attempts = 3
	var lastErr error
	for i := 0; i < attempts; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := stroppyHTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("download stroppy binary: %w", err)
			// Transient network errors (TLS handshake / read timeout) — back off and retry.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(i+1) * 2 * time.Second):
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			// 5xx is retryable; 4xx is not (wrong version, missing release).
			if resp.StatusCode >= 500 && i < attempts-1 {
				lastErr = fmt.Errorf("download stroppy binary %d: %s", resp.StatusCode, string(body))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(i+1) * 2 * time.Second):
				}
				continue
			}
			return nil, fmt.Errorf("download stroppy binary %d: %s", resp.StatusCode, string(body))
		}
		return resp, nil
	}
	return nil, lastErr
}

func writeExecutable(path string, src io.Reader) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("create stroppy binary: %w", err)
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return fmt.Errorf("write stroppy binary: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close stroppy binary: %w", err)
	}
	return os.Chmod(path, 0755)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func safeCacheName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
