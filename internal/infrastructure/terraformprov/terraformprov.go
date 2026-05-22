// Package terraformprov is the Yandex Cloud TerraformProvisioner (dag.Deps seam): it
// turns a deployment.Yandex (whose Input IS the terraform variables — see
// deployment/yandex.proto) into running infrastructure via the embedded yandex
// terraform module + terraform.Runner, and reads the VM endpoints back into
// Yandex.Output. Apply and Destroy share a workdir (derived from the run network
// name) so teardown reuses the same state.
package terraformprov

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

// Provisioner implements dag.TerraformProvisioner over the terraform Runner.
type Provisioner struct {
	runner *terraform.Runner
}

func New(runner *terraform.Runner) *Provisioner { return &Provisioner{runner: runner} }

// Apply provisions the Yandex deployment and fills Yandex.Output with the VM endpoints.
func (p *Provisioner) Apply(ctx context.Context, y *deployment.Yandex) (*deployment.Yandex, error) {
	op, err := tfOp(ops.TfOperation_ACTION_APPLY, y)
	if err != nil {
		return nil, err
	}
	out, err := p.runner.Run(ctx, op)
	if err != nil {
		return nil, err
	}
	if msg := out.GetError(); msg != "" {
		return nil, fmt.Errorf("terraform apply: %s", msg)
	}
	y.Output = parseOutput(out)
	return y, nil
}

// Destroy tears the deployment down, reusing the shared workdir/state.
func (p *Provisioner) Destroy(ctx context.Context, y *deployment.Yandex) error {
	op, err := tfOp(ops.TfOperation_ACTION_DESTROY, y)
	if err != nil {
		return err
	}
	out, err := p.runner.Run(ctx, op)
	if err != nil {
		return err
	}
	if msg := out.GetError(); msg != "" {
		return fmt.Errorf("terraform destroy: %s", msg)
	}
	return nil
}

// tfOp builds the terraform operation: the embedded yandex module + the Yandex.Input
// marshalled (proto field names) as terraform.tfvars.json — the proto IS the tfvars.
func tfOp(action ops.TfOperation_Action, y *deployment.Yandex) (*ops.TfOperation, error) {
	files, err := yandex.EmbeddedFiles()
	if err != nil {
		return nil, fmt.Errorf("embed tf module: %w", err)
	}
	data, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(y.GetInput())
	if err != nil {
		return nil, fmt.Errorf("marshal tfvars: %w", err)
	}
	return &ops.TfOperation{Input: &ops.TfOperation_Input{
		Action:    action,
		WorkdirId: "yc-" + y.GetInput().GetNetwork().GetName(), // stable across apply+destroy
		Files:     files,
		VarFile:   textFile("terraform.tfvars.json", string(data)),
	}}, nil
}

// parseOutput reads the module's "vms" output (name -> {id, internal_ip, public_ip}).
func parseOutput(out *ops.TfOperation_Output) *deployment.Yandex_Output {
	res := &deployment.Yandex_Output{Vms: map[string]*deployment.Yandex_VmOutput{}}
	raw, ok := out.GetOutputsJson()["vms"]
	if !ok {
		return res
	}
	var vms map[string]struct {
		Id         string `json:"id"`
		PublicIP   string `json:"public_ip"`
		InternalIP string `json:"internal_ip"`
	}
	if err := json.Unmarshal(raw, &vms); err != nil {
		return res
	}
	for name, v := range vms {
		res.Vms[name] = &deployment.Yandex_VmOutput{Id: v.Id, InternalIp: v.InternalIP, PublicIp: v.PublicIP}
	}
	if sa, ok := out.GetOutputsJson()["service_account_id"]; ok {
		var id string
		_ = json.Unmarshal(sa, &id)
		res.ServiceAccountId = id
	}
	return res
}

func textFile(path, content string) *system.File {
	return &system.File{
		Info: &system.File_Info{Path: path},
		Source: &system.File_Content_{Content: &system.File_Content{
			Content: &system.File_Content_Text{Text: content},
		}},
	}
}
