package terraform

import (
	"context"
	"encoding/json"
	"fmt"

	schemapb "github.com/stroppy-io/schemapb/schemapb"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, input *deployment.Terraform_Input) (*deployment.Terraform_Output, error) {
	if input == nil {
		return nil, fmt.Errorf("terraform input is required")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}

	operation := input.GetOperation()
	if operation == nil {
		return nil, fmt.Errorf("terraform operation is required")
	}
	action := operation.GetAction()
	if action == deployment.Terraform_ACTION_UNSPECIFIED {
		action = deployment.Terraform_ACTION_APPLY
	}

	options, err := workdirOptions(input)
	if err != nil {
		return nil, err
	}
	workdir := NewWorkdirWithParams(NewWdId(operation.GetWorkdirId()), options...)
	actor, err := NewActor()
	if err != nil {
		return nil, err
	}

	output := &deployment.Terraform_Output{
		Action:    action,
		WorkdirId: operation.GetWorkdirId(),
		Workdir:   workdir.WorkdirPath(),
	}

	switch action {
	case deployment.Terraform_ACTION_PLAN:
		changed, err := actor.PlanTerraform(ctx, workdir)
		if err != nil {
			return nil, err
		}
		output.Plan = mustBaked("terraform.plan", map[string]any{"has_changes": changed})
	case deployment.Terraform_ACTION_APPLY:
		raw, err := actor.ApplyTerraform(ctx, workdir)
		if err != nil {
			return nil, err
		}
		output.Outputs, err = bakedTerraformOutputs(raw)
		if err != nil {
			return nil, err
		}
	case deployment.Terraform_ACTION_DESTROY:
		if err := actor.DestroyExisting(ctx, NewWdId(operation.GetWorkdirId()), options...); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported terraform action %s", action)
	}

	output.StateFilePresent = workdir.StateFilePresent()
	if err := output.Validate(); err != nil {
		return nil, err
	}
	return output, nil
}

func workdirOptions(input *deployment.Terraform_Input) ([]Option, error) {
	operation := input.GetOperation()
	files := make([]TfFile, 0, len(operation.GetFiles()))
	for _, file := range operation.GetFiles() {
		files = append(files, NewTfFile(file.GetContent(), file.GetPath()))
	}

	var opts []Option
	opts = append(opts,
		WithTfFiles(files),
		WithEnv(operation.GetEnv()),
		WithWorkdirRoot(operation.GetWorkdirRoot()),
		WithVarFileName(operation.GetVarFileName()),
		WithTerraformExecPath(operation.GetTerraformExecPath()),
		WithParallelism(int(operation.GetParallelism())),
		WithPreserveExistingState(operation.GetPreserveExistingState()),
		WithDestroyOnApplyError(operation.GetDestroyOnApplyError()),
	)
	if input.GetTfvars() != nil {
		data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(input.GetTfvars().GetValues())
		if err != nil {
			return nil, fmt.Errorf("marshal terraform tfvars: %w", err)
		}
		opts = append(opts, WithVarFile(TfVarFile(data)))
	}
	return opts, nil
}

func bakedTerraformOutputs(output TfOutput) (*schemapb.Baked, error) {
	values := make(map[string]any, len(output))
	for key, raw := range output {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("decode terraform output %q: %w", key, err)
		}
		values[key] = decoded
	}
	return baked("terraform.outputs", values)
}

func mustBaked(name string, values map[string]any) *schemapb.Baked {
	baked, err := baked(name, values)
	if err != nil {
		panic(err)
	}
	return baked
}

func baked(name string, values map[string]any) (*schemapb.Baked, error) {
	st, err := structpb.NewStruct(values)
	if err != nil {
		return nil, err
	}
	return &schemapb.Baked{
		Schema: &schemapb.Schema{
			Id: &schemapb.SchemaIdentity{
				Namespace: "stroppy-cloud",
				Name:      name,
				Version:   "v1",
			},
		},
		Values: st,
	}, nil
}
