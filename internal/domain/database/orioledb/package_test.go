package orioledb

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestResolveDatabasePackageInstallsDockerAndMirror(t *testing.T) {
	db := &domain.Database{
		Kind: domain.Database_KIND_ORIOLEDB,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Orioledb{Orioledb: &domain.OrioledbParams{}},
			},
		},
	}
	pkg, err := PackageResolver{}.ResolveDatabasePackage(db)
	if err != nil {
		t.Fatalf("ResolveDatabasePackage: %v", err)
	}
	if got := strings.Join(pkg.GetAptPackages(), ","); !strings.Contains(got, "docker.io") {
		t.Fatalf("want docker.io apt package, got %q", got)
	}
	joined := strings.Join(pkg.GetPreInstall(), "\n")
	if !strings.Contains(joined, "/etc/docker/daemon.json") {
		t.Fatalf("pre_install must write the docker registry mirror config, got:\n%s", joined)
	}
	if !strings.Contains(joined, "systemctl enable --now docker") {
		t.Fatalf("pre_install must start docker, got:\n%s", joined)
	}
}
