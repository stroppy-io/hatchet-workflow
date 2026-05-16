package verbs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// InstallPackage dispatches to the correct installation strategy.
func InstallPackage(ctx context.Context, action *agentpb.Action) *agentpb.Report {
	ip := action.GetInstallPackage()
	if ip == nil {
		return failReport("InstallPackage: nil payload")
	}
	pkg := ip.GetPackage()
	if pkg == nil {
		return failReport("InstallPackage: nil package")
	}

	src := pkg.GetSource()
	if src == nil {
		return failReport("InstallPackage: no package source")
	}

	switch {
	case src.GetApt() != nil:
		return installApt(ctx, src.GetApt())
	case src.GetBinary() != nil:
		return installBinary(ctx, src.GetBinary())
	case src.GetDebBlob() != nil:
		return failReport("InstallPackage: deb_blob source not supported by this agent version")
	case src.GetContainer() != nil:
		return failReport("InstallPackage: container source not supported by this agent version")
	default:
		return failReport("InstallPackage: unknown source variant")
	}
}

func installApt(ctx context.Context, apt *catalogpb.Package_AptSource) *agentpb.Report {
	pkgs := apt.GetAptPackages()
	if len(pkgs) == 0 {
		return failReport("InstallPackage/apt: no packages listed")
	}

	// Allow test environments to simulate success without running apt.
	if os.Getenv("STROPPY_AGENT_SKIP_APT") != "" {
		return &agentpb.Report{
			Status:     agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
			Output:     []byte("apt install skipped (STROPPY_AGENT_SKIP_APT set)"),
			FinishedAt: timestamppb.Now(),
		}
	}

	args := append([]string{"install", "-y"}, pkgs...)
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "apt-get", args...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	exitCode := int32(0)
	status := agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED
	errMsg := ""

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
		}
		status = agentpb.ReportStatus_REPORT_STATUS_FAILED
		errMsg = err.Error()
	}

	return &agentpb.Report{
		Status:     status,
		Error:      errMsg,
		Output:     buf.Bytes(),
		ExitCode:   exitCode,
		FinishedAt: timestamppb.Now(),
	}
}

func installBinary(ctx context.Context, bd *catalogpb.Package_BinaryDownload) *agentpb.Report {
	prefix := bd.GetInstallPrefix()
	if prefix == "" {
		prefix = "/usr/local"
	}

	switch {
	case bd.GetUrl() != nil:
		return installBinaryFromURL(ctx, bd.GetUrl().GetUrl(), prefix, bd.GetPostExtract())
	case bd.GetCachedArtifact() != nil:
		return failReport("InstallPackage/binary: cached_artifact not supported — use url variant")
	default:
		return failReport("InstallPackage/binary: no origin set")
	}
}

func installBinaryFromURL(ctx context.Context, url, prefix string, postExtract []string) *agentpb.Report {
	if url == "" {
		return failReport("InstallPackage/binary: empty URL")
	}

	data, err := fetchURL(ctx, url)
	if err != nil {
		return failReport(fmt.Sprintf("InstallPackage/binary: fetch %s: %v", url, err))
	}

	// Write to temp file and extract.
	tmp, err := os.CreateTemp("", "stroppy-agent-pkg-*.tar.gz")
	if err != nil {
		return failReport(fmt.Sprintf("InstallPackage/binary: create temp: %v", err))
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return failReport(fmt.Sprintf("InstallPackage/binary: write temp: %v", err))
	}
	_ = tmp.Close()

	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return failReport(fmt.Sprintf("InstallPackage/binary: mkdir %s: %v", prefix, err))
	}

	var buf bytes.Buffer
	tar := exec.CommandContext(ctx, "tar", "-xzf", tmp.Name(), "-C", prefix)
	tar.Stdout = &buf
	tar.Stderr = &buf
	if err := tar.Run(); err != nil {
		return failReport(fmt.Sprintf("InstallPackage/binary: extract: %v\n%s", err, buf.String()))
	}

	// Run post-extract commands.
	for _, cmd := range postExtract {
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}
		var cbuf bytes.Buffer
		c := exec.CommandContext(ctx, parts[0], parts[1:]...)
		c.Dir = filepath.Join(prefix)
		c.Stdout = &cbuf
		c.Stderr = &cbuf
		if err := c.Run(); err != nil {
			return failReport(fmt.Sprintf("InstallPackage/binary: post_extract %q: %v\n%s", cmd, err, cbuf.String()))
		}
		buf.Write(cbuf.Bytes())
	}

	return &agentpb.Report{
		Status:     agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
		Output:     buf.Bytes(),
		FinishedAt: timestamppb.Now(),
	}
}
