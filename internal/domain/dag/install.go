package dag

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

// HandlerAgentCommand marks an agent-locus node. The server never runs it; the
// agent leases it via Poll and executes the embedded ops.Operation.
const HandlerAgentCommand = "agent.command"

// RecipeInstallBuilder is the CONCRETE InstallBuilder: it builds the install_and_run
// sub-dag from the engine recipes (the compat matrix) + the topology graph —
// per-component agent install chains (kind-ranked, replication-ordered) plus the
// run_stroppy workload node. Every node is AGENT-locus: the agent on each host
// leases it (Poll) and runs it locally via opexec, then Reports. This is the real
// "install the databases + run stroppy" code, reused by the dag blueprint.
type RecipeInstallBuilder struct{}

var _ InstallBuilder = RecipeInstallBuilder{}

func (RecipeInstallBuilder) Build(preset *domain.TestPreset, runID string) *primitive.Dag {
	sub, err := BuildInstallDag(preset, runID)
	if err != nil {
		// Real callers validate the preset at plan time; an empty sub-dag keeps the
		// surrounding deploy/teardown dag well-formed for illustration.
		return &primitive.Dag{Id: "install_and_run", Status: primitive.Status_STATUS_PENDING}
	}
	return sub
}

// kindRank orders component install across the topology: coordinators (etcd) come
// up first, then databases, then proxies/monitors, then the workload.
func kindRank(k domain.Topology_Component_Kind) int {
	switch k {
	case domain.Topology_Component_KIND_COORDINATOR:
		return 0
	case domain.Topology_Component_KIND_DATABASE:
		return 1
	case domain.Topology_Component_KIND_PROXY, domain.Topology_Component_KIND_MONITOR:
		return 2
	case domain.Topology_Component_KIND_STROPPY:
		return 3
	default:
		return 1
	}
}

// componentGroup is one component's node chain plus its install rank.
type componentGroup struct {
	id    string
	rank  int
	nodes []*primitive.Dag_Node
}

// BuildInstallDag builds the install_and_run sub-dag: per-component agent recipe
// chains (kind-ranked, replication-ordered) plus the run_stroppy workload node —
// all agent-locus nodes the agent leases and runs on its host. This is the concrete
// "install the databases + run stroppy" code reused by the dag blueprint.
// BuildInstallDag builds the per-component install/run sub-dag. runID is the OWNING
// run's dag id — it labels every metric (vmagent/stroppy run_id) and every Vector log
// line (dag_id), so QueryRunLogs/GetRunMetrics (which key on the run's dag id)
// correlate logs + metrics to the run across agent-command output AND Vector/vmagent.
func BuildInstallDag(preset *domain.TestPreset, runID string) (*primitive.Dag, error) {
	topo := preset.GetTopology()
	db := preset.GetDatabase()
	if runID == "" {
		runID = ids.New()
	}

	var groups []componentGroup
	byID := map[string]*componentGroup{}
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			nodes, err := componentChain(c, db, preset.GetWorkload(), topo, runID, int(m.GetMemoryGb())*1024)
			if err != nil {
				return nil, err
			}
			if len(nodes) == 0 {
				continue // AGENT/ADDON, or a component with nothing to do
			}
			groups = append(groups, componentGroup{id: c.GetId(), rank: kindRank(c.GetKind()), nodes: nodes})
			byID[c.GetId()] = &groups[len(groups)-1]
		}
	}

	var allNodes []*primitive.Dag_Node
	var edges []*primitive.Dag_Edge

	// Per-MACHINE monitoring chain: node_exporter + (DB exporter) + vmagent on
	// every host, remote_writing to the server's VictoriaMetrics ingest. Monitoring
	// runs in PARALLEL with the component install (no cross edge to the DB) — vmagent
	// retries scraping a not-yet-up exporter, so there is no hard ordering need.
	for _, m := range topo.GetMachines() {
		monNodes, err := monitorChain(m, db, runID)
		if err != nil {
			return nil, err
		}
		allNodes = append(allNodes, monNodes...)
		for j := 1; j < len(monNodes); j++ {
			edges = append(edges, chainEdge(monNodes[j-1], monNodes[j])) // sequential within the machine
		}
	}

	byRank := map[int][]*componentGroup{}
	for i := range groups {
		g := &groups[i]
		allNodes = append(allNodes, g.nodes...)
		for j := 1; j < len(g.nodes); j++ {
			edges = append(edges, chainEdge(g.nodes[j-1], g.nodes[j])) // intra-component chain
		}
		byRank[g.rank] = append(byRank[g.rank], g)
	}

	// Cross-rank ordering: every component of rank r must finish before any
	// component of the next present rank starts.
	ranks := presentRanks(byRank)
	for i := 1; i < len(ranks); i++ {
		for _, prev := range byRank[ranks[i-1]] {
			for _, cur := range byRank[ranks[i]] {
				edges = append(edges, chainEdge(last(prev.nodes), cur.nodes[0]))
			}
		}
	}
	// REPLICATION connections refine intra-database order: source (primary) before target.
	for _, conn := range topo.GetConnections() {
		if conn.GetKind() != domain.Topology_Connection_KIND_REPLICATION {
			continue
		}
		src, okS := byID[conn.GetFrom()]
		tgt, okT := byID[conn.GetTo()]
		if okS && okT && src.rank == tgt.rank {
			edges = append(edges, chainEdge(last(src.nodes), tgt.nodes[0]))
		}
	}

	sub := &primitive.Dag{
		Id:     ids.New(),
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  allNodes,
		Edges:  edges,
	}
	return sub, nil
}

// componentChain builds one component's ordered agent.command nodes. STROPPY is a
// single run node; AGENT/ADDON contribute nothing.
func componentChain(c *domain.Topology_Component, db *domain.Database, wl *domain.Workload, topo *domain.Topology, runID string, memoryMB int) ([]*primitive.Dag_Node, error) {
	switch c.GetKind() {
	case domain.Topology_Component_KIND_STROPPY:
		n, err := stroppyNode(c, db, wl, topo, runID)
		if err != nil {
			return nil, err
		}
		return []*primitive.Dag_Node{n}, nil
	case domain.Topology_Component_KIND_AGENT, domain.Topology_Component_KIND_ADDON:
		return nil, nil
	}

	config, err := componentConfig(c, db, topo, memoryMB)
	if err != nil {
		return nil, err
	}
	rec := recipeForComponent(c, db)
	prefix := c.GetId()
	var nodes []*primitive.Dag_Node
	add := func(suffix string, op *ops.Operation, bindings []*renderpb.Config_Binding) error {
		n, err := agentCommand(prefix+"."+suffix, op)
		if err != nil {
			return err
		}
		render.AttachBindings(n, bindings)
		nodes = append(nodes, n)
		return nil
	}

	for i, cmd := range rec.PreInstall {
		if err := add(fmt.Sprintf("pre_install_%d", i), scriptOp(cmd), nil); err != nil {
			return nil, err
		}
	}
	if len(rec.AptPackages) > 0 {
		if err := add("apt_install",
			scriptOp("DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(rec.AptPackages, " ")), nil); err != nil {
			return nil, err
		}
	}
	// Config FILE items are written before the service starts.
	for _, item := range config.GetItems() {
		if item.GetFile() == nil {
			continue
		}
		if err := add("write_"+item.GetId(), writeFileOpFor(item.GetFile()), item.GetBindings()); err != nil {
			return nil, err
		}
	}
	switch {
	case rec.ServiceName != "":
		if err := add("start_service", scriptOp("systemctl enable --now "+rec.ServiceName), nil); err != nil {
			return nil, err
		}
	case rec.StartScript != "":
		if err := add("start_service", scriptOp(rec.StartScript), nil); err != nil {
			return nil, err
		}
	}
	// Config COMMAND items run after the service is up (e.g. CHANGE MASTER TO on a
	// replica, with a late binding to the primary's ip).
	for _, item := range config.GetItems() {
		if item.GetCommand() == nil {
			continue
		}
		op := &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: item.GetCommand()}}
		if err := add("cmd_"+item.GetId(), op, item.GetBindings()); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// stroppyNode is the workload command: stroppy run against its FLOW target (the
// proxy if present, else the primary database), via a late-binding host token.
func stroppyNode(c *domain.Topology_Component, db *domain.Database, wl *domain.Workload, topo *domain.Topology, runID string) (*primitive.Dag_Node, error) {
	target := flowTarget(c.GetId(), topo)
	n, err := agentCommand(c.GetId()+".run_stroppy", runStroppyOp(db, wl, runID))
	if err != nil {
		return nil, err
	}
	if target != "" {
		render.AttachBindings(n, []*renderpb.Config_Binding{{
			Token:        stroppyDBHostToken,
			ComponentIds: []string{target},
			Attr:         render.AttrPrivateIP,
		}})
	}
	return n, nil
}

// flowTarget is the component the STROPPY load points at: its FLOW connection
// target, else the first PROXY, else the first DATABASE.
func flowTarget(stroppyID string, topo *domain.Topology) string {
	for _, conn := range topo.GetConnections() {
		if conn.GetKind() == domain.Topology_Connection_KIND_FLOW && conn.GetFrom() == stroppyID {
			return conn.GetTo()
		}
	}
	var firstDB string
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			if c.GetKind() == domain.Topology_Component_KIND_PROXY {
				return c.GetId()
			}
			if firstDB == "" && c.GetKind() == domain.Topology_Component_KIND_DATABASE {
				firstDB = c.GetId()
			}
		}
	}
	return firstDB
}

func presentRanks(byRank map[int][]*componentGroup) []int {
	var ranks []int
	for r := 0; r <= 3; r++ {
		if len(byRank[r]) > 0 {
			ranks = append(ranks, r)
		}
	}
	return ranks
}

func last(nodes []*primitive.Dag_Node) *primitive.Dag_Node { return nodes[len(nodes)-1] }

// agentCommand builds an agent-locus node whose input is Any(agent.Command{op}).
func agentCommand(id string, op *ops.Operation) (*primitive.Dag_Node, error) {
	input, err := anypb.New(&rtagent.Command{Operation: op})
	if err != nil {
		return nil, fmt.Errorf("dag: wrap command %q: %w", id, err)
	}
	return &primitive.Dag_Node{
		Id:          id,
		ExecutionId: "install_and_run." + id, // unique across the whole aggregate
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant: &primitive.Dag_Node_TaskState_{TaskState: &primitive.Dag_Node_TaskState{
			HandlerName: HandlerAgentCommand,
			Locus:       primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT,
			Input:       input,
		}},
	}, nil
}

// chainEdge connects two install nodes (node-typed, for the install sub-dag chains).
func chainEdge(source, target *primitive.Dag_Node) *primitive.Dag_Edge {
	return &primitive.Dag_Edge{
		Id:     source.GetId() + "->" + target.GetId(),
		Source: source.GetId(),
		Target: target.GetId(),
	}
}

// scriptOp wraps a shell script into a RUN_CMD operation (recipe steps use shell
// features — pipes, $(...) — so they run via the shell, not argv).
func scriptOp(text string) *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_RUN_CMD, Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{
		Command: &system.Cmd_Spec_Script{Script: &system.Cmd_Script{Text: text, Shell: "/bin/bash"}},
	}}}
}

// writeFileOpFor wraps a rendered config file into a WRITE_FILE operation.
func writeFileOpFor(file *system.File) *ops.Operation {
	return &ops.Operation{Kind: ops.Operation_KIND_WRITE_FILE, Operation: &ops.Operation_WriteFile{WriteFile: file}}
}

// runStroppyOp builds the workload command. stroppy v5 is k6-based and takes a run
// config file (`stroppy run -f <config>`), NOT --url flags. The script writes the
// stroppy RunConfig JSON (driver type + url + script) then runs stroppy against it.
// The DB host in the url is a late-binding token resolved to the target component's
// private ip at the plan->execute seam. OTEL_EXPORTER_OTLP_ENDPOINT (expanded by the
// agent shell) points stroppy's metrics at the server's VictoriaMetrics ingest (/vm)
// so workload metrics land alongside the vmagent-scraped host/DB metrics.
func runStroppyOp(db *domain.Database, wl *domain.Workload, runID string) *ops.Operation {
	script, sql := stroppyScriptPaths(wl.GetScript(), db.GetKind())
	cfg := stroppyRunConfigJSON(db, wl, runID, script, sql)
	// Fetch the pinned stroppy from the server cache (not the image bake), so the
	// version (with its embedded workloads) is identical on local Docker and YC VMs.
	ver := strings.TrimPrefix(wl.GetStroppyVersion(), "v")
	if ver == "" {
		ver = "5.1.3"
	}
	// Unquoted heredoc so the agent shell expands ${STROPPY_SERVER_ADDR} in the OTLP
	// endpoint (no other $ in the JSON — the DB host is a render token, not a shell var).
	return scriptOp(strings.Join([]string{
		"set -e",
		fetchBinary("stroppy", ver, "stroppy_linux_amd64.tar.gz", "stroppy", "stroppy"),
		"mkdir -p /etc/stroppy",
		"cat > /etc/stroppy/run-config.json << STROPPYCFG",
		cfg,
		"STROPPYCFG",
		"stroppy run -f /etc/stroppy/run-config.json",
	}, "\n"))
}

func stroppyRunConfigJSON(db *domain.Database, wl *domain.Workload, runID, script, sql string) string {
	params := wl.GetParameters()
	poolSize := params.GetPoolSize()
	if poolSize == 0 {
		poolSize = 16
	}
	scaleFactor := params.GetScaleFactor()
	if scaleFactor == 0 {
		scaleFactor = 1
	}
	defaultInsertMethod := strings.TrimSpace(params.GetDefaultInsertMethod())
	if defaultInsertMethod == "" {
		defaultInsertMethod = "native"
	}

	env := map[string]string{
		"OTEL_RESOURCE_ATTRIBUTES": "service.name=stroppy,run_id=" + runID,
		"SCALE_FACTOR":             fmt.Sprintf("%g", scaleFactor),
		"POOL_SIZE":                fmt.Sprintf("%d", poolSize),
	}
	for k, v := range params.GetEnv() {
		key := strings.ToUpper(strings.TrimSpace(k))
		if key != "" {
			env[key] = v
		}
	}

	driver := map[string]any{
		"driver_type":           stroppyDriverType(db.GetKind()),
		"url":                   stroppyURL(db),
		"default_insert_method": defaultInsertMethod,
		"pool": map[string]any{
			"max_conns": poolSize,
			"min_conns": poolSize,
		},
	}
	cfg := map[string]any{
		"version": "1",
		"script":  script,
		"global": map[string]any{"exporter": map[string]any{"otlp_export": map[string]any{
			"otlp_http_endpoint":          "${STROPPY_AGENT_SERVER}",
			"otlp_http_exporter_url_path": "/vm/insert/0/opentelemetry/api/v1/push",
			"otlp_endpoint_insecure":      true,
			"otlp_metrics_prefix":         "stroppy_",
		}}},
		"env":     env,
		"drivers": map[string]any{"0": driver},
	}
	if sql != "" {
		cfg["sql"] = sql
	}
	if exec := wl.GetExecution(); exec != nil {
		k6Args := []string{}
		if exec.GetQuiet() {
			k6Args = append(k6Args, "-q")
		}
		if exec.GetVus() > 0 {
			k6Args = append(k6Args, "--vus", fmt.Sprintf("%d", exec.GetVus()))
		}
		if iterations := exec.GetIterations(); iterations > 0 {
			k6Args = append(k6Args, "--iterations", fmt.Sprintf("%d", iterations))
		} else if duration := exec.GetDuration(); duration != "" {
			k6Args = append(k6Args, "--duration", duration)
		}
		if exec.GetNoThresholds() {
			k6Args = append(k6Args, "--no-thresholds")
		}
		if len(k6Args) > 0 {
			cfg["k6_args"] = k6Args
		}
	}
	if steps := params.GetSteps(); len(steps) > 0 {
		cfg["steps"] = steps
	}
	if noSteps := params.GetNoSteps(); len(noSteps) > 0 {
		cfg["no_steps"] = noSteps
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// stroppyScriptPaths maps a friendly workload name + engine to stroppy's embedded
// script (.ts) and SQL paths (relative to the embedded workloads/ dir). The bare
// preset name (e.g. "tpcc") is a directory, not a runnable file — stroppy needs the
// concrete .ts. Engine selects the matching .sql dialect.
func stroppyScriptPaths(name string, _ domain.Database_Kind) (script, sql string) {
	// Map friendly preset names to stroppy's embedded script path (old code's proven
	// default). stroppy auto-resolves the per-driver .sql from the preset dir, so sql
	// stays empty. An explicit path (contains "/" or ".ts") is used verbatim.
	switch {
	case name == "" || name == "tpcc":
		return "tpcc/procs", ""
	case name == "tpcb":
		return "tpcb/procs", ""
	case name == "tpch":
		return "tpch/tx.ts", "" // tpch is analytical — tx variant only (no procs)
	default:
		return name, "" // explicit path (e.g. "tpcc/tx.ts") used verbatim
	}
}

// stroppyDriverType maps a database engine to stroppy's driver selector (-d).
func stroppyDriverType(kind domain.Database_Kind) string {
	// stroppy driverType allowlist: csv, mysql, noop, picodata, postgres, ydb.
	switch kind {
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return "mysql"
	case domain.Database_KIND_PICODATA:
		return "picodata"
	case domain.Database_KIND_YDB:
		return "ydb"
	default: // postgres, cockroach (pg-wire)
		return "postgres"
	}
}

// ── per-machine monitoring chain ───────────────────────────────────────────────

// machineHostsDatabase reports whether any component on the machine is a DATABASE,
// so its monitor chain installs/scrapes the engine exporter.
func machineHostsDatabase(m *domain.Topology_Machine) bool {
	for _, c := range m.GetComponents() {
		if c.GetKind() == domain.Topology_Component_KIND_DATABASE {
			return true
		}
	}
	return false
}

// monitorChain builds one machine's monitoring node-chain (all agent.command,
// sequential within the machine): node_exporter → [DB exporter] → vmagent. It is
// the new-dag port of the old executor's installMonitor/configMonitor pipeline.
//
// Everything ships metrics to VictoriaMetrics via vmagent's -remoteWrite.url. The
// server reverse-proxies /vm/* to VM, so the remote_write URL is
// ${STROPPY_SERVER_ADDR}/vm/api/v1/write. ${STROPPY_SERVER_ADDR} is expanded by the
// agent's shell when the heredoc writes the unit/config — NOT resolved at plan time.
// An external label run_id is added so dashboards filter per run.
//
// System-log shipping: command output is already streamed to the server via the
// agent's SendLogs; vmagent additionally remote_writes node/DB metrics. Journald →
// VictoriaLogs (/vl) scraping is a deliberate follow-up (the old vector-based
// shipper, executor.go startVector) — not ported here to keep the chain to the
// must-have metrics path; see internal/old/domain/agent/executor.go for the recipe.
func monitorChain(m *domain.Topology_Machine, db *domain.Database, runID string) ([]*primitive.Dag_Node, error) {
	prefix := m.GetId() + ".monitor"
	var nodes []*primitive.Dag_Node
	add := func(suffix string, op *ops.Operation) error {
		n, err := agentCommand(prefix+"_"+suffix, op)
		if err != nil {
			return err
		}
		nodes = append(nodes, n)
		return nil
	}

	// 1. node_exporter (host metrics) on every machine — binary from the server
	//    cache (not apt), so it installs identically on a fresh YC VM. Port 9100.
	if err := add("node_exporter", scriptOp(nodeExporterInstallScript())); err != nil {
		return nil, err
	}

	// 2. DB-kind exporter ONLY on machines hosting a DATABASE component.
	//    - postgres/mysql: install a separate exporter + systemd unit at the LOCAL db.
	//    - cockroach/ydb/picodata: native Prometheus endpoint, no exporter (scraped directly).
	var exp dbExporter
	var hasExp bool
	if machineHostsDatabase(m) {
		if e, ok := dbExporterFor(db.GetKind()); ok {
			exp, hasExp = e, true
			if !e.native {
				if err := add("db_exporter", scriptOp(dbExporterInstallScript(e))); err != nil {
					return nil, err
				}
			}
		}
	}

	// 3. vmagent: install (download vmutils, extract vmagent-prod), write scrape
	//    config + systemd unit, start. Last in the chain so its scrape targets
	//    (node_exporter / DB exporter) are already started.
	if err := add("vmagent_install", scriptOp(vmagentInstallScript())); err != nil {
		return nil, err
	}
	if err := add("vmagent_config", scriptOp(vmagentConfigScript(runID, hasExp, exp))); err != nil {
		return nil, err
	}
	if err := add("vmagent_start", scriptOp(vmagentStartScript())); err != nil {
		return nil, err
	}

	// 4. vector: collects ALL logs on the host — journald (every systemd service:
	//    db, patroni, haproxy, vmagent, …) + DB log files — and ships them to the
	//    server's VictoriaLogs ingest, tagged with run/machine/unit for filtering.
	if err := add("vector_install", scriptOp(vectorInstallScript())); err != nil {
		return nil, err
	}
	if err := add("vector_config", scriptOp(vectorConfigScript(runID, m.GetId(), db.GetKind()))); err != nil {
		return nil, err
	}
	if err := add("vector_start", scriptOp(vectorStartScript())); err != nil {
		return nil, err
	}

	return nodes, nil
}

// vectorInstallScript installs vector from the server cache.
func vectorInstallScript() string {
	file := "vector-" + vectorVersion + "-x86_64-unknown-linux-musl.tar.gz"
	archive := "vector-x86_64-unknown-linux-musl/bin/vector"
	return "set -e\n" + fetchBinary("vector", vectorVersion, file, archive, "vector")
}

// vectorConfigScript writes /etc/stroppy/vector.yaml: tail journald + DB log files,
// tag each event with run_id/machine_id/unit, and POST to the server's VictoriaLogs
// ingest. The heredoc is UNQUOTED so the agent shell expands ${STROPPY_SERVER_ADDR}
// into the sink URI at write time.
func vectorConfigScript(runID, machineID string, kind domain.Database_Kind) string {
	var b strings.Builder
	b.WriteString("data_dir: /var/lib/vector\n\nsources:\n")
	b.WriteString("  journald:\n    type: journald\n    current_boot_only: true\n")
	dbInputs := ""
	switch kind {
	case domain.Database_KIND_POSTGRES, domain.Database_KIND_COCKROACH:
		b.WriteString("  db_files:\n    type: file\n    include: ['/var/log/postgresql/*.log']\n    read_from: end\n")
		dbInputs = ", 'db_files'"
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		b.WriteString("  db_files:\n    type: file\n    include: ['/var/log/mysql/*.log']\n    read_from: end\n")
		dbInputs = ", 'db_files'"
	}
	b.WriteString("\ntransforms:\n  enrich:\n    inputs: ['journald'" + dbInputs + "]\n    type: remap\n    source: |\n")
	// Tag with the SAME labels as agent command logs (logs.SendLogs) so logs filter
	// uniformly: dag_id (the run, queried by QueryRunLogs), tenant_id, machine_id, unit.
	// ${STROPPY_AGENT_TENANT} is expanded by the agent shell when the config is written.
	fmt.Fprintf(&b, "      .dag_id = %q\n", runID)
	b.WriteString("      .tenant_id = \"${STROPPY_AGENT_TENANT}\"\n")
	fmt.Fprintf(&b, "      .machine_id = %q\n", machineID)
	b.WriteString("      .unit = \"\"\n")
	b.WriteString("      if is_string(.SYSTEMD_UNIT) { .unit = .SYSTEMD_UNIT } else if is_string(.source_type) { .unit = .source_type }\n")
	b.WriteString("      if !exists(.timestamp) { .timestamp = now() }\n")
	b.WriteString("      if is_string(.message) { .message = .message } else if is_string(.MESSAGE) { .message = .MESSAGE } else { .message = encode_json(.) }\n")
	b.WriteString("\nsinks:\n  victorialogs:\n    type: http\n    inputs: ['enrich']\n")
	b.WriteString("    uri: ${STROPPY_SERVER_ADDR}" + vlInsertPath + "\n")
	b.WriteString("    method: post\n    encoding:\n      codec: json\n    framing:\n      method: newline_delimited\n")
	b.WriteString("    batch:\n      max_events: 1000\n      timeout_secs: 5\n")
	return "mkdir -p /etc/stroppy /var/lib/vector\ncat > /etc/stroppy/vector.yaml << VECTORCFG\n" + b.String() + "VECTORCFG"
}

// vectorStartScript writes the vector systemd unit and starts it.
func vectorStartScript() string {
	unit := strings.Join([]string{
		"[Unit]", "Description=stroppy vector log shipper", "After=network.target",
		"[Service]",
		"EnvironmentFile=-/etc/stroppy-agent.env",
		"ExecStart=/usr/local/bin/vector --config /etc/stroppy/vector.yaml",
		"Restart=always",
		"[Install]", "WantedBy=multi-user.target",
	}, "\n")
	return systemdUnitScript("stroppy-vector", unit)
}

// dbExporterInstallScript installs a separate DB exporter (postgres/mysql) and runs
// it as a systemd unit pointed at the LOCAL database.
func dbExporterInstallScript(e dbExporter) string {
	unit := strings.Join([]string{
		"[Unit]",
		"Description=stroppy " + e.job + " exporter",
		"After=network.target",
		"[Service]",
		e.dataSourceEnv,
		"ExecStart=" + e.execStart,
		"Restart=always",
		"[Install]",
		"WantedBy=multi-user.target",
	}, "\n")
	return "set -e\n" +
		fetchBinary(e.binName, e.binVersion, e.binFile, e.binArchive, e.binName) + "\n" +
		systemdUnitScript(e.serviceName, unit)
}

// fetchBinary builds a shell snippet that downloads a binary archive from the SERVER
// cache (${STROPPY_SERVER_ADDR}/binary/<name>/<ver>/<file>, expanded by the agent
// shell at exec time), extracts it, and installs <archivePath> to /usr/local/bin/<dest>.
// Routing through the server (bincache) means the recipe is identical for local Docker
// and Yandex Cloud, and upstream is fetched + cached exactly once.
func fetchBinary(name, ver, file, archivePath, dest string) string {
	url := "${STROPPY_SERVER_ADDR}/binary/" + name + "/" + ver + "/" + file
	// Install via a temp file + atomic mv: rename() over a running ("Text file busy")
	// binary succeeds on Linux, so a retried command never fails on the busy dest.
	return strings.Join([]string{
		`curl -fsSL --retry 5 --retry-delay 3 "` + url + `" -o /tmp/` + name + `.tar.gz`,
		"tar xzf /tmp/" + name + ".tar.gz -C /tmp",
		"chmod +x /tmp/" + archivePath,
		"mv -f /tmp/" + archivePath + " /usr/local/bin/" + dest,
	}, "\n")
}

// systemdUnitScript writes a systemd unit file and enables+starts it.
func systemdUnitScript(serviceName, unit string) string {
	return "cat > /etc/systemd/system/" + serviceName + ".service << 'UNIT'\n" + unit + "\nUNIT\n" +
		"systemctl daemon-reload\nsystemctl enable --now " + serviceName
}

// nodeExporterInstallScript installs node_exporter from the server cache + a unit.
func nodeExporterInstallScript() string {
	file := "node_exporter-" + nodeExporterVersion + ".linux-amd64.tar.gz"
	archive := "node_exporter-" + nodeExporterVersion + ".linux-amd64/node_exporter"
	unit := strings.Join([]string{
		"[Unit]", "Description=node_exporter", "After=network.target",
		"[Service]", "ExecStart=/usr/local/bin/node_exporter", "Restart=always",
		"[Install]", "WantedBy=multi-user.target",
	}, "\n")
	return "set -e\n" +
		fetchBinary("node_exporter", nodeExporterVersion, file, archive, "node_exporter") + "\n" +
		systemdUnitScript("node_exporter", unit)
}

// vmagentInstallScript installs vmagent from the server cache (vmutils tarball →
// vmagent-prod → /usr/local/bin/vmagent).
func vmagentInstallScript() string {
	file := "vmutils-linux-amd64-v" + vmagentVersion + ".tar.gz"
	return "set -e\n" + fetchBinary("vmagent", vmagentVersion, file, "vmagent-prod", "vmagent")
}

// vmagentConfigScript writes /etc/stroppy/vmagent.yml: scrape node_exporter on
// localhost:9100 and, on DB machines, the DB exporter / native metrics port. An
// external label run_id tags every series. The heredoc is single-quoted so the
// scrape config is written verbatim (no shell expansion needed — the remote_write
// URL is a vmagent FLAG, set in the systemd unit where ${STROPPY_SERVER_ADDR} is
// expanded at write time).
func vmagentConfigScript(runID string, hasExp bool, exp dbExporter) string {
	var cfg strings.Builder
	cfg.WriteString("# Generated by stroppy-cloud install dag\n")
	cfg.WriteString("global:\n  scrape_interval: 5s\n")
	cfg.WriteString("  external_labels:\n    run_id: '" + runID + "'\n")
	cfg.WriteString("\nscrape_configs:\n")
	cfg.WriteString("  - job_name: node\n    static_configs:\n      - targets: ['localhost:" + nodeExporterPort + "']\n")
	if hasExp {
		cfg.WriteString("  - job_name: " + exp.job + "\n")
		if exp.metricPath != "" {
			cfg.WriteString("    metrics_path: " + exp.metricPath + "\n")
		}
		cfg.WriteString("    static_configs:\n      - targets: ['localhost:" + exp.scrapePort + "']\n")
	}
	return "mkdir -p /etc/stroppy && cat > /etc/stroppy/vmagent.yml << 'VMSCRAPE'\n" + cfg.String() + "VMSCRAPE"
}

// vmagentStartScript writes the vmagent systemd unit and starts it. The
// -remoteWrite.url points at the server's VM ingest (${STROPPY_SERVER_ADDR}/vm/...).
// The heredoc is UNquoted so the agent's shell expands ${STROPPY_SERVER_ADDR} into
// the unit at write time (systemd would not expand a process-env var on its own).
func vmagentStartScript() string {
	unit := strings.Join([]string{
		"[Unit]",
		"Description=stroppy vmagent (remote_write to VictoriaMetrics)",
		"After=network.target",
		"[Service]",
		"ExecStart=/usr/local/bin/vmagent " +
			"-promscrape.config=/etc/stroppy/vmagent.yml " +
			"-remoteWrite.url=${STROPPY_SERVER_ADDR}" + remoteWritePath + " " +
			"-remoteWrite.tmpDataPath=/var/lib/vmagent",
		"Restart=always",
		"[Install]",
		"WantedBy=multi-user.target",
	}, "\n")
	return "mkdir -p /var/lib/vmagent && cat > /etc/systemd/system/stroppy-vmagent.service << UNIT\n" + unit + "\nUNIT\n" +
		"systemctl daemon-reload && systemctl enable --now stroppy-vmagent"
}

// stroppyURL is the engine-specific connection URL with the late-binding host token.
func stroppyURL(db *domain.Database) string {
	switch db.GetKind() {
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return "stroppy@tcp(" + stroppyDBHostToken + ":3306)/stroppy"
	case domain.Database_KIND_COCKROACH:
		// SQL on 5432 (the recipe's --sql-addr); 26257 is cockroach's RPC/listen port
		// and resets pgwire SQL connections.
		return "postgresql://root@" + stroppyDBHostToken + ":5432/defaultdb?sslmode=disable"
	case domain.Database_KIND_PICODATA:
		// picodata pg-wire: user `admin`, password from PICODATA_ADMIN_PASSWORD set at
		// start (see the picodata recipe). No db in the URL (picodata default).
		return "postgres://admin:T0psecret@" + stroppyDBHostToken + ":5432?sslmode=disable"
	case domain.Database_KIND_YDB:
		// The dynamic compute node (recipe start_database) serves the tenant on grpc 2136;
		// /Root/db1 is the tenant created from the ssd pool (the static /Root domain rejects
		// table DDL). The host token resolves to the FLOW target = first ydb node.
		return "grpc://" + stroppyDBHostToken + ":2136/Root/db1"
	default: // postgres pg-wire (old code's proven URL)
		return "postgresql://postgres@" + stroppyDBHostToken + ":5432/postgres?sslmode=disable"
	}
}
