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
	versions VersionSource
}

var _ dslpb.DslServiceServer = (*DslService)(nil)

// VersionSource lists known Stroppy release versions Check/Preview validate
// a recipe's stroppy service image tag against (Task 7 — audit Part A #3:
// nothing validated a recipe's stroppy image/version before this, so a
// bogus pin only failed late, at container-pull/exec). Structurally
// identical to internal/services/stroppy.VersionSource; kept as its own
// interface here rather than importing that unrelated service package —
// any implementation of one already satisfies the other.
type VersionSource interface {
	List(ctx context.Context) ([]string, error)
}

// Option configures an optional DslService dependency. WithVersionSource is
// the only one today (Task 7); see NewDslService's doc comment for why
// DslService has no XDeps-struct constructor like its siblings.
type Option func(*DslService)

// WithVersionSource injects the known-stroppy-versions source Check/Preview
// validate a recipe's stroppy service image tag against. Omitted (every
// call site but internal/app/run.go's production wiring omits it, including
// every existing test), Check/Preview behave exactly as before this task:
// no stroppy-version diagnostic at all, never a false "unknown version" for
// a tag no source was configured to confirm.
func WithVersionSource(vs VersionSource) Option {
	return func(s *DslService) { s.versions = vs }
}

// NewDslService constructs the DSL connect handler. It is deliberately
// dependency-free by default, unlike every sibling service's XDeps-struct
// constructor — v1 has no storage or collaborator of its own to inject (see
// the package doc); a future persistent provider catalog (spec §11) would
// add one. WithVersionSource (Task 7) is the sole opt-in exception, added
// as a variadic Option rather than a Deps struct so every pre-existing
// zero-arg NewDslService() call site keeps compiling unchanged.
func NewDslService(opts ...Option) *DslService {
	s := &DslService{UnimplementedDslServiceServer: &dslpb.UnimplementedDslServiceServer{}}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
		params, ext, deriveDiags, derr := deriveProviderSchema(files, name)
		if derr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "derive provider %q schema: %v", name, derr)
		}
		if deriveDiags.HasErrors() {
			// ComposedSchemaResponse carries no diagnostics channel (see the
			// doc comment above), so a rejected path-traversal key here must
			// still surface as an RPC error rather than silently composing a
			// schema derived from a partially-rejected module.
			return nil, status.Errorf(codes.InvalidArgument, "derive provider %q schema: %s", name, joinDiagMessages(deriveDiags))
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
// (unresolvable provider.use, missing manifest, failed schema derivation)
// and, when a VersionSource was injected via WithVersionSource, Task 7's
// advisory stroppy-version diagnostic (see validateStroppyVersion). It
// NEVER returns an RPC error for a problem in the bundle itself — every
// user-input problem comes back as a Diagnostic entry, exactly like an IDE
// linter would report it. An RPC error would mean a transport-level failure
// only; nothing in this method's current implementation produces one.
//
// This calls CompileBundle directly rather than the CheckBundle package
// function (which wraps it and discards the plan): validateStroppyVersion
// needs the compiled plan's services, and CheckBundle's error return is
// documented as always nil today anyway (see its own doc comment), so
// nothing is lost by not going through it here.
func (s *DslService) Check(ctx context.Context, req *dslpb.CheckRequest) (*dslpb.CheckResponse, error) {
	plan, diags := CompileBundle(req.GetFiles())
	validateStroppyVersion(ctx, plan, s.versions, &diags)
	return &dslpb.CheckResponse{Diagnostics: toProtoDiagnostics(diags)}, nil
}

// Preview compiles req's bundle via CompileBundle and returns the resolved
// CompiledPlan (machine groups, services, job DAG) alongside every
// diagnostic — the "what will be provisioned" surface the RecipeEditor's
// preview panel renders before a user clicks Run — plus, when a
// VersionSource was injected via WithVersionSource, Task 7's advisory
// stroppy-version diagnostic (see validateStroppyVersion). Like Check, it
// never returns an RPC error for a problem in the bundle itself: a bundle
// that fails to compile still comes back with a nil Plan and the
// diagnostics explaining why, never a transport-level error.
func (s *DslService) Preview(ctx context.Context, req *dslpb.PreviewRequest) (*dslpb.PreviewResponse, error) {
	plan, diags := CompileBundle(req.GetFiles())
	validateStroppyVersion(ctx, plan, s.versions, &diags)
	if diags.HasErrors() {
		plan = nil
	}
	return &dslpb.PreviewResponse{Plan: plan, Diagnostics: toProtoDiagnostics(diags)}, nil
}

// CheckBundle runs the same check-mode compile pipeline as Check
// (provider resolution + dsl.Compile) directly over a bundle's files,
// returning wire-shaped diagnostics. It is the shared implementation behind
// DslService.Check and — via injection as internal/services/recipe.Deps.Checker
// — RecipeService's create-time Summary.compiles computation and
// CheckRecipe, so neither caller duplicates the path-traversal-safe
// provider derivation in resolveProvider/deriveProviderSchema. Like Check,
// it never returns a Go error for a problem in the bundle itself (every such
// problem is folded into a diagnostic instead); the error return exists so
// callers have a seam for a future failure mode that genuinely isn't
// bundle-shaped (e.g. an IO error), not because one exists today.
//
// CheckBundle discards the compiled plan CompileBundle also produces — it
// only ever needs the wire-shaped diagnostics for the connect/recipe check
// surfaces. Task 4's RecipeActivities.CompileRecipeActivity (internal/
// infrastructure/execution) needs the plan itself, so it calls CompileBundle
// directly instead of duplicating the provider-resolution + dsl.Compile call
// pair here.
//
// This package-level function has no VersionSource, so it never appends
// Task 7's advisory stroppy-version diagnostic — bind the (*DslService)
// CheckBundle method instead (see below) wherever that warning should also
// cover the stored-recipe Create/CheckRecipe path.
func CheckBundle(_ context.Context, files map[string][]byte) ([]*dslpb.Diagnostic, error) {
	_, diags := CompileBundle(files)
	return toProtoDiagnostics(diags), nil
}

// CheckBundle (method) is the (*DslService) counterpart of the package-level
// CheckBundle function above: same check-mode compile pipeline, but also
// applies Task 7's advisory stroppy-version diagnostic via this service's
// injected VersionSource (validateStroppyVersion; a no-op when none was
// injected — see WithVersionSource). internal/app/run.go binds this method
// (not the package function) as recipe.Deps.Checker so a stored recipe's
// Create/CheckRecipe path surfaces the same bogus-stroppy-version warning
// DslService.Check already gives the live editor, instead of two check paths
// silently disagreeing on what they validate.
func (s *DslService) CheckBundle(ctx context.Context, files map[string][]byte) ([]*dslpb.Diagnostic, error) {
	plan, diags := CompileBundle(files)
	validateStroppyVersion(ctx, plan, s.versions, &diags)
	return toProtoDiagnostics(diags), nil
}

// CompileBundle runs the check-mode compile pipeline (provider resolution +
// dsl.Compile) over a bundle's raw files and returns both the compiled plan
// and every diagnostic (errors and warnings) gathered along the way — the
// same (plan, diag.List) shape dsl.Compile itself returns. It is the single
// place that resolves a bundle's provider manifest via the path-traversal-
// safe resolveProvider/deriveProviderSchema pair; CheckBundle and
// RecipeActivities.CompileRecipeActivity (internal/infrastructure/execution,
// Task 4) both call it rather than duplicating that resolution logic.
//
// The returned plan may be non-nil even when diags.HasErrors() is true (see
// resolveProvider's own doc comment: a schema-derivation failure still hands
// the decoded manifest to dsl.Compile so contract checking keeps running) —
// callers must gate on diags.HasErrors(), never on a nil plan check alone,
// mirroring dsl.Compile's own contract.
func CompileBundle(files map[string][]byte) (*dslpb.CompiledPlan, diag.List) {
	sources := include.Sources{Files: files}

	provider, composed, diags := resolveProvider(files)

	plan, compileDiags := dsl.Compile(dsl.Input{Sources: sources, Provider: provider, Composed: composed})
	diags = append(diags, compileDiags...)

	return plan, diags
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

	params, ext, deriveDiags, err := deriveProviderSchema(files, name)
	diags = append(diags, deriveDiags...)
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
//
// files is caller-controlled request input: a "providers/<name>/module/"-
// prefixed key may contain ".." segments (e.g.
// "providers/yandex/module/../../../../etc/cron.d/evil") that, if joined
// onto tmp unchecked, resolve outside tmp entirely — an arbitrary
// server-side file write. Every candidate rel path is therefore required to
// be filepath.IsLocal (never escapes tmp via ".." or an absolute path)
// before it is joined and written; a rejected key is skipped and reported
// as an Error diagnostic instead of a Go error, so one malicious/malformed
// key in an otherwise-valid bundle does not abort deriving the rest of the
// module's schema.
func deriveProviderSchema(files map[string][]byte, name string) (params, ext map[string]any, diags diag.List, err error) {
	prefix := moduleFilePrefix(name)

	tmp, err := os.MkdirTemp("", "dsl-provider-module-*")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create temp module dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	wrote := 0
	for p, content := range files {
		rel, ok := strings.CutPrefix(p, prefix)
		if !ok || rel == "" {
			continue
		}
		relOS := filepath.FromSlash(rel)
		if !filepath.IsLocal(relOS) {
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Path:     p,
				Message:  fmt.Sprintf("provider %q: module file %q escapes the module directory and was rejected (path traversal)", name, rel),
				Module:   name,
			})
			continue
		}
		dest := filepath.Join(tmp, relOS)
		// 0o700/0o600: tmp is a private, request-scoped temp dir removed
		// before this function returns (see the RemoveAll above) — no reason
		// to leave it group/world-readable in the meantime.
		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o700); mkErr != nil {
			return nil, nil, diags, fmt.Errorf("mkdir %q: %w", filepath.Dir(dest), mkErr)
		}
		if writeErr := os.WriteFile(dest, content, 0o600); writeErr != nil {
			return nil, nil, diags, fmt.Errorf("write %q: %w", dest, writeErr)
		}
		wrote++
	}
	if wrote == 0 {
		return nil, nil, diags, fmt.Errorf("no module files found under %s", prefix)
	}

	params, ext, err = schema.DeriveParamsSchema(tmp)
	return params, ext, diags, err
}

// validateStroppyVersion appends a Warning diagnostic to diags when plan
// names an identifiable stroppy service (findStroppySvc) whose image tag is
// absent from versions' known-releases list (Task 7). It is deliberately
// advisory, never an Error: the known-versions list (backed by GitHub
// Releases — see internal/services/stroppy.Service.ListStroppyVersions and
// internal/infrastructure/adapters.GitHubStroppyVersionSource) may be stale
// or momentarily unreachable, and a self-hosted/custom stroppy image is a
// legitimate recipe that will never appear in stroppy-io/stroppy's release
// list at all — neither should ever hard-fail a compile.
//
// versions == nil (WithVersionSource was never passed to NewDslService — the
// default) or a plan with no identifiable stroppy service skip silently:
// this function only flags a KNOWN-BAD pinned tag, it does not enforce that
// a recipe have a stroppy service in the first place (the graph/contract
// compile stages already do that where required). "latest" and an untagged
// image (Docker's own implicit "latest") are likewise skipped — a floating
// tag has no fixed version to check against a release list. A
// versions.List failure (network error, GitHub rate limit, ...) also skips
// rather than warning: a transient version-source outage must not read as
// "every recipe's stroppy tag is unknown".
func validateStroppyVersion(ctx context.Context, plan *dslpb.CompiledPlan, versions VersionSource, diags *diag.List) {
	if versions == nil || plan == nil {
		return
	}
	svc := findStroppySvc(plan.GetServices())
	if svc == nil {
		return
	}
	tag := strings.ToLower(stroppyImageTag(svc.GetImage()))
	if tag == "" || tag == "latest" {
		return
	}
	known, err := versions.List(ctx)
	if err != nil {
		return
	}
	normalized := strings.TrimPrefix(tag, "v")
	for _, v := range known {
		if strings.EqualFold(strings.TrimPrefix(v, "v"), normalized) {
			return
		}
	}
	diags.Add(diag.Diagnostic{
		Severity: diag.Warning,
		Path:     clusterFile,
		Message:  fmt.Sprintf("stroppy version %q not found in known releases; run may fail if the image is unavailable", tag),
		Module:   "stroppy",
	})
}

// findStroppySvc identifies plan's benchmark-runner ServiceSpec using the
// same best-effort convention internal/workflows.findStroppySvc anchors its
// own Summary derivation on (see that function's doc comment): named
// "stroppy", or an image prefixed "stroppy:"/"stroppy/". That function is
// unexported in an otherwise-unrelated, Temporal-SDK-heavy package, so the
// tiny heuristic is mirrored here rather than imported.
func findStroppySvc(services []*dslpb.ServiceSpec) *dslpb.ServiceSpec {
	for _, svc := range services {
		name := strings.ToLower(svc.GetName())
		image := strings.ToLower(svc.GetImage())
		if name == "stroppy" || strings.HasPrefix(image, "stroppy:") || strings.HasPrefix(image, "stroppy/") {
			return svc
		}
	}
	return nil
}

// stroppyImageTag extracts the tag portion of a ServiceSpec image
// ("stroppy:1.2.3" -> "1.2.3"). An image with no ":" (bare "stroppy",
// implicitly Docker's "latest") returns "".
func stroppyImageTag(image string) string {
	_, tag, _ := strings.Cut(image, ":")
	return tag
}

// joinDiagMessages flattens a diag.List's messages into one string, for the
// rare paths (ComposedSchema) that must fold a diag.List into a single Go
// error/RPC-status message rather than a Diagnostic list of their own.
func joinDiagMessages(diags diag.List) string {
	msgs := make([]string, 0, len(diags))
	for _, d := range diags {
		msgs = append(msgs, d.Message)
	}
	return strings.Join(msgs, "; ")
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
