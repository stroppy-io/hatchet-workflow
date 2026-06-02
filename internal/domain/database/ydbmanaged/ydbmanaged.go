// Package ydbmanaged builds the topology + deployment shape for a Yandex
// Managed Service for YDB database.
//
// Unlike the self-hosted ydb engine, a managed YDB database is provisioned by
// the cloud provider (Yandex Managed YDB), not by an agent on a VM. So the
// topology emits a single DATABASE/EXTERNAL component on a dedicated node that
// carries NO provider VM — the infrastructure builder skips spawning a VM for a
// node labelled as managed (see internal/domain/infrastructure), and the
// provider terraform input gets a `managed_ydb` block instead. The endpoint of
// the provisioned database is fed back as the node's private endpoint so the
// stroppy workload connects to it exactly like it connects to a self-hosted DB.
package ydbmanaged

import (
	"errors"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// LabelManagedInput is the topology-spec label key under which the managed-YDB
// provider input (a deployment.Yandex_ManagedYdb encoded as protojson) is
// carried. The infrastructure builder copies it onto the InfrastructurePlan so
// the provider terraform-input render can attach it as the `managed_ydb` block.
// The provider-account fields (name/folder_id/location_id/service_account) are
// left empty here and filled at render time from provider settings.
const LabelManagedInput = "managed_ydb_input"

// LabelManaged marks the spec/node as a provider-managed database (no VM).
const LabelManaged = "managed"

const (
	// Engine is the topology engine name for managed YDB. It is intentionally
	// distinct from the self-hosted "ydb" engine so the deployment renderer
	// registry routes managed databases to the no-op renderer (no install).
	Engine = "ydb-managed"

	// Role is the managed database component role.
	Role = "managed"

	// NodeID / ComponentID are the stable ids for the managed YDB database.
	NodeID      = "ydb-managed-1"
	ComponentID = "ydb-managed-1"

	// grpcPort is the default YDB client gRPC(S) port. Used as the connection
	// port until the provider reports the real endpoint port.
	grpcPort = 2136
)

// Database builds the managed-YDB topology spec.
type Database struct{}

func (d *Database) ValidateInput(input *domain.YdbManagedParams) error {
	if input == nil {
		return errors.New("ydb managed params are required")
	}
	return input.Validate()
}

// BuildTopologySpec emits a single managed-YDB database component on its own
// node. The node is NOT backed by a provider VM (the infrastructure builder
// recognises the engine label and skips VM allocation); instead the provider
// provisions the managed database and reports its endpoint, which is wired back
// as the node's private endpoint.
func (d *Database) BuildTopologySpec(input *domain.YdbManagedParams) (*topology.TopologySpec, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, err
	}

	managedInput := BuildManagedInput(input)
	encoded, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(managedInput)
	if err != nil {
		return nil, fmt.Errorf("encode managed ydb input: %w", err)
	}

	component := dbspec.Component(Engine, ComponentID, topology.Component_KIND_EXTERNAL, Role, NodeID)
	node := dbspec.Node(Engine, NodeID, Role, 1, []string{ComponentID})
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	node.Labels[LabelManaged] = "true"

	spec := &topology.TopologySpec{
		Nodes:       []*topology.Node{node},
		Components:  []*topology.Component{component},
		Connections: make([]*topology.Connection, 0),
		Labels: map[string]string{
			"kind":               "database",
			"engine":             Engine,
			LabelManaged:         "true",
			"managed_type":       managedTypeLabel(input.GetType()),
			"resource_preset_id": input.GetResourcePresetId(),
			"node_count":         strconv.FormatUint(uint64(input.GetNodeCount()), 10),
			"storage_groups":     strconv.FormatUint(uint64(input.GetStorageGroups()), 10),
			"storage_type":       input.GetStorageType(),
			"managed_grpc_port":  strconv.Itoa(grpcPort),
			"managed_component":  ComponentID,
			"managed_node":       NodeID,
			LabelManagedInput:    string(encoded),
		},
		Tags: dbspec.Tags(Engine, Role),
	}
	return spec, nil
}

// BuildManagedInput maps the domain YdbManagedParams onto the provider-neutral
// deployment.Yandex_ManagedYdb input. The serverless/dedicated oneof is driven
// by YdbManagedParams.Type. Account-scoped fields (name/folder_id/location_id/
// service_account_name) are filled at render time from provider settings.
func BuildManagedInput(input *domain.YdbManagedParams) *deployment.Yandex_ManagedYdb {
	managed := &deployment.Yandex_ManagedYdb{}

	switch input.GetType() {
	case domain.YdbManagedParams_TYPE_DEDICATED:
		dedicated := &deployment.Yandex_ManagedYdb_Dedicated{
			ResourcePresetId: input.GetResourcePresetId(),
			StorageConfig: &deployment.Yandex_ManagedYdb_StorageConfig{
				GroupCount:    input.GetStorageGroups(),
				StorageTypeId: input.GetStorageType(),
			},
			ScalePolicy: scalePolicy(input),
		}
		managed.Kind = &deployment.Yandex_ManagedYdb_Dedicated_{Dedicated: dedicated}
	default:
		// SERVERLESS (and unspecified → serverless default).
		serverless := &deployment.Yandex_ManagedYdb_Serverless{}
		if rcu := input.GetThrottlingRcus(); rcu > 0 {
			serverless.EnableThrottlingRcuLimit = true
			serverless.ThrottlingRcuLimit = rcu
		}
		managed.Kind = &deployment.Yandex_ManagedYdb_Serverless_{Serverless: serverless}
	}
	return managed
}

// scalePolicy maps the dedicated node_count / auto_scale knobs to the provider
// scale-policy oneof (fixed vs auto).
func scalePolicy(input *domain.YdbManagedParams) *deployment.Yandex_ManagedYdb_ScalePolicy {
	if auto := input.GetAutoScale(); auto != nil {
		pct := auto.GetCpuUtilizationPercent()
		if pct == 0 {
			pct = 70
		}
		return &deployment.Yandex_ManagedYdb_ScalePolicy{
			Policy: &deployment.Yandex_ManagedYdb_ScalePolicy_Auto_{
				Auto: &deployment.Yandex_ManagedYdb_ScalePolicy_Auto{
					MinSize:               auto.GetMinSize(),
					MaxSize:               auto.GetMaxSize(),
					CpuUtilizationPercent: pct,
				},
			},
		}
	}
	size := input.GetNodeCount()
	if size == 0 {
		size = 1
	}
	return &deployment.Yandex_ManagedYdb_ScalePolicy{
		Policy: &deployment.Yandex_ManagedYdb_ScalePolicy_Fixed_{
			Fixed: &deployment.Yandex_ManagedYdb_ScalePolicy_Fixed{Size: size},
		},
	}
}

func managedTypeLabel(t domain.YdbManagedParams_Type) string {
	switch t {
	case domain.YdbManagedParams_TYPE_DEDICATED:
		return "dedicated"
	case domain.YdbManagedParams_TYPE_SERVERLESS:
		return "serverless"
	default:
		return "serverless"
	}
}
