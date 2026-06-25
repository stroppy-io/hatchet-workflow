package orioledb

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func orioledbTestDB() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_ORIOLEDB,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Orioledb{Orioledb: &domain.OrioledbParams{}},
			},
		},
	}
}

func TestResolveDatabasePackageInstallsDocker(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(orioledbTestDB())
	if err != nil {
		t.Fatalf("ResolveDatabasePackage: %v", err)
	}
	if got := strings.Join(pkg.GetAptPackages(), ","); !strings.Contains(got, "docker.io") {
		t.Fatalf("want docker.io apt package, got %q", got)
	}
	// The docker daemon config / start must NOT be in PreInstall: PreInstall runs
	// before the apt install, when docker.service does not exist yet.
	if pre := strings.Join(pkg.GetPreInstall(), "\n"); strings.Contains(pre, "docker") {
		t.Fatalf("PreInstall must not touch docker (runs before apt install), got:\n%s", pre)
	}
}

// TestInstallCommandsOrder guards the ordering bug that failed the first stage
// run: docker.io must be apt-installed BEFORE daemon.json is written and BEFORE
// `systemctl enable docker`, otherwise the unit does not exist yet.
func TestInstallCommandsOrder(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(orioledbTestDB())
	if err != nil {
		t.Fatalf("ResolveDatabasePackage: %v", err)
	}
	cmds := orioledbInstallCommands(pkg)
	all := strings.Join(cmds, "\n")
	aptIdx, daemonIdx, enableIdx := -1, -1, -1
	for i, c := range cmds {
		switch {
		case strings.Contains(c, "apt-get install") && strings.Contains(c, "docker.io"):
			aptIdx = i
		case strings.Contains(c, "/etc/docker/daemon.json"):
			daemonIdx = i
		case strings.Contains(c, "systemctl enable --now docker"):
			enableIdx = i
		}
	}
	if aptIdx < 0 || daemonIdx < 0 || enableIdx < 0 {
		t.Fatalf("missing a required install step (apt=%d daemon=%d enable=%d) in:\n%s", aptIdx, daemonIdx, enableIdx, all)
	}
	if !(aptIdx < daemonIdx && daemonIdx < enableIdx) {
		t.Fatalf("install order wrong: apt(%d) must precede daemon.json(%d) must precede enable(%d)\n%s", aptIdx, daemonIdx, enableIdx, all)
	}
}
