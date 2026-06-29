package pgnoop

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_PG_NOOP && database.GetParams() != nil
}

// ResolveDatabasePackage points the agent at the pg-noop release tarball served
// through the gateway binary cache (so an egress-less node never reaches GitHub
// directly). pg-noop ships a single static binary, not an apt package, so the
// download URL rides DebFilename exactly like cockroach/ydb.
func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	version := database.GetParams().GetVersion()
	if version == "" {
		version = "default"
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/pgnoop/" + version
	}

	downloadVersion := defaultPgNoopVersion
	if version != "default" {
		downloadVersion = version
	}

	return &domain.Package{
		Id:          packageID,
		Name:        "pg-noop " + version,
		DbKind:      domain.Database_KIND_PG_NOOP,
		DbVersion:   version,
		IsBuiltin:   true,
		DebFilename: serverBinaryURL("pgnoop", downloadVersion, downloadAsset),
	}, nil
}

func serverBinaryURL(name, version, filename string) string {
	return fmt.Sprintf("${STROPPY_SERVER_ADDR%%/}/api/binaries/%s/%s/%s", name, version, filename)
}
