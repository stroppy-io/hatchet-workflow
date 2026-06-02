package ydb

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_YDB && database.GetParams() != nil
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	version := database.GetParams().GetVersion()
	if version == "" {
		version = "default"
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/ydb/" + version
	}

	downloadURL := defaultYdbDownloadURL
	if version != "default" {
		downloadURL = fmt.Sprintf("https://binaries.ydb.tech/release/%s/ydbd-%s-linux-amd64.tar.gz", version, version)
	}

	// YDB has no apt package: the renderer fetches the ydbd binary tarball from
	// deb_filename (reused as the download URL for download-only engines).
	return &domain.Package{
		Id:          packageID,
		Name:        "YDB " + version,
		DbKind:      domain.Database_KIND_YDB,
		DbVersion:   version,
		IsBuiltin:   true,
		DebFilename: downloadURL,
	}, nil
}
