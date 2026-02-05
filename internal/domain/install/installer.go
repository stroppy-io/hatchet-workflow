package install

import (
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

var (
	InstallerDoesNotImplementStart = fmt.Errorf("installer does not implement start")
)

type Installer interface {
	Install() error
	Start() error
}

func Install(software Installer, start hatchet.Software_SetupStrategy) error {
	if err := software.Install(); err != nil {
		return err
	}
	if start == hatchet.Software_SETUP_STRATEGY_INSTALL_AND_START {
		return software.Start()
	}
	return nil
}
