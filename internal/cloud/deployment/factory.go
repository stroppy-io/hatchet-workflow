package deployment

import (
	"fmt"
	"net"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

type ResourceBuilder interface {
	BuildNetwork(
		template *crossplane.Network_Template,
	) (*crossplane.Network, error)
	BuildSubnet(
		network *crossplane.Network,
		template *crossplane.Subnet_Template,
	) (*crossplane.Subnet, error)
	BuildVm(
		network *crossplane.Network,
		subnet *crossplane.Subnet,
		template *crossplane.Vm_Template,
	) (*crossplane.Vm, error)
}

type BuildersMap map[crossplane.SupportedCloud]ResourceBuilder

type Factory struct {
	mapping BuildersMap
}

func NewDeploymentFactory(mapping BuildersMap) *Factory {
	return &Factory{
		mapping: mapping,
	}
}

var ErrUnsupportedCloudType = fmt.Errorf("unsupported cloud type")

func cidrContainsIP(cidr *crossplane.Cidr, ip *crossplane.Ip) (bool, error) {
	_, ipNet, err := net.ParseCIDR(cidr.GetValue())
	if err != nil {
		return false, err
	}
	return ipNet.Contains(net.ParseIP(ip.GetValue())), nil
}

func foundSubnetForVm(subnets []*crossplane.Subnet, vmTemplate *crossplane.Vm_Template) (*crossplane.Subnet, error) {
	for _, subnet := range subnets {
		has, err := cidrContainsIP(subnet.GetTemplate().GetCidr(), vmTemplate.GetInternalIp())
		if err != nil {
			return nil, fmt.Errorf(
				"failed to check if subnet %s contains ip %s : %w",
				subnet.GetTemplate().GetCidr().GetValue(),
				vmTemplate.GetInternalIp().GetValue(),
				err,
			)
		}
		if has {
			return subnet, nil
		}
	}
	return nil, fmt.Errorf("subnet not found for vmTemplate %s", vmTemplate.GetIdentifier().GetName())
}

func (d *Factory) CreateNewDeployment(
	template *crossplane.Deployment_Template,
) (*crossplane.Deployment, error) {
	err := template.Validate()
	if err != nil {
		return nil, err
	}

	var resourceBuilder ResourceBuilder
	switch template.GetIdentifier().GetSupportedCloud() {
	case crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX:
		resourceBuilder = d.mapping[crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX]
	default:
		return nil, ErrUnsupportedCloudType
	}

	network, err := resourceBuilder.BuildNetwork(template.GetNetworkTemplate())
	if err != nil {
		return nil, err
	}
	subnets := make([]*crossplane.Subnet, len(template.GetNetworkTemplate().GetSubnets()))
	for i, subnetTemplate := range template.GetNetworkTemplate().GetSubnets() {
		subnet, err := resourceBuilder.BuildSubnet(network, subnetTemplate)
		if err != nil {
			return nil, err
		}
		subnets[i] = subnet
	}
	network.Subnets = append(network.Subnets, subnets...)

	vms := make([]*crossplane.Vm, len(template.GetVmTemplates()))
	for i, vmTemplate := range template.GetVmTemplates() {
		subnet, err := foundSubnetForVm(network.Subnets, vmTemplate)
		if err != nil {
			return nil, err
		}
		vms[i], err = resourceBuilder.BuildVm(network, subnet, vmTemplate)
		if err != nil {
			return nil, err
		}
	}

	return &crossplane.Deployment{
		Template: template,
		Vms:      vms,
		Network:  network,
	}, nil
}
