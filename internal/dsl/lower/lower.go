// Package lower converts a fully validated/expanded DSL compiler pipeline
// result (a cluster.yaml document, its compiled domain graph, and the flat,
// matrix-expanded job map produced by internal/dsl/graph.Expand) into the
// runtime-facing dslpb.CompiledPlan proto: the request for machines the
// provider must satisfy, the service (Nomad job) specs, and the job DAG the
// executor walks at runtime.
//
// Lower assumes its input already passed every earlier compiler stage
// (schema, ast decode, include.Resolve, graph.Build, contract.Check,
// graph.Validate/Expand) — see internal/dsl.Compile, which is the only
// intended caller. It performs no validation of its own beyond what is
// needed to build the proto (e.g. it does not re-check that a job's On/
// Service reference an existing machine group/service).
//
// Every "${{ ... }}" expression embedded in a string field (job.When,
// step.Cmd, a service's Env value, ...) is copied through byte-for-byte —
// Lower never evaluates CEL; that is the runtime executor's job once actual
// topology/machine data is available.
package lower

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

const (
	// defaultFileMode/defaultDirMode are applied to every write_file/fetch/
	// dir step's common.File_Info/common.Dir_Info.Mode: the DSL v1 step
	// shapes (ast.WriteFile/ast.Fetch/ast.Step.Dir) carry no author-facing
	// mode field, so Lower fills in the same conservative defaults the
	// existing engine.go/builder.go step constructors use for
	// generated files/dirs (see internal/domain/deployment/builder.go
	// WriteFileStep/CreateDirStep call sites).
	defaultFileMode = 0o644
	defaultDirMode  = 0o755

	// shell is the interpreter every lowered `cmd:` step runs under,
	// mirroring internal/domain/deployment/builder.go's ShellCmd script
	// mode exactly (Cmd_Spec_Script, not Cmd_Spec_Argv).
	shell = "/bin/sh"
)

// Lower builds the CompiledPlan for cluster/dom/jobs. cluster supplies the
// provider reference and service specs, dom supplies the (already lowered
// disk-type, ext-merged) machine group states, jobs is the flat,
// matrix-expanded job map (internal/dsl/graph.Expand's output) to compile
// into the job DAG, and components is every include.BoundComponent
// instantiation the recipe's `include:` jobs produced (include.Resolve's
// Resolved.Components) — the source Lower reads each CompiledJob's
// resolved_inputs/input_groups/target_group from (see jobInputFields).
//
// Output is deterministic: MachineGroups, Services and Jobs are each sorted
// by name, so two calls with equal inputs produce proto.Equal plans.
//
// Precondition: cluster and dom must be non-nil (Lower's caller,
// internal/dsl.Compile, only reaches this stage once ast.DecodeCluster and
// graph.Build have both already succeeded).
//
// The only failure mode is a step that reaches Lower with zero step actions
// set (ast.Step{Cmd,WriteFile,Fetch,Dir,Wait} all zero) — a shape
// graph.Validate's embedded-expression walk does not itself reject, since
// that invariant is actually enforced earlier, at ast.DecodeWorkflow time
// (see decodeStep's "expected exactly one action" diagnostic). Reaching
// Lower with such a step is therefore a programmatic inconsistency between
// compiler stages, reported as an error rather than silently producing an
// empty DslStep or panicking on a nil pointer deref.
func Lower(cluster *ast.ClusterDoc, dom *graph.Domain, jobs map[string]ast.Job, components []include.BoundComponent) (*dslpb.CompiledPlan, error) {
	provider, err := lowerProvider(cluster)
	if err != nil {
		return nil, err
	}

	groups, err := lowerMachineGroups(dom)
	if err != nil {
		return nil, err
	}

	compiledJobs, err := lowerJobs(jobs, components)
	if err != nil {
		return nil, err
	}

	return &dslpb.CompiledPlan{
		Provider:      provider,
		MachineGroups: groups,
		Services:      lowerServices(cluster),
		Jobs:          compiledJobs,
	}, nil
}

// lowerProvider builds the ProviderRef: name is the cluster's provider
// selection (cluster.yaml's "provider.use", not the provider manifest's own
// Name — the two are validated equal upstream, by whichever stage resolved
// in.Provider from cluster.Provider.Use in the first place), params_json is
// cluster.Provider.Params re-encoded as JSON, since the params shape is
// provider-specific and not typed in the CompiledPlan proto. provider.use may
// carry a catalog pin ("slug@version"); providerSlug strips that suffix so
// the compiled name still matches the provider manifest's bare Name.
func lowerProvider(cluster *ast.ClusterDoc) (*dslpb.ProviderRef, error) {
	paramsJSON, err := json.Marshal(cluster.Provider.Params)
	if err != nil {
		return nil, fmt.Errorf("provider params: %w", err)
	}
	return &dslpb.ProviderRef{
		Name:       providerSlug(cluster.Provider.Use),
		ParamsJson: string(paramsJSON),
	}, nil
}

// providerSlug strips a catalog pin suffix from a "provider.use" value: for
// "slug@version" it returns "slug" (the substring before the LAST "@", so a
// slug containing "@" — unlikely but not forbidden by the grammar — is still
// handled correctly); for a bare "slug" (the legacy, bundle-local path, which
// never contains "@") it returns the input unchanged.
func providerSlug(use string) string {
	if i := strings.LastIndex(use, "@"); i >= 0 {
		return use[:i]
	}
	return use
}

// lowerMachineGroups builds one MachineGroup per dom.Groups entry, sorted by
// name for determinism.
func lowerMachineGroups(dom *graph.Domain) ([]*dslpb.MachineGroup, error) {
	names := make([]string, 0, len(dom.Groups))
	for name := range dom.Groups {
		names = append(names, name)
	}
	sort.Strings(names)

	groups := make([]*dslpb.MachineGroup, 0, len(names))
	for _, name := range names {
		mg, err := lowerMachineGroup(name, dom.Groups[name])
		if err != nil {
			return nil, err
		}
		groups = append(groups, mg)
	}
	return groups, nil
}

func lowerMachineGroup(name string, gs graph.GroupState) (*dslpb.MachineGroup, error) {
	extJSON, err := json.Marshal(gs.Ext)
	if err != nil {
		return nil, fmt.Errorf("machine group %q: ext: %w", name, err)
	}

	var disks []*dslpb.DiskSpec
	if gs.Spec.Resources.Disk != nil {
		disks = []*dslpb.DiskSpec{{
			SizeGb: uint64(gs.Spec.Resources.Disk.Size) / (1 << 30),
			// Type is already lowered (graph.Build's precedence: domain
			// value -> provider.Lowering["disk.type"] -> ext override) or,
			// for a provider with no lowering table for disk.type at all,
			// passed through as the domain value verbatim — GroupState's
			// LoweredDiskType captures either case correctly.
			Type: gs.LoweredDiskType,
		}}
	}

	if gs.Spec.Count < 0 {
		return nil, fmt.Errorf("machine group %q: negative count %d", name, gs.Spec.Count)
	}
	if gs.Spec.Resources.CPU < 0 {
		return nil, fmt.Errorf("machine group %q: negative cpu %d", name, gs.Spec.Resources.CPU)
	}

	return &dslpb.MachineGroup{
		Name:    name,
		Count:   uint32(gs.Spec.Count),         //nolint:gosec // guarded non-negative above.
		Cpu:     uint32(gs.Spec.Resources.CPU), //nolint:gosec // guarded non-negative above.
		RamMb:   uint64(gs.Spec.Resources.RAM) / (1 << 20),
		Disks:   disks,
		ExtJson: string(extJSON),
	}, nil
}

// lowerServices builds one ServiceSpec per cluster.Services entry, sorted by
// name for determinism.
func lowerServices(cluster *ast.ClusterDoc) []*dslpb.ServiceSpec {
	names := make([]string, 0, len(cluster.Services))
	for name := range cluster.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	services := make([]*dslpb.ServiceSpec, 0, len(names))
	for _, name := range names {
		svc := cluster.Services[name]
		services = append(services, &dslpb.ServiceSpec{
			Name:    name,
			OnGroup: svc.On,
			Image:   svc.Image,
			Network: svc.Network,
			Volumes: append([]string(nil), svc.Volumes...),
			Env:     svc.Env,
			Configs: lowerConfigs(svc.Configs),
			Health:  lowerHealth(svc.Health),
		})
	}
	return services
}

func lowerConfigs(configs []ast.ConfigFile) []*dslpb.ConfigFile {
	if len(configs) == 0 {
		return nil
	}
	out := make([]*dslpb.ConfigFile, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, &dslpb.ConfigFile{TemplatePath: cfg.Template, Dest: cfg.Dest})
	}
	return out
}

func lowerHealth(h *ast.Health) *dslpb.HealthCheck {
	if h == nil {
		return nil
	}
	return &dslpb.HealthCheck{
		Http:    h.HTTP,
		Timeout: time.Duration(h.Timeout).String(),
	}
}

// lowerJobs builds one CompiledJob per jobs entry, sorted by (instance) name
// for determinism.
func lowerJobs(jobs map[string]ast.Job, components []include.BoundComponent) ([]*dslpb.CompiledJob, error) {
	names := make([]string, 0, len(jobs))
	for name := range jobs {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]*dslpb.CompiledJob, 0, len(names))
	for _, name := range names {
		cj, err := lowerJob(name, jobs[name], components)
		if err != nil {
			return nil, err
		}
		out = append(out, cj)
	}
	return out, nil
}

func lowerJob(name string, job ast.Job, components []include.BoundComponent) (*dslpb.CompiledJob, error) {
	resolvedInputs, inputGroups, targetGroup := jobInputFields(name, components)

	cj := &dslpb.CompiledJob{
		Id:             name,
		Needs:          append([]string(nil), job.Needs...),
		OnGroup:        job.On,
		Matrix:         job.MatrixValues,
		When:           job.When,
		With:           job.With,
		ResolvedInputs: resolvedInputs,
		InputGroups:    inputGroups,
		TargetGroup:    targetGroup,
	}

	if job.Service != "" {
		cj.Action = &dslpb.CompiledJob_Service{Service: job.Service}
		return cj, nil
	}

	steps, err := lowerSteps(name, job.Steps)
	if err != nil {
		return nil, err
	}
	cj.Action = &dslpb.CompiledJob_Steps{Steps: &dslpb.StepList{Steps: steps}}
	return cj, nil
}

// jobInputFields computes a CompiledJob's resolved_inputs/input_groups/
// target_group from the include.BoundComponent that produced jobID, per
// componentForJob's association rule. A jobID with no matching
// BoundComponent (a top-level workflow job, never reached via an
// `include:`) yields all three zero values, matching the brief's "jobs not
// from a component -> all three empty".
//
// resolvedInputs holds every scalar (non machine_group) input as a typed
// google.protobuf.Value (structpb.NewValue of the Go value include.Resolve
// bound — int/string/bool per include.checkInputType); inputGroups holds
// every machine_group input's bound group name; targetGroup is that single
// group name when the component declares exactly one machine_group input,
// else "" (0 or >1).
func jobInputFields(jobID string, components []include.BoundComponent) (resolvedInputs map[string]*structpb.Value, inputGroups map[string]string, targetGroup string) {
	bc := componentForJob(baseJobName(jobID), components)
	if bc == nil {
		return nil, nil, ""
	}

	resolvedInputs = map[string]*structpb.Value{}
	inputGroups = map[string]string{}
	for inputName, spec := range bc.Doc.Inputs {
		val, has := bc.Inputs[inputName]
		if !has {
			continue
		}
		if spec.Type == "machine_group" {
			if group, ok := val.(string); ok {
				inputGroups[inputName] = group
			}
			continue
		}
		v, err := structpb.NewValue(val)
		if err != nil {
			// Unrepresentable scalar (should not happen for the int/string/
			// bool types checkInputType binds) — skip rather than fail the
			// whole job, matching this function's existing permissive
			// contract.
			continue
		}
		resolvedInputs[inputName] = v
	}

	if len(inputGroups) == 1 {
		for _, group := range inputGroups {
			targetGroup = group
		}
	}

	return resolvedInputs, inputGroups, targetGroup
}

// baseJobName strips a matrix instance's "[k=v,...]" suffix (see
// graph.Expand), recovering the include.Resolve-produced job name a
// BoundComponent.Name prefix match compares against — Expand appends that
// suffix only after Resolve has already fixed every job's name.
func baseJobName(id string) string {
	if idx := strings.IndexByte(id, '['); idx >= 0 {
		return id[:idx]
	}
	return id
}

// componentForJob finds the BoundComponent that directly produced the job
// named baseName, per include.Resolve's naming rule: an include job
// instantiated as name N produces its own fragment's jobs under the "N/"
// prefix (see include.BoundComponent's godoc and expandInclude). When
// baseName matches more than one candidate (a nested include: an outer
// component's own fragment includes another), the most specific — longest
// Name — match wins, so a job is attributed to the innermost component that
// literally declared it, not an ancestor it happens to be nested under.
func componentForJob(baseName string, components []include.BoundComponent) *include.BoundComponent {
	var best *include.BoundComponent
	for i := range components {
		c := &components[i]
		if baseName != c.Name && !strings.HasPrefix(baseName, c.Name+"/") {
			continue
		}
		if best == nil || len(c.Name) > len(best.Name) {
			best = c
		}
	}
	return best
}

func lowerSteps(jobID string, steps []ast.Step) ([]*dslpb.DslStep, error) {
	out := make([]*dslpb.DslStep, 0, len(steps))
	for idx, step := range steps {
		ds, err := lowerStep(jobID, idx, step)
		if err != nil {
			return nil, err
		}
		out = append(out, ds)
	}
	return out, nil
}

// lowerStep lowers one ast.Step. Every branch but Wait produces an
// AgentStep (routed to a node agent, see internal/proto/cloud/v1/deployment
// AgentStep); Wait has no agent action at all — it is evaluated by the
// executor directly (DslStep_Wait), matching the brief's `Wait ->
// DslStep{wait: WaitStep{...}}` mapping.
func lowerStep(jobID string, idx int, step ast.Step) (*dslpb.DslStep, error) {
	switch {
	case step.Cmd != "":
		return &dslpb.DslStep{Step: &dslpb.DslStep_Agent{Agent: callCmdStep(jobID, idx, step.Cmd)}}, nil
	case step.WriteFile != nil:
		return &dslpb.DslStep{Step: &dslpb.DslStep_Agent{Agent: writeFileStep(jobID, idx, step.WriteFile)}}, nil
	case step.Fetch != nil:
		return &dslpb.DslStep{Step: &dslpb.DslStep_Agent{Agent: fetchFileStep(jobID, idx, step.Fetch)}}, nil
	case step.Dir != "":
		return &dslpb.DslStep{Step: &dslpb.DslStep_Agent{Agent: createDirStep(jobID, idx, step.Dir)}}, nil
	case step.Wait != nil:
		return &dslpb.DslStep{Step: &dslpb.DslStep_Wait{Wait: &dslpb.WaitStep{
			Http:    step.Wait.HTTP,
			Timeout: time.Duration(step.Wait.Timeout).String(),
		}}}, nil
	default:
		return nil, fmt.Errorf("job %q step %d: no action set", jobID, idx)
	}
}

// stepID renders an AgentStep.Id per the brief: "<jobID>/<idx>".
func stepID(jobID string, idx int) string {
	return fmt.Sprintf("%s/%d", jobID, idx)
}

// stepOrder converts a step's list index (always small and non-negative —
// bounded by one job's step count) to AgentStep.Order's uint32.
func stepOrder(idx int) uint32 {
	return uint32(idx) //nolint:gosec // idx is a slice index, never negative or large enough to overflow uint32.
}

// callCmdStep lowers ast.Step.Cmd in script mode, mirroring
// internal/domain/deployment/builder.go's ShellCmd exactly (Cmd_Spec_Script
// with Shell "/bin/sh" and ExpectedExitCodes [0], not Cmd_Spec_Argv).
func callCmdStep(jobID string, idx int, script string) *deploymentpb.AgentStep {
	return &deploymentpb.AgentStep{
		Id:    stepID(jobID, idx),
		Order: stepOrder(idx),
		Action: &deploymentpb.AgentStep_CallCmd{
			CallCmd: &commonpb.Cmd{
				Spec: &commonpb.Cmd_Spec{
					Command: &commonpb.Cmd_Spec_Script{
						Script: &commonpb.Cmd_Script{
							Text:  script,
							Shell: shell,
						},
					},
					ExpectedExitCodes: []int32{0},
				},
			},
		},
	}
}

// writeFileStep lowers ast.Step.WriteFile: Dest/Content map directly onto
// common.File_Info.Path/common.File_Text.Text. wf.Template (a
// render-at-runtime template path, as opposed to inline Content) has no
// corresponding common.File field yet — a write_file step in v1 is expected
// to carry Content, not Template; Template is copied through as empty text
// rather than silently dropped-with-no-trace, since Lower has no separate
// diagnostic channel to flag it.
func writeFileStep(jobID string, idx int, wf *ast.WriteFile) *deploymentpb.AgentStep {
	return &deploymentpb.AgentStep{
		Id:    stepID(jobID, idx),
		Order: stepOrder(idx),
		Action: &deploymentpb.AgentStep_WriteFile{
			WriteFile: &commonpb.File{
				Info: &commonpb.File_Info{
					Path:          wf.Dest,
					Mode:          defaultFileMode,
					CreateParents: true,
				},
				Content: &commonpb.File_Text{Text: wf.Content},
			},
		},
	}
}

// fetchFileStep lowers ast.Step.Fetch: Dest maps onto common.File_Info.Path,
// URL onto common.File_AsRef.Uri (the agent fetches the URL onto Dest at
// execution time — there is no content to inline at compile time).
func fetchFileStep(jobID string, idx int, f *ast.Fetch) *deploymentpb.AgentStep {
	return &deploymentpb.AgentStep{
		Id:    stepID(jobID, idx),
		Order: stepOrder(idx),
		Action: &deploymentpb.AgentStep_FetchFile{
			FetchFile: &commonpb.File{
				Info: &commonpb.File_Info{
					Path:          f.Dest,
					Mode:          defaultFileMode,
					CreateParents: true,
				},
				Content: &commonpb.File_AsRef_{AsRef: &commonpb.File_AsRef{Uri: f.URL}},
			},
		},
	}
}

// createDirStep lowers ast.Step.Dir (a bare path string) onto
// common.Dir_Info.Path, mirroring builder.go's CreateDirStep default of
// CreateParents true.
func createDirStep(jobID string, idx int, path string) *deploymentpb.AgentStep {
	return &deploymentpb.AgentStep{
		Id:    stepID(jobID, idx),
		Order: stepOrder(idx),
		Action: &deploymentpb.AgentStep_CreateDir{
			CreateDir: &commonpb.Dir{
				Info: &commonpb.Dir_Info{
					Path: path,
					Mode: defaultDirMode,
				},
				CreateParents: true,
			},
		},
	}
}
