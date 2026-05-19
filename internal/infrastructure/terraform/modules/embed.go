// Package modules bundles the .tf source files for each supported
// TerraformTask_Module into the server binary at build time. Resolvers
// elsewhere in the package walk this FS to materialise a workdir for
// terraform-exec.
package modules

import "embed"

//go:embed yandex_cloud/*.tf yandex_managed_ydb/*.tf
var FS embed.FS
