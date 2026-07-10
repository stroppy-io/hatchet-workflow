package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	dockerclient "github.com/docker/docker/client"
	"github.com/gopherex/protoc-gen-go-graphql/graphqlrt"
	graphqlhandler "github.com/graphql-go/handler"
	"github.com/ogen-go/ogen/middleware"
	"github.com/stroppy-io/schemapb/schemapb"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	yandextf "github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
	dslexamples "github.com/stroppy-io/stroppy-cloud/examples/dsl"
	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	domsettings "github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/gateway"
	"github.com/stroppy-io/stroppy-cloud/internal/gitrepo"
	"github.com/stroppy-io/stroppy-cloud/internal/ide"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/adapters"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/docker"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/execution"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/identity"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/provider"
	quotainfra "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/quotas"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/openapidoc"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/agentconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/apiconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/gqlapi"
	rest "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/rest"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog/catalogconnect"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl/dslconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/services/agent_shell"
	catalogsvc "github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/services/compare"
	dslsvc "github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/favorite"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
	packagessvc "github.com/stroppy-io/stroppy-cloud/internal/services/packages"
	publicrating "github.com/stroppy-io/stroppy-cloud/internal/services/public_rating"
	publicshare "github.com/stroppy-io/stroppy-cloud/internal/services/public_share"
	quotasvc "github.com/stroppy-io/stroppy-cloud/internal/services/quota"
	ratingsvc "github.com/stroppy-io/stroppy-cloud/internal/services/rating"
	recipesvc "github.com/stroppy-io/stroppy-cloud/internal/services/recipe"
	sharesvc "github.com/stroppy-io/stroppy-cloud/internal/services/share"
	stroppysvc "github.com/stroppy-io/stroppy-cloud/internal/services/stroppy"
	systemsettings "github.com/stroppy-io/stroppy-cloud/internal/services/system_settings"
	tenantdashboard "github.com/stroppy-io/stroppy-cloud/internal/services/tenant_dashboard"
	tenantsettings "github.com/stroppy-io/stroppy-cloud/internal/services/tenant_settings"
	testrunoverview "github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
	"github.com/stroppy-io/stroppy-cloud/internal/temporalopts"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
	"github.com/stroppy-io/stroppy-cloud/web"
)

// dashboardRatingMetricKey is the headline metric the tenant dashboard ranks its
// top-benchmark tile by.
const dashboardRatingMetricKey = "db_tps"

// Run boots the full control plane and blocks until ctx is cancelled, then tears
// everything down in reverse order. It owns the entire object graph.
func Run(ctx context.Context, cfg Config) error {
	log := slog.Default()

	// 1) Postgres store + transaction manager.
	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate postgres: %w", err)
	}
	store := postgres.New(db)
	trm := db.Trm()

	// 2) Temporal client. Raise the gRPC message size well above the 4 MiB
	// default so large self-hosted smoke WorkflowTasks do not wedge in a
	// ResourceExhausted retry loop.
	tc, err := client.Dial(client.Options{
		HostPort:          cfg.TemporalHostPort,
		Namespace:         cfg.TemporalNS,
		ConnectionOptions: temporalopts.ConnectionOptions(nil),
	})
	if err != nil {
		return fmt.Errorf("dial temporal: %w", err)
	}
	defer tc.Close()

	// 2.5) Gitea instance-repo bootstrap (SP-C git backend, spec
	// docs/superpowers/specs/2026-07-08-sp-c-internal-git-ide.md §3 C1).
	// Gated on GiteaToken being set (not just GiteaBackend, which defaults to
	// "http://gitea:3000" — see cmd/cli/serve_cmd.go): the credential is the
	// operator's signal that Gitea has actually been provisioned. Deployments
	// that have not yet set up Gitea are unaffected — this is a softer
	// posture than the plan's "fail server boot like an unreachable
	// postgres" recommendation, chosen because the gitea sidecar is not yet
	// wired into every existing deployment (including the live dev stand) and
	// this task explicitly must not force a redeploy there. Once Gitea is
	// universally provisioned, tighten this to unconditional (matching
	// postgres/temporal's fail-fast severity).
	// giteaClient is nil when GiteaToken is unset (feature off). It is reused
	// below both for catalogBundles (Task 4: selectable GitBundleStore) and
	// for internal/ide's per-org worktree manager — one client, one
	// credential, never duplicated.
	var giteaClient *gitrepo.Client
	if cfg.GiteaToken != "" {
		giteaClient, err = gitrepo.NewClient(gitrepo.Config{BaseURL: cfg.GiteaBackend, Token: cfg.GiteaToken})
		if err != nil {
			return fmt.Errorf("gitea client: %w", err)
		}
		if err := gitrepo.Bootstrap(ctx, giteaClient); err != nil {
			return fmt.Errorf("gitea instance-repo bootstrap: %w", err)
		}
	}

	// 3) Identity layer.
	idCfg := identity.Config{
		SigningSecret: cfg.JWTSecret,
		// A 32-byte AES key derived deterministically from the signing secret so
		// provider secrets can be sealed without a second configured key.
		SecretEncryptionKey: deriveSecretKey(cfg.JWTSecret),
	}
	tokenSvc, err := identity.NewJWTTokenService(idCfg, store.RefreshSessions())
	if err != nil {
		return fmt.Errorf("identity token service: %w", err)
	}
	agentTokens, err := agentdomain.NewTokenService(cfg.JWTSecret)
	if err != nil {
		return fmt.Errorf("agent token service: %w", err)
	}
	providerSecrets, err := identity.NewProviderSecrets(store.Secrets(), idCfg)
	if err != nil {
		return fmt.Errorf("identity provider secrets: %w", err)
	}
	settingsReader := identity.NewSettingsReader(store.Settings())
	gates := identity.NewGates(settingsReader)
	permResolver := identity.NewPermissionResolver(store.Memberships(), store.Roles())
	tenantReader := identity.NewTenantReader(store.Tenants())
	hasher := identity.NewBcryptHasher(0)
	apiTokenMinter := identity.NewApiTokenMinter()
	apiTokenSecrets := identity.NewApiTokenSecrets(store.Secrets())
	apiTokenVerifier := identity.NewApiTokenVerifier(store.ApiTokens(), store.Accounts(), apiTokenSecrets)
	bearerVerifier := identity.NewCompositeTokenVerifier(tokenSvc, apiTokenVerifier)
	authn := identity.NewJWTAuthnWithVerifier(bearerVerifier)
	authzGate := iamsvc.NewAuthInterceptor(bearerVerifier, permResolver)
	oidc := identity.NewOIDCFlows(idCfg, store.SSOStates())
	notifier := identity.NewSlogNotifier(log)
	catalog := identity.NewCatalog()
	ttl := identity.NewTokenTTL(idCfg)

	// First-boot seeding: on a brand-new database create the initial admin
	// account + default tenant + owner role + membership and default platform
	// settings, so the install has a principal that can log in. Idempotent: a
	// no-op once any account exists, and skipped entirely if no admin password
	// is configured. Runs after the store + identity layer are built, before
	// services start.
	if err := seedFirstBoot(ctx, log, store, db, hasher, catalog, cfg); err != nil {
		return fmt.Errorf("first-boot seeding: %w", err)
	}

	// 4) Execution layer.
	bid := byIDReader{db: db}

	// Settings resolver feeds the agent bootstrap baked into launched workflows.
	resolver := domsettings.Resolver{
		PlatformSource:           settingsReader,
		TenantSource:             tenantSettingsSource{repo: store.TenantSettings()},
		DefaultServerAddr:        cfg.AgentServerAddr,
		DefaultTemporalNamespace: cfg.TemporalNS,
	}

	snapReader := snapshotRunReader{r: bid}
	runtimeStore := runtimePersistenceStore{r: bid, runs: store.Runs()}
	runLogWriter := execution.NewRunLogWriter(cfg.MonitoringURL, cfg.MonitoringToken)
	runtimeActivities := execution.NewRunPersistenceActivities(runtimeStore, runLogWriter)
	agentRegistry := execution.NewAgentRegistryService(log)
	overviewReader := execution.NewOverviewReader(tc, snapReader, agentRegistry)
	logReader := execution.NewLogReader(cfg.MonitoringURL, cfg.MonitoringToken, log)
	agentLogIngest := execution.NewAgentLogIngestService(cfg.MonitoringURL, cfg.MonitoringToken, agentTokens, log)
	metricsReader := execution.NewMetricsReader(cfg.MonitoringURL, cfg.MonitoringToken, snapReader, log)

	stroppyVersions, err := adapters.NewGitHubStroppyVersionSource(adapters.GitHubStroppyVersionConfig{
		Repo:       cfg.StroppyGitHubRepo,
		MinVersion: cfg.StroppyMinVersion,
		Token:      cfg.StroppyGitHubToken,
	})
	if err != nil {
		return fmt.Errorf("stroppy versions: %w", err)
	}
	quotaSnapshotTTL, err := parseDurationDefault(cfg.QuotaSnapshotTTL, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("quota snapshot ttl: %w", err)
	}
	quotaReservationTTL, err := parseDurationDefault(cfg.QuotaReservationTTL, 30*time.Minute)
	if err != nil {
		return fmt.Errorf("quota reservation ttl: %w", err)
	}
	quotaRefreshInterval, err := parseDurationDefault(cfg.QuotaRefreshInterval, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("quota refresh interval: %w", err)
	}
	quotaStore := quotainfra.NewStore(db)
	quotaManager := quotainfra.NewManager(
		quotaStore,
		store.TenantSettings(),
		map[deploymentpb.Provider]quotainfra.ProviderSource{
			deploymentpb.Provider_PROVIDER_YANDEX: quotainfra.NewYandexSource(quotaSnapshotTTL),
			deploymentpb.Provider_PROVIDER_DOCKER: quotainfra.NewDockerSource(quotaSnapshotTTL),
		},
		quotainfra.ManagerConfig{
			SnapshotTTL:    quotaSnapshotTTL,
			ReservationTTL: quotaReservationTTL,
		},
	)
	quotainfra.StartRefresher(ctx, quotaManager, quotaRefreshInterval, log)

	// 5) Adapters layer.
	blobStore, err := adapters.NewLocalBlobStore(cfg.PackageBlobDir, "")
	if err != nil {
		return fmt.Errorf("packages blob store: %w", err)
	}
	storageKeys := adapters.NewStorageKeys()
	limits := adapters.NewStaticLimits(0)
	uploadTTL := adapters.NewStaticUploadTTL(0)
	tokenMinter := adapters.NewRandomTokenMinter(0)

	// catalogBundles is the SP-C seam's BundleStore impl. Default: a
	// filesystem tree rooted under cfg.CatalogBundleDir, mirroring
	// blobStore's local-filesystem convention above. When Gitea is
	// provisioned (giteaClient != nil, same GiteaToken gate as the
	// instance-repo bootstrap above) it is swapped for a GitBundleStore over
	// the singleton instance repo instead — selectable, defaulting OFF, so
	// every existing deployment (including the live dev stand, which does
	// not have GITEA_TOKEN set) keeps behaving exactly as before. Neither
	// catalogService nor dslService's provider resolver below change either
	// way — both depend only on the catalogsvc.BundleStore interface.
	var catalogBundles catalogsvc.BundleStore
	if giteaClient != nil {
		catalogBundles = catalogsvc.NewGitBundleStore(giteaClient, gitrepo.InstanceRepoOwner, gitrepo.InstanceRepoName, gitrepo.InstanceRepoBranch)
	} else {
		fsBundles, err := catalogsvc.NewFSBundleStore(cfg.CatalogBundleDir)
		if err != nil {
			return fmt.Errorf("catalog bundle store: %w", err)
		}
		catalogBundles = fsBundles
	}

	shareRuns := shareRunReader{r: bid}
	snapshotBuilder := adapters.NewRunSnapshotBuilder(shareRuns, metricsReader)

	compareRuns := runRecordGetter{r: bid}
	testRunReader := adapters.NewTestRunReader(compareRuns)
	metricsComparator := adapters.NewMetricsComparator(metricsReader)

	ratingNames := ratingNames{accounts: store.Accounts(), tenants: store.Tenants()}
	ratingRuns := ratingRunsLister{db: db, runs: store.Runs()}
	ratingBoard := adapters.NewRatingBoard(ratingRuns, metricsReader, ratingNames)
	publicRatingBoard := adapters.NewPublicRatingBoard(ratingRuns, metricsReader)

	// Only TestRuns and Recipes favorite targets are resolvable now that the
	// preset/suite services (and their repos) were deleted; DatabasePresets/
	// WorkloadPresets/TestPresets/Suites/SuiteRuns are left nil, so favoriting
	// those kinds reports not-found (favorite.TargetResolver's documented
	// behavior for an unresolvable getter) rather than referencing a repo that
	// no longer exists.
	favoriteTargets := adapters.NewFavoriteTargetResolver(adapters.FavoriteTargetRepos{
		// test_run is keyed by id only — the tenant is validated by the favorite
		// service after resolution. SP-E Task 5: reads run_records (models.Run)
		// rather than test_run_records, mirroring the run-model cutover.
		TestRuns: adapters.EntityGetterFunc(func(ctx context.Context, _, id string) (*commonEntity, error) {
			rec, err := bid.run(ctx, id)
			if err != nil {
				return nil, err
			}
			return rec.GetEntity(), nil
		}),
		// recipe is tenant-partitioned — pass tenantID through so the lookup
		// scopes to the caller's tenant.
		Recipes: adapters.EntityGetterFunc(func(ctx context.Context, tenantID, id string) (*commonEntity, error) {
			rec, err := store.Recipes().Get(ctx, tenantID, id)
			if err != nil {
				return nil, err
			}
			return rec.GetEntity(), nil
		}),
	})

	shellHub := adapters.NewInMemoryShellHub(log)
	machineLocator := adapters.NewMachineLocator(allMachinesResolver{}, shellHub)
	tenantGuard := adapters.NewTenantGuard(tenantMembership{memberships: store.Memberships()})
	shellAudit := adapters.NewSlogShellAudit(log)

	dashRuns := dashboardRuns{runs: store.Runs()}
	runStats := adapters.NewRunStatsReader(dashRuns)
	recentRuns := adapters.NewRecentRunsReader(dashRuns)
	scheduleReader := adapters.NewScheduleReader(dashRuns)
	dashRating := adapters.NewDashboardRatingReader(ratingBoard, dashboardRatingMetricKey)

	// 6) Services.

	// DslService is stateless (no XDeps: see internal/services/dsl's package
	// doc) — the browser IDE's schema/lint surface over internal/dsl, not
	// exposed via GraphQL/REST (map<string, bytes> has no clean surface
	// there; see (graphqlopt.service).skip on cloud/v1/dsl/service.proto).
	// WithVersionSource reuses the same stroppyVersions source ListStroppyVersions
	// serves (Task 7): Check/Preview add an advisory warning when a recipe's
	// stroppy service pins an image tag absent from that known-releases list,
	// so a bogus version fails fast in the editor instead of at container-pull.
	// WithProviderResolver (SP-B Task 9) resolves provider.use against the org
	// catalog (catalogEntries below), so a recipe/workflow bundle can
	// reference a catalog-managed provider by slug instead of shipping its own
	// providers/<name>/ subtree; constructed here (ahead of catalogService)
	// since it only needs the entry repo + bundle store, not catalogService
	// itself — catalogService's own Checker dependency closes the loop the
	// other direction, over dslService.CheckBundle below.
	catalogEntries := store.CatalogEntries()
	providerResolver := &catalogsvc.CatalogProviderResolver{Entries: catalogEntries, Bundles: catalogBundles}
	dslService := dslsvc.NewDslService(
		dslsvc.WithVersionSource(stroppyVersions),
		dslsvc.WithProviderResolver(providerResolver),
	)

	// catalogService is the SP-B catalog domain: provider/workflow objects
	// promoted to first-class catalog entries at LEVEL_INSTANCE/LEVEL_ORG.
	// BuiltinProviders seeds the docker builtin (no tf module — see
	// dslsvc.BuiltinDockerManifest) and the yandex builtin (real terraform
	// module, deployments/terraform/yandex — see yandextf.BuiltinCatalogFiles)
	// as LEVEL_INSTANCE rows; BuiltinWorkflows seeds the postgres-ha example
	// bundle as a launchable LEVEL_INSTANCE KIND_WORKFLOW row. Both are seeded
	// unconditionally at boot via SeedBuiltinCatalog below (P1), and again,
	// idempotently, whenever SeedOrgCatalog links a new tenant's org catalog.
	// Check reuses dslService's own check-mode compile pipeline for
	// KIND_WORKFLOW bundles, and a
	// manifest-only decode for KIND_PROVIDER bundles (see newCatalogChecker's
	// doc).
	// yandexCatalogFiles is the "yandex" builtin provider's catalog bundle
	// (manifest.yaml + module/*.tf), embedded independently of yandexTfFiles
	// below (that embed feeds the live terraform.Actor's moduleDir instead —
	// see its own doc comment) so it is available here, ahead of
	// catalogService's construction (P2/P3).
	yandexCatalogFiles, err := yandextf.BuiltinCatalogFiles()
	if err != nil {
		return fmt.Errorf("load embedded yandex catalog manifest: %w", err)
	}
	// postgresHABundle is the only example DSL bundle shipped today
	// (examples/dsl/postgres-ha) — seeded as the catalog's first launchable
	// KIND_WORKFLOW entry (P5) so a fresh install has something to launch.
	postgresHABundle, err := dslexamples.PostgresHABundle()
	if err != nil {
		return fmt.Errorf("load embedded postgres-ha example bundle: %w", err)
	}
	catalogService := catalogsvc.NewService(catalogsvc.Deps{
		Entries: catalogEntries,
		Bundles: catalogBundles,
		Check:   newCatalogChecker(dslService),
		Authn:   authn,
		BuiltinProviders: map[string]map[string][]byte{
			"docker": {"manifest.yaml": []byte(dslsvc.BuiltinDockerManifest)},
			"yandex": yandexCatalogFiles,
		},
		BuiltinWorkflows: map[string]map[string][]byte{
			dslexamples.PostgresHABundleName: postgresHABundle,
		},
	})
	// SeedBuiltinCatalog (P1): a stand whose tenant predates SeedOrgCatalog's
	// per-tenant-create wiring (internal/services/iam.Service's account
	// creation path) never gets its catalog seeded otherwise — this call is
	// idempotent and independent of any tenant existing, so it is safe (and
	// simplest) to run unconditionally on every boot, in addition to (not
	// instead of) SeedOrgCatalog's own seeding on new-tenant creation.
	if err := catalogService.SeedBuiltinCatalog(ctx); err != nil {
		return fmt.Errorf("seed builtin catalog: %w", err)
	}

	iamService := iamsvc.NewIamService(iamsvc.IamDeps{
		Authn:                authn,
		Authz:                permResolver,
		Accounts:             store.Accounts(),
		RegistrationRequests: store.RegistrationRequests(),
		Credentials:          store.Credentials(),
		Hasher:               hasher,
		Tokens:               tokenSvc,
		OneTimeTokens:        store.OneTimeTokens(),
		Notifier:             notifier,
		Gates:                gates,
		Catalog:              catalog,
		Tenants:              store.Tenants(),
		Roles:                store.Roles(),
		Memberships:          store.Memberships(),
		CatalogSeeder:        catalogService,
		Providers:            store.IdentityProviders(),
		ProviderSecrets:      providerSecrets,
		ExternalIdentities:   store.ExternalIdentities(),
		SSO:                  oidc,
		TTL:                  ttl,
		ApiTokens:            store.ApiTokens(),
		ApiTokenSecrets:      apiTokenSecrets,
		ApiTokenMinter:       apiTokenMinter,
		Tx:                   trm,
	})

	systemSettingsService := systemsettings.NewSystemSettingsService(systemsettings.SystemSettingsDeps{
		Authn:    authn,
		Settings: store.Settings(),
		Tx:       trm,
	})

	tenantSettingsService := tenantsettings.NewTenantSettingsService(tenantsettings.TenantSettingsDeps{
		Authn:    authn,
		Tenants:  tenantReader,
		Settings: store.TenantSettings(),
		Tx:       trm,
	})

	quotaService := quotasvc.NewService(quotasvc.Deps{
		Authn:   authn,
		Tenants: tenantReader,
		Runs:    store.Runs(),
		Quotas:  quotaManager,
	})

	packageService := packagessvc.NewPackageService(packagessvc.PackageDeps{
		Authn:    authn,
		Packages: store.Packages(),
		Blobs:    blobStore,
		Keys:     storageKeys,
		Limits:   limits,
		TTL:      uploadTTL,
		Tx:       trm,
	})

	testRunOverviewService := testrunoverview.NewTestRunOverviewService(testrunoverview.TestRunOverviewDeps{
		Authn:    authn,
		Runs:     store.Runs(),
		Overview: overviewReader,
		Logs:     logReader,
		Metrics:  metricsReader,
		Tx:       trm,
	})

	stroppyService := stroppysvc.NewService(stroppysvc.Deps{
		Authn:    authn,
		Versions: stroppyVersions,
	})

	compareService := compare.NewCompareService(compare.CompareDeps{
		Authn:   authn,
		Runs:    testRunReader,
		Metrics: metricsComparator,
		Tx:      trm,
	})

	ratingService := ratingsvc.NewRatingService(ratingsvc.RatingDeps{
		Authn:   authn,
		Board:   ratingBoard,
		Tenants: tenantReader,
		Tx:      trm,
	})

	publicRatingService := publicrating.NewPublicRatingService(publicrating.PublicRatingDeps{
		Board: publicRatingBoard,
		Tx:    trm,
	})

	shareService := sharesvc.NewShareService(sharesvc.ShareDeps{
		Authn:     authn,
		Shares:    store.Shares(),
		Minter:    tokenMinter,
		Snapshots: snapshotBuilder,
		Tx:        trm,
	})

	publicShareService := publicshare.NewPublicShareService(publicshare.PublicShareDeps{
		Shares: store.Shares(),
		Tx:     trm,
	})

	favoriteService := favorite.NewFavoriteService(favorite.FavoriteDeps{
		Authn:     authn,
		Favorites: store.Favorites(),
		Targets:   favoriteTargets,
		Tx:        trm,
	})

	agentShellService := agent_shell.NewAgentShellService(agent_shell.AgentShellDeps{
		Authn:    authn,
		Tenants:  tenantGuard,
		Machines: machineLocator,
		Hub:      shellHub,
		Audit:    shellAudit,
		Tx:       trm,
	})

	tenantDashboardService := tenantdashboard.NewTenantDashboardService(tenantdashboard.TenantDashboardDeps{
		Authn:    authn,
		Tenants:  tenantReader,
		RunStats: runStats,
		Recent:   recentRuns,
		Schedule: scheduleReader,
		Rating:   dashRating,
		Tx:       trm,
	})

	// recipeWorkflows launches RunRecipeWorkflow for RecipeService.StartRun.
	recipeWorkflows := execution.NewRecipeWorkflows(tc, resolver, log)

	// recipeActivities backs RunRecipeWorkflow's three by-name activities
	// (CompileRecipeActivity/ProvisionActivity/TeardownActivity — see
	// internal/workflows/register.go's RegisterRecipeActivities) with the
	// real provider executors: a docker.Executor for the builtin "docker"
	// provider and a terraform.Actor for terraform-module providers (today
	// only the builtin "yandex" module, resolved through ModuleDir below).
	// Neither the old (pre-DSL-pivot) deployment flow nor any other app
	// wiring constructs these today, so this is their first real
	// construction site; recipe-supplied provider modules (phase 1A recipe
	// storage) are a later ModuleDir extension, not needed for the builtin
	// providers wired here.
	dockerExecutor, err := docker.NewExecutor()
	if err != nil {
		return fmt.Errorf("docker executor: %w", err)
	}
	terraformActor, err := terraform.NewActor()
	if err != nil {
		return fmt.Errorf("terraform actor: %w", err)
	}
	// providerDeployCreds resolves per-tenant terraform deploy credentials
	// (F1, docs/superpowers/specs/2026-07-08-sp-f-execution-shapeup.md §3
	// F1): each tenant's YC_TOKEN and friends are sealed at rest (same
	// SecretEncryptionKey/mechanism as providerSecrets above, distinct
	// namespace) and resolved fresh inside ProvisionActivity/TeardownActivity
	// per call — never built once, process-wide, from os.Getenv, and never
	// carried as a plaintext value through Temporal workflow input (only the
	// TenantID identifier crosses that boundary; see provider.Deps.EnvFn's
	// doc comment and ProvisionActivityInput/TeardownActivityInput.TenantID).
	providerDeployCreds, err := identity.NewProviderDeployCreds(store.Secrets(), idCfg)
	if err != nil {
		return fmt.Errorf("provider deploy creds: %w", err)
	}
	// yandexTfFiles is the "yandex" terraform module's embedded HCL, loaded
	// once here (I/O only happens at wiring time) and handed to
	// YandexModuleDirResolver, which derives a per-run workdir/state id from
	// RunID (F4) instead of the pre-F4 constant "yandex" two runs would
	// otherwise collide on.
	yandexTfFiles, err := yandextf.EmbeddedTfFiles()
	if err != nil {
		return fmt.Errorf("load embedded yandex terraform files: %w", err)
	}
	providerDeps := provider.Deps{
		DockerExec: provider.NewDockerExecutorExec(dockerExecutor),
		Actor:      terraformActor,
		EnvFn:      providerDeployCreds.Get,
		// LogSinkFn (F2) mirrors every terraform apply/destroy's output into a
		// run_id-prefixed line written to the control-plane's own logger, in
		// addition to the terraform.Actor's process-wide default writer. This
		// is a v1 stopgap, not the target backend: the target (Vector/
		// VictoriaMetrics Logs, matching agent-side logs — product-vision §6)
		// needs the control-plane process to ship logs the way agent nodes'
		// Vector sidecar does, which it does not today (spec §9.5, open
		// question 5) — tracked, not solved, by this wiring.
		LogSinkFn: func(_ context.Context, runID string) (io.Writer, io.Writer) {
			return &slogLineWriter{log: log, runID: runID, stream: "stdout"},
				&slogLineWriter{log: log, runID: runID, stream: "stderr"}
		},
		AgentTokens: agentTokens,
		ModuleDir:   provider.YandexModuleDirResolver(yandexTfFiles),
	}
	recipeActivities := execution.NewRecipeActivities(providerDeps, quotaManager)
	recipeService := recipesvc.NewService(recipesvc.Deps{
		Repo:  store.Recipes(),
		Authn: authn,
		// dslService.CheckBundle (the *DslService method), not the package-level
		// dslsvc.CheckBundle function: the method applies the same advisory
		// stroppy-version diagnostic (Task 7's validateStroppyVersion, via the
		// VersionSource dslService was constructed with above) that
		// DslService.Check already gives the live editor, so a stored recipe's
		// Create/CheckRecipe path warns on a bogus stroppy version too.
		Checker: dslService.CheckBundle,
		// FormSchema composes the launch-form schemapb.Schema for a stored
		// bundle's files (workflow.inputs ⊕ provider.params). Wrapped in a
		// closure (rather than passed as a bare method/func value) because
		// dslsvc.ComposeLaunchFormSchema's package-level signature — files
		// only, no ctx — doesn't match Deps.FormSchema's ctx-carrying shape
		// (kept for consistency with Checker/every other Deps func).
		FormSchema: func(_ context.Context, files map[string][]byte) (*schemapb.Schema, diag.List, error) {
			return dslsvc.ComposeLaunchFormSchema(files)
		},
		Runs:      store.Runs(),
		Workflows: recipeWorkflows,
	})

	// 7) Connect handlers + embedded SPA on one mux.
	mux := http.NewServeMux()
	handlerOpts := []connect.HandlerOption{
		connect.WithInterceptors(grpcStatusToConnect{}, requestValidationInterceptor{}, authzGate.Connect()),
	}
	agentLogHandlerOpts := []connect.HandlerOption{
		connect.WithInterceptors(grpcStatusToConnect{}, requestValidationInterceptor{}),
	}
	agentHandlerOpts := []connect.HandlerOption{
		connect.WithInterceptors(grpcStatusToConnect{}, requestValidationInterceptor{}, execution.NewAgentAuthInterceptor(agentTokens)),
	}
	register(mux,
		func() (string, http.Handler) {
			return agentconnect.NewAgentLogServiceHandler(agentLogIngest, agentLogHandlerOpts...)
		},
		func() (string, http.Handler) {
			return agentconnect.NewAgentRegistryServiceHandler(agentRegistry, agentHandlerOpts...)
		},
		func() (string, http.Handler) { return apiconnect.NewIamServiceHandler(iamService, handlerOpts...) },
		func() (string, http.Handler) {
			return apiconnect.NewSystemSettingsServiceHandler(systemSettingsService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewTenantSettingsServiceHandler(tenantSettingsService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewQuotaServiceHandler(quotaService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewPackageServiceHandler(packageService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewTestRunOverviewServiceHandler(testrunoverview.NewConnectHandler(testRunOverviewService), handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewStroppyServiceHandler(stroppyService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewRecipeServiceHandler(recipeService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewCompareServiceHandler(compareService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewRatingServiceHandler(ratingService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewPublicRatingServiceHandler(publicRatingService, handlerOpts...)
		},
		func() (string, http.Handler) { return apiconnect.NewShareServiceHandler(shareService, handlerOpts...) },
		func() (string, http.Handler) {
			return apiconnect.NewPublicShareServiceHandler(publicShareService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewFavoriteServiceHandler(favoriteService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return apiconnect.NewTenantDashboardServiceHandler(tenantDashboardService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return dslconnect.NewDslServiceHandler(dslService, handlerOpts...)
		},
		func() (string, http.Handler) {
			return catalogconnect.NewCatalogServiceHandler(catalogService, handlerOpts...)
		},
	)
	mux.Handle(blobStore.UploadPathPrefix()+"/", blobStore.UploadHandler())
	mux.Handle(blobStore.DownloadPathPrefix()+"/", blobStore.DownloadHandler(agentTokens))

	// GraphQL: one endpoint over the SAME API gRPC handlers. The generated
	// resolvers delegate to the pb.*ServiceServer impls registered above; Authorize
	// applies the identical per-method authn+authz as the connect interceptors
	// (the HTTP wrapper bridges the Authorization header into gRPC metadata so the
	// shared authenticate() path works). Queries/mutations over HTTP POST/GET;
	// subscriptions (server-streaming RPCs) over the graphql-transport-ws protocol.
	gqlSchema, err := gqlapi.NewSchema(&gqlapi.Server{
		CompareService:         compareService,
		FavoriteService:        favoriteService,
		IamService:             iamService,
		PackageService:         packageService,
		RatingService:          ratingService,
		PublicRatingService:    publicRatingService,
		PublicShareService:     publicShareService,
		QuotaService:           quotaService,
		RecipeService:          recipeService,
		ShareService:           shareService,
		StroppyService:         stroppyService,
		SystemSettingsService:  systemSettingsService,
		TenantDashboardService: tenantDashboardService,
		TenantSettingsService:  tenantSettingsService,
		TestRunOverviewService: testRunOverviewService,
		Authorize:              authzGate.AuthorizeGraphQL,
	})
	if err != nil {
		return fmt.Errorf("graphql schema: %w", err)
	}
	// Bridge the bearer credential into the gRPC incoming metadata the shared
	// authenticate() path reads (for both HTTP and the WS upgrade request).
	gqlAuthCtx := func(r *http.Request) context.Context {
		ctx := r.Context()
		if authz := r.Header.Get("Authorization"); authz != "" {
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authz))
		}
		return ctx
	}
	// graphql-go/handler's built-in GraphiQL pins React 15 / GraphiQL 0.x from a
	// dead jsdelivr path (blank page), so serve our own modern GraphiQL build
	// (see graphiqlPage) on a browser GET instead. The IDE auto-documents the
	// schema via introspection; real queries use its header editor (bridged into
	// gRPC metadata by gqlAuthCtx).
	gqlHTTP := graphqlhandler.New(&graphqlhandler.Config{Schema: &gqlSchema, Pretty: true})
	gqlWS := graphqlrt.SubscriptionHandler(&gqlSchema, gqlAuthCtx)
	mux.Handle("/graphql", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Subscriptions arrive as a websocket upgrade; queries/mutations as POST/GET.
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			gqlWS.ServeHTTP(w, r)
			return
		}
		// A browser navigating to /graphql (GET, wants HTML) gets the IDE.
		if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") && r.URL.Query().Get("query") == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(graphiqlPage)
			return
		}
		gqlHTTP.ContextHandler(gqlAuthCtx(r), w, r)
	}))

	// REST/OpenAPI: a second HTTP surface over the SAME API gRPC handlers
	// (the ogen adapter delegates to the pb.*ServiceServer impls). Like the
	// GraphQL path it bypasses the connect/gRPC interceptors, so an ogen
	// middleware re-applies the identical per-method authn+authz: it bridges
	// the Authorization header into gRPC incoming metadata and calls the
	// shared AuthorizeGraphQL gate keyed by the gRPC procedure.
	ogenAdapter := api.NewOgenAdapter(
		compareService,
		favoriteService,
		iamService,
		packageService,
		ratingService,
		publicRatingService,
		publicShareService,
		quotaService,
		recipeService,
		shareService,
		stroppyService,
		systemSettingsService,
		tenantDashboardService,
		tenantSettingsService,
		testRunOverviewService,
	)
	restProcedures := apiProcedureByMethod()
	restAuthMW := func(req middleware.Request, next middleware.Next) (middleware.Response, error) {
		ctx := req.Context
		if authz := req.Raw.Header.Get("Authorization"); authz != "" {
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authz))
		}
		procedure, ok := restProcedures[req.OperationName]
		if !ok {
			return middleware.Response{}, fmt.Errorf("rest auth: no gRPC procedure for operation %q", req.OperationName)
		}
		ctx, err := authzGate.AuthorizeGraphQL(ctx, procedure, req.Body)
		if err != nil {
			return middleware.Response{}, err
		}
		req.SetContext(ctx)
		return next(req)
	}
	restSrv, err := rest.NewServer(ogenAdapter, rest.WithMiddleware(restAuthMW))
	if err != nil {
		return fmt.Errorf("rest server: %w", err)
	}
	// The generated ogen routes already carry the full /api/v1/... prefix, so
	// mount the server at that subtree WITHOUT stripping (stripping would leave
	// ogen with /v1/... and 404 every call).
	mux.Handle("/api/v1/", restSrv)
	// Docs live under /api/ but outside /api/v1/, so they need their own routes.
	mux.HandleFunc("/api/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(openapidoc.Spec)
	})
	mux.HandleFunc("/api/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(swaggerPage)
	})

	spa, err := spaHandler()
	if err != nil {
		return fmt.Errorf("embedded spa: %w", err)
	}
	mux.Handle("/", spa)

	// TestRunOverview (server-streaming) and AgentShell (bidi-streaming) are
	// gRPC-native streaming services: their generated method sets use
	// grpc.ServerStream / grpc.BidiStream, which the connect handler interfaces do
	// not accept. They are served by a real grpc.Server multiplexed onto the same
	// HTTP/2 cleartext handler by content-type ("application/grpc").
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcValidationUnary(), authzGate.Unary()),
		grpc.ChainStreamInterceptor(grpcValidationStream(), authzGate.Stream()),
	)
	api.RegisterTestRunOverviewServiceServer(grpcSrv, testRunOverviewService)
	api.RegisterAgentShellServiceServer(grpcSrv, agentShellService)

	// Wrap with h2c so connect-over-HTTP/2 cleartext works behind the gateway, and
	// route native gRPC traffic to the grpc.Server.
	h2cHandler := h2c.NewHandler(grpcOrHTTP(grpcSrv, mux), &http2.Server{})

	// 8) Temporal server worker.
	w := worker.New(tc, "stroppy-cloud", worker.Options{})
	workflows.RegisterWorkflows(w)
	workflows.RegisterActivities(w, runtimeActivities)
	workflows.RegisterRecipeActivities(w, recipeActivities)
	if err := w.Start(); err != nil {
		return fmt.Errorf("start temporal worker: %w", err)
	}
	defer w.Stop()

	// 8.5) Task 4: per-org code-server lifecycle + worktree materialization
	// (spec SP-C §3 C2/C4), gated on IdeManagerEnabled — distinct from the
	// GiteaToken gate above because this spins up real docker containers,
	// not just a storage backend swap. When disabled (the default),
	// ideBackends/ideAuthorizer stay nil and gateway.Config falls back to
	// the manual cfg.IdeBackend override (or 404s, same as before this
	// task).
	var ideBackends gateway.IdeBackendResolver
	var ideAuthorizer gateway.IdeAuthorizer
	if cfg.IdeManagerEnabled {
		if giteaClient == nil {
			return fmt.Errorf("ide manager requires GITEA_TOKEN to be set")
		}
		instanceOwner, err := giteaClient.Whoami(ctx)
		if err != nil {
			return fmt.Errorf("ide manager: resolve instance repo owner: %w", err)
		}
		dockerCli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
		if err != nil {
			return fmt.Errorf("ide manager: docker client: %w", err)
		}
		ideNetwork := cfg.IdeDockerNetwork
		if ideNetwork == "" {
			ideNetwork = cfg.AttachNetwork
		}
		ideManager := ide.NewManager(ide.Config{
			Docker:         dockerCli,
			Gitea:          giteaClient,
			GiteaBaseURL:   cfg.GiteaBackend,
			GiteaToken:     cfg.GiteaToken,
			WorktreeRoot:   cfg.IdeWorktreeRoot,
			WorktreeVolume: cfg.IdeWorktreeVolume,
			Image:          cfg.IdeImage,
			Network:        ideNetwork,
			InstanceOwner:  instanceOwner,
			LSPBinaryPath:  cfg.IdeLSPBinaryPath,
		})
		ideBackends = &ide.BackendResolver{Manager: ideManager, Tenants: store.Tenants()}
		ideAuthorizer = &ide.Authorizer{Tokens: bearerVerifier, Perms: permResolver, Tenants: store.Tenants()}
	}

	// 9) Gateway: single agent-facing entrypoint sharing the connect API + SPA.
	gw, err := gateway.New(gateway.Config{
		TemporalHostPort:  cfg.TemporalHostPort,
		AgentBinaryPath:   cfg.AgentBinaryPath,
		CacheDir:          cfg.CacheDir,
		Artifacts:         map[string]string{"stroppy": cfg.StroppyUpstream},
		AptBackend:        cfg.AptBackend,
		MonitoringBackend: cfg.MonitoringURL,   // relay agent /insert/* + /select/* -> vmauth
		MonitoringToken:   cfg.MonitoringToken, // backend monitoring bearer injected into vmauth
		AgentTokens:       agentTokens,
		GrafanaBackend:    cfg.GrafanaBackend,  // serve /grafana/* from the server origin
		RegistryBackend:   cfg.RegistryBackend, // serve /v2/* registry mirror from the server origin
		// IdeBackend is the manual-override fallback (see its doc); IdeBackends
		// (Task 4's per-org manager) takes priority whenever IdeManagerEnabled
		// wired it above — see gateway.go's New().
		IdeBackend:    cfg.IdeBackend,
		IdeBackends:   ideBackends,
		IdeAuthorizer: ideAuthorizer,
		HTTPFallback:  h2cHandler,
		Logger:        log,
	})
	if err != nil {
		return fmt.Errorf("build gateway: %w", err)
	}

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.ListenAddr, err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- gw.Serve(lis) }()
	log.Info("control plane listening", slog.String("addr", cfg.ListenAddr))

	select {
	case <-ctx.Done():
		gw.Close()
		return nil
	case err := <-serveErr:
		gw.Close()
		if err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("gateway serve: %w", err)
		}
		return nil
	}
}

// grpcOrHTTP dispatches native gRPC requests (HTTP/2 with an application/grpc
// content-type) to the gRPC server and everything else (connect, SPA) to the
// HTTP mux, sharing one HTTP/2 cleartext handler.
// swaggerPage renders the REST OpenAPI bundle (served at /api/openapi.yaml) via
// Swagger UI: an interactive doc with Try-it-out, an Authorize box and live
// requests against the same-origin /api surface.
var swaggerPage = []byte(`<!DOCTYPE html>
<html>
  <head>
    <title>Stroppy Cloud API</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css"/>
    <style>body { margin: 0; }</style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script crossorigin src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
    <script crossorigin src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-standalone-preset.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: '/api/openapi.yaml',
        dom_id: '#swagger-ui',
        deepLinking: true,
        persistAuthorization: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: 'StandaloneLayout',
      });
    </script>
  </body>
</html>`)

// graphiqlPage is a modern, self-hosted-markup GraphiQL build (the upstream
// graphql-go/handler page pins React 15 from a dead CDN path). It posts to the
// same-origin /graphql endpoint; the header editor lets you set Authorization.
var graphiqlPage = []byte(`<!DOCTYPE html>
<html>
  <head>
    <title>Stroppy Cloud GraphQL</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <link rel="stylesheet" href="https://unpkg.com/graphiql@3.8.3/graphiql.min.css"/>
    <style>html, body, #graphiql { height: 100%; margin: 0; overflow: hidden; }</style>
  </head>
  <body>
    <div id="graphiql">Loading GraphiQL…</div>
    <script crossorigin src="https://unpkg.com/react@18.2.0/umd/react.production.min.js"></script>
    <script crossorigin src="https://unpkg.com/react-dom@18.2.0/umd/react-dom.production.min.js"></script>
    <script crossorigin src="https://unpkg.com/graphiql@3.8.3/graphiql.min.js"></script>
    <script>
      const fetcher = GraphiQL.createFetcher({ url: '/graphql' });
      const root = ReactDOM.createRoot(document.getElementById('graphiql'));
      root.render(React.createElement(GraphiQL, { fetcher, defaultEditorToolsVisibility: true }));
    </script>
  </body>
</html>`)

// apiProcedureByMethod maps each cloud.v1.api gRPC method name to its full gRPC
// procedure ("/cloud.v1.api.IamService/Login"). ogen uses the proto method name
// as the (unique) operation name, so the REST auth middleware can recover the
// procedure the AuthorizeGraphQL gate needs from middleware.Request.OperationName.
func apiProcedureByMethod() map[string]string {
	out := map[string]string{}
	protoregistry.GlobalFiles.RangeFilesByPackage("cloud.v1.api", func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				out[string(m.Name())] = "/" + string(svc.FullName()) + "/" + string(m.Name())
			}
		}
		return true
	})
	return out
}

func grpcOrHTTP(grpcSrv *grpc.Server, httpHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcSrv.ServeHTTP(w, r)
			return
		}
		httpHandler.ServeHTTP(w, r)
	})
}

// register wires each (path, handler) pair from the connect handler constructors
// into the mux.
func register(mux *http.ServeMux, handlers ...func() (string, http.Handler)) {
	for _, h := range handlers {
		path, handler := h()
		mux.Handle(path, handler)
	}
}

// spaHandler serves the embedded SPA (web.Dist/dist) as the fallback for every
// non-API, non-/cloud. path, falling back to index.html for client-side routes.
func spaHandler() (http.Handler, error) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the asset if it exists; otherwise fall back to index.html so the
		// SPA router can resolve the deep link.
		if _, statErr := fs.Stat(dist, trimLeadingSlash(r.URL.Path)); statErr == nil || r.URL.Path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	}), nil
}

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}

func parseDurationDefault(raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	return time.ParseDuration(raw)
}

// slogLineWriter adapts *slog.Logger to io.Writer for F2's LogSinkFn: every
// terraform apply/destroy write (a line, or a chunk of lines, from
// tfexec's stdout/stderr) is logged with run_id/stream fields so it can be
// found later. This is a v1 stopgap, not the target log backend — the
// control-plane process has no Vector sidecar of its own the way agent
// nodes do (product-vision §6, spec §9.5 open question 5), so this per-run
// output lands in the control-plane's own structured log rather than the
// same pipeline agent-side logs use. Reachable via `log_ref` fields SP-E can
// key on (run_id) even before a dedicated backend exists.
type slogLineWriter struct {
	log    *slog.Logger
	runID  string
	stream string
}

func (w *slogLineWriter) Write(p []byte) (int, error) {
	w.log.Info("provisioning output",
		slog.String("run_id", w.runID),
		slog.String("stream", w.stream),
		slog.String("component", "terraform"),
		slog.String("line", strings.TrimRight(string(p), "\n")),
	)
	return len(p), nil
}
