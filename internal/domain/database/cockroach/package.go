package cockroach

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

const defaultCockroachVersion = "23.2.5"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_COCKROACH && database.GetParams() != nil
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	version := database.GetParams().GetVersion()
	if version == "" {
		version = "default"
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/cockroach/" + version
	}

	downloadVersion := defaultCockroachVersion
	if version != "default" {
		downloadVersion = version
	}
	downloadURL := serverBinaryURL("cockroach", downloadVersion, fmt.Sprintf("cockroach-v%s.linux-amd64.tgz", downloadVersion))

	// CockroachDB has no apt package: the renderer fetches the cockroach binary
	// tarball from deb_filename (reused as the download URL).
	return &domain.Package{
		Id:          packageID,
		Name:        "CockroachDB " + version,
		DbKind:      domain.Database_KIND_COCKROACH,
		DbVersion:   version,
		IsBuiltin:   true,
		DebFilename: downloadURL,
	}, nil
}

func serverBinaryURL(name, version, filename string) string {
	return fmt.Sprintf("${STROPPY_SERVER_ADDR%%/}/api/binaries/%s/%s/%s", name, version, filename)
}
