// Package terraformprov is the Yandex Cloud Terraform provisioner seam: it turns
// a deployment.Yandex (whose Input IS the terraform variables — see
// deployment/yandex.proto) into running infrastructure via the single embedded
// yandex terraform root + the terraform.Actor, and reads the VM/YDB endpoints
// back into Yandex.Output. Apply and Destroy share a workdir (keyed by run id)
// so teardown reuses the same state.
package terraformprov

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

// Apply provisions the single Yandex terraform root from y.Input (which IS the
// tfvars), reusing a workdir keyed by runID so teardown reuses state, and fills
// y.Output with the VM + managed-YDB endpoints.
func Apply(ctx context.Context, runID string, y *deployment.Yandex, env map[string]string) (*deployment.Yandex, error) {
	files, err := yandex.EmbeddedTfFiles()
	if err != nil {
		return nil, fmt.Errorf("terraformprov: embed module: %w", err)
	}
	// proto IS the tfvars: protojson with proto field names yields
	// terraform.tfvars.json keys that match variables.tf 1:1.
	data, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(y.GetInput())
	if err != nil {
		return nil, fmt.Errorf("terraformprov: marshal tfvars: %w", err)
	}
	wd := terraform.NewWorkdirWithParams(terraform.NewWdId(runID),
		terraform.WithTfFiles(files),
		terraform.WithVarFile(terraform.TfVarFile(data)),
		terraform.WithEnv(terraform.TfEnv(env)),
	)
	actor, err := terraform.NewActor()
	if err != nil {
		return nil, fmt.Errorf("terraformprov: new actor: %w", err)
	}
	out, err := actor.ApplyTerraform(ctx, wd)
	if err != nil {
		return nil, fmt.Errorf("terraformprov: apply: %w", err)
	}
	y.Output = parseOutput(out)
	return y, nil
}

// Destroy tears the deployment down, reusing the shared workdir/state.
func Destroy(ctx context.Context, runID string, env map[string]string) error {
	actor, err := terraform.NewActor()
	if err != nil {
		return fmt.Errorf("terraformprov: new actor: %w", err)
	}
	return actor.DestroyExisting(ctx, terraform.NewWdId(runID), terraform.WithEnv(terraform.TfEnv(env)))
}

// vmOut mirrors the terraform "vms" output entry.
type vmOut struct {
	ID         string `json:"id"`
	PublicIP   string `json:"public_ip"`
	InternalIP string `json:"internal_ip"`
}

// managedYdbOut mirrors the terraform "managed_ydb" output object (null when no
// managed YDB is requested).
type managedYdbOut struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	FolderID            string `json:"folder_id"`
	LocationID          string `json:"location_id"`
	DatabasePath        string `json:"database_path"`
	YdbAPIEndpoint      string `json:"ydb_api_endpoint"`
	YdbFullEndpoint     string `json:"ydb_full_endpoint"`
	DocumentAPIEndpoint string `json:"document_api_endpoint"`
	TLSEnabled          bool   `json:"tls_enabled"`
	Status              string `json:"status"`
	CreatedAt           string `json:"created_at"`
	ResourcePresetID    string `json:"resource_preset_id"`
	NetworkID           string `json:"network_id"`
}

// parseOutput reads the module outputs (vms, service_account_id, managed_ydb)
// into the typed Yandex.Output. Missing/null outputs are tolerated.
func parseOutput(out terraform.TfOutput) *deployment.Yandex_Output {
	res := &deployment.Yandex_Output{Vms: map[string]*deployment.Yandex_VmOutput{}}

	if vms, err := terraform.GetTfOutputVal[map[string]vmOut](out, "vms"); err == nil {
		for name, v := range vms {
			res.Vms[name] = &deployment.Yandex_VmOutput{Id: v.ID, InternalIp: v.InternalIP, PublicIp: v.PublicIP}
		}
	}
	if said, err := terraform.GetTfOutputVal[string](out, "service_account_id"); err == nil {
		res.ServiceAccountId = said
	}
	if my, err := terraform.GetTfOutputVal[*managedYdbOut](out, "managed_ydb"); err == nil && my != nil {
		res.ManagedYdb = &deployment.Yandex_ManagedYdbOutput{
			Id:                  my.ID,
			Name:                my.Name,
			Type:                my.Type,
			FolderId:            my.FolderID,
			LocationId:          my.LocationID,
			DatabasePath:        my.DatabasePath,
			YdbApiEndpoint:      my.YdbAPIEndpoint,
			YdbFullEndpoint:     my.YdbFullEndpoint,
			DocumentApiEndpoint: my.DocumentAPIEndpoint,
			TlsEnabled:          my.TLSEnabled,
			Status:              my.Status,
			CreatedAt:           my.CreatedAt,
			ResourcePresetId:    my.ResourcePresetID,
			NetworkId:           my.NetworkID,
		}
	}
	return res
}
