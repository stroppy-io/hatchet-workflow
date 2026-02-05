package yandex

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

// Test helper to create a basic ProviderConfig
func getTestProviderConfig() *ProviderConfig {
	return &ProviderConfig{
		K8sNamespace:        "test-namespace",
		DefaultVmZone:       "ru-central1-a",
		DefaultVmPlatformId: "standard-v2",
	}
}

func TestNewCloudBuilder(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	require.NotNil(t, builder)
	require.Equal(t, config, builder.Config)
}

func TestResourceKindToString(t *testing.T) {
	require.Equal(t, "Network", resourceKindToString(crossplane.YandexCloud_NETWORK))
	require.Equal(t, "Subnet", resourceKindToString(crossplane.YandexCloud_SUBNET))
	require.Equal(t, "Instance", resourceKindToString(crossplane.YandexCloud_INSTANCE))
}

func TestCloudBuilder_BuildNetwork(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	template := &crossplane.Network_Template{
		Identifier: &crossplane.Identifier{
			Name: "test-network",
		},
		ExternalId: "ext-net-id",
	}

	network, err := builder.BuildNetwork(template)
	require.NoError(t, err)
	require.NotNil(t, network)

	// Check ResourceDef
	def := network.Resource.ResourceDef
	require.Equal(t, CloudVPCCrossplaneApiVersion, def.ApiVersion)
	require.Equal(t, "Network", def.Kind)
	require.Equal(t, "test-network", def.Metadata.Name)
	require.Equal(t, config.K8sNamespace, def.Metadata.Namespace)
	require.Equal(t, "ext-net-id", def.Metadata.Annotations[ExternalNameAnnotation])
	require.Equal(t, "Orphan", def.Spec.DeletionPolicy)

	// Check ForProvider
	yandexNet := def.Spec.GetYandexCloudNetwork()
	require.NotNil(t, yandexNet)
	require.Equal(t, "test-network", yandexNet.Name)

	// Check YAML replacement
	require.Contains(t, network.Resource.ResourceYaml, "forProvider")
	require.NotContains(t, network.Resource.ResourceYaml, "yandexCloudNetwork")
}

func TestCloudBuilder_BuildSubnet(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	networkTemplate := &crossplane.Network_Template{
		Identifier: &crossplane.Identifier{Name: "test-network"},
	}
	network, _ := builder.BuildNetwork(networkTemplate)
	network.Template = networkTemplate

	subnetTemplate := &crossplane.Subnet_Template{
		Identifier: &crossplane.Identifier{
			Name: "test-subnet",
		},
		Cidr: &crossplane.Cidr{
			Value: "10.0.0.0/24",
		},
	}

	subnet, err := builder.BuildSubnet(network, subnetTemplate)
	require.NoError(t, err)
	require.NotNil(t, subnet)

	// Check ResourceDef
	def := subnet.Resource.ResourceDef
	require.Equal(t, CloudVPCCrossplaneApiVersion, def.ApiVersion)
	require.Equal(t, "Subnet", def.Kind)
	require.Equal(t, "test-subnet", def.Metadata.Name)
	require.Equal(t, config.K8sNamespace, def.Metadata.Namespace)
	require.Equal(t, "Delete", def.Spec.DeletionPolicy)

	// Check ForProvider
	yandexSubnet := def.Spec.GetYandexCloudSubnet()
	require.NotNil(t, yandexSubnet)
	require.Equal(t, "test-subnet", yandexSubnet.Name)
	require.Equal(t, "test-network", yandexSubnet.NetworkIdRef.Name)
	require.Equal(t, []string{"10.0.0.0/24"}, yandexSubnet.V4CidrBlocks)
	require.Equal(t, config.DefaultVmZone, yandexSubnet.Zone)

	// Check YAML replacement
	require.Contains(t, subnet.Resource.ResourceYaml, "forProvider")
	require.NotContains(t, subnet.Resource.ResourceYaml, "yandexCloudSubnet")
}

func TestCloudBuilder_BuildVm(t *testing.T) {
	config := getTestProviderConfig()
	builder := NewCloudBuilder(config)

	networkTemplate := &crossplane.Network_Template{
		Identifier: &crossplane.Identifier{Name: "test-network"},
	}
	network, _ := builder.BuildNetwork(networkTemplate)
	network.Template = networkTemplate

	subnetTemplate := &crossplane.Subnet_Template{
		Identifier: &crossplane.Identifier{Name: "test-subnet"},
		Cidr:       &crossplane.Cidr{Value: "10.0.0.0/24"},
	}
	subnet, _ := builder.BuildSubnet(network, subnetTemplate)
	subnet.Template = subnetTemplate

	vmTemplate := &crossplane.Vm_Template{
		Identifier: &crossplane.Identifier{
			Name: "test-vm",
		},
		Hardware: &crossplane.Hardware{
			Cores:  2,
			Memory: 4,
		},
		BaseImageId: "image-id",
		PublicIp:    true,
		InternalIp: &crossplane.Ip{
			Value: "10.0.0.5",
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

	vm, err := builder.BuildVm(network, subnet, vmTemplate)
	require.NoError(t, err)
	require.NotNil(t, vm)

	// Check ResourceDef
	def := vm.Resource.ResourceDef
	require.Equal(t, CloudComputeCrossplaneApiVersion, def.ApiVersion)
	require.Equal(t, "Instance", def.Kind)
	require.Equal(t, "test-vm", def.Metadata.Name)
	require.Equal(t, config.K8sNamespace, def.Metadata.Namespace)
	require.Equal(t, "Delete", def.Spec.DeletionPolicy)

	// Check ForProvider
	yandexVm := def.Spec.GetYandexCloudVm()
	require.NotNil(t, yandexVm)
	require.Equal(t, "test-vm", yandexVm.Name)
	require.Equal(t, config.DefaultVmPlatformId, yandexVm.PlatformId)
	require.Equal(t, config.DefaultVmZone, yandexVm.Zone)
	require.Equal(t, uint32(2), yandexVm.Resources[0].Cores)
	require.Equal(t, uint32(4), yandexVm.Resources[0].Memory)
	require.Equal(t, "image-id", yandexVm.BootDisk[0].InitializeParams[0].ImageId)
	require.Equal(t, "test-subnet", yandexVm.NetworkInterface[0].SubnetIdRef.Name)
	require.True(t, yandexVm.NetworkInterface[0].Nat)
	require.Equal(t, "10.0.0.5", yandexVm.NetworkInterface[0].IpAddress)
	require.Contains(t, yandexVm.Metadata, constUserDataKey)

	// Check Quotas
	require.Len(t, vm.Resource.UsingQuotas, 2) // VM + Public IP

	// Check YAML replacement
	require.Contains(t, vm.Resource.ResourceYaml, "forProvider")
	require.NotContains(t, vm.Resource.ResourceYaml, "yandexCloudVm")
}
