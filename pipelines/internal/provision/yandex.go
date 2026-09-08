package provision

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	compute "github.com/yandex-cloud/crossplane-provider-yc/apis/cluster/compute/v1alpha1"
	vpc "github.com/yandex-cloud/crossplane-provider-yc/apis/cluster/vpc/v1alpha1"

	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// YandexSettings is the baked provider.yandex.settings@1 value.
type YandexSettings struct {
	CloudID    string `json:"cloud_id"`
	FolderID   string `json:"folder_id"`
	Zone       string `json:"zone"`
	PlatformID string `json:"platform_id"`
	Network    struct {
		Kind            string `json:"kind"`
		SubnetCIDR      string `json:"subnet_cidr"`
		NetworkID       string `json:"network_id"`
		SubnetID        string `json:"subnet_id"`
		SecurityGroupID string `json:"security_group_id"`
	} `json:"network"`
	PublicIPs   bool   `json:"public_ips"`
	ImageFamily string `json:"image_family"`
	Preemptible bool   `json:"preemptible"`
}

// Yandex provisions through crossplane-provider-yc.
type Yandex struct{}

// Kind is yandex.
func (Yandex) Kind() spec.ProviderKind { return spec.ProviderYandex }

// Scheme teaches a k8s client the provider's types.
func (Yandex) Scheme() k8slib.ClientOption {
	return k8slib.WithScheme(compute.SchemeBuilder.AddToScheme, vpc.SchemeBuilder.AddToScheme)
}

// Record declares one of every kind (see Provider).
func (y Yandex) Record(ctx pipeline.Context, k8s *k8slib.Client) {
	k8slib.Resource(ctx, k8s, "record-net", &vpc.Network{}, k8slib.WithReady(yandexNetworkReady))
	k8slib.Resource(ctx, k8s, "record-subnet", &vpc.Subnet{}, k8slib.WithReady(yandexSubnetReady))
	k8slib.Resource(ctx, k8s, "record-sg", &vpc.SecurityGroup{}, k8slib.WithReady(yandexSGReady))
	k8slib.Resource(ctx, k8s, "record-disk", &compute.Disk{}, k8slib.WithReady(yandexDiskReady))
	k8slib.Resource(ctx, k8s, "record-vm", &compute.Instance{}, k8slib.WithReady(yandexInstanceReady))
}

// Provision declares network → subnet → security group → disks + instances.
//
//nolint:funlen // one declaration per resource kind, read top to bottom
func (y Yandex) Provision(ctx pipeline.Context, k8s *k8slib.Client, run spec.Run, agents map[string]pipeline.AgentHandle) (Infra, error) {
	st, err := decodeSettings[YandexSettings](run.Provider.Settings)
	if err != nil {
		return Infra{}, err
	}
	if st.FolderID == "" {
		return Infra{}, fmt.Errorf("provision/yandex: folder_id is required")
	}
	names := NewNames(run.Tenant, run.RunID)
	pc := run.Provider.ProviderConfigName
	zone := st.Zone
	public := run.Network.AllowPublicIPs || st.PublicIPs

	// Network → subnet → security group: one chain, each a child of the
	// previous, so handing the network to the stand moves all of it and a
	// finishing run never strands a subnet under itself.
	net := k8slib.Resource(ctx, k8s, names.Network(), &vpc.Network{
		Spec: vpc.NetworkSpec{
			ResourceSpec: providerRef(pc),
			ForProvider:  vpc.NetworkParameters{FolderID: ptr(st.FolderID), Name: ptr(names.Network())},
		},
	}, k8slib.WithReady(yandexNetworkReady))

	sub := k8slib.Resource(ctx, k8s, names.Subnet(), &vpc.Subnet{
		Spec: vpc.SubnetSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: vpc.SubnetParameters{
				FolderID:     ptr(st.FolderID),
				Name:         ptr(names.Subnet()),
				NetworkIDRef: &xpv1.Reference{Name: names.Network()},
				Zone:         ptr(zone),
				V4CidrBlocks: strPtrs(intraCIDR(run)),
			},
		},
	}, k8slib.WithReady(yandexSubnetReady), k8slib.WithResourceOption[vpc.Subnet](pipeline.Parent(net)))

	sg := k8slib.Resource(ctx, k8s, names.SecurityGroup(), &vpc.SecurityGroup{
		Spec: vpc.SecurityGroupSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: vpc.SecurityGroupParameters{
				FolderID:     ptr(st.FolderID),
				Name:         ptr(names.SecurityGroup()),
				NetworkIDRef: &xpv1.Reference{Name: names.Network()},
				Ingress:      yandexIngress(run),
				Egress: []vpc.SecurityGroupEgressParameters{{
					Description:  ptr("all egress"),
					Protocol:     ptr("ANY"),
					FromPort:     ptr(0.0),
					ToPort:       ptr(65535.0),
					V4CidrBlocks: strPtrs("0.0.0.0/0"),
				}},
			},
		},
	}, k8slib.WithReady(yandexSGReady), k8slib.WithResourceOption[vpc.SecurityGroup](pipeline.Parent(sub)))

	infra := Infra{Root: net, Machines: map[string]*Machine{}}
	for _, m := range run.Machines {
		agent, ok := agents[m.Name]
		if !ok {
			return Infra{}, fmt.Errorf("provision/yandex: no agent declared for machine %q", m.Name)
		}
		// Secondary disks are their own managed resources, attached by
		// reference; the boot disk is created inline from the image.
		var secondary []compute.SecondaryDiskParameters
		var diskHandles []pipeline.Handle
		for _, d := range m.Disks {
			diskName := names.Disk(m.Name, d.Name)
			disk := k8slib.Resource(ctx, k8s, diskName, &compute.Disk{
				Spec: compute.DiskSpec{
					ResourceSpec: providerRef(pc),
					ForProvider: compute.DiskParameters{
						FolderID: ptr(st.FolderID),
						Name:     ptr(diskName),
						Zone:     ptr(zone),
						Size:     ptr(float64(d.GB)),
						Type:     ptr(d.Type),
						Labels:   labels(run, m.Labels),
					},
				},
			}, k8slib.WithReady(yandexDiskReady), k8slib.WithResourceOption[compute.Disk](pipeline.Parent(sub)))
			diskHandles = append(diskHandles, disk)
			secondary = append(secondary, compute.SecondaryDiskParameters{
				DeviceName: ptr(d.Name),
				DiskIDRef:  &xpv1.Reference{Name: diskName},
				AutoDelete: ptr(false), // the disk record deletes it; two deleters race
			})
		}
		vmName := names.Machine(m.Name)
		children := append([]pipeline.Handle{agent}, diskHandles...)
		vm := k8slib.Resource(ctx, k8s, vmName, &compute.Instance{
			Spec: compute.InstanceSpec{
				ResourceSpec: providerRef(pc),
				ForProvider: compute.InstanceParameters{
					FolderID:   ptr(st.FolderID),
					Name:       ptr(vmName),
					Hostname:   ptr(m.Name),
					Zone:       ptr(zone),
					PlatformID: ptr(m.InstanceType),
					Resources: []compute.ResourcesParameters{{
						Cores:  ptr(float64(m.CPU)),
						Memory: ptr(float64(m.MemoryGB)),
					}},
					BootDisk: []compute.BootDiskParameters{{
						AutoDelete: ptr(true),
						InitializeParams: []compute.InitializeParamsParameters{{
							ImageID: ptr(m.Image),
							Size:    ptr(float64(yandexBootDiskGB)),
							Type:    ptr("network-ssd"),
						}},
					}},
					SecondaryDisk: secondary,
					NetworkInterface: []compute.NetworkInterfaceParameters{{
						SubnetIDRef:          &xpv1.Reference{Name: names.Subnet()},
						SecurityGroupIdsRefs: []xpv1.Reference{{Name: names.SecurityGroup()}},
						NAT:                  ptr(public),
					}},
					SchedulingPolicy: []compute.SchedulingPolicyParameters{{Preemptible: ptr(st.Preemptible)}},
					Metadata: map[string]*string{
						"user-data":          ptr(agent.CloudInit()),
						"serial-port-enable": ptr("1"),
					},
					Labels: labels(run, m.Labels),
				},
			},
		},
			k8slib.WithReady(yandexInstanceReady),
			k8slib.WithTimeout[compute.Instance](machineTimeout),
			k8slib.WithResourceOption[compute.Instance](pipeline.Parent(sg), pipeline.Children(children...)),
		)
		mach := &Machine{Spec: m, Agent: agent, VM: vm}
		mach.wait = func(ctx pipeline.Context) (MachineInfo, error) {
			live, err := vm.TryReady(ctx)
			if err != nil {
				return MachineInfo{}, err
			}
			return yandexInfo(live), nil
		}
		infra.Machines[m.Name] = mach
		infra.Order = append(infra.Order, m.Name)
	}
	return infra, nil
}

// yandexBootDiskGB is the boot disk of every VM; data lives on secondary
// disks or in the container volumes.
const yandexBootDiskGB = 40

// yandexIngress opens the run network to itself plus the spec's ingress.
func yandexIngress(run spec.Run) []vpc.SecurityGroupIngressParameters {
	rules := []vpc.SecurityGroupIngressParameters{{
		Description:  ptr("intra-run"),
		Protocol:     ptr("ANY"),
		FromPort:     ptr(0.0),
		ToPort:       ptr(65535.0),
		V4CidrBlocks: strPtrs(intraCIDR(run)),
	}}
	for _, in := range run.Network.Ingress {
		proto := "TCP"
		if in.Proto == "udp" {
			proto = "UDP"
		}
		rules = append(rules, vpc.SecurityGroupIngressParameters{
			Description:  ptr(fmt.Sprintf("ingress %d", in.Port)),
			Protocol:     ptr(proto),
			Port:         ptr(float64(in.Port)),
			V4CidrBlocks: strPtrs(in.CIDR),
		})
	}
	return rules
}

func yandexInfo(live *compute.Instance) MachineInfo {
	info := MachineInfo{}
	if live == nil {
		return info
	}
	if live.Status.AtProvider.ID != nil {
		info.ID = *live.Status.AtProvider.ID
	}
	for _, ni := range live.Status.AtProvider.NetworkInterface {
		if ni.IPAddress != nil && info.PrivateIP == "" {
			info.PrivateIP = *ni.IPAddress
		}
		if ni.NATIPAddress != nil && info.PublicIP == "" {
			info.PublicIP = *ni.NATIPAddress
		}
	}
	return info
}

func yandexNetworkReady(live *vpc.Network) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func yandexSubnetReady(live *vpc.Subnet) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func yandexSGReady(live *vpc.SecurityGroup) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func yandexDiskReady(live *compute.Disk) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

// yandexInstanceReady: Ready condition AND the instance runs — the
// condition alone flips true while the VM still boots.
func yandexInstanceReady(live *compute.Instance) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady)) &&
		live.Status.AtProvider.Status != nil && *live.Status.AtProvider.Status == "running"
}
