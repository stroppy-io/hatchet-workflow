package orioledb

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// RegistryMirrorEnv is the env var the agent expands (from the bootstrap
// ExtraEnv) into the gateway's pull-through registry URL, e.g.
// "http://gateway-host:5000". Empty disables the mirror (direct pull).
const RegistryMirrorEnv = "STROPPY_REGISTRY_MIRROR"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_ORIOLEDB && database.GetParams() != nil
}

// ResolveDatabasePackage installs Docker (from the Ubuntu universe repo, which
// rides the existing apt-cacher proxy) and points dockerd at the gateway's
// pull-through registry mirror so an egress-less node can pull the OrioleDB
// image. The mirror URL is injected at run time via $STROPPY_REGISTRY_MIRROR.
func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/orioledb/docker"
	}
	return &domain.Package{
		Id:          packageID,
		Name:        "OrioleDB (docker)",
		DbKind:      domain.Database_KIND_ORIOLEDB,
		DbVersion:   database.GetParams().GetVersion(),
		IsBuiltin:   true,
		AptPackages: []string{"docker.io"},
		PreInstall:  dockerMirrorPreInstall(),
	}, nil
}

// dockerMirrorPreInstall configures /etc/docker/daemon.json with the registry
// mirror (when $STROPPY_REGISTRY_MIRROR is set) and a matching insecure-registry
// entry (the in-VPC mirror is plain HTTP), then enables dockerd. apt installs
// docker.io itself; these run as the package pre_install steps (order 100).
func dockerMirrorPreInstall() []string {
	const daemonJSON = "/etc/docker/daemon.json"
	return []string{
		"install -d /etc/docker",
		// Write daemon.json only when a mirror is provided; strip the scheme for
		// the insecure-registries entry (docker wants host:port there).
		fmt.Sprintf(`sh -c 'mirror="${%s}"; if [ -n "$mirror" ]; then host="${mirror#http://}"; host="${host#https://}"; printf "{\"registry-mirrors\":[\"%%s\"],\"insecure-registries\":[\"%%s\"]}\n" "$mirror" "$host" > %s; fi'`,
			RegistryMirrorEnv, daemonJSON),
		"systemctl enable --now docker",
		// Apply daemon.json if dockerd was already running.
		"systemctl reload docker 2>/dev/null || systemctl restart docker || true",
	}
}
