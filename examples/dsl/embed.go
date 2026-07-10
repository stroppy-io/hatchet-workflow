// Package dslexamples embeds the example DSL bundles under examples/dsl/**
// so they can be shipped inside the server binary and seeded into the
// catalog as builtin KIND_WORKFLOW entries (internal/services/catalog's
// Deps.BuiltinWorkflows) — mirroring deployments/terraform/yandex's
// go:embed pattern for provider modules (see that package's embed.go).
package dslexamples

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed postgres-ha
var bundlesFS embed.FS

// PostgresHABundleName is the catalog slug the postgres-ha example bundle
// is seeded under.
const PostgresHABundleName = "postgres-ha"

// PostgresHABundle returns the postgres-ha example bundle's files, keyed
// bundle-relative (cluster.yaml, workflow.yaml, components/etcd/
// component.yaml, providers/yandex/manifest.yaml, ...) exactly as
// catalog.Deps.BuiltinWorkflows and dslService.CheckBundle expect.
func PostgresHABundle() (map[string][]byte, error) {
	return bundleFiles(PostgresHABundleName)
}

// bundleFiles walks bundlesFS under root and returns every regular file's
// contents keyed by its path relative to root (embed.FS always uses
// forward-slash paths regardless of host OS, so a plain prefix trim is
// sufficient — no filepath/path.Rel needed).
func bundleFiles(root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	prefix := root + "/"
	err := fs.WalkDir(bundlesFS, root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, prefix)
		data, readErr := bundlesFS.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		out[rel] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
