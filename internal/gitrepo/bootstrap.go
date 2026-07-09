package gitrepo

import (
	"context"
	"fmt"
)

const (
	// InstanceRepoOwner is empty because the instance repo lives under the
	// service account user the server's GITEA_TOKEN belongs to (Gitea's
	// "/api/v1/user/repos" create-under-self endpoint), mirroring EnsureRepo's
	// owner="" convention.
	InstanceRepoOwner = ""
	// InstanceRepoName is the singleton instance repo's name: the global
	// catalog spec §3 C1 describes ("providers/<name>/...",
	// "workflows/<name>/...").
	InstanceRepoName = "instance-catalog"
	// InstanceRepoBranch is the branch Bootstrap seeds the on-disk layout on.
	// Gitea's auto_init:true (EnsureRepo's body) creates this as the repo's
	// default branch. Exported so other packages that read/write the
	// instance repo (catalog.GitBundleStore, internal/ide) target the same
	// branch without re-declaring the literal.
	InstanceRepoBranch = "main"
	// bootstrapAuthorName/Email stamp the layout-seed commit's author — a
	// service identity distinct from any real end-user, so this commit reads
	// as infrastructure bootstrap in `git log`, not an authored edit.
	bootstrapAuthorName  = "stroppy-bootstrap"
	bootstrapAuthorEmail = "bootstrap@stroppy.local"
)

// instanceLayout is the catalog's fixed on-disk layout (spec §3 C1),
// mirroring the directory constants internal/services/dsl/service.go
// declares for bundle files: clusterFile ("cluster.yaml"), providersDir
// ("providers"), manifestFile ("manifest.yaml"), moduleDirName ("module") —
// a workflow bundle lives at "workflows/<name>/cluster.yaml" +
// "workflows/<name>/workflow.yaml" (+ "workflows/<name>/components/*"), a
// provider bundle at "providers/<name>/manifest.yaml" (+
// "providers/<name>/module/*.tf"). Bootstrap seeds only the two top-level
// directories (as .gitkeep placeholders) — it does not invent example
// providers/workflows; SP-B's catalog seeding (internal/services/catalog's
// SeedOrgCatalog / BuiltinProviders) owns populating real entries.
var instanceLayout = map[string][]byte{
	"providers/.gitkeep": {},
	"workflows/.gitkeep": {},
	"README.md": []byte(
		"# instance-catalog\n\n" +
			"This repository is the SP-C internal-git instance catalog (spec\n" +
			"docs/superpowers/specs/2026-07-08-sp-c-internal-git-ide.md §3 C1).\n\n" +
			"Layout:\n" +
			"- providers/<name>/manifest.yaml (+ module/*.tf) — provider bundles\n" +
			"- workflows/<name>/cluster.yaml + workflow.yaml (+ components/*) — workflow bundles\n\n" +
			"Managed by internal/gitrepo.Bootstrap at server startup and by the\n" +
			"SP-B catalog service thereafter. Do not edit by hand outside the IDE.\n",
	),
}

// Bootstrap ensures the singleton instance repo exists and carries the
// catalog's fixed on-disk layout. Called once at server startup
// (internal/app/run.go): errors here fail server boot the same way an
// unreachable postgres does when Gitea is configured — the instance repo is
// load-bearing for the whole catalog, not an optional feature (see
// internal/app/run.go's call site doc for the deployment-optionality caveat
// this change actually ships: Bootstrap is only invoked when GiteaBackend/
// GiteaToken are both set, so existing deployments without Gitea provisioned
// yet are unaffected).
func Bootstrap(ctx context.Context, c *Client) error {
	if err := c.EnsureRepo(ctx, InstanceRepoOwner, InstanceRepoName, true); err != nil {
		return fmt.Errorf("gitrepo: ensure instance repo: %w", err)
	}
	if err := c.CommitFiles(ctx, InstanceRepoOwner, InstanceRepoName, InstanceRepoBranch, instanceLayout, bootstrapAuthorName, bootstrapAuthorEmail, "chore: seed catalog layout"); err != nil {
		return fmt.Errorf("gitrepo: seed instance repo layout: %w", err)
	}
	return nil
}
