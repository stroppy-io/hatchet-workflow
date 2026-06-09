package picodata

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_PICODATA && database.GetParams() != nil
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	version := database.GetParams().GetVersion()
	if version == "" {
		version = "default"
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/picodata/" + version
	}

	aptPackages := []string{"picodata"}
	if version != "default" {
		aptPackages = []string{"picodata=" + version}
	}

	return &domain.Package{
		Id:          packageID,
		Name:        "Picodata " + version,
		DbKind:      domain.Database_KIND_PICODATA,
		DbVersion:   version,
		IsBuiltin:   true,
		AptPackages: aptPackages,
		PreInstall:  picodataPreInstall(),
	}, nil
}

func picodataPreInstall() []string {
	return []string{
		"apt-get update",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl gpg",
		`sh -ec '. /etc/os-release
case "$ID" in
  ubuntu|debian) distro="$ID" ;;
  *) echo "unsupported Picodata apt repository distro: $ID" >&2; exit 1 ;;
esac
curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import
chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg
printf "deb [arch=amd64] https://download.picodata.io/tarantool-picodata/%s/ %s main\n" "$distro" "$VERSION_CODENAME" > /etc/apt/sources.list.d/picodata.list'`,
		"apt-get update",
	}
}
