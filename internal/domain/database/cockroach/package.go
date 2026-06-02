package cockroach

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

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

	downloadURL := defaultCockroachDownloadURL
	if version != "default" {
		downloadURL = fmt.Sprintf("https://binaries.cockroachdb.com/cockroach-v%s.linux-amd64.tgz", version)
	}

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
