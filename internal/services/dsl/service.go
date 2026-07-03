// Package dsl implements the DslService connect handler: the browser IDE's
// schema/lint surface over the internal/dsl compiler (the YAML-DSL pivot's
// Tasks 2-14). Both RPCs are stateless — v1 has no persistent provider
// catalog (spec §11's storage question is a separate sub-project); the
// provider(s) a bundle uses are read directly out of the request's own
// providers/<name>/ subtree, matching every other decode/compile stage in
// internal/dsl which reads its recipe bundle purely from an in-memory
// include.Sources rather than the filesystem.
package dsl

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// clusterFile/providersDir/manifestFile/moduleDirName are the fixed
// bundle-relative layout v1 expects for a provider: providers/<name>/manifest.yaml
// (the decoded ast.ProviderManifest) and providers/<name>/module/*.tf (the
// Terraform variables schema.DeriveParamsSchema derives params/ext from).
const (
	clusterFile   = "cluster.yaml"
	providersDir  = "providers"
	manifestFile  = "manifest.yaml"
	moduleDirName = "module"
)

// DslService implements dslpb.DslServiceServer: dynamic composed JSON Schema
// + check-mode compilation, both stateless and side-effect-free.
type DslService struct {
	*dslpb.UnimplementedDslServiceServer
}

var _ dslpb.DslServiceServer = (*DslService)(nil)

// NewDslService constructs the DSL connect handler. It is deliberately
// dependency-free, unlike every sibling service's XDeps-struct constructor
// — v1 has no storage or collaborator of its own to inject (see the package
// doc); a future persistent provider catalog (spec §11) would add one.
func NewDslService() *DslService {
	return &DslService{UnimplementedDslServiceServer: &dslpb.UnimplementedDslServiceServer{}}
}

// ComposedSchema returns the dynamic JSON Schema for req's bundle: the core
// schema tightened to the bundle's single provider (params/ext derived from
// providers/<name>/module/variables.tf), or the core schema's original
// permissive placeholder when the bundle carries no provider directory at
// all. Unlike Check, an unresolvable "which provider" situation here IS an
// RPC error (InvalidArgument) — ComposedSchemaResponse carries no
// diagnostics channel to report it through instead.
func (s *DslService) ComposedSchema(_ context.Context, req *dslpb.ComposedSchemaRequest) (*dslpb.ComposedSchemaResponse, error) {
	files := req.GetFiles()

	name, err := resolveProviderName(files)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	providers := map[string]schema.ProviderSchemas{}
	if name != "" {
		params, ext, derr := deriveProviderSchema(files, name)
		if derr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "derive provider %q schema: %v", name, derr)
		}
		providers[name] = schema.ProviderSchemas{Params: params, Ext: ext}
	}

	_, raw, err := schema.Compose(providers, nil)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "compose schema: %v", err)
	}

	return &dslpb.ComposedSchemaResponse{SchemaJson: string(raw)}, nil
}

// Check compiles req's bundle in check-mode and returns every diagnostic the
// full dsl.Compile pipeline finds (schema, decode, contract, graph,
// lowering), plus this handler's own provider-resolution diagnostics
// (unresolvable provider.use, missing manifest, failed schema derivation).
// It NEVER returns an RPC error for a problem in the bundle itself — every
// user-input problem comes back as a Diagnostic entry, exactly like an IDE
// linter would report it. An RPC error would mean a transport-level failure
// only; nothing in this method's current implementation produces one.
func (s *DslService) Check(_ context.Context, req *dslpb.CheckRequest) (*dslpb.CheckResponse, error) {
	files := req.GetFiles()
	sources := include.Sources{Files: files}

	provider, composed, diags := resolveProvider(files)

	_, compileDiags := dsl.Compile(dsl.Input{Sources: sources, Provider: provider, Composed: composed})
	diags = append(diags, compileDiags...)

	return &dslpb.CheckResponse{Diagnostics: toProtoDiagnostics(diags)}, nil
}

// resolveProvider finds and decodes the provider cluster.yaml's provider.use
// selects, then derives and composes its params/ext JSON Schema. Every
// failure along the way (no provider.use at all, missing manifest, malformed
// manifest, module derivation failure, compose failure) is reported as a
// diagnostic and short-circuits the remaining steps — never a Go error, so
// Check can fold the result straight into the same diag.List dsl.Compile
// itself produces. A cluster.yaml with no resolvable provider.use returns
// (nil, nil, empty diags): dsl.Compile's own graph.Build stage reports "no
// provider manifest" for that case, so this function does not duplicate it.
func resolveProvider(files map[string][]byte) (*ast.ProviderManifest, *jsonschema.Schema, diag.List) {
	var diags diag.List

	name := peekProviderUse(files[clusterFile])
	if name == "" {
		return nil, nil, diags
	}

	mp := manifestPath(name)
	manifestSrc, ok := files[mp]
	if !ok {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Path:     clusterFile,
			Message:  fmt.Sprintf("provider %q: manifest not found at %s", name, mp),
			Module:   name,
		})
		return nil, nil, diags
	}

	manifest, manifestDiags := ast.DecodeProviderManifest(mp, manifestSrc)
	diags = append(diags, manifestDiags...)
	if manifestDiags.HasErrors() {
		return nil, nil, diags
	}

	params, ext, err := deriveProviderSchema(files, name)
	if err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Path:     mp,
			Message:  fmt.Sprintf("derive provider %q schema: %v", name, err),
			Module:   name,
		})
		// The manifest itself decoded cleanly — still hand it to Compile so
		// contract/capability checking (which only needs the manifest, not
		// the derived JSON Schema) still runs; only schema-tightening is lost.
		return manifest, nil, diags
	}

	composed, _, err := schema.Compose(map[string]schema.ProviderSchemas{name: {Params: params, Ext: ext}}, nil)
	if err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Path:     mp,
			Message:  fmt.Sprintf("compose provider %q schema: %v", name, err),
			Module:   name,
		})
		return manifest, nil, diags
	}

	return manifest, composed, diags
}

// resolveProviderName picks the single provider ComposedSchema composes
// against: cluster.yaml's provider.use when it names one of the bundle's
// providers/<name>/ directories; else the bundle's sole provider directory
// if it has exactly one; else "" when the bundle has no provider directory
// at all (the permissive core placeholder is composed); else an error —
// more than one providers/<name>/ directory and cluster.yaml does not
// disambiguate (absent, unparseable, or naming a provider not in the
// bundle).
func resolveProviderName(files map[string][]byte) (string, error) {
	names := distinctProviderNames(files)
	use := peekProviderUse(files[clusterFile])

	switch {
	case use != "" && slices.Contains(names, use):
		return use, nil
	case len(names) == 1:
		return names[0], nil
	case len(names) == 0:
		return "", nil
	case use != "":
		return "", fmt.Errorf("cluster.yaml selects provider %q, but the bundle has no %s directory", use, moduleFilePrefix(use))
	default:
		return "", fmt.Errorf("bundle has %d providers (%s) and cluster.yaml does not select one via provider.use", len(names), strings.Join(names, ", "))
	}
}

// peekProviderUse best-effort reads cluster.yaml's provider.use field
// without going through the full strict ast.DecodeCluster, which itself
// requires knowing the provider name up front (see ast.DecodeCluster's
// providerKey parameter: it names which top-level machine-group key holds
// the provider's ext block). A missing/unparseable cluster.yaml, or one with
// no provider.use, yields "" — every caller treats that as "cannot resolve a
// provider from cluster.yaml alone", never as a Go error; the real,
// position-accurate diagnostics for a malformed cluster.yaml come from
// dsl.Compile's own schema/decode stages.
func peekProviderUse(clusterSrc []byte) string {
	if len(clusterSrc) == 0 {
		return ""
	}
	var doc struct {
		Provider struct {
			Use string `yaml:"use"`
		} `yaml:"provider"`
	}
	if err := yaml.Unmarshal(clusterSrc, &doc); err != nil {
		return ""
	}
	return doc.Provider.Use
}

// distinctProviderNames returns the sorted set of provider names present in
// the bundle, i.e. every distinct <name> in a "providers/<name>/..." path.
func distinctProviderNames(files map[string][]byte) []string {
	seen := map[string]struct{}{}
	for p := range files {
		rest, ok := strings.CutPrefix(p, providersDir+"/")
		if !ok {
			continue
		}
		name, _, ok := strings.Cut(rest, "/")
		if !ok || name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// manifestPath/moduleFilePrefix build the fixed bundle-relative paths for
// one provider's manifest/module files (see the const block above). Bundle
// keys are always slash-separated logical paths (include.Sources' own
// convention), never OS paths, so these use the "path" package rather than
// "path/filepath".
func manifestPath(name string) string {
	return path.Join(providersDir, name, manifestFile)
}

func moduleFilePrefix(name string) string {
	return providersDir + "/" + name + "/" + moduleDirName + "/"
}

// deriveProviderSchema materializes providers/<name>/module/**'s bundle
// bytes into a temp dir and derives the params/ext JSON Schema fragments
// from it, then removes the temp dir before returning.
//
// This is the bytes-to-filesystem bridge: schema.DeriveParamsSchema's only
// supported input is a directory (internal/dsl/schema/tfvars.go's doc
// comment: it is the sole filesystem-I/O site in internal/dsl, built on
// terraform-config-inspect's tfconfig.LoadModule, which itself only reads a
// directory — not an fs.FS or an in-memory tree). A hypothetical
// DeriveParamsSchemaFromFiles(map[string][]byte) was considered instead (per
// this task's brief) but rejected: tfconfig.LoadModule takes a path
// exclusively, with no fs.FS-abstraction overload to hand it an in-memory
// tree, so avoiding the temp-dir write would mean re-implementing (or
// vendoring a fork of) terraform-config-inspect's directory walk — far more
// risk than one MkdirTemp/RemoveAll per request.
func deriveProviderSchema(files map[string][]byte, name string) (params, ext map[string]any, err error) {
	prefix := moduleFilePrefix(name)

	tmp, err := os.MkdirTemp("", "dsl-provider-module-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp module dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	wrote := 0
	for p, content := range files {
		rel, ok := strings.CutPrefix(p, prefix)
		if !ok || rel == "" {
			continue
		}
		dest := filepath.Join(tmp, filepath.FromSlash(rel))
		// 0o700/0o600: tmp is a private, request-scoped temp dir removed
		// before this function returns (see the RemoveAll above) — no reason
		// to leave it group/world-readable in the meantime.
		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o700); mkErr != nil {
			return nil, nil, fmt.Errorf("mkdir %q: %w", filepath.Dir(dest), mkErr)
		}
		if writeErr := os.WriteFile(dest, content, 0o600); writeErr != nil {
			return nil, nil, fmt.Errorf("write %q: %w", dest, writeErr)
		}
		wrote++
	}
	if wrote == 0 {
		return nil, nil, fmt.Errorf("no module files found under %s", prefix)
	}

	return schema.DeriveParamsSchema(tmp)
}

// toProtoDiagnostics maps a diag.List to the wire Diagnostic shape.
func toProtoDiagnostics(diags diag.List) []*dslpb.Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	out := make([]*dslpb.Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, &dslpb.Diagnostic{
			Severity: toProtoSeverity(d.Severity),
			Path:     d.Path,
			Line:     clampUint32(d.Pos.Line),
			Col:      clampUint32(d.Pos.Col),
			Message:  d.Message,
			Module:   d.Module,
		})
	}
	return out
}

func toProtoSeverity(s diag.Severity) dslpb.Severity {
	switch s {
	case diag.Error:
		return dslpb.Severity_SEVERITY_ERROR
	case diag.Warning:
		return dslpb.Severity_SEVERITY_WARNING
	default:
		return dslpb.Severity_SEVERITY_UNSPECIFIED
	}
}

// clampUint32 converts a diag.Pos component (1-based, expected non-negative)
// to the wire uint32 form, clamping a (never expected, but not guarded
// upstream) negative value to 0 rather than silently wrapping around.
func clampUint32(v int) uint32 {
	if v < 0 {
		return 0
	}
	return uint32(v) //nolint:gosec // guarded non-negative above; a diag.Pos line/col never approaches uint32's range.
}
