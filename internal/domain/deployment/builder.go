package deployment

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	topologyindex "github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

type Renderer interface {
	Supports(component *topologypb.Component) bool
	RenderComponent(ctx RenderContext) (*deploymentpb.ComponentDeployment, error)
}

type PreviewRenderer interface {
	Supports(component *topologypb.Component) bool
	RenderPreview(ctx PreviewContext) ([]*deploymentpb.RenderArtifact, error)
}

type Registry struct {
	renderers []Renderer
}

func NewRegistry(renderers ...Renderer) Registry {
	return Registry{renderers: append([]Renderer(nil), renderers...)}
}

func (r Registry) IsEmpty() bool {
	return len(r.renderers) == 0
}

func (r Registry) Find(component *topologypb.Component) (Renderer, bool) {
	for _, renderer := range r.renderers {
		if renderer.Supports(component) {
			return renderer, true
		}
	}
	return nil, false
}

func (r Registry) FindPreview(component *topologypb.Component) (PreviewRenderer, bool) {
	for _, renderer := range r.renderers {
		if !renderer.Supports(component) {
			continue
		}
		previewRenderer, ok := renderer.(PreviewRenderer)
		if ok {
			return previewRenderer, true
		}
	}
	return nil, false
}

type RenderContext struct {
	Topology        *topologyindex.Index
	Component       *topologypb.Component
	Node            *topologypb.Node
	Machine         *deploymentpb.MachineState
	Runtime         RuntimeView
	Database        *domain.Database
	Workload        *domain.Workload
	DatabasePackage *domain.Package
	RenderOverrides *deploymentpb.RenderOverrideSet
}

type RuntimeEndpoint struct {
	Name    string
	Address string
	Port    uint32
	HasPort bool
	// Labels carries the resolved endpoint's labels (e.g. a managed YDB
	// endpoint's database_path). They reach the renderers through the same
	// runtime view that surfaces the host/port, so the stroppy URL resolver
	// can read database_path the same way it reads the address.
	Labels map[string]string
}

type RuntimeComponent struct {
	ComponentID string
	NodeID      string
	Engine      string
	Role        string
	Endpoints   map[string]RuntimeEndpoint
}

func (c RuntimeComponent) Endpoint(name string) (RuntimeEndpoint, bool) {
	endpoint, ok := c.Endpoints[name]
	return endpoint, ok
}

func (c RuntimeComponent) PrivateEndpoint() (RuntimeEndpoint, bool) {
	return c.Endpoint("private")
}

type RuntimeView struct {
	components map[string]RuntimeComponent
}

func (v RuntimeView) Component(componentID string) (RuntimeComponent, bool) {
	component, ok := v.components[componentID]
	return component, ok
}

func (v RuntimeView) PrivateEndpoint(componentID string) (RuntimeEndpoint, bool) {
	component, ok := v.Component(componentID)
	if !ok {
		return RuntimeEndpoint{}, false
	}
	return component.PrivateEndpoint()
}

type RuntimeTarget struct {
	ComponentID  string
	NodeID       string
	Engine       string
	Role         string
	EndpointName string
	Address      string
	Port         uint32
	// Labels carries the resolved endpoint's labels (e.g. database_path for a
	// managed YDB endpoint), mirrored from the RuntimeEndpoint.
	Labels map[string]string
}

func (t RuntimeTarget) AddressPort() string {
	return AddressPort(t.Address, t.Port)
}

type PreviewContext struct {
	Topology        *topologyindex.Index
	Component       *topologypb.Component
	Node            *topologypb.Node
	Database        *domain.Database
	Workload        *domain.Workload
	DatabasePackage *domain.Package
	RenderOverrides *deploymentpb.RenderOverrideSet
}

type BuildOptions struct {
	Database        *domain.Database
	Workload        *domain.Workload
	PackageResolver packages.Resolver
	Renderers       Registry
	RenderOverrides *deploymentpb.RenderOverrideSet
	Labels          map[string]string
	Tags            *common.Tags
}

type PreviewOptions struct {
	Database        *domain.Database
	Workload        *domain.Workload
	PackageResolver packages.Resolver
	Renderers       Registry
	RenderOverrides *deploymentpb.RenderOverrideSet
	Labels          map[string]string
	Tags            *common.Tags
}

func BuildPlan(spec *topologypb.TopologySpec, state *deploymentpb.InfrastructureState, options BuildOptions) (*deploymentpb.DeploymentPlan, error) {
	idx, err := topologyindex.NewIndex(spec)
	if err != nil {
		return nil, err
	}
	stateIndex, err := newStateIndex(state)
	if err != nil {
		return nil, err
	}
	runtime, err := newRuntimeView(idx, stateIndex)
	if err != nil {
		return nil, err
	}

	if err := validateRenderOverrides(options.RenderOverrides); err != nil {
		return nil, err
	}

	dbPackage, err := resolvePackage(options.Database, options.PackageResolver)
	if err != nil {
		return nil, err
	}

	plan := &deploymentpb.DeploymentPlan{
		Components: make([]*deploymentpb.ComponentDeployment, 0, len(spec.GetComponents())),
		Labels: MergeLabels(spec.GetLabels(), options.Labels, map[string]string{
			"stage": "deployment_plan",
		}),
		Tags: TagsOrDefault(options.Tags, "deployment"),
	}

	for _, component := range spec.GetComponents() {
		node, ok := idx.NodeForComponentID(component.GetId())
		if !ok {
			return nil, fmt.Errorf("component %q is not assigned to a node", component.GetId())
		}
		machine, ok := stateIndex[node.GetId()]
		if !ok {
			return nil, fmt.Errorf("infrastructure state is missing node %q", node.GetId())
		}

		renderer, ok := options.Renderers.Find(component)
		if !ok {
			return nil, fmt.Errorf("no deployment renderer for component %q engine=%q role=%q", component.GetId(), component.GetEngine(), component.GetRole())
		}

		renderCtx := RenderContext{
			Topology:        idx,
			Component:       component,
			Node:            node,
			Machine:         machine,
			Runtime:         runtime,
			Database:        options.Database,
			Workload:        options.Workload,
			DatabasePackage: dbPackage,
			RenderOverrides: options.RenderOverrides,
		}
		if _, err := DependencyTargets(renderCtx, nil); err != nil {
			return nil, fmt.Errorf("resolve runtime dependencies for component %q: %w", component.GetId(), err)
		}

		deployment, err := renderer.RenderComponent(renderCtx)
		if err != nil {
			return nil, err
		}
		plan.Components = append(plan.Components, deployment)
	}

	sortComponentDeployments(plan.GetComponents())

	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return plan, nil
}

func BuildPreview(spec *topologypb.TopologySpec, options PreviewOptions) (*deploymentpb.RenderPreview, error) {
	idx, err := topologyindex.NewIndex(spec)
	if err != nil {
		return nil, err
	}
	if err := validateRenderOverrides(options.RenderOverrides); err != nil {
		return nil, err
	}

	dbPackage, err := resolvePackage(options.Database, options.PackageResolver)
	if err != nil {
		return nil, err
	}

	preview := &deploymentpb.RenderPreview{
		Components: make([]*deploymentpb.ComponentRender, 0, len(spec.GetComponents())),
		Artifacts:  make([]*deploymentpb.RenderArtifact, 0, len(spec.GetComponents())*4),
		Overrides:  options.RenderOverrides,
		Labels: MergeLabels(spec.GetLabels(), options.Labels, map[string]string{
			"stage": "render_preview",
		}),
		Tags: TagsOrDefault(options.Tags, "render", "preview"),
	}

	for _, component := range spec.GetComponents() {
		node, ok := idx.NodeForComponentID(component.GetId())
		if !ok {
			return nil, fmt.Errorf("component %q is not assigned to a node", component.GetId())
		}

		renderer, ok := options.Renderers.FindPreview(component)
		if !ok {
			return nil, fmt.Errorf("no render preview renderer for component %q engine=%q role=%q", component.GetId(), component.GetEngine(), component.GetRole())
		}

		artifacts, err := renderer.RenderPreview(PreviewContext{
			Topology:        idx,
			Component:       component,
			Node:            node,
			Database:        options.Database,
			Workload:        options.Workload,
			DatabasePackage: dbPackage,
			RenderOverrides: options.RenderOverrides,
		})
		if err != nil {
			return nil, err
		}
		sortRenderArtifacts(artifacts)

		artifactIDs := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			artifactIDs = append(artifactIDs, artifact.GetId())
		}

		preview.Components = append(preview.Components, &deploymentpb.ComponentRender{
			ComponentId: component.GetId(),
			NodeId:      node.GetId(),
			ArtifactIds: artifactIDs,
			Labels: MergeLabels(component.GetLabels(), map[string]string{
				"engine": component.GetEngine(),
				"role":   component.GetRole(),
			}),
		})
		preview.Artifacts = append(preview.Artifacts, artifacts...)
	}

	sortComponentRenders(preview.GetComponents())
	sortRenderArtifacts(preview.GetArtifacts())

	if err := preview.Validate(); err != nil {
		return nil, err
	}
	return preview, nil
}

func resolvePackage(database *domain.Database, resolver packages.Resolver) (*domain.Package, error) {
	if database == nil || resolver == nil {
		return nil, nil
	}
	return resolver.ResolveDatabasePackage(database)
}

func validateRenderOverrides(overrides *deploymentpb.RenderOverrideSet) error {
	if overrides == nil {
		return nil
	}
	return overrides.Validate()
}

func newStateIndex(state *deploymentpb.InfrastructureState) (map[string]*deploymentpb.MachineState, error) {
	if state == nil {
		return nil, errors.New("infrastructure state is required")
	}
	if err := state.Validate(); err != nil {
		return nil, err
	}

	machines := make(map[string]*deploymentpb.MachineState, len(state.GetMachines()))
	for _, machine := range state.GetMachines() {
		if _, ok := machines[machine.GetNodeId()]; ok {
			return nil, fmt.Errorf("duplicate machine state for node %q", machine.GetNodeId())
		}
		machines[machine.GetNodeId()] = machine
	}
	return machines, nil
}

func newRuntimeView(idx *topologyindex.Index, machines map[string]*deploymentpb.MachineState) (RuntimeView, error) {
	components := make(map[string]RuntimeComponent, len(idx.Spec().GetComponents()))
	for _, component := range idx.Spec().GetComponents() {
		node, ok := idx.NodeForComponentID(component.GetId())
		if !ok {
			return RuntimeView{}, fmt.Errorf("component %q is not assigned to a node", component.GetId())
		}
		machine, ok := machines[node.GetId()]
		if !ok {
			return RuntimeView{}, fmt.Errorf("infrastructure state is missing node %q", node.GetId())
		}

		endpoints := make(map[string]RuntimeEndpoint, len(machine.GetEndpoints()))
		for _, endpoint := range machine.GetEndpoints() {
			runtimeEndpoint := RuntimeEndpoint{
				Name:    endpoint.GetName(),
				Address: endpoint.GetAddress(),
				Labels:  endpoint.GetLabels(),
			}
			if endpoint.Port != nil {
				runtimeEndpoint.Port = endpoint.GetPort()
				runtimeEndpoint.HasPort = true
			}
			endpoints[runtimeEndpoint.Name] = runtimeEndpoint
		}
		if _, ok := endpoints["private"]; !ok {
			return RuntimeView{}, fmt.Errorf("machine state for node %q is missing private endpoint", node.GetId())
		}

		components[component.GetId()] = RuntimeComponent{
			ComponentID: component.GetId(),
			NodeID:      node.GetId(),
			Engine:      component.GetEngine(),
			Role:        component.GetRole(),
			Endpoints:   endpoints,
		}
	}
	return RuntimeView{components: components}, nil
}

func sortComponentDeployments(components []*deploymentpb.ComponentDeployment) {
	sort.SliceStable(components, func(i, j int) bool {
		left := components[i]
		right := components[j]
		if left.GetGlobalPriority() != right.GetGlobalPriority() {
			return left.GetGlobalPriority() < right.GetGlobalPriority()
		}
		if left.GetNodeId() != right.GetNodeId() {
			return left.GetNodeId() < right.GetNodeId()
		}
		if left.GetNodePriority() != right.GetNodePriority() {
			return left.GetNodePriority() < right.GetNodePriority()
		}
		return left.GetComponentId() < right.GetComponentId()
	})
}

func sortComponentRenders(components []*deploymentpb.ComponentRender) {
	sort.SliceStable(components, func(i, j int) bool {
		left := components[i]
		right := components[j]
		if left.GetNodeId() != right.GetNodeId() {
			return left.GetNodeId() < right.GetNodeId()
		}
		return left.GetComponentId() < right.GetComponentId()
	})
}

func sortRenderArtifacts(artifacts []*deploymentpb.RenderArtifact) {
	sort.SliceStable(artifacts, func(i, j int) bool {
		left := artifacts[i]
		right := artifacts[j]
		if left.GetComponentId() != right.GetComponentId() {
			return left.GetComponentId() < right.GetComponentId()
		}
		return left.GetId() < right.GetId()
	})
}

func DependencyIDs(ctx RenderContext, include func(*topologypb.Component) bool) []string {
	seen := map[string]struct{}{}
	for _, connection := range ctx.Topology.OutgoingConnections(ctx.Component.GetId()) {
		target, ok := ctx.Topology.Component(connection.GetToComponentId())
		if !ok || target.GetId() == ctx.Component.GetId() {
			continue
		}
		if include != nil && !include(target) {
			continue
		}
		seen[target.GetId()] = struct{}{}
	}

	dependencies := make([]string, 0, len(seen))
	for componentID := range seen {
		dependencies = append(dependencies, componentID)
	}
	sort.Strings(dependencies)
	return dependencies
}

func DependencyTargets(ctx RenderContext, include func(*topologypb.Connection, *topologypb.Component) bool) ([]RuntimeTarget, error) {
	targets := make([]RuntimeTarget, 0)
	seen := map[string]struct{}{}
	for _, connection := range ctx.Topology.OutgoingConnections(ctx.Component.GetId()) {
		target, ok := ctx.Topology.Component(connection.GetToComponentId())
		if !ok || target.GetId() == ctx.Component.GetId() {
			continue
		}
		if include != nil && !include(connection, target) {
			continue
		}
		targetNode, ok := ctx.Topology.NodeForComponentID(target.GetId())
		if !ok {
			continue
		}
		private, ok := ctx.Runtime.PrivateEndpoint(target.GetId())
		if !ok {
			return nil, fmt.Errorf("component %q on node %q has no private endpoint", target.GetId(), targetNode.GetId())
		}

		endpointName := connection.GetEndpointName()
		if endpointName == "" {
			endpointName = "private"
		}
		port := connection.GetPort()
		if port == 0 && private.HasPort {
			port = private.Port
		}

		key := target.GetId() + "\x00" + endpointName + "\x00" + fmt.Sprint(port)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, RuntimeTarget{
			ComponentID:  target.GetId(),
			NodeID:       targetNode.GetId(),
			Engine:       target.GetEngine(),
			Role:         target.GetRole(),
			EndpointName: endpointName,
			Address:      private.Address,
			Port:         port,
			Labels:       private.Labels,
		})
	}
	sortRuntimeTargets(targets)
	return targets, nil
}

func ComponentTargets(ctx RenderContext, include func(*topologypb.Component) bool, endpointName string, port uint32) ([]RuntimeTarget, error) {
	if endpointName == "" {
		endpointName = "private"
	}
	targets := make([]RuntimeTarget, 0)
	for _, component := range ctx.Topology.Spec().GetComponents() {
		if include != nil && !include(component) {
			continue
		}
		node, ok := ctx.Topology.NodeForComponentID(component.GetId())
		if !ok {
			continue
		}
		private, ok := ctx.Runtime.PrivateEndpoint(component.GetId())
		if !ok {
			return nil, fmt.Errorf("component %q on node %q has no private endpoint", component.GetId(), node.GetId())
		}
		targetPort := port
		if targetPort == 0 && private.HasPort {
			targetPort = private.Port
		}
		targets = append(targets, RuntimeTarget{
			ComponentID:  component.GetId(),
			NodeID:       node.GetId(),
			Engine:       component.GetEngine(),
			Role:         component.GetRole(),
			EndpointName: endpointName,
			Address:      private.Address,
			Port:         targetPort,
			Labels:       private.Labels,
		})
	}
	sortRuntimeTargets(targets)
	return targets, nil
}

func OwnPrivateEndpoint(ctx RenderContext) (RuntimeEndpoint, bool) {
	return ctx.Runtime.PrivateEndpoint(ctx.Component.GetId())
}

func sortRuntimeTargets(targets []RuntimeTarget) {
	sort.SliceStable(targets, func(i, j int) bool {
		left := targets[i]
		right := targets[j]
		if left.ComponentID != right.ComponentID {
			return left.ComponentID < right.ComponentID
		}
		if left.EndpointName != right.EndpointName {
			return left.EndpointName < right.EndpointName
		}
		return left.Port < right.Port
	})
}

func ContextFile(ctx RenderContext, dependencies []string) *common.File {
	var b strings.Builder
	fmt.Fprintf(&b, "COMPONENT_ID=%s\n", ShellValue(ctx.Component.GetId()))
	fmt.Fprintf(&b, "NODE_ID=%s\n", ShellValue(ctx.Node.GetId()))
	fmt.Fprintf(&b, "ENGINE=%s\n", ShellValue(ctx.Component.GetEngine()))
	fmt.Fprintf(&b, "ROLE=%s\n", ShellValue(ctx.Component.GetRole()))
	if len(dependencies) > 0 {
		fmt.Fprintf(&b, "DEPENDS_ON=%s\n", ShellValue(strings.Join(dependencies, ",")))
	}
	endpoints := append([]*deploymentpb.Endpoint(nil), ctx.Machine.GetEndpoints()...)
	sort.SliceStable(endpoints, func(i, j int) bool {
		return endpoints[i].GetName() < endpoints[j].GetName()
	})
	for _, endpoint := range endpoints {
		name := strings.ToUpper(strings.ReplaceAll(endpoint.GetName(), "-", "_"))
		fmt.Fprintf(&b, "ENDPOINT_%s_ADDRESS=%s\n", name, ShellValue(endpoint.GetAddress()))
		if endpoint.GetPort() > 0 {
			fmt.Fprintf(&b, "ENDPOINT_%s_PORT=%d\n", name, endpoint.GetPort())
		}
	}
	for _, target := range contextDependencyTargets(ctx, dependencies) {
		writeDependencyTargetEnv(&b, target)
	}

	return &common.File{
		Info: &common.File_Info{
			Path:          ConfigDir(ctx.Component.GetId()) + "/topology.env",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: b.String()},
	}
}

func contextDependencyTargets(ctx RenderContext, dependencies []string) []RuntimeTarget {
	dependencySet := make(map[string]struct{}, len(dependencies))
	for _, dependency := range dependencies {
		dependencySet[dependency] = struct{}{}
	}
	targets, err := DependencyTargets(ctx, func(_ *topologypb.Connection, target *topologypb.Component) bool {
		if len(dependencySet) == 0 {
			return true
		}
		_, ok := dependencySet[target.GetId()]
		return ok
	})
	if err != nil {
		return nil
	}
	return targets
}

func writeDependencyTargetEnv(b *strings.Builder, target RuntimeTarget) {
	componentName := EnvName(target.ComponentID)
	endpointName := EnvName(target.EndpointName)
	fmt.Fprintf(b, "DEPENDENCY_%s_NODE_ID=%s\n", componentName, ShellValue(target.NodeID))
	fmt.Fprintf(b, "DEPENDENCY_%s_%s_ADDRESS=%s\n", componentName, endpointName, ShellValue(target.Address))
	if target.Port > 0 {
		fmt.Fprintf(b, "DEPENDENCY_%s_%s_PORT=%d\n", componentName, endpointName, target.Port)
		fmt.Fprintf(b, "DEPENDENCY_%s_%s_ADDR=%s\n", componentName, endpointName, ShellValue(target.AddressPort()))
	}
}

func AddressPort(address string, port uint32) string {
	if port == 0 {
		return address
	}
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") {
		return fmt.Sprintf("[%s]:%d", address, port)
	}
	return fmt.Sprintf("%s:%d", address, port)
}

func EnvName(value string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		var out rune
		switch {
		case r >= 'a' && r <= 'z':
			out = r - 'a' + 'A'
		case r >= 'A' && r <= 'Z':
			out = r
		case r >= '0' && r <= '9':
			out = r
		default:
			out = '_'
		}
		if out == '_' {
			if lastUnderscore {
				continue
			}
			lastUnderscore = true
		} else {
			lastUnderscore = false
		}
		b.WriteRune(out)
	}
	return strings.Trim(b.String(), "_")
}

func ServiceFile(componentID, role, configDir string) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          "/etc/systemd/system/" + ServiceName(componentID) + ".service",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: ServiceUnit(componentID, role, configDir)},
	}
}

func ServiceUnit(componentID, role, configDir string) string {
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud %s component %s
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
EnvironmentFile=%s/topology.env
ExecStart=/bin/true

[Install]
WantedBy=multi-user.target
`, role, componentID, configDir)
}

func CreateDirStep(id string, order uint32, path string, mode uint32) *deploymentpb.AgentStep {
	return &deploymentpb.AgentStep{
		Id:    id,
		Order: order,
		Action: &deploymentpb.AgentStep_CreateDir{
			CreateDir: &common.Dir{
				Info: &common.Dir_Info{
					Path: path,
					Mode: mode,
				},
				CreateParents: true,
			},
		},
		Status: common.Status_STATUS_PENDING,
		Labels: map[string]string{"kind": "create_dir"},
		Tags:   Tags("deployment", "filesystem"),
	}
}

func WriteFileStep(id string, order uint32, file *common.File) *deploymentpb.AgentStep {
	return &deploymentpb.AgentStep{
		Id:    id,
		Order: order,
		Action: &deploymentpb.AgentStep_WriteFile{
			WriteFile: file,
		},
		Status: common.Status_STATUS_PENDING,
		Labels: map[string]string{"kind": "write_file"},
		Tags:   Tags("deployment", "filesystem"),
	}
}

func CallCmdStep(id string, order uint32, script string) *deploymentpb.AgentStep {
	return &deploymentpb.AgentStep{
		Id:    id,
		Order: order,
		Action: &deploymentpb.AgentStep_CallCmd{
			CallCmd: ShellCmd(script),
		},
		Status: common.Status_STATUS_PENDING,
		Labels: map[string]string{"kind": "call_cmd"},
		Tags:   Tags("deployment", "command"),
	}
}

func ShellCmd(script string) *common.Cmd {
	return &common.Cmd{
		Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{
				Script: &common.Cmd_Script{
					Text:  script,
					Shell: "/bin/sh",
				},
			},
			ExpectedExitCodes: []int32{0},
		},
	}
}

func ConfigDir(componentID string) string {
	return "/etc/stroppy-cloud/" + componentID
}

func ServiceName(componentID string) string {
	return "stroppy-" + componentID
}

func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func ShellValue(value string) string {
	return ShellQuote(value)
}

func MergeLabels(maps ...map[string]string) map[string]string {
	total := 0
	for _, labels := range maps {
		total += len(labels)
	}
	if total == 0 {
		return nil
	}

	merged := make(map[string]string, total)
	for _, labels := range maps {
		for key, value := range labels {
			merged[key] = value
		}
	}
	return merged
}

func Tags(values ...string) *common.Tags {
	return &common.Tags{Tags: values}
}

func TagsOrDefault(input *common.Tags, values ...string) *common.Tags {
	if input != nil {
		return input
	}
	return Tags(values...)
}

func ArtifactID(componentID, name string) string {
	return componentID + "/" + strings.TrimPrefix(name, "/")
}

func OverrideFile(overrides *deploymentpb.RenderOverrideSet, componentID, artifactID string) (*deploymentpb.FileOverride, bool) {
	for _, file := range overrides.GetFiles() {
		if file.GetComponentId() == componentID && file.GetArtifactId() == artifactID {
			return file, true
		}
	}
	return nil, false
}

func FileHash(file *common.File) string {
	if file == nil {
		return ""
	}

	hash := sha256.New()
	write := func(value string) {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}

	if info := file.GetInfo(); info != nil {
		write(info.GetPath())
		write(fmt.Sprintf("%04o", info.GetMode()))
		write(info.GetOwner())
		write(info.GetGroup())
		write(fmt.Sprintf("%t", info.GetCreateParents()))
	}
	write(fmt.Sprintf("%t", file.GetAppend()))
	write(file.GetText())
	if len(file.GetBytes()) > 0 {
		_, _ = hash.Write(file.GetBytes())
	}
	if ref := file.GetAsRef(); ref != nil {
		write(ref.GetUri())
		write(ref.GetChecksum())
	}

	return hex.EncodeToString(hash.Sum(nil))
}
