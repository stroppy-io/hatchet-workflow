// Real Terraform handler: applies/destroys the module bundled with the
// binary, persists the workdir id to DagRunStateEntry under the configured
// state key so a restart can recover instead of stranding cloud resources.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// TerraformHandler implements nodeworker.Handler for TerraformTask. APPLY
// allocates or reuses a workdir id (read from state-store on retry),
// instantiates the bundled module, runs terraform apply, and persists the
// raw outputs to the state-store as JSON under wd_id_state_key + ".output".
// DESTROY reads the saved wd_id and tears the workdir down.
type TerraformHandler struct {
	actor   *terraform.Actor
	modules ModuleResolver
	issuer  BootstrapTokenIssuer
	log     *zap.Logger
}

// ModuleResolver maps a TerraformTask_Module enum to a directory of
// embedded .tf files plus tfvars. Provided by the host application
// because the actual file contents live next to the binary at deploy time.
type ModuleResolver interface {
	Resolve(module taskspb.TerraformTask_Module, vars map[string]any) ([]terraform.TfFile, terraform.TfVarFile, error)
}

// BootstrapTokenIssuer is the subset of agent.Service the terraform handler
// needs to mint per-machine bootstrap JWTs before `terraform apply`. Defined
// locally to avoid an import cycle with the iam / agent services.
type BootstrapTokenIssuer interface {
	IssueBootstrap(ctx context.Context, tenantID, dagRunID, machineID, role string) (string, error)
}

// NewTerraformHandler wires the handler. `modules` may be nil only when
// every TerraformTask in the pipeline uses MODULE_UNSPECIFIED (no-op);
// production wiring must supply a real resolver. `issuer` may be nil for
// tests that do not exercise machine fan-out.
func NewTerraformHandler(actor *terraform.Actor, modules ModuleResolver, log *zap.Logger, issuer BootstrapTokenIssuer) *TerraformHandler {
	return &TerraformHandler{actor: actor, modules: modules, issuer: issuer, log: log}
}

// Kind matches the Any.type_url suffix the registry filters on.
func (h *TerraformHandler) Kind() string { return "TerraformTask" }

// Execute applies or destroys the requested module. State key conventions:
//
//	<wd_id_state_key>          → workdir id (string)
//	<wd_id_state_key>.output   → JSON-encoded module outputs
func (h *TerraformHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.TerraformTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("terraform: unmarshal spec: %w", err)
	}
	if task.GetModule() == taskspb.TerraformTask_MODULE_UNSPECIFIED {
		h.log.Debug("terraform: noop for MODULE_UNSPECIFIED",
			zap.String("node_run_id", node.GetId().GetValue()))
		return nil, nil
	}
	if h.actor == nil {
		return nil, fmt.Errorf("terraform: actor not configured")
	}

	stateKey := task.GetWdIdStateKey()
	if stateKey == "" {
		stateKey = "yc.wd"
	}

	switch task.GetOp() {
	case taskspb.TerraformTask_OP_APPLY:
		return h.apply(ctx, &task, node, stateKey, state)
	case taskspb.TerraformTask_OP_DESTROY:
		return h.destroy(ctx, &task, stateKey, state)
	default:
		return nil, fmt.Errorf("terraform: unsupported op %s", task.GetOp())
	}
}

func (h *TerraformHandler) apply(ctx context.Context, task *taskspb.TerraformTask, node *systempb.NodeRun, stateKey string, state nodeworker.StateStore) (*anypb.Any, error) {
	if h.modules == nil {
		return nil, fmt.Errorf("terraform: ModuleResolver missing for module=%s", task.GetModule())
	}
	vars := structToMap(task.GetVars())

	// Issue one bootstrap JWT per planned machine BEFORE the terraform
	// apply so the rendered cloud-init userdata (which the TF module
	// templates) can embed the token. The machines list, role tags and
	// tenant_id are written into Vars by the DAG builder.
	if h.issuer != nil {
		machines := readMachines(vars)
		if len(machines) > 0 {
			tenantID, _ := vars["tenant_id"].(string)
			dagRunID := node.GetDagRunId().GetValue()
			tokens := make(map[string]any, len(machines))
			for _, m := range machines {
				id, _ := m["id"].(string)
				role, _ := m["role"].(string)
				if id == "" {
					continue
				}
				tok, err := h.issuer.IssueBootstrap(ctx, tenantID, dagRunID, id, role)
				if err != nil {
					return nil, fmt.Errorf("terraform: issue bootstrap for %s: %w", id, err)
				}
				tokens[id] = tok
			}
			vars["agent_bootstrap_tokens"] = tokens
		}
	}

	files, varsFile, err := h.modules.Resolve(task.GetModule(), vars)
	if err != nil {
		return nil, fmt.Errorf("terraform: resolve module %s: %w", task.GetModule(), err)
	}

	// Reuse existing workdir id from state on retry; otherwise generate.
	var wdID terraform.WdId
	if v, ok, _ := state.Get(ctx, stateKey); ok && v != nil {
		var sv structpb.Value
		_ = anypb.UnmarshalTo(v, &sv, proto.UnmarshalOptions{})
		if sv.GetStringValue() != "" {
			wdID = terraform.NewWdId(sv.GetStringValue())
		}
	}
	if wdID.String() == "" {
		wdID = terraform.NewWdId(ids.New())
	}

	wd := terraform.NewWorkdirWithParams(wdID, terraform.WithTfFiles(files), terraform.WithVarFile(varsFile))
	if _, err := state.Put(ctx, stateKey, mustWrapString(wdID.String())); err != nil {
		h.log.Warn("terraform: persist wd_id failed", zap.Error(err))
	}

	output, err := h.actor.ApplyTerraform(ctx, wd)
	if err != nil {
		return nil, fmt.Errorf("terraform apply: %w", err)
	}
	if output != nil {
		if raw, err := json.Marshal(output); err == nil {
			_, _ = state.Put(ctx, stateKey+".output", mustWrapBytes(raw))
		}
		// Persist named terraform outputs into the DagRun state-store under
		// the configured destination keys. Each output value is the raw JSON
		// terraform emitted; we decode strings as structpb.StringValue (so
		// downstream OneShot state_var_refs picks them up) and structured
		// outputs as their JSON encoding.
		for outName, dstKey := range task.GetOutputStateKeys() {
			raw, ok := output[outName]
			if !ok || len(raw) == 0 {
				continue
			}
			var asStr string
			if err := json.Unmarshal(raw, &asStr); err == nil {
				if _, perr := state.Put(ctx, dstKey, mustWrapString(asStr)); perr != nil {
					h.log.Warn("terraform: persist output failed", zap.String("key", dstKey), zap.Error(perr))
				}
				continue
			}
			if _, perr := state.Put(ctx, dstKey, mustWrapBytes(raw)); perr != nil {
				h.log.Warn("terraform: persist output failed", zap.String("key", dstKey), zap.Error(perr))
			}
		}
	}
	return nil, nil
}

func (h *TerraformHandler) destroy(ctx context.Context, _ *taskspb.TerraformTask, stateKey string, state nodeworker.StateStore) (*anypb.Any, error) {
	v, ok, err := state.Get(ctx, stateKey)
	if err != nil {
		return nil, fmt.Errorf("terraform destroy: read state %s: %w", stateKey, err)
	}
	if !ok || v == nil {
		// Nothing to tear down: TF apply never ran.
		return nil, nil
	}
	var sv structpb.Value
	_ = anypb.UnmarshalTo(v, &sv, proto.UnmarshalOptions{})
	wd := terraform.NewWdId(sv.GetStringValue())
	if wd.String() == "" {
		return nil, nil
	}
	if err := h.actor.DestroyExisting(ctx, wd); err != nil {
		return nil, fmt.Errorf("terraform destroy %s: %w", wd, err)
	}
	return nil, nil
}

// readMachines extracts the planned machine inventory from a tfvars map.
// The builder writes vars["machines"] as a list of {id, role, kind} maps.
// Anything else returns nil so the handler skips the bootstrap-token step.
func readMachines(vars map[string]any) []map[string]any {
	raw, ok := vars["machines"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, m)
	}
	return out
}

func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(s.GetFields()))
	for k, v := range s.GetFields() {
		out[k] = v.AsInterface()
	}
	return out
}

func mustWrapString(s string) *anypb.Any {
	v := structpb.NewStringValue(s)
	a, _ := anypb.New(v)
	return a
}

func mustWrapBytes(b []byte) *anypb.Any {
	v := structpb.NewStringValue(string(b))
	a, _ := anypb.New(v)
	return a
}
