package installers

import (
	"fmt"
	"os/exec"

	"github.com/dhillondeep/go-getrelease"
)

type StroppyConfig struct {
	Version     string
	InstallPath string
	StartEnv    map[string]string
}
type stroppyInstaller struct {
	config *StroppyConfig
	client getrelease.Client
}

func StroppyInstaller(config *StroppyConfig) Software {
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
		s.config.Version,
	)
	if err != nil {
		return err
	}

	// Unpack to /usr/bin
	cmd := exec.Command("tar", "-xzf", downloadPath, "-C", "/usr/bin")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unpack stroppy: %s: %w", string(output), err)
	}

	return nil
}

func (s *stroppyInstaller) Start() error {
	//TODO implement me
	panic("implement me")
}
