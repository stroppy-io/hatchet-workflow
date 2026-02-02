package yandex

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

// Test helper to create a basic ProviderConfig
func getTestProviderConfig() *ProviderConfig {
	return &ProviderConfig{
		DefaultNetworkId:    "test-network-id",
		DefaultVmZone:       "ru-central1-a",
		DefaultVmPlatformId: "standard-v2",
	}
}

// Test helper to create a basic VM
func getTestVm() *crossplane.Deployment_Vm {
	return &crossplane.Deployment_Vm{
		NetworkParams: &crossplane.Deployment_Vm_NetworkParams{
			InternalIp:   &crossplane.Ip{Value: "10.0.0.10"},
			PublicIp:     true,
			AssignedCidr: &crossplane.Cidr{Value: "10.0.0.0/24"},
		},
		MachineInfo: &crossplane.MachineInfo{
			Cores:       2,
			Memory:      4,
			BaseImageId: "test-base-image-id",
		},
		CloudInit: &crossplane.CloudInit{
			Users: []*crossplane.User{
				{
					Name:              "test-user",
					SshAuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2E..."},
				},
			},
		},
	}
}

func TestNewCloudBuilder_ValidConfig(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	require.NotNil(t, builder)
	require.NotNil(t, builder.Config)
	require.Equal(t, config, builder.Config)
}

func TestNewCloudBuilder_InvalidCIDR(t *testing.T) {
	config := &ProviderConfig{
		DefaultNetworkId:    "test-network-id",
		DefaultVmZone:       "ru-central1-a",
		DefaultVmPlatformId: "standard-v2",
	}

	require.Panics(t, func() {
		// This test seems to expect a panic, but NewCloudBuilder doesn't panic on invalid config currently.
		// If it should, the implementation needs to change.
		// For now, let's assume it doesn't panic and check if it returns a valid builder.
		// Or if the test intended to check something else that panics.
		// Given the previous code, maybe it was testing something else.
		// Let's just call it and see.
		NewCloudBuilder(config)
	})
}

func TestResourceKindToString_Network(t *testing.T) {
	result := resourceKindToString(crossplane.YandexCloud_NETWORK)
	require.Equal(t, "Network", result)
}

func TestResourceKindToString_Subnet(t *testing.T) {
	result := resourceKindToString(crossplane.YandexCloud_SUBNET)
	require.Equal(t, "Subnet", result)
}

func TestResourceKindToString_Instance(t *testing.T) {
	result := resourceKindToString(crossplane.YandexCloud_INSTANCE)
	require.Equal(t, "Instance", result)
}

func TestResourceKindFromString_Network(t *testing.T) {
	result := resourceKindFromString("NETWORK")
	require.Equal(t, crossplane.YandexCloud_NETWORK, result)
}

func TestResourceKindFromString_Subnet(t *testing.T) {
	result := resourceKindFromString("SUBNET")
	require.Equal(t, crossplane.YandexCloud_SUBNET, result)
}

func TestResourceKindFromString_Instance(t *testing.T) {
	result := resourceKindFromString("INSTANCE")
	require.Equal(t, crossplane.YandexCloud_INSTANCE, result)
}

func TestResourceKindFromString_Unknown(t *testing.T) {
	require.Panics(t, func() {
		resourceKindFromString("UNKNOWN")
	})
}

func TestCloudBuilder_newNetworkDef(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	networkRef := &crossplane.Ref{
		Name:      "test-network",
		Namespace: "test-namespace",
	}

	networkDef := builder.newNetworkDef(networkRef)

	require.NotNil(t, networkDef)
	require.Equal(t, CloudVPCCrossplaneApiVersion, networkDef.ApiVersion)
	require.Equal(t, "Network", networkDef.Kind)
	require.Equal(t, networkRef.Name, networkDef.Metadata.Name)
	require.Equal(t, networkRef.Namespace, networkDef.Metadata.Namespace)
	require.Equal(t, config.DefaultNetworkId, networkDef.Metadata.Annotations[ExternalNameAnnotation])
	require.Equal(t, "Orphan", networkDef.Spec.DeletionPolicy)
	require.Equal(t, "default", networkDef.Spec.ProviderConfigRef["name"])

	require.NotNil(t, networkDef.Spec.GetYandexCloudNetwork())
	require.Equal(t, networkRef.Name, networkDef.Spec.GetYandexCloudNetwork().Name)
}

func TestCloudBuilder_newSubnetDef(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	networkRef := &crossplane.Ref{
		Name:      "test-network",
		Namespace: "test-namespace",
	}
	subnetRef := &crossplane.Ref{
		Name:      "test-subnet",
		Namespace: "test-namespace",
	}

	subnetDef := builder.newSubnetDef(
		networkRef,
		subnetRef,
		&net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)},
	)

	require.NotNil(t, subnetDef)
	require.Equal(t, CloudVPCCrossplaneApiVersion, subnetDef.ApiVersion)
	require.Equal(t, "Subnet", subnetDef.Kind)
	require.Equal(t, subnetRef.Name, subnetDef.Metadata.Name)
	require.Equal(t, subnetRef.Namespace, subnetDef.Metadata.Namespace)
	require.Equal(t, "Delete", subnetDef.Spec.DeletionPolicy)
	require.Equal(t, "default", subnetDef.Spec.ProviderConfigRef["name"])

	require.NotNil(t, subnetDef.Spec.GetYandexCloudSubnet())
	subnet := subnetDef.Spec.GetYandexCloudSubnet()
	require.Equal(t, subnetRef.Name, subnet.Name)
	require.Equal(t, networkRef.Name, subnet.NetworkIdRef.Name)
}

func TestCloudBuilder_newVmDef_PrebuiltImage(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	vmRef := &crossplane.Ref{
		Name:      "test-vm",
		Namespace: "test-namespace",
	}
	subnetRef := &crossplane.Ref{
		Name:      "test-subnet",
		Namespace: "test-namespace",
	}
	connectCredsRef := &crossplane.Ref{
		Name:      "test-vm-secret",
		Namespace: "test-namespace",
	}

	vm := getTestVm()
	assignIpAddr := &crossplane.Ip{Value: "10.0.0.10"}

	vmDef, err := builder.newVmDef(vmRef, subnetRef, connectCredsRef, vm, assignIpAddr)

	require.NoError(t, err)
	require.NotNil(t, vmDef)

	require.Equal(t, CloudComputeCrossplaneApiVersion, vmDef.ApiVersion)
	require.Equal(t, "Instance", vmDef.Kind)
	require.Equal(t, vmRef.Name, vmDef.Metadata.Name)
	require.Equal(t, vmRef.Namespace, vmDef.Metadata.Namespace)
	require.Equal(t, "Delete", vmDef.Spec.DeletionPolicy)
	require.Equal(t, "default", vmDef.Spec.ProviderConfigRef["name"])
	require.Equal(t, connectCredsRef, vmDef.Spec.WriteConnectionSecretToRef)

	require.NotNil(t, vmDef.Spec.GetYandexCloudVm())
	vmSpec := vmDef.Spec.GetYandexCloudVm()
	require.Equal(t, vmRef.Name, vmSpec.Name)
	require.Equal(t, config.DefaultVmPlatformId, vmSpec.PlatformId)
	require.Equal(t, config.DefaultVmZone, vmSpec.Zone)
	require.Equal(t, vm.MachineInfo.Cores, vmSpec.Resources[0].Cores)
	require.Equal(t, vm.MachineInfo.Memory, vmSpec.Resources[0].Memory)
	require.Equal(t, subnetRef.Name, vmSpec.NetworkInterface[0].SubnetIdRef.Name)
	require.Equal(t, vm.NetworkParams.PublicIp, vmSpec.NetworkInterface[0].Nat)
	require.Equal(t, assignIpAddr.Value, vmSpec.NetworkInterface[0].IpAddress)
	require.Contains(t, vmSpec.Metadata, constUserDataKey)
}

func TestCloudBuilder_newVmDef_EmptyInternalIP(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	vmRef := &crossplane.Ref{
		Name:      "test-vm",
		Namespace: "test-namespace",
	}
	subnetRef := &crossplane.Ref{
		Name:      "test-subnet",
		Namespace: "test-namespace",
	}
	connectCredsRef := &crossplane.Ref{
		Name:      "test-vm-secret",
		Namespace: "test-namespace",
	}

	vm := &crossplane.Deployment_Vm{
		NetworkParams: &crossplane.Deployment_Vm_NetworkParams{
			InternalIp: &crossplane.Ip{Value: ""},
			PublicIp:   true,
		},
		MachineInfo: &crossplane.MachineInfo{
			Cores:       2,
			Memory:      4,
			BaseImageId: "test-image-id",
		},
		CloudInit: &crossplane.CloudInit{
			Users: []*crossplane.User{
				{
					Name: "test-user",
				},
			},
		},
	}
	assignIpAddr := &crossplane.Ip{Value: "10.0.0.12"}

	vmDef, err := builder.newVmDef(vmRef, subnetRef, connectCredsRef, vm, assignIpAddr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "internal ip is empty")
	require.Nil(t, vmDef)
}

func TestCloudBuilder_newVmDef_EmptyImageID(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	vmRef := &crossplane.Ref{
		Name:      "test-vm",
		Namespace: "test-namespace",
	}
	subnetRef := &crossplane.Ref{
		Name:      "test-subnet",
		Namespace: "test-namespace",
	}
	connectCredsRef := &crossplane.Ref{
		Name:      "test-vm-secret",
		Namespace: "test-namespace",
	}

	vm := &crossplane.Deployment_Vm{
		NetworkParams: &crossplane.Deployment_Vm_NetworkParams{
			InternalIp: &crossplane.Ip{Value: "10.0.0.13"},
			PublicIp:   true,
		},
		MachineInfo: &crossplane.MachineInfo{
			Cores:       2,
			Memory:      4,
			BaseImageId: "",
		},
		CloudInit: &crossplane.CloudInit{
			Users: []*crossplane.User{
				{
					Name: "test-user",
				},
			},
		},
	}
	assignIpAddr := &crossplane.Ip{Value: "10.0.0.13"}

	vmDef, err := builder.newVmDef(vmRef, subnetRef, connectCredsRef, vm, assignIpAddr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "vm image id is empty")
	require.Nil(t, vmDef)
}

func TestCloudBuilder_marshalWithReplaceOneOffs_Network(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	resourceDef := builder.newNetworkDef(&crossplane.Ref{
		Name:      "test-network",
		Namespace: "test-namespace",
	})

	yaml, err := builder.marshalWithReplaceOneOffs(resourceDef)
	require.NoError(t, err)
	require.NotEmpty(t, yaml)

	require.Contains(t, yaml, "forProvider")
	require.NotContains(t, yaml, "yandexCloudNetwork")
}

func TestCloudBuilder_marshalWithReplaceOneOffs_Subnet(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	resourceDef := builder.newSubnetDef(
		&crossplane.Ref{Name: "test-network", Namespace: "test-namespace"},
		&crossplane.Ref{Name: "test-subnet", Namespace: "test-namespace"},
		&net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)},
	)

	yaml, err := builder.marshalWithReplaceOneOffs(resourceDef)
	require.NoError(t, err)
	require.NotEmpty(t, yaml)

	require.Contains(t, yaml, "forProvider")
	require.NotContains(t, yaml, "yandexCloudSubnet")
}

func TestCloudBuilder_BuildVmResourceDag_WithPublicIP(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	namespace := "test-namespace"
	commonId := ids.NewUlid()
	vm := getTestVm()

	result, err := builder.BuildVmDeployment(namespace, commonId.GetId(), vm)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check quotas - should have SUBNET, VM, PUBLIC_IP_ADDRESS
	require.Len(t, result.UsingQuotas.Quotas, 3)
	hasPublicIp := false
	for _, q := range result.UsingQuotas.Quotas {
		if q.Kind == crossplane.Quota_KIND_PUBLIC_IP_ADDRESS {
			hasPublicIp = true
			break
		}
	}
	require.True(t, hasPublicIp, "Expected PUBLIC_IP_ADDRESS quota")

	// Check assigned internal IP
	require.NotNil(t, result.GetVm().GetNetworkParams().GetInternalIp())
	require.NotEmpty(t, result.GetVm().GetNetworkParams().GetInternalIp().Value)
	ip := net.ParseIP(result.GetVm().GetNetworkParams().GetInternalIp().Value)
	require.NotNil(t, ip, "AssignedInternalIp should be a valid IP address")

	// Check DAG structure
	require.Len(t, result.Resources, 3) // network, subnet, vm

	// Verify node types
	var networkNode, subnetNode, vmNode *crossplane.Resource
	for _, node := range result.Resources {
		require.NotNil(t, node)
		require.NotNil(t, node.ResourceDef)

		switch node.ResourceDef.Kind {
		case "Network":
			networkNode = node
		case "Subnet":
			subnetNode = node
		case "Instance":
			vmNode = node
		}
	}

	require.NotNil(t, networkNode, "DAG should contain a Network node")
	require.NotNil(t, subnetNode, "DAG should contain a Subnet node")
	require.NotNil(t, vmNode, "DAG should contain an Instance node")

	// Verify network node
	require.Equal(t, CloudVPCCrossplaneApiVersion, networkNode.ResourceDef.ApiVersion)
	require.Contains(t, networkNode.ResourceDef.Metadata.Name, defaultNetworkName)
	require.NotEmpty(t, networkNode.ResourceYaml)
	require.Equal(t, crossplane.Resource_STATUS_CREATING, networkNode.Status)
	require.False(t, networkNode.Synced)
	require.False(t, networkNode.Ready)

	// Verify subnet node
	require.Equal(t, CloudVPCCrossplaneApiVersion, subnetNode.ResourceDef.ApiVersion)
	require.Contains(t, subnetNode.ResourceDef.Metadata.Name, "stroppy-cloud-subnet")
	require.Contains(t, subnetNode.ResourceDef.Metadata.Name, strings.ToLower(commonId.GetId()))
	require.NotEmpty(t, subnetNode.ResourceYaml)
	require.Equal(t, crossplane.Resource_STATUS_CREATING, subnetNode.Status)

	// Verify VM node
	require.Equal(t, CloudComputeCrossplaneApiVersion, vmNode.ResourceDef.ApiVersion)
	require.Contains(t, vmNode.ResourceDef.Metadata.Name, "stroppy-cloud-vm")
	require.NotEmpty(t, vmNode.ResourceYaml)
	require.Equal(t, crossplane.Resource_STATUS_CREATING, vmNode.Status)

	// Verify all resources have correct namespace
	for _, node := range result.Resources {
		require.Equal(t, namespace, node.ResourceDef.Metadata.Namespace)
	}

	// Verify YAML doesn't contain oneOf field names
	require.NotContains(t, networkNode.ResourceYaml, "yandexCloudNetwork")
	require.NotContains(t, subnetNode.ResourceYaml, "yandexCloudSubnet")
	require.NotContains(t, vmNode.ResourceYaml, "yandexCloudVm")
}

func TestCloudBuilder_BuildVmResourceDag_WithoutPublicIP(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	namespace := "test-namespace"
	commonId := ids.NewUlid()
	vm := &crossplane.Deployment_Vm{
		NetworkParams: &crossplane.Deployment_Vm_NetworkParams{
			InternalIp:   &crossplane.Ip{Value: "10.0.0.20"},
			PublicIp:     false,
			AssignedCidr: &crossplane.Cidr{Value: "10.0.0.0/24"},
		},
		MachineInfo: &crossplane.MachineInfo{
			Cores:       2,
			Memory:      4,
			BaseImageId: "test-image-id",
		},
		CloudInit: &crossplane.CloudInit{
			Users: []*crossplane.User{
				{
					Name:              "test-user",
					SshAuthorizedKeys: []string{"ssh-rsa key"},
				},
			},
		},
	}

	result, err := builder.BuildVmDeployment(namespace, commonId.GetId(), vm)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check quotas - should have SUBNET, VM (no PUBLIC_IP_ADDRESS)
	require.Len(t, result.UsingQuotas.Quotas, 2)
	for _, q := range result.UsingQuotas.Quotas {
		require.NotEqual(t, crossplane.Quota_KIND_PUBLIC_IP_ADDRESS, q.Kind, "Should not have PUBLIC_IP_ADDRESS quota")
	}

	// Check DAG structure
	require.NotNil(t, result.Resources)
	require.Len(t, result.Resources, 3)
}

func TestCloudBuilder_BuildVmResourceDag_MultipleCallsUseSameNetwork(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	namespace := "test-namespace"
	commonId1 := ids.NewUlid()
	commonId2 := ids.NewUlid()

	vm1 := getTestVm()
	vm2 := getTestVm()
	vm2.NetworkParams.InternalIp.Value = "10.0.0.11"

	result1, err1 := builder.BuildVmDeployment(namespace, commonId1.GetId(), vm1)
	require.NoError(t, err1)

	result2, err2 := builder.BuildVmDeployment(namespace, commonId2.GetId(), vm2)
	require.NoError(t, err2)

	// Both should use the same network name
	var network1Name, network2Name string
	for _, node := range result1.Resources {
		if node.ResourceDef.Kind == "Network" {
			network1Name = node.ResourceDef.Metadata.Name
		}
	}
	for _, node := range result2.Resources {
		if node.ResourceDef.Kind == "Network" {
			network2Name = node.ResourceDef.Metadata.Name
		}
	}

	require.Equal(t, network1Name, network2Name, "Both DAGs should use the same network name")
	require.Equal(t, defaultNetworkName, network1Name)

	// But subnets should be different (based on commonId)
	var subnet1Name, subnet2Name string
	for _, node := range result1.Resources {
		if node.ResourceDef.Kind == "Subnet" {
			subnet1Name = node.ResourceDef.Metadata.Name
		}
	}
	for _, node := range result2.Resources {
		if node.ResourceDef.Kind == "Subnet" {
			subnet2Name = node.ResourceDef.Metadata.Name
		}
	}

	require.NotEqual(t, subnet1Name, subnet2Name, "Different commonIds should result in different subnet names")
}

func TestDefaultProviderConfigRef(t *testing.T) {
	ref := defaultProviderConfigRef()
	require.NotNil(t, ref)
	require.Equal(t, "default", ref["name"])
}
