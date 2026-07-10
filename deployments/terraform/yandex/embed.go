package yandex

import (
	"embed"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
)

//go:embed *.tf
var tfFilesEmbed embed.FS

//go:embed manifest.yaml
var manifestEmbed embed.FS

// EmbeddedTfFiles returns all .tf files from the embedded filesystem as TfFile slices.
func EmbeddedTfFiles() ([]terraform.TfFile, error) {
	entries, err := tfFilesEmbed.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var files []terraform.TfFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := tfFilesEmbed.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, terraform.NewTfFile(content, e.Name()))
	}
	return files, nil
}

// BuiltinCatalogFiles returns the yandex provider's catalog bundle:
// manifest.yaml plus the module's *.tf files rekeyed under module/<name>.tf
// — the bare-keyed providers/<slug>/ layout catalog.Deps.BuiltinProviders
// and dsl.ProviderResolver/rekeyUnderProvider expect (see internal/services/
// dsl/service.go's clusterFile/providersDir/manifestFile/moduleDirName
// consts). Used by internal/app/run.go to seed "yandex" as a LEVEL_INSTANCE
// KIND_PROVIDER catalog entry alongside the docker builtin.
func BuiltinCatalogFiles() (map[string][]byte, error) {
	manifest, err := manifestEmbed.ReadFile("manifest.yaml")
	if err != nil {
		return nil, err
	}
	tfFiles, err := EmbeddedTfFiles()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(tfFiles)+1)
	out["manifest.yaml"] = manifest
	for _, f := range tfFiles {
		out["module/"+f.Name()] = f.Content()
	}
	return out, nil
}
