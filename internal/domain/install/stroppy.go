package install

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/dhillondeep/go-getrelease"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

type stroppyInstaller struct {
	config *stroppy.StroppyCli
	client getrelease.Client
}

func StroppyInstaller(config *stroppy.StroppyCli) Installer {
	return &stroppyInstaller{
		config: config,
		client: getrelease.NewGithubClient(nil),
	}
}

func (s *stroppyInstaller) Install() error {
	downloadPath, err := getrelease.GetTagAsset(
		s.client,
		"/tmp/",
		"stroppy_linux_amd64.tar.gz",
		"stroppy-io",
		"stroppy",
		s.config.GetVersion(),
	)
	if err != nil {
		return err
	}

	// Unpack to /usr/bin
	cmd := exec.Command("tar", "-xzf", downloadPath, "-C", filepath.Dir(s.config.GetBinaryPath()))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unpack stroppy: %s: %w", string(output), err)
	}

	return nil
}

func (s *stroppyInstaller) Start() error {
	return InstallerDoesNotImplementStart
}
