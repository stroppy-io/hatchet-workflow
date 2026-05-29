

<a name="cloud-v1-deployment"></a>
# cloud.v1.deployment

## Table of Contents
- Messages
  - [cloud.v1.deployment.Docker.Container](#cloud-v1-deployment-docker-container)
  - [cloud.v1.deployment.Docker.Container.EnvEntry](#cloud-v1-deployment-docker-container-enventry)
  - [cloud.v1.deployment.Docker.Container.LabelsEntry](#cloud-v1-deployment-docker-container-labelsentry)
  - [cloud.v1.deployment.Docker.Container.TmpfsEntry](#cloud-v1-deployment-docker-container-tmpfsentry)
  - [cloud.v1.deployment.Docker.ContainerOutput](#cloud-v1-deployment-docker-containeroutput)
  - [cloud.v1.deployment.Docker.ContainerOutput.MappedPortsEntry](#cloud-v1-deployment-docker-containeroutput-mappedportsentry)
  - [cloud.v1.deployment.Docker.File](#cloud-v1-deployment-docker-file)
  - [cloud.v1.deployment.Docker.Healthcheck](#cloud-v1-deployment-docker-healthcheck)
  - [cloud.v1.deployment.Docker.Input](#cloud-v1-deployment-docker-input)
  - [cloud.v1.deployment.Docker.Input.ContainersEntry](#cloud-v1-deployment-docker-input-containersentry)
  - [cloud.v1.deployment.Docker.Network](#cloud-v1-deployment-docker-network)
  - [cloud.v1.deployment.Docker.Output](#cloud-v1-deployment-docker-output)
  - [cloud.v1.deployment.Docker.Output.ContainersEntry](#cloud-v1-deployment-docker-output-containersentry)
  - [cloud.v1.deployment.Docker.PortBinding](#cloud-v1-deployment-docker-portbinding)
  - [cloud.v1.deployment.Docker.Protocol](#cloud-v1-deployment-docker-protocol)
  - [cloud.v1.deployment.Docker.Resources](#cloud-v1-deployment-docker-resources)
  - [cloud.v1.deployment.Docker.RestartPolicy](#cloud-v1-deployment-docker-restartpolicy)
  - [cloud.v1.deployment.Docker.VolumeMount](#cloud-v1-deployment-docker-volumemount)
  - [cloud.v1.deployment.MachineInfo](#cloud-v1-deployment-machineinfo)
  - [cloud.v1.deployment.Provider](#cloud-v1-deployment-provider)
  - [cloud.v1.deployment.ProviderSettings](#cloud-v1-deployment-providersettings)
  - [cloud.v1.deployment.Quota.Allocation](#cloud-v1-deployment-quota-allocation)
  - [cloud.v1.deployment.Quota.Info](#cloud-v1-deployment-quota-info)
  - [cloud.v1.deployment.Quota.Request](#cloud-v1-deployment-quota-request)
  - [cloud.v1.deployment.Terraform.Action](#cloud-v1-deployment-terraform-action)
  - [cloud.v1.deployment.Terraform.Input](#cloud-v1-deployment-terraform-input)
  - [cloud.v1.deployment.Terraform.Operation](#cloud-v1-deployment-terraform-operation)
  - [cloud.v1.deployment.Terraform.Operation.EnvEntry](#cloud-v1-deployment-terraform-operation-enventry)
  - [cloud.v1.deployment.Terraform.Operation.SourceFile](#cloud-v1-deployment-terraform-operation-sourcefile)
  - [cloud.v1.deployment.Terraform.Output](#cloud-v1-deployment-terraform-output)

<a name="cloud-v1-deployment-messages"></a>
## Messages

<a name="cloud-v1-deployment-docker-container"></a>
### cloud.v1.deployment.Docker.Container

<pre>
Container is the runtime container spec consumed by the Docker daemon.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>binds</td>
<td>string</td>
<td><pre>
binds lists host:container[:opts] bind mounts.<br>

json_name: binds
go_name: Binds</pre></td>
</tr><tr>
<td>cgroupns_mode</td>
<td>string</td>
<td><pre>
cgroupns_mode sets cgroup namespace, e.g. "host" or "private".<br>

json_name: cgroupnsMode
go_name: CgroupnsMode</pre></td>
</tr><tr>
<td>cmd</td>
<td>string</td>
<td><pre>
cmd overrides the image CMD.<br>

json_name: cmd
go_name: Cmd</pre></td>
</tr><tr>
<td>depends_on</td>
<td>string</td>
<td><pre>
depends_on lists container names that must start first.<br>

json_name: dependsOn
go_name: DependsOn</pre></td>
</tr><tr>
<td>entrypoint</td>
<td>string</td>
<td><pre>
entrypoint overrides the image ENTRYPOINT.<br>

json_name: entrypoint
go_name: Entrypoint</pre></td>
</tr><tr>
<td>env</td>
<td><a href="#cloud-v1-deployment-docker-container-enventry">cloud.v1.deployment.Docker.Container.EnvEntry</a></td>
<td><pre>
env contains environment variables keyed by name.<br>

json_name: env
go_name: Env</pre></td>
</tr><tr>
<td>fallback_image</td>
<td>string</td>
<td><pre>
fallback_image is pulled when image is missing. Empty means no fallback.<br>

json_name: fallbackImage
go_name: FallbackImage</pre></td>
</tr><tr>
<td>files</td>
<td><a href="#cloud-v1-deployment-docker-file">cloud.v1.deployment.Docker.File</a></td>
<td><pre>
files lists files written into the container before start.<br>

json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>healthcheck</td>
<td><a href="#cloud-v1-deployment-docker-healthcheck">cloud.v1.deployment.Docker.Healthcheck</a></td>
<td><pre>
healthcheck overrides image healthcheck.<br>

json_name: healthcheck
go_name: Healthcheck</pre></td>
</tr><tr>
<td>hostname</td>
<td>string</td>
<td><pre>
hostname overrides container hostname.<br>

json_name: hostname
go_name: Hostname</pre></td>
</tr><tr>
<td>image</td>
<td>string</td>
<td><pre>
image is the Docker image reference, e.g. stroppy-agent:latest.<br>

json_name: image
go_name: Image</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-docker-container-labelsentry">cloud.v1.deployment.Docker.Container.LabelsEntry</a></td>
<td><pre>
labels are applied to the container.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>ports</td>
<td><a href="#cloud-v1-deployment-docker-portbinding">cloud.v1.deployment.Docker.PortBinding</a></td>
<td><pre>
ports lists port bindings exposed to the host.<br>

json_name: ports
go_name: Ports</pre></td>
</tr><tr>
<td>privileged</td>
<td>bool</td>
<td><pre>
privileged enables privileged mode. Required for systemd containers.<br>

json_name: privileged
go_name: Privileged</pre></td>
</tr><tr>
<td>resources</td>
<td><a href="#cloud-v1-deployment-docker-resources">cloud.v1.deployment.Docker.Resources</a></td>
<td><pre>
resources limits CPU and memory.<br>

json_name: resources
go_name: Resources</pre></td>
</tr><tr>
<td>restart_policy</td>
<td><a href="#cloud-v1-deployment-docker-restartpolicy">cloud.v1.deployment.Docker.RestartPolicy</a></td>
<td><pre>
restart_policy selects Docker restart policy.<br>

json_name: restartPolicy
go_name: RestartPolicy</pre></td>
</tr><tr>
<td>tmpfs</td>
<td><a href="#cloud-v1-deployment-docker-container-tmpfsentry">cloud.v1.deployment.Docker.Container.TmpfsEntry</a></td>
<td><pre>
tmpfs mounts keyed by container path with mount options as value.<br>

json_name: tmpfs
go_name: Tmpfs</pre></td>
</tr><tr>
<td>volumes</td>
<td><a href="#cloud-v1-deployment-docker-volumemount">cloud.v1.deployment.Docker.VolumeMount</a></td>
<td><pre>
volumes lists named volume mounts.<br>

json_name: volumes
go_name: Volumes</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-container-enventry"></a>
### cloud.v1.deployment.Docker.Container.EnvEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td>string</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-container-labelsentry"></a>
### cloud.v1.deployment.Docker.Container.LabelsEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td>string</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-container-tmpfsentry"></a>
### cloud.v1.deployment.Docker.Container.TmpfsEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td>string</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-containeroutput"></a>
### cloud.v1.deployment.Docker.ContainerOutput

<pre>
ContainerOutput describes one created container.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
id is the Docker container id assigned by the daemon.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>internal_ip</td>
<td>string</td>
<td><pre>
internal_ip is the container IP on the Docker network.<br>

json_name: internalIp
go_name: InternalIp</pre></td>
</tr><tr>
<td>mapped_ports</td>
<td><a href="#cloud-v1-deployment-docker-containeroutput-mappedportsentry">cloud.v1.deployment.Docker.ContainerOutput.MappedPortsEntry</a></td>
<td><pre>
mapped_ports maps container ports to the host ports they bound to.<br>

json_name: mappedPorts
go_name: MappedPorts</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the resolved container name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>started_at</td>
<td>string</td>
<td><pre>
started_at is the container start timestamp as reported by Docker.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td>string</td>
<td><pre>
status is the container status string reported by Docker.<br>

json_name: status
go_name: Status</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-containeroutput-mappedportsentry"></a>
### cloud.v1.deployment.Docker.ContainerOutput.MappedPortsEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>uint32</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td>uint32</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-file"></a>
### cloud.v1.deployment.Docker.File

<pre>
File describes a file written into the container before start.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>content</td>
<td>bytes</td>
<td><pre>
content is the raw file content.<br>

json_name: content
go_name: Content</pre></td>
</tr><tr>
<td>mode</td>
<td>uint32</td>
<td><pre>
mode is the octal file mode. Zero means 0644.<br>

json_name: mode
go_name: Mode</pre></td>
</tr><tr>
<td>path</td>
<td>string</td>
<td><pre>
path is the absolute path inside the container.<br>

json_name: path
go_name: Path</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-healthcheck"></a>
### cloud.v1.deployment.Docker.Healthcheck

<pre>
Healthcheck overrides image-level healthcheck.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>interval_seconds</td>
<td>uint32</td>
<td><pre>
interval_seconds is the delay between checks.<br>

json_name: intervalSeconds
go_name: IntervalSeconds</pre></td>
</tr><tr>
<td>retries</td>
<td>uint32</td>
<td><pre>
retries is the number of consecutive failures before unhealthy.<br>

json_name: retries
go_name: Retries</pre></td>
</tr><tr>
<td>start_period_seconds</td>
<td>uint32</td>
<td><pre>
start_period_seconds is the grace period before failures count.<br>

json_name: startPeriodSeconds
go_name: StartPeriodSeconds</pre></td>
</tr><tr>
<td>test</td>
<td>string</td>
<td><pre>
test is the healthcheck command, e.g. ["CMD", "curl", "-f", "..."].<br>

json_name: test
go_name: Test</pre></td>
</tr><tr>
<td>timeout_seconds</td>
<td>uint32</td>
<td><pre>
timeout_seconds is the max time one check may run.<br>

json_name: timeoutSeconds
go_name: TimeoutSeconds</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-input"></a>
### cloud.v1.deployment.Docker.Input

<pre>
Input contains runtime parameters passed to the Docker deployer.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="#cloud-v1-deployment-docker-input-containersentry">cloud.v1.deployment.Docker.Input.ContainersEntry</a></td>
<td><pre>
containers contains runtime container specs keyed by stable name.<br>

json_name: containers
go_name: Containers</pre></td>
</tr><tr>
<td>network</td>
<td><a href="#cloud-v1-deployment-docker-network">cloud.v1.deployment.Docker.Network</a></td>
<td><pre>
network describes the Docker network and DNS settings.<br>

json_name: network
go_name: Network</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-input-containersentry"></a>
### cloud.v1.deployment.Docker.Input.ContainersEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td><a href="#cloud-v1-deployment-docker-container">cloud.v1.deployment.Docker.Container</a></td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-network"></a>
### cloud.v1.deployment.Docker.Network

<pre>
Network describes the Docker network attached to containers.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>dns</td>
<td>string</td>
<td><pre>
dns lists DNS server IPs injected into containers.<br>

json_name: dns
go_name: Dns</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the Docker network name. Empty means default bridge.<br>

json_name: name
go_name: Name</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-output"></a>
### cloud.v1.deployment.Docker.Output

<pre>
Output mirrors created Docker containers.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="#cloud-v1-deployment-docker-output-containersentry">cloud.v1.deployment.Docker.Output.ContainersEntry</a></td>
<td><pre>
containers contains container outputs keyed by container name.<br>

json_name: containers
go_name: Containers</pre></td>
</tr><tr>
<td>network_id</td>
<td>string</td>
<td><pre>
network_id is the Docker network id when a custom network is used.<br>

json_name: networkId
go_name: NetworkId</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-output-containersentry"></a>
### cloud.v1.deployment.Docker.Output.ContainersEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td><a href="#cloud-v1-deployment-docker-containeroutput">cloud.v1.deployment.Docker.ContainerOutput</a></td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-portbinding"></a>
### cloud.v1.deployment.Docker.PortBinding

<pre>
PortBinding maps a container port to the host.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>container_port</td>
<td>uint32</td>
<td><pre>
container_port is the port exposed inside the container.<br>

json_name: containerPort
go_name: ContainerPort</pre></td>
</tr><tr>
<td>host_ip</td>
<td>string</td>
<td><pre>
host_ip is the host bind address. Empty means 0.0.0.0.<br>

json_name: hostIp
go_name: HostIp</pre></td>
</tr><tr>
<td>host_port</td>
<td>uint32</td>
<td><pre>
host_port is the host port. Zero means assign dynamically.<br>

json_name: hostPort
go_name: HostPort</pre></td>
</tr><tr>
<td>protocol</td>
<td><a href="#cloud-v1-deployment-docker-protocol">cloud.v1.deployment.Docker.Protocol</a></td>
<td><pre>
protocol selects tcp or udp.<br>

json_name: protocol
go_name: Protocol</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-protocol"></a>
### cloud.v1.deployment.Docker.Protocol

<pre>
Protocol maps to Docker port protocol.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>PROTOCOL_UNSPECIFIED</td>
<td><pre>
PROTOCOL_UNSPECIFIED is the unset zero value; treated as tcp.
</pre></td>
</tr><tr>
<td>PROTOCOL_TCP</td>
<td><pre>
PROTOCOL_TCP binds the port over TCP.
</pre></td>
</tr><tr>
<td>PROTOCOL_UDP</td>
<td><pre>
PROTOCOL_UDP binds the port over UDP.
</pre></td>
</tr>
</table>

<a name="cloud-v1-deployment-docker-resources"></a>
### cloud.v1.deployment.Docker.Resources

<pre>
Resources limits container CPU and memory.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cpu_cores</td>
<td>double</td>
<td><pre>
cpu_cores limits CPU cores. Zero means unlimited.<br>

json_name: cpuCores
go_name: CpuCores</pre></td>
</tr><tr>
<td>memory_mb</td>
<td>uint64</td>
<td><pre>
memory_mb limits RAM in MiB. Zero means unlimited.<br>

json_name: memoryMb
go_name: MemoryMb</pre></td>
</tr><tr>
<td>pids_limit</td>
<td>uint64</td>
<td><pre>
pids_limit caps process count. Zero means unlimited.<br>

json_name: pidsLimit
go_name: PidsLimit</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-docker-restartpolicy"></a>
### cloud.v1.deployment.Docker.RestartPolicy

<pre>
RestartPolicy maps to Docker HostConfig.RestartPolicy.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>RESTART_POLICY_UNSPECIFIED</td>
<td><pre>
RESTART_POLICY_UNSPECIFIED is the unset zero value; uses daemon default.
</pre></td>
</tr><tr>
<td>RESTART_POLICY_NO</td>
<td><pre>
RESTART_POLICY_NO never restarts the container.
</pre></td>
</tr><tr>
<td>RESTART_POLICY_ON_FAILURE</td>
<td><pre>
RESTART_POLICY_ON_FAILURE restarts only on non-zero exit.
</pre></td>
</tr><tr>
<td>RESTART_POLICY_ALWAYS</td>
<td><pre>
RESTART_POLICY_ALWAYS always restarts the container.
</pre></td>
</tr><tr>
<td>RESTART_POLICY_UNLESS_STOPPED</td>
<td><pre>
RESTART_POLICY_UNLESS_STOPPED always restarts unless explicitly stopped.
</pre></td>
</tr>
</table>

<a name="cloud-v1-deployment-docker-volumemount"></a>
### cloud.v1.deployment.Docker.VolumeMount

<pre>
VolumeMount describes a named volume attached to a container.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>name</td>
<td>string</td>
<td><pre>
name is Docker named volume.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>read_only</td>
<td>bool</td>
<td><pre>
read_only mounts the volume read-only.<br>

json_name: readOnly
go_name: ReadOnly</pre></td>
</tr><tr>
<td>target</td>
<td>string</td>
<td><pre>
target is path inside the container.<br>

json_name: target
go_name: Target</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-machineinfo"></a>
### cloud.v1.deployment.MachineInfo

<pre>
//MachineInfo describes the hardware resources of one compute node: CPU,
//memory and attached disks. It captures the desired/observed shape of a
//machine without binding to any provider-specific instance type.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cores</td>
<td>uint32</td>
<td><pre>
//cores is the number of CPU cores on the machine. Must be positive.<br>

json_name: cores
go_name: Cores</pre></td>
</tr><tr>
<td>data_disks_gb</td>
<td>uint64</td>
<td><pre>
//data_disks_gb lists the sizes in gigabytes of additional data disks
//attached to the machine, one entry per extra disk.<br>

json_name: dataDisksGb
go_name: DataDisksGb</pre></td>
</tr><tr>
<td>disk_gb</td>
<td>uint64</td>
<td><pre>
//disk_gb is the size of the boot/root disk in gigabytes. Must be positive.<br>

json_name: diskGb
go_name: DiskGb</pre></td>
</tr><tr>
<td>memory_gb</td>
<td>uint64</td>
<td><pre>
//memory_gb is the total RAM in gigabytes. Must be positive.<br>

json_name: memoryGb
go_name: MemoryGb</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-provider"></a>
### cloud.v1.deployment.Provider

<pre>
//Provider identifies the deployment backend that materializes a topology.
//It selects which provider-specific message (Docker, Yandex) and QuotaKind
//enum apply to a given deployment or quota reading.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>PROVIDER_UNSPECIFIED</td>
<td><pre>
PROVIDER_UNSPECIFIED is the unset zero value; never a valid backend.
</pre></td>
</tr><tr>
<td>PROVIDER_DOCKER</td>
<td><pre>
PROVIDER_DOCKER is the local Docker daemon backend (Docker message).
</pre></td>
</tr><tr>
<td>PROVIDER_YANDEX</td>
<td><pre>
PROVIDER_YANDEX is the Yandex Cloud Terraform backend (Yandex message).
</pre></td>
</tr>
</table>

<a name="cloud-v1-deployment-providersettings"></a>
### cloud.v1.deployment.ProviderSettings

<pre>
//ProviderSettings binds a chosen Provider to its baked, schema-backed
//backend configuration. provider selects the backend and, with it, the
//schema that settings is validated against.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
provider selects the deployment backend these settings configure.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>settings</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
settings is the sealed, schema-backed configuration for the provider.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-quota-allocation"></a>
### cloud.v1.deployment.Quota.Allocation

<pre>
//Allocation is the amount one deployment intends to consume from a quota.
//Summed across deployments and checked against State to decide if an
//apply fits within the remaining headroom.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>info</td>
<td><a href="#cloud-v1-deployment-quota-info">cloud.v1.deployment.Quota.Info</a></td>
<td><pre>
info identifies the provider and quota kind being allocated.<br>

json_name: info
go_name: Info</pre></td>
</tr><tr>
<td>used</td>
<td>uint64</td>
<td><pre>
used is the amount this deployment requests, in info.units.<br>

json_name: used
go_name: Used</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-quota-info"></a>
### cloud.v1.deployment.Quota.Info

<pre>
//Info identifies which quota a reading refers to, independent of any
//numbers. It is the shared key embedded in both State and Allocation.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the provider-reported quota name, e.g. the metric/quota id
//that quota_kind_enum_value resolves to.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
provider selects the deployment backend this reading belongs to and,
//with it, the QuotaKind enum that quota_kind_enum_value indexes.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>units</td>
<td>string</td>
<td><pre>
units is the provider-reported unit of used and limit, e.g.
//"count", "cores", "bytes", "GB".<br>

json_name: units
go_name: Units</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-quota-request"></a>
### cloud.v1.deployment.Quota.Request

<pre>
//Request is the amount one deployment asks to consume from a quota
//during a preflight check, before it is committed as an Allocation.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>info</td>
<td><a href="#cloud-v1-deployment-quota-info">cloud.v1.deployment.Quota.Info</a></td>
<td><pre>
info identifies the provider and quota kind being requested.<br>

json_name: info
go_name: Info</pre></td>
</tr><tr>
<td>request</td>
<td>uint64</td>
<td><pre>
request is the amount this deployment requests, in info.units.<br>

json_name: request
go_name: Request</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-terraform-action"></a>
### cloud.v1.deployment.Terraform.Action

<pre>
Action selects the Terraform lifecycle command represented by input.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>ACTION_UNSPECIFIED</td>
<td><pre>
ACTION_UNSPECIFIED is invalid.
</pre></td>
</tr><tr>
<td>ACTION_PLAN</td>
<td><pre>
ACTION_PLAN prepares the workdir, runs init/plan, and records the plan result.
</pre></td>
</tr><tr>
<td>ACTION_APPLY</td>
<td><pre>
ACTION_APPLY prepares the workdir, runs init/apply/output.
</pre></td>
</tr><tr>
<td>ACTION_DESTROY</td>
<td><pre>
ACTION_DESTROY destroys resources from an existing workdir state.
</pre></td>
</tr>
</table>

<a name="cloud-v1-deployment-terraform-input"></a>
### cloud.v1.deployment.Terraform.Input



<a name="cloud-v1-deployment-terraform-operation"></a>
### cloud.v1.deployment.Terraform.Operation



<a name="cloud-v1-deployment-terraform-operation-enventry"></a>
### cloud.v1.deployment.Terraform.Operation.EnvEntry



<a name="cloud-v1-deployment-terraform-operation-sourcefile"></a>
### cloud.v1.deployment.Terraform.Operation.SourceFile



<a name="cloud-v1-deployment-terraform-output"></a>
### cloud.v1.deployment.Terraform.Output

