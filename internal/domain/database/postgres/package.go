package postgres

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_POSTGRES && database.GetParams() != nil
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	params := database.GetParams()
	version := params.GetVersion()
	if version == "" {
		version = "default"
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/postgres/" + version
	}

	aptPackages := []string{"postgresql", "postgresql-contrib"}
	preInstall := []string{"apt-get update"}
	if version != "default" {
		// A pinned version (e.g. "16") is not in the Ubuntu/Debian distro archive
		// (jammy ships 14); pull it from the official PostgreSQL apt repository
		// (PGDG). apt traffic is relayed through the gateway -> apt-cacher-ng, which
		// caches PGDG like any other apt repo. The signing key also goes through
		// that proxy path, so keep it on http: the public gateway is reached over
		// HTTPS, while stock Caddy is not a generic CONNECT tunnel.
		aptPackages = []string{"postgresql-" + version, "postgresql-contrib-" + version}
		preInstall = pgdgPreInstall()
	}

	return &domain.Package{
		Id:          packageID,
		Name:        "PostgreSQL " + version,
		DbKind:      domain.Database_KIND_POSTGRES,
		DbVersion:   version,
		IsBuiltin:   true,
		AptPackages: aptPackages,
		PreInstall:  preInstall,
	}, nil
}

// pgdgPreInstall returns the commands that register the PostgreSQL APT (PGDG)
// repository for the running distro codename, then refresh the package index.
func pgdgPreInstall() []string {
	const keyring = "/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc"
	return []string{
		"install -d /usr/share/postgresql-common/pgdg",
		"curl -fsSL -o " + keyring + " http://www.postgresql.org/media/keys/ACCC4CF8.asc",
		`sh -c 'echo "deb [signed-by=` + keyring + `] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
		"apt-get update",
	}
}
