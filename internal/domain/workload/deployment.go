package workload

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

type DeploymentRenderer struct{}

const workloadCurlOpts = `--connect-timeout 20 --max-time 300 --retry 8 --retry-delay 5 --retry-all-errors --retry-connrefused --retry-max-time 900`

var stroppyReleaseVersionRE = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){1,3}([-.][0-9A-Za-z.]+)?$`)
var stroppyCommitVersionRE = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == Engine && component.GetRole() == RunnerRole
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	if ctx.Workload == nil {
		return nil, fmt.Errorf("workload renderer requires workload input")
	}
	dependencies := deploymentbuilder.DependencyIDs(ctx, nil)
	target, err := resolveDatabaseTarget(ctx)
	if err != nil {
		return nil, err
	}
	componentID := ctx.Component.GetId()
	labels := ctx.Topology.Spec().GetLabels()
	loadWorkers := loadWorkersFromMachine(ctx.Machine)

	steps := []*deploymentpb.AgentStep{
		deploymentbuilder.CreateDirStep("010_create_config_dir", 10, deploymentbuilder.ConfigDir(componentID), 0755),
		deploymentbuilder.WriteFileStep("020_write_context", 20, deploymentbuilder.ContextFile(ctx, dependencies)),
	}
	// One config file (and its run-scoped files) per segment, written before the
	// stroppy install. The run steps that execute each config are injected by the
	// workload workflow stage, not here.
	order := uint32(30)
	for i, segment := range ctx.Workload.GetSegments() {
		configFile, _, cerr := effectiveConfigFile(
			componentID, ctx.Workload, segment, i, ctx.Database,
			ctx.RenderOverrides, labels, target, loadWorkers, ctx.AgentToken,
		)
		if cerr != nil {
			return nil, cerr
		}
		steps = append(steps, deploymentbuilder.WriteFileStep(fmt.Sprintf("%03d_write_config_seg%d", order, i), order, configFile))
		order++
		for _, file := range segmentFiles(componentID, segment) {
			steps = append(steps, deploymentbuilder.WriteFileStep(fmt.Sprintf("%03d_write_workload_file", order), order, file))
			order++
		}
	}
	steps = append(steps,
		deploymentbuilder.CallCmdStep("100_prepare_stroppy", 100, installCommand(ctx.Workload, ctx.Topology.Spec().GetLabels()[deploymentbuilder.LabelServerAddr])),
		deploymentbuilder.CallCmdStep("110_healthcheck", 110, "test -d "+deploymentbuilder.ShellQuote(deploymentbuilder.ConfigDir(ctx.Component.GetId()))),
	)
	// The stroppy runner emits the k6/stroppy OTEL metrics the dashboards read,
	// so it needs the same node_exporter + vmagent + vector collectors as the DB
	// machines (the DB engine renderers get these via RenderComponentDeployment;
	// the workload renderer builds its own step list, so append them here too).
	steps = append(steps, deploymentbuilder.MonitorSteps(ctx)...)

	return &deploymentpb.ComponentDeployment{
		ComponentId:           ctx.Component.GetId(),
		NodeId:                ctx.Node.GetId(),
		GlobalPriority:        90,
		NodePriority:          90,
		DependsOnComponentIds: dependencies,
		Steps:                 steps,
		Status:                common.Status_STATUS_PENDING,
		Labels: deploymentbuilder.MergeLabels(ctx.Component.GetLabels(), map[string]string{
			"engine": Engine,
			"role":   RunnerRole,
		}),
		Tags: ctx.Component.GetTags(),
	}, nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	if ctx.Workload == nil {
		return nil, fmt.Errorf("workload renderer requires workload input")
	}
	dependencies := deploymentbuilder.DependencyIDs(deploymentbuilder.RenderContext{
		Topology:  ctx.Topology,
		Component: ctx.Component,
		Node:      ctx.Node,
	}, nil)
	labels := ctx.Topology.Spec().GetLabels()
	// The preview runs before infrastructure is provisioned, so the managed DB
	// path (a terraform output) is not known yet — leave it empty, which keeps
	// the URL free of a `?database=` (consistent with the address placeholders
	// the preview also leaves unresolved). The real path is substituted in
	// RenderComponent once the DB endpoint is resolved.
	componentID := ctx.Component.GetId()
	artifacts := []*deploymentpb.RenderArtifact{
		deploymentbuilder.DirArtifact(ctx, Engine, "config-dir", &common.Dir{
			Info:          &common.Dir_Info{Path: deploymentbuilder.ConfigDir(componentID), Mode: 0755},
			CreateParents: true,
		}),
		deploymentbuilder.FileArtifact(
			ctx,
			Engine,
			deploymentbuilder.ArtifactID(
				componentID,
				"topology.env",
			),
			deploymentbuilder.PreviewContextFile(ctx, dependencies),
			deploymentpb.RenderArtifact_ORIGIN_SYSTEM,
			deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY,
			"topology context is generated from topology and runtime state",
			"",
			map[string]string{"artifact": "context"},
		),
	}
	// One editable config artifact (and its read-only files) per segment.
	for i, segment := range ctx.Workload.GetSegments() {
		configFile, configOrigin, cerr := effectiveConfigFile(
			componentID, ctx.Workload, segment, i, ctx.Database,
			ctx.RenderOverrides, labels, databaseTarget{}, 0, "",
		)
		if cerr != nil {
			return nil, cerr
		}
		defConfig := defaultConfigFile(
			componentID, ctx.Workload, segment, i, ctx.Database,
			labels, databaseTarget{}, 0, "",
		)
		artifacts = append(artifacts, deploymentbuilder.FileArtifact(
			ctx,
			Engine,
			segmentConfigArtifactID(componentID, i),
			configFile,
			configOrigin,
			deploymentpb.RenderArtifact_MUTABILITY_EDITABLE,
			"",
			deploymentbuilder.FileHash(defConfig),
			map[string]string{"artifact": "config"},
		))
		for _, file := range segmentFiles(componentID, segment) {
			artifacts = append(artifacts, deploymentbuilder.FileArtifact(
				ctx,
				Engine,
				deploymentbuilder.ArtifactID(componentID, "files/"+filepath.Base(file.GetInfo().GetPath())),
				file,
				deploymentpb.RenderArtifact_ORIGIN_SYSTEM,
				deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY,
				"workload files come from workload input",
				"",
				map[string]string{"artifact": "workload_file"},
			))
		}
	}
	artifacts = append(artifacts,
		deploymentbuilder.CommandArtifact(
			ctx,
			Engine,
			deploymentbuilder.ArtifactID(ctx.Component.GetId(), "install/100"),
			installCommand(ctx.Workload, labels[deploymentbuilder.LabelServerAddr]),
			"stroppy install is renderer-owned",
			map[string]string{"artifact": "install"},
		),
		deploymentbuilder.CommandArtifact(
			ctx,
			Engine,
			deploymentbuilder.ArtifactID(ctx.Component.GetId(), "healthcheck"),
			"test -d "+deploymentbuilder.ShellQuote(deploymentbuilder.ConfigDir(ctx.Component.GetId())),
			"healthcheck command is renderer-owned",
			map[string]string{"artifact": "healthcheck"},
		),
		deploymentbuilder.RuntimeArtifact(
			ctx,
			Engine,
			"runtime/private-address",
			"machine."+ctx.Node.GetId()+".endpoint.private.address",
		),
	)
	return artifacts, nil
}

func effectiveConfigFile(
	componentID string,
	input *domain.Workload,
	segment *domain.Workload_Segment,
	index int,
	database *domain.Database,
	overrides *deploymentpb.RenderOverrideSet,
	labels map[string]string,
	target databaseTarget,
	loadWorkers uint32,
	bearerToken string,
) (*common.File, deploymentpb.RenderArtifact_Origin, error) {
	target = target.withDefaults(database)
	artifactID := segmentConfigArtifactID(componentID, index)
	if override, ok := deploymentbuilder.OverrideFile(overrides, componentID, artifactID); ok && override.GetFile() != nil {
		file, err := patchStroppyConfigFile(override.GetFile(), labels, target, bearerToken)
		if err != nil {
			return nil, deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE, err
		}
		return file, deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE, nil
	}
	return defaultConfigFile(
		componentID,
		input,
		segment,
		index,
		database,
		labels,
		target,
		loadWorkers,
		bearerToken,
	), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT, nil
}

func defaultConfigFile(
	componentID string,
	input *domain.Workload,
	segment *domain.Workload_Segment,
	index int,
	database *domain.Database,
	labels map[string]string,
	target databaseTarget,
	loadWorkers uint32,
	bearerToken string,
) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          SegmentConfigPath(componentID, index),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: renderStroppyConfigJSON(input, segment, database, labels, target, loadWorkers, bearerToken)},
	}
}

// resolveDatabaseTarget returns the concrete runtime DB endpoint the workload
// runner should use. Preview configs may carry sentinels, but deployment plans
// must write a real host/port into stroppy-config.json.
func resolveDatabaseTarget(ctx deploymentbuilder.RenderContext) (databaseTarget, error) {
	targets, err := deploymentbuilder.DependencyTargets(ctx, nil)
	if err != nil {
		return databaseTarget{}, err
	}
	if len(targets) == 0 {
		return databaseTarget{}, fmt.Errorf("workload runner %q has no database target", ctx.Component.GetId())
	}
	target := targets[0]
	if strings.TrimSpace(target.Address) == "" {
		return databaseTarget{}, fmt.Errorf("workload runner %q database target %q has no address", ctx.Component.GetId(), target.ComponentID)
	}
	if target.Port == 0 {
		return databaseTarget{}, fmt.Errorf("workload runner %q database target %q has no port", ctx.Component.GetId(), target.ComponentID)
	}
	return databaseTarget{
		Host:         target.Address,
		Port:         target.Port,
		DatabasePath: target.Labels[dbDatabaseLabel],
	}, nil
}

func loadWorkersFromMachine(machine *deploymentpb.MachineState) uint32 {
	if machine == nil {
		return 0
	}
	for _, allocation := range machine.GetAllocatedQuotas() {
		info := allocation.GetInfo()
		if !isCPUQuota(info.GetName(), info.GetUnits()) || allocation.GetUsed() == 0 {
			continue
		}
		if allocation.GetUsed() > uint64(^uint32(0)) {
			return ^uint32(0)
		}
		return uint32(allocation.GetUsed())
	}
	for _, key := range []string{"cpu_cores", "cpus", "cores"} {
		value := strings.TrimSpace(machine.GetLabels()[key])
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err == nil && parsed > 0 {
			return uint32(parsed)
		}
	}
	return 0
}

func isCPUQuota(name, units string) bool {
	return strings.EqualFold(strings.TrimSpace(units), "cores") && strings.Contains(strings.ToLower(name), "cpu")
}

func segmentFiles(componentID string, segment *domain.Workload_Segment) []*common.File {
	src := segment.GetFiles()
	files := make([]*common.File, 0, len(src))
	for _, file := range src {
		files = append(files, &common.File{
			Info: &common.File_Info{
				Path:          deploymentbuilder.ConfigDir(componentID) + "/files/" + file.GetName(),
				Mode:          0644,
				CreateParents: true,
			},
			Content: &common.File_Text{Text: file.GetContent()},
		})
	}
	return files
}

func installCommand(input *domain.Workload, serverAddr string) string {
	version := strings.TrimSpace(input.GetStroppyVersion())
	if version == "" {
		version = "latest"
	}
	serverAddr = strings.TrimRight(strings.TrimSpace(serverAddr), "/")
	if serverAddr == "" {
		return "set -e\n" +
			"if command -v stroppy >/dev/null 2>&1; then\n" +
			"  stroppy version || stroppy --version || true\n" +
			"else\n" +
			"  echo 'stroppy server address is not configured; cannot fetch binary' >&2\n" +
			"  exit 127\n" +
			"fi\n"
	}
	downloadURL := stroppyDownloadURL(serverAddr, version)
	return fmt.Sprintf(`set -e
if command -v stroppy >/dev/null 2>&1; then
  echo "existing stroppy:"
  stroppy version || stroppy --version || true
fi
mkdir -p /usr/local/bin
tmp="$(mktemp /tmp/stroppy-artifact.XXXXXX)"
extract_dir="$(mktemp -d /tmp/stroppy-extract.XXXXXX)"
cleanup() {
  rm -f "$tmp"
  rm -rf "$extract_dir"
}
trap cleanup EXIT
curl -fsSL %s %s -o "$tmp"
if tar tzf "$tmp" >/dev/null 2>&1; then
  tar xzf "$tmp" -C "$extract_dir"
  bin="$(find "$extract_dir" -type f -name stroppy -perm /111 | head -n 1)"
  if [ -z "$bin" ]; then
    bin="$(find "$extract_dir" -type f -name stroppy | head -n 1)"
  fi
  if [ -z "$bin" ]; then
    echo "stroppy binary not found in downloaded archive" >&2
    exit 1
  fi
  install -m 0755 "$bin" /usr/local/bin/stroppy
else
  install -m 0755 "$tmp" /usr/local/bin/stroppy
fi
echo "installed stroppy:"
stroppy version || stroppy --version || true
`, workloadCurlOpts, deploymentbuilder.ShellQuote(downloadURL))
}

// SegmentConfigFileName is the stroppy config filename for a segment by index.
// Index 0 keeps the legacy "stroppy-config.json" so single-segment runs stay
// byte-identical (config path, artifact id, and render-override keys unchanged);
// later segments get an indexed name.
func SegmentConfigFileName(index int) string {
	if index <= 0 {
		return "stroppy-config.json"
	}
	return fmt.Sprintf("stroppy-config-%d.json", index)
}

// SegmentConfigPath is the absolute path the workload runner writes/reads for a
// segment's stroppy config. Shared by the renderer (which writes it) and the
// workflow (which runs `stroppy run -f <path>`).
func SegmentConfigPath(componentID string, index int) string {
	return deploymentbuilder.ConfigDir(componentID) + "/" + SegmentConfigFileName(index)
}

func segmentConfigArtifactID(componentID string, index int) string {
	return deploymentbuilder.ArtifactID(componentID, SegmentConfigFileName(index))
}

func stroppyDownloadURL(serverAddr, version string) string {
	version = strings.TrimSpace(version)
	trimmed := strings.TrimPrefix(version, "v")
	switch {
	case version == "" || strings.EqualFold(version, "latest"):
		return serverAddr + "/artifacts/stroppy"
	case stroppyReleaseVersionRE.MatchString(version):
		return serverAddr + "/api/binaries/stroppy/" + trimmed + "/stroppy_linux_amd64.tar.gz"
	case stroppyCommitVersionRE.MatchString(version):
		if len(trimmed) > 7 {
			trimmed = trimmed[:7]
		}
		return serverAddr + "/api/binaries/stroppy_nightly/" + strings.ToLower(trimmed) + "/stroppy"
	default:
		return serverAddr + "/artifacts/stroppy"
	}
}
