package ydb

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

const defaultYdbVersion = "24.1.18"

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

	downloadVersion := defaultYdbVersion
	if version != "default" {
		downloadVersion = version
	}
	downloadURL := ydbServerBinaryURL(downloadVersion, fmt.Sprintf("ydbd-%s-linux-amd64.tar.gz", downloadVersion))

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

func ydbServerBinaryURL(version, filename string) string {
	return fmt.Sprintf("${STROPPY_SERVER_ADDR%%/}/api/binaries/ydbd/%s/%s", version, filename)
}
