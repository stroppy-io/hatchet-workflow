package orioledb

import (
	"fmt"
	"sort"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const containerName = "stroppy-orioledb"

// DeploymentRenderer implements the deployment renderer for OrioleDB.
type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == orioledbEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	wiring, err := orioledbResolveWiring(ctx)
	if err != nil {
		return nil, err
	}
	ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil), wiring)
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentDeployment(ctx, ec), nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	deps := deploymentbuilder.DependencyIDs(deploymentbuilder.RenderContext{
		Topology:  ctx.Topology,
		Component: ctx.Component,
		Node:      ctx.Node,
	}, nil)
	// Preview has no provisioned infrastructure, so peers are unresolved.
	ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deps, orioledbWiring{})
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

// orioledbWiring carries the runtime-resolved peers for one component.
type orioledbWiring struct {
	masterHost string   // private host of the master (streaming source); "" in preview
	dbHosts    []string // master + every replica private host (HAProxy backends)
}

func orioledbResolveWiring(ctx deploymentbuilder.RenderContext) (orioledbWiring, error) {
	masters, err := deploymentbuilder.ComponentTargets(ctx, func(c *topologypb.Component) bool {
		return c.GetEngine() == orioledbEngine && c.GetRole() == orioledbRoleMaster
	}, "rep", pgPort)
	if err != nil {
		return orioledbWiring{}, err
	}
	dbs, err := deploymentbuilder.ComponentTargets(ctx, func(c *topologypb.Component) bool {
		return c.GetEngine() == orioledbEngine && (c.GetRole() == orioledbRoleMaster || c.GetRole() == orioledbRoleReplica)
	}, "rep", pgPort)
	if err != nil {
		return orioledbWiring{}, err
	}
	w := orioledbWiring{}
	if len(masters) > 0 {
		w.masterHost = masters[0].Address
	}
	for _, d := range dbs {
		w.dbHosts = append(w.dbHosts, d.Address)
	}
	return w, nil
}

func orioledbEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	dbPackage *domain.Package,
	dependencies []string,
	wiring orioledbWiring,
) (deploymentbuilder.EngineComponent, error) {
	switch component.GetRole() {
	case orioledbRoleMaster:
		return orioledbDBComponent(component, database, dbPackage, dependencies, wiring, true)
	case orioledbRoleReplica:
		return orioledbDBComponent(component, database, dbPackage, dependencies, wiring, false)
	case orioledbRoleHaproxy:
		return orioledbHAProxyComponent(component, database, dependencies, wiring)
	default:
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported orioledb component role %q", component.GetRole())
	}
}

// orioledbDBComponent builds a master or replica DB container component. Both
// carry the per-node primary/replica health endpoint HAProxy probes.
func orioledbDBComponent(
	component *topologypb.Component,
	database *domain.Database,
	dbPackage *domain.Package,
	dependencies []string,
	wiring orioledbWiring,
	isMaster bool,
) (deploymentbuilder.EngineComponent, error) {
	params := database.GetParams().GetOrioledb()
	image := imageForVersion(database.GetParams().GetVersion())
	locale := params.GetInitdbLocale()
	if locale == "" {
		locale = "C"
	}
	configDir := deploymentbuilder.ConfigDir(component.GetId())
	configFile := orioledbDefaultConfigFile(configDir)

	extraFiles := orioledbHealthFiles()
	var unit string
	if isMaster {
		// The master must trust replication connections so a replica's
		// pg_basebackup can attach. POSTGRES_HOST_AUTH_METHOD=trust only writes a
		// `host all all all trust` pg_hba line (not the `replication` pseudo-db),
		// so append the replication line via an initdb.d hook (runs on first init,
		// before the final server start reads pg_hba).
		hbaPath := configDir + "/repl-hba.sh"
		extraFiles = append(extraFiles, orioledbReplHBAFile(hbaPath))
		unit = orioledbServiceUnit(component.GetId(), image, locale, streamingMasterOptions(pgOptions(params)), hbaPath)
	} else {
		master := wiring.masterHost
		if master == "" {
			master = "PREVIEW-MASTER" // preview: no resolved peer
		}
		unit = orioledbReplicaUnit(component.GetId(), image, master, streamingReplicaOptions(mergeMaps(pgOptions(params), params.GetReplicaOptions())))
	}

	return deploymentbuilder.EngineComponent{
		Engine:            orioledbEngine,
		GlobalPriority:    dbPriority(isMaster).global,
		NodePriority:      dbPriority(isMaster).node,
		Dependencies:      dependencies,
		ConfigArtifactID:  deploymentbuilder.ArtifactID(component.GetId(), "orioledb.env"),
		ConfigFile:        configFile,
		ConfigOrigin:      deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT,
		DefaultConfigFile: configFile,
		InstallCommands:   orioledbInstallCommands(dbPackage),
		ServiceFile:       deploymentbuilder.EngineServiceFile(component.GetId(), unit),
		Healthcheck:       "for i in $(seq 1 60); do docker exec " + containerName + " pg_isready -h 127.0.0.1 -p " + fmt.Sprint(pgPort) + " && exit 0; sleep 3; done; exit 1",
		ExtraFiles:        extraFiles,
		PostStartCommands: orioledbHealthPostStart(),
	}, nil
}

// orioledbReplHBAFile is a /docker-entrypoint-initdb.d hook the master mounts to
// trust replication connections (needed for replica pg_basebackup over TCP).
func orioledbReplHBAFile(path string) deploymentbuilder.EngineFile {
	return deploymentbuilder.EngineFile{
		StepID:        "055_write_repl_hba",
		StepOrder:     55,
		ArtifactName:  "repl-hba.sh",
		ArtifactLabel: "repl_hba",
		File: &common.File{
			Info:    &common.File_Info{Path: path, Mode: 0755, CreateParents: true},
			Content: &common.File_Text{Text: "#!/bin/sh\nset -e\necho \"host replication all all trust\" >> \"$PGDATA/pg_hba.conf\"\n"},
		},
	}
}

type prio struct{ global, node uint32 }

func dbPriority(isMaster bool) prio {
	if isMaster {
		return prio{20, 20}
	}
	return prio{30, 20}
}

// streamingMasterOptions overlays the streaming-master postgresql.conf knobs on
// top of the user's postgres_options (user values win). Enables WAL streaming so
// replicas can attach. Trust auth already permits replication connections.
func streamingMasterOptions(base map[string]string) map[string]string {
	out := map[string]string{
		// The OrioleDB/postgres image defaults listen_addresses to localhost, so a
		// replica on another node (and the stroppy runner) could not connect.
		"listen_addresses":      "*",
		"wal_level":             "replica",
		"max_wal_senders":       "10",
		"max_replication_slots": "10",
		"hot_standby":           "on",
	}
	for k, v := range base {
		out[k] = v
	}
	return out
}

// streamingReplicaOptions overlays the standby's required knobs (listen
// externally + hot_standby reads) on the user's replica_options (user wins).
func streamingReplicaOptions(base map[string]string) map[string]string {
	out := map[string]string{
		"listen_addresses": "*",
		"hot_standby":      "on",
	}
	for k, v := range base {
		out[k] = v
	}
	return out
}

func mergeMaps(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// orioledbDefaultConfigFile returns the env file written to the config dir.
// Trust auth (not a password) mirrors the native-Postgres deploy: the stroppy
// workload and the local postgres_exporter both connect passwordless over
// 127.0.0.1, so the container must accept unauthenticated local/TCP connections
// exactly like native pg. The EnvironmentFile in the systemd unit loads it.
func orioledbDefaultConfigFile(configDir string) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          configDir + "/orioledb.env",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: "POSTGRES_HOST_AUTH_METHOD=trust\n"},
	}
}

// pgOptions merges the explicit shared_buffers_mb knob into postgres_options.
func pgOptions(params *domain.OrioledbParams) map[string]string {
	out := map[string]string{}
	for k, v := range params.GetPostgresOptions() {
		out[k] = v
	}
	if mb := params.GetSharedBuffersMb(); mb > 0 {
		out["shared_buffers"] = fmt.Sprintf("%dMB", mb)
	}
	return out
}

// orioledbInstallCommands installs docker.io then configures + starts dockerd.
// dbPackage is always set for builtin orioledb.
func orioledbInstallCommands(dbPackage *domain.Package) []string {
	var commands []string
	if dbPackage != nil {
		commands = append(commands, dbPackage.GetPreInstall()...)
		if pkgs := dbPackage.GetAptPackages(); len(pkgs) > 0 {
			// Refresh the index first: the cloud-init apt update goes stale by
			// deploy time, so apt asks the apt-cacher for .deb versions the mirror
			// has already superseded -> 404. `apt-get update` realigns the index
			// with what the mirror (via apt-cacher) actually serves.
			commands = append(commands,
				"apt-get update",
				"DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(pkgs, " "))
		}
	}
	// Configure the registry mirror + start dockerd AFTER docker.io is installed
	// (docker.service does not exist before the apt install).
	commands = append(commands, dockerDaemonSetupCommands()...)
	return commands
}

// orioledbServiceUnit renders a systemd unit that pulls and runs the OrioleDB
// container with benchmark-faithful flags so the container does not skew the
// numbers vs bare metal: --network host (no NAT, 5432 directly on the node),
// --pid/--ipc host (no namespace isolation; shared memory like a host process),
// --privileged (direct/async IO, huge pages, sysctl access the engine may use),
// and a bind mount of the host data dir to PGDATA (writes hit the real disk, not
// docker's overlay/CoW layer). Trust auth is loaded from the EnvironmentFile.
func orioledbServiceUnit(componentID, image, locale string, options map[string]string, initHBAPath string) string {
	var optStr strings.Builder
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&optStr, " -c %s=%s", k, options[k])
	}
	dataDir := deploymentbuilder.DataDir(componentID)
	configDir := deploymentbuilder.ConfigDir(componentID)
	initMount := ""
	if initHBAPath != "" {
		initMount = fmt.Sprintf("  -v %s:/docker-entrypoint-initdb.d/00-repl-hba.sh:ro \\\n", deploymentbuilder.ShellQuote(initHBAPath))
	}
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud OrioleDB %s
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
EnvironmentFile=%s/orioledb.env
ExecStartPre=-/usr/bin/docker rm -f %s
ExecStartPre=/usr/bin/docker pull %s
ExecStartPre=/bin/mkdir -p %s
ExecStart=/usr/bin/docker run --rm --name %s --network host --pid host --ipc host --privileged \
  -e POSTGRES_HOST_AUTH_METHOD \
  -e POSTGRES_INITDB_ARGS=--locale=%s \
  -v %s:/var/lib/postgresql/data \
%s  %s postgres -D /etc/postgresql%s
ExecStop=/usr/bin/docker rm -f %s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`,
		componentID,
		configDir,
		containerName,
		image,
		deploymentbuilder.ShellQuote(dataDir),
		containerName,
		locale,
		deploymentbuilder.ShellQuote(dataDir),
		initMount,
		image,
		optStr.String(),
		containerName,
	)
}

// orioledbReplicaUnit renders a systemd unit whose container bootstraps a
// streaming standby: pg_basebackup into an empty PGDATA (idempotent), then exec
// the image's normal postgres entrypoint (-R wrote standby.signal +
// primary_conninfo). PGDATA defaults to /var/lib/postgresql/data in the image.
func orioledbReplicaUnit(componentID, image, masterHost string, options map[string]string) string {
	var optStr strings.Builder
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&optStr, " -c %s=%s", k, options[k])
	}
	dataDir := deploymentbuilder.DataDir(componentID)
	configDir := deploymentbuilder.ConfigDir(componentID)
	// MUST be a single line: systemd Exec lines cannot contain literal newlines
	// inside a quoted argument ("Unbalanced quoting"). Join with ';'.
	// -D /etc/postgresql keeps the image's orioledb config (shared_preload_libraries
	// = orioledb, default_table_access_method = orioledb); without it the standby
	// starts on the bare initdb config and cannot read the orioledb tables.
	inner := fmt.Sprintf(`set -e; if [ ! -s "$PGDATA/PG_VERSION" ]; then pg_basebackup -h %s -p %d -U postgres -D "$PGDATA" -R -X stream -P; test -f "$PGDATA/standby.signal"; fi; exec docker-entrypoint.sh postgres -D /etc/postgresql%s`,
		masterHost, pgPort, optStr.String())
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud OrioleDB replica %s
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
EnvironmentFile=%s/orioledb.env
ExecStartPre=-/usr/bin/docker rm -f %s
ExecStartPre=/usr/bin/docker pull %s
ExecStartPre=/bin/mkdir -p %s
ExecStart=/usr/bin/docker run --rm --name %s --network host --pid host --ipc host --privileged \
  -e POSTGRES_HOST_AUTH_METHOD \
  -v %s:/var/lib/postgresql/data \
  --entrypoint /bin/bash %s -c %s
ExecStop=/usr/bin/docker rm -f %s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`,
		componentID, configDir, containerName, image,
		deploymentbuilder.ShellQuote(dataDir), containerName,
		deploymentbuilder.ShellQuote(dataDir), image,
		deploymentbuilder.ShellQuote(inner), containerName,
	)
}

// orioledbHealthScript answers HAProxy's /primary and /replica probes from the
// local container's recovery state: pg_is_in_recovery()=f means this node is the
// primary. Forward-compatible with Patroni's REST (phase 2 swaps the responder).
func orioledbHealthScript() string {
	return `#!/bin/bash
read -r line
path=$(echo "$line" | awk '{print $2}')
rec=$(docker exec ` + containerName + ` psql -U postgres -tAc "SELECT pg_is_in_recovery()" 2>/dev/null | tr -d '[:space:]')
ok=$'HTTP/1.0 200 OK\r\nContent-Length: 2\r\n\r\nok'
no=$'HTTP/1.0 503 Service Unavailable\r\nContent-Length: 4\r\n\r\nfail'
case "$path" in
  /primary) if [ "$rec" = "f" ]; then printf '%s' "$ok"; else printf '%s' "$no"; fi ;;
  /replica) if [ "$rec" = "t" ]; then printf '%s' "$ok"; else printf '%s' "$no"; fi ;;
  *) printf '%s' "$no" ;;
esac
`
}

func orioledbHealthUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud OrioleDB health endpoint
After=docker.service

[Service]
ExecStart=/usr/bin/socat TCP-LISTEN:%d,reuseaddr,fork SYSTEM:/usr/local/bin/orioledb-health.sh
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`, healthPort)
}

// orioledbHealthFiles writes the per-node health responder script + its unit.
func orioledbHealthFiles() []deploymentbuilder.EngineFile {
	return []deploymentbuilder.EngineFile{
		{
			StepID:        "060_write_health_script",
			StepOrder:     60,
			ArtifactName:  "orioledb-health.sh",
			ArtifactLabel: "health_script",
			File: &common.File{
				Info:    &common.File_Info{Path: "/usr/local/bin/orioledb-health.sh", Mode: 0755, CreateParents: true},
				Content: &common.File_Text{Text: orioledbHealthScript()},
			},
		},
		{
			StepID:        "061_write_health_unit",
			StepOrder:     61,
			ArtifactName:  "orioledb-health.service",
			ArtifactLabel: "health_unit",
			File: &common.File{
				Info:    &common.File_Info{Path: "/etc/systemd/system/orioledb-health.service", Mode: 0644, CreateParents: true},
				Content: &common.File_Text{Text: orioledbHealthUnit()},
			},
		},
	}
}

// orioledbHealthPostStart installs socat and starts the health responder after
// the DB container is up (order 300+).
func orioledbHealthPostStart() []deploymentbuilder.EngineCommand {
	return []deploymentbuilder.EngineCommand{
		{StepID: "300_install_socat", StepOrder: 300, Command: "DEBIAN_FRONTEND=noninteractive apt-get install -y socat"},
		{StepID: "301_reload_systemd", StepOrder: 301, Command: "systemctl daemon-reload"},
		{StepID: "302_start_health", StepOrder: 302, Command: "systemctl enable --now orioledb-health"},
	}
}

// orioledbHAProxyComponent builds the native (apt) HAProxy LB node that splits
// write->primary (:5432) and read->replicas (:5433) via the per-node httpchk.
func orioledbHAProxyComponent(
	component *topologypb.Component,
	database *domain.Database,
	dependencies []string,
	wiring orioledbWiring,
) (deploymentbuilder.EngineComponent, error) {
	hosts := wiring.dbHosts
	if len(hosts) == 0 {
		hosts = []string{"PREVIEW-DB"} // preview: no resolved peers
	}
	cfg := orioledbHAProxyConfig(hosts, database.GetParams().GetOrioledb().GetHaproxyOptions())
	// Write to the component config dir, NOT /etc/haproxy/haproxy.cfg: the latter
	// is a dpkg conffile, and writing it before `apt-get install haproxy` makes
	// the install prompt on the existing conffile and fail non-interactively.
	cfgPath := deploymentbuilder.ConfigDir(component.GetId()) + "/haproxy.cfg"
	cfgFile := &common.File{
		Info:    &common.File_Info{Path: cfgPath, Mode: 0644, CreateParents: true},
		Content: &common.File_Text{Text: cfg},
	}
	return deploymentbuilder.EngineComponent{
		Engine:            orioledbEngine,
		GlobalPriority:    60,
		NodePriority:      50,
		Dependencies:      dependencies,
		ConfigArtifactID:  deploymentbuilder.ArtifactID(component.GetId(), "haproxy.cfg"),
		ConfigFile:        cfgFile,
		ConfigOrigin:      deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT,
		DefaultConfigFile: cfgFile,
		InstallCommands:   []string{"apt-get update", "DEBIAN_FRONTEND=noninteractive apt-get install -y haproxy"},
		ServiceFile:       deploymentbuilder.EngineServiceFile(component.GetId(), orioledbHAProxyUnit(cfgPath)),
		Healthcheck:       "systemctl is-active --quiet " + deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(component.GetId())),
	}, nil
}

func orioledbHAProxyUnit(cfgPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud OrioleDB HAProxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStartPre=/usr/sbin/haproxy -c -f %s
ExecStart=/usr/sbin/haproxy -W -db -f %s
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`, cfgPath, cfgPath)
}

// orioledbHAProxyConfig renders haproxy.cfg: a write frontend (:5432 -> primary)
// and a read frontend (:5433 -> replicas), each backend probing every DB node's
// :8008 health endpoint so membership tracks the live primary/replica roles.
func orioledbHAProxyConfig(dbHosts []string, opts map[string]string) string {
	maxconn := opts["maxconn"]
	if maxconn == "" {
		maxconn = "4096"
	}
	var servers strings.Builder
	for i, h := range dbHosts {
		fmt.Fprintf(&servers, "  server db%d %s:%d check port %d inter 2s\n", i+1, h, pgPort, healthPort)
	}
	srv := servers.String()
	return fmt.Sprintf(`global
  maxconn %s
defaults
  mode tcp
  timeout connect 5s
  timeout client 1m
  timeout server 1m

frontend write
  bind *:%d
  default_backend primary

frontend read
  bind *:%d
  default_backend replicas

backend primary
  option httpchk GET /primary
  http-check expect status 200
%s
backend replicas
  option httpchk GET /replica
  http-check expect status 200
%s`,
		maxconn, haproxyWritePort, haproxyReadPort, srv, srv)
}
