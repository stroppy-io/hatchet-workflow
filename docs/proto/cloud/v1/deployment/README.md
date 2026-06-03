

<a name="cloud-v1-deployment"></a>
# cloud.v1.deployment

## Table of Contents
- Messages
  - [cloud.v1.deployment.AgentStep](#cloud-v1-deployment-agentstep)
  - [cloud.v1.deployment.AgentStep.LabelsEntry](#cloud-v1-deployment-agentstep-labelsentry)
  - [cloud.v1.deployment.ComponentDeployment](#cloud-v1-deployment-componentdeployment)
  - [cloud.v1.deployment.ComponentDeployment.LabelsEntry](#cloud-v1-deployment-componentdeployment-labelsentry)
  - [cloud.v1.deployment.ComponentRender](#cloud-v1-deployment-componentrender)
  - [cloud.v1.deployment.ComponentRender.LabelsEntry](#cloud-v1-deployment-componentrender-labelsentry)
  - [cloud.v1.deployment.DeploymentPlan](#cloud-v1-deployment-deploymentplan)
  - [cloud.v1.deployment.DeploymentPlan.LabelsEntry](#cloud-v1-deployment-deploymentplan-labelsentry)
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
  - [cloud.v1.deployment.Docker.Settings](#cloud-v1-deployment-docker-settings)
  - [cloud.v1.deployment.Docker.VolumeMount](#cloud-v1-deployment-docker-volumemount)
  - [cloud.v1.deployment.Endpoint](#cloud-v1-deployment-endpoint)
  - [cloud.v1.deployment.Endpoint.LabelsEntry](#cloud-v1-deployment-endpoint-labelsentry)
  - [cloud.v1.deployment.FileOverride](#cloud-v1-deployment-fileoverride)
  - [cloud.v1.deployment.FileOverride.LabelsEntry](#cloud-v1-deployment-fileoverride-labelsentry)
  - [cloud.v1.deployment.InfrastructurePlan](#cloud-v1-deployment-infrastructureplan)
  - [cloud.v1.deployment.InfrastructurePlan.LabelsEntry](#cloud-v1-deployment-infrastructureplan-labelsentry)
  - [cloud.v1.deployment.InfrastructureState](#cloud-v1-deployment-infrastructurestate)
  - [cloud.v1.deployment.InfrastructureState.LabelsEntry](#cloud-v1-deployment-infrastructurestate-labelsentry)
  - [cloud.v1.deployment.MachinePlan](#cloud-v1-deployment-machineplan)
  - [cloud.v1.deployment.MachinePlan.LabelsEntry](#cloud-v1-deployment-machineplan-labelsentry)
  - [cloud.v1.deployment.MachineState](#cloud-v1-deployment-machinestate)
  - [cloud.v1.deployment.MachineState.LabelsEntry](#cloud-v1-deployment-machinestate-labelsentry)
  - [cloud.v1.deployment.Provider](#cloud-v1-deployment-provider)
  - [cloud.v1.deployment.ProviderSettings](#cloud-v1-deployment-providersettings)
  - [cloud.v1.deployment.Quota.Allocation](#cloud-v1-deployment-quota-allocation)
  - [cloud.v1.deployment.Quota.Info](#cloud-v1-deployment-quota-info)
  - [cloud.v1.deployment.Quota.Request](#cloud-v1-deployment-quota-request)
  - [cloud.v1.deployment.Quota.ReservationStatus](#cloud-v1-deployment-quota-reservationstatus)
  - [cloud.v1.deployment.RenderArtifact](#cloud-v1-deployment-renderartifact)
  - [cloud.v1.deployment.RenderArtifact.Kind](#cloud-v1-deployment-renderartifact-kind)
  - [cloud.v1.deployment.RenderArtifact.LabelsEntry](#cloud-v1-deployment-renderartifact-labelsentry)
  - [cloud.v1.deployment.RenderArtifact.Mutability](#cloud-v1-deployment-renderartifact-mutability)
  - [cloud.v1.deployment.RenderArtifact.Origin](#cloud-v1-deployment-renderartifact-origin)
  - [cloud.v1.deployment.RenderOverrideSet](#cloud-v1-deployment-renderoverrideset)
  - [cloud.v1.deployment.RenderOverrideSet.LabelsEntry](#cloud-v1-deployment-renderoverrideset-labelsentry)
  - [cloud.v1.deployment.RenderPreview](#cloud-v1-deployment-renderpreview)
  - [cloud.v1.deployment.RenderPreview.LabelsEntry](#cloud-v1-deployment-renderpreview-labelsentry)
  - [cloud.v1.deployment.Terraform.Action](#cloud-v1-deployment-terraform-action)
  - [cloud.v1.deployment.Terraform.Input](#cloud-v1-deployment-terraform-input)
  - [cloud.v1.deployment.Terraform.Operation](#cloud-v1-deployment-terraform-operation)
  - [cloud.v1.deployment.Terraform.Operation.EnvEntry](#cloud-v1-deployment-terraform-operation-enventry)
  - [cloud.v1.deployment.Terraform.Operation.SourceFile](#cloud-v1-deployment-terraform-operation-sourcefile)
  - [cloud.v1.deployment.Terraform.Output](#cloud-v1-deployment-terraform-output)
  - [cloud.v1.deployment.Yandex.Disk](#cloud-v1-deployment-yandex-disk)
  - [cloud.v1.deployment.Yandex.Settings](#cloud-v1-deployment-yandex-settings)
  - [cloud.v1.deployment.Yandex.Settings.PlatformId](#cloud-v1-deployment-yandex-settings-platformid)
  - [cloud.v1.deployment.Yandex.Settings.Zone](#cloud-v1-deployment-yandex-settings-zone)
  - [cloud.v1.deployment.Yandex.Vm](#cloud-v1-deployment-yandex-vm)
  - [cloud.v1.deployment.Yandex.VmOutput](#cloud-v1-deployment-yandex-vmoutput)

<a name="cloud-v1-deployment-messages"></a>
## Messages

<a name="cloud-v1-deployment-agentstep"></a>
### cloud.v1.deployment.AgentStep

<pre>
//AgentStep is one filesystem or command action routed to a node agent.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>call_cmd</td>
<td><a href="../common/README.md#cloud-v1-common-cmd">cloud.v1.common.Cmd</a></td>
<td><pre>
json_name: callCmd
go_name: CallCmd</pre></td>
</tr><tr>
<td>create_dir</td>
<td><a href="../common/README.md#cloud-v1-common-dir">cloud.v1.common.Dir</a></td>
<td><pre>
json_name: createDir
go_name: CreateDir</pre></td>
</tr><tr>
<td>fetch_file</td>
<td><a href="../common/README.md#cloud-v1-common-file">cloud.v1.common.File</a></td>
<td><pre>
json_name: fetchFile
go_name: FetchFile</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is stable within the component deployment.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-agentstep-labelsentry">cloud.v1.deployment.AgentStep.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the step.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>order</td>
<td>uint32</td>
<td><pre>
//order is the component-local execution order.<br>

json_name: order
go_name: Order</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is execution status, filled by plan execution.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the step.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>write_file</td>
<td><a href="../common/README.md#cloud-v1-common-file">cloud.v1.common.File</a></td>
<td><pre>
json_name: writeFile
go_name: WriteFile</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-agentstep-labelsentry"></a>
### cloud.v1.deployment.AgentStep.LabelsEntry

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



<a name="cloud-v1-deployment-componentdeployment"></a>
### cloud.v1.deployment.ComponentDeployment

<pre>
//ComponentDeployment is the agent plan for one logical component.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id links this plan to topology.Component.id.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>depends_on_component_ids</td>
<td>string</td>
<td><pre>
//depends_on_component_ids are explicit component dependencies.<br>

json_name: dependsOnComponentIds
go_name: DependsOnComponentIds</pre></td>
</tr><tr>
<td>global_priority</td>
<td>uint32</td>
<td><pre>
//global_priority orders this component relative to components on other
//nodes.<br>

json_name: globalPriority
go_name: GlobalPriority</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-componentdeployment-labelsentry">cloud.v1.deployment.ComponentDeployment.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to this component deployment.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
//node_id links this plan to topology.Node.id / infrastructure.MachineState.<br>

json_name: nodeId
go_name: NodeId</pre></td>
</tr><tr>
<td>node_priority</td>
<td>uint32</td>
<td><pre>
//node_priority orders this component relative to colocated components on
//the same node.<br>

json_name: nodePriority
go_name: NodePriority</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is execution status, filled by plan execution.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>steps</td>
<td><a href="#cloud-v1-deployment-agentstep">cloud.v1.deployment.AgentStep</a></td>
<td><pre>
//steps are ordered agent actions for this component.<br>

json_name: steps
go_name: Steps</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to this component deployment.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-componentdeployment-labelsentry"></a>
### cloud.v1.deployment.ComponentDeployment.LabelsEntry

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



<a name="cloud-v1-deployment-componentrender"></a>
### cloud.v1.deployment.ComponentRender

<pre>
//ComponentRender groups artifact ids for one component.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>artifact_ids</td>
<td>string</td>
<td><pre>
//artifact_ids are ids of RenderArtifact records belonging to this component.<br>

json_name: artifactIds
go_name: ArtifactIds</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id links to topology.Component.id.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-componentrender-labelsentry">cloud.v1.deployment.ComponentRender.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to this component preview.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
//node_id links to topology.Node.id.<br>

json_name: nodeId
go_name: NodeId</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-componentrender-labelsentry"></a>
### cloud.v1.deployment.ComponentRender.LabelsEntry

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



<a name="cloud-v1-deployment-deploymentplan"></a>
### cloud.v1.deployment.DeploymentPlan

<pre>
//DeploymentPlan is the ordered set of component deployments for a topology.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>components</td>
<td><a href="#cloud-v1-deployment-componentdeployment">cloud.v1.deployment.ComponentDeployment</a></td>
<td><pre>
//components are the per-component deployment plans.<br>

json_name: components
go_name: Components</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-deploymentplan-labelsentry">cloud.v1.deployment.DeploymentPlan.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the plan.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the plan.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-deploymentplan-labelsentry"></a>
### cloud.v1.deployment.DeploymentPlan.LabelsEntry

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

<a name="cloud-v1-deployment-docker-settings"></a>
### cloud.v1.deployment.Docker.Settings



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



<a name="cloud-v1-deployment-endpoint"></a>
### cloud.v1.deployment.Endpoint

<pre>
//Endpoint is a runtime address reachable on a machine or managed resource.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>address</td>
<td>string</td>
<td><pre>
//address is an IP or DNS name allocated/resolved at runtime.<br>

json_name: address
go_name: Address</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-endpoint-labelsentry">cloud.v1.deployment.Endpoint.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the endpoint.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is a stable endpoint name, e.g. private, public, agent, postgres.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>port</td>
<td>uint32</td>
<td><pre>
//port is the endpoint port, when applicable.<br>

json_name: port
go_name: Port</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-endpoint-labelsentry"></a>
### cloud.v1.deployment.Endpoint.LabelsEntry

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



<a name="cloud-v1-deployment-fileoverride"></a>
### cloud.v1.deployment.FileOverride

<pre>
//FileOverride replaces one editable RenderArtifact file.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>artifact_id</td>
<td>string</td>
<td><pre>
//artifact_id matches RenderArtifact.id.<br>

json_name: artifactId
go_name: ArtifactId</pre></td>
</tr><tr>
<td>base_hash</td>
<td>string</td>
<td><pre>
//base_hash is the rendered default hash the user edited from.<br>

json_name: baseHash
go_name: BaseHash</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id links to topology.Component.id.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>file</td>
<td><a href="../common/README.md#cloud-v1-common-file">cloud.v1.common.File</a></td>
<td><pre>
//file is the user's replacement file payload and metadata.<br>

json_name: file
go_name: File</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-fileoverride-labelsentry">cloud.v1.deployment.FileOverride.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to this override.<br>

json_name: labels
go_name: Labels</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-fileoverride-labelsentry"></a>
### cloud.v1.deployment.FileOverride.LabelsEntry

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



<a name="cloud-v1-deployment-infrastructureplan"></a>
### cloud.v1.deployment.InfrastructurePlan

<pre>
//InfrastructurePlan is the provider-specific machine/resource intent.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-infrastructureplan-labelsentry">cloud.v1.deployment.InfrastructurePlan.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the plan.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>machines</td>
<td><a href="#cloud-v1-deployment-machineplan">cloud.v1.deployment.MachinePlan</a></td>
<td><pre>
//machines are provider-specific resources keyed back to topology.Node.id.<br>

json_name: machines
go_name: Machines</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider selects the backend that will materialize this plan.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>settings</td>
<td><a href="#cloud-v1-deployment-providersettings">cloud.v1.deployment.ProviderSettings</a></td>
<td><pre>
//settings are provider account/network/auth settings.<br>

json_name: settings
go_name: Settings</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the plan.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-infrastructureplan-labelsentry"></a>
### cloud.v1.deployment.InfrastructurePlan.LabelsEntry

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



<a name="cloud-v1-deployment-infrastructurestate"></a>
### cloud.v1.deployment.InfrastructureState

<pre>
//InfrastructureState is provider output after provisioning.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-infrastructurestate-labelsentry">cloud.v1.deployment.InfrastructureState.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the state.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>machines</td>
<td><a href="#cloud-v1-deployment-machinestate">cloud.v1.deployment.MachineState</a></td>
<td><pre>
//machines are runtime states keyed back to topology.Node.id.<br>

json_name: machines
go_name: Machines</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider echoes the backend that produced this state.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the state.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-infrastructurestate-labelsentry"></a>
### cloud.v1.deployment.InfrastructureState.LabelsEntry

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



<a name="cloud-v1-deployment-machineplan"></a>
### cloud.v1.deployment.MachinePlan

<pre>
//MachinePlan is provider input for one logical node.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>docker</td>
<td><a href="#cloud-v1-deployment-docker-container">cloud.v1.deployment.Docker.Container</a></td>
<td><pre>
json_name: docker
go_name: Docker</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-machineplan-labelsentry">cloud.v1.deployment.MachinePlan.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the machine plan.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
//node_id links this machine to topology.Node.id.<br>

json_name: nodeId
go_name: NodeId</pre></td>
</tr><tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-deployment-quota-request">cloud.v1.deployment.Quota.Request</a></td>
<td><pre>
//quota_requests are computed preflight quota requests for this machine.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the machine plan.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>yandex</td>
<td><a href="#cloud-v1-deployment-yandex-vm">cloud.v1.deployment.Yandex.Vm</a></td>
<td><pre>
json_name: yandex
go_name: Yandex</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-machineplan-labelsentry"></a>
### cloud.v1.deployment.MachinePlan.LabelsEntry

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



<a name="cloud-v1-deployment-machinestate"></a>
### cloud.v1.deployment.MachineState

<pre>
//MachineState is runtime provider output for one logical node.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>allocated_quotas</td>
<td><a href="#cloud-v1-deployment-quota-allocation">cloud.v1.deployment.Quota.Allocation</a></td>
<td><pre>
//allocated_quotas are provider quotas granted/consumed by this machine.<br>

json_name: allocatedQuotas
go_name: AllocatedQuotas</pre></td>
</tr><tr>
<td>docker</td>
<td><a href="#cloud-v1-deployment-docker-containeroutput">cloud.v1.deployment.Docker.ContainerOutput</a></td>
<td><pre>
json_name: docker
go_name: Docker</pre></td>
</tr><tr>
<td>endpoints</td>
<td><a href="#cloud-v1-deployment-endpoint">cloud.v1.deployment.Endpoint</a></td>
<td><pre>
//endpoints are addresses allocated by the provider.<br>

json_name: endpoints
go_name: Endpoints</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-machinestate-labelsentry">cloud.v1.deployment.MachineState.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the machine state.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
//node_id links this state to topology.Node.id.<br>

json_name: nodeId
go_name: NodeId</pre></td>
</tr><tr>
<td>provider_resource_id</td>
<td>string</td>
<td><pre>
//provider_resource_id is the VM/container/resource id assigned by the
//provider.<br>

json_name: providerResourceId
go_name: ProviderResourceId</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the runtime status reported by the deployment layer.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the machine state.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>yandex</td>
<td><a href="#cloud-v1-deployment-yandex-vmoutput">cloud.v1.deployment.Yandex.VmOutput</a></td>
<td><pre>
json_name: yandex
go_name: Yandex</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-machinestate-labelsentry"></a>
### cloud.v1.deployment.MachineState.LabelsEntry

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
//PROVIDER_UNSPECIFIED is the unset zero value; never a valid backend.
</pre></td>
</tr><tr>
<td>PROVIDER_DOCKER</td>
<td><pre>
//PROVIDER_DOCKER is the local Docker daemon backend (Docker message).
</pre></td>
</tr><tr>
<td>PROVIDER_YANDEX</td>
<td><pre>
//PROVIDER_YANDEX is the Yandex Cloud Terraform backend (Yandex message).
</pre></td>
</tr>
</table>

<a name="cloud-v1-deployment-providersettings"></a>
### cloud.v1.deployment.ProviderSettings

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>docker</td>
<td><a href="#cloud-v1-deployment-docker-settings">cloud.v1.deployment.Docker.Settings</a></td>
<td><pre>
json_name: docker
go_name: Docker</pre></td>
</tr><tr>
<td>yandex</td>
<td><a href="#cloud-v1-deployment-yandex-settings">cloud.v1.deployment.Yandex.Settings</a></td>
<td><pre>
json_name: yandex
go_name: Yandex</pre></td>
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



<a name="cloud-v1-deployment-quota-reservationstatus"></a>
### cloud.v1.deployment.Quota.ReservationStatus

<pre>
//ReservationStatus is the control-plane lifecycle of our own quota ledger.
//Provider usage is observed separately through quota snapshots; RESERVED
//rows are the only rows subtracted from provider headroom for new runs.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>RESERVATION_STATUS_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>RESERVATION_STATUS_RESERVED</td>
<td></td>
</tr><tr>
<td>RESERVATION_STATUS_ALLOCATED</td>
<td></td>
</tr><tr>
<td>RESERVATION_STATUS_RELEASED</td>
<td></td>
</tr><tr>
<td>RESERVATION_STATUS_EXPIRED</td>
<td></td>
</tr><tr>
<td>RESERVATION_STATUS_FAILED</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-deployment-renderartifact"></a>
### cloud.v1.deployment.RenderArtifact

<pre>
//RenderArtifact is one previewable file/command/dir/runtime placeholder.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>base_hash</td>
<td>string</td>
<td><pre>
//base_hash is a hash of the rendered default content, used for conflict
//detection when user overrides survive rerenders.<br>

json_name: baseHash
go_name: BaseHash</pre></td>
</tr><tr>
<td>cmd</td>
<td><a href="../common/README.md#cloud-v1-common-cmd">cloud.v1.common.Cmd</a></td>
<td><pre>
json_name: cmd
go_name: Cmd</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id links to topology.Component.id.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>dir</td>
<td><a href="../common/README.md#cloud-v1-common-dir">cloud.v1.common.Dir</a></td>
<td><pre>
json_name: dir
go_name: Dir</pre></td>
</tr><tr>
<td>file</td>
<td><a href="../common/README.md#cloud-v1-common-file">cloud.v1.common.File</a></td>
<td><pre>
json_name: file
go_name: File</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is stable across rerenders, e.g. postgres-master/postgresql.conf.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-deployment-renderartifact-kind">cloud.v1.deployment.RenderArtifact.Kind</a></td>
<td><pre>
//kind is the artifact payload family.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-renderartifact-labelsentry">cloud.v1.deployment.RenderArtifact.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to this artifact.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>lock_reason</td>
<td>string</td>
<td><pre>
//lock_reason explains why a visible artifact is read-only.<br>

json_name: lockReason
go_name: LockReason</pre></td>
</tr><tr>
<td>mutability</td>
<td><a href="#cloud-v1-deployment-renderartifact-mutability">cloud.v1.deployment.RenderArtifact.Mutability</a></td>
<td><pre>
//mutability tells the UI whether editing is allowed.<br>

json_name: mutability
go_name: Mutability</pre></td>
</tr><tr>
<td>origin</td>
<td><a href="#cloud-v1-deployment-renderartifact-origin">cloud.v1.deployment.RenderArtifact.Origin</a></td>
<td><pre>
//origin tells whether it is system generated, user overridden, or runtime.<br>

json_name: origin
go_name: Origin</pre></td>
</tr><tr>
<td>renderer_name</td>
<td>string</td>
<td><pre>
//renderer_name identifies which renderer produced this artifact.<br>

json_name: rendererName
go_name: RendererName</pre></td>
</tr><tr>
<td>runtime_value</td>
<td>string</td>
<td><pre>
json_name: runtimeValue
go_name: RuntimeValue</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to this artifact.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-renderartifact-kind"></a>
### cloud.v1.deployment.RenderArtifact.Kind

<pre>
//Kind identifies the artifact payload family.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>KIND_FILE</td>
<td></td>
</tr><tr>
<td>KIND_COMMAND</td>
<td></td>
</tr><tr>
<td>KIND_DIRECTORY</td>
<td></td>
</tr><tr>
<td>KIND_RUNTIME_VALUE</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-deployment-renderartifact-labelsentry"></a>
### cloud.v1.deployment.RenderArtifact.LabelsEntry

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



<a name="cloud-v1-deployment-renderartifact-mutability"></a>
### cloud.v1.deployment.RenderArtifact.Mutability

<pre>
//Mutability tells whether the wizard may edit this artifact.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>MUTABILITY_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>MUTABILITY_READ_ONLY</td>
<td></td>
</tr><tr>
<td>MUTABILITY_EDITABLE</td>
<td></td>
</tr><tr>
<td>MUTABILITY_RUNTIME_ONLY</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-deployment-renderartifact-origin"></a>
### cloud.v1.deployment.RenderArtifact.Origin

<pre>
//Origin tells where this artifact came from.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>ORIGIN_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>ORIGIN_SYSTEM</td>
<td></td>
</tr><tr>
<td>ORIGIN_RENDERED_DEFAULT</td>
<td></td>
</tr><tr>
<td>ORIGIN_USER_OVERRIDE</td>
<td></td>
</tr><tr>
<td>ORIGIN_RUNTIME</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-deployment-renderoverrideset"></a>
### cloud.v1.deployment.RenderOverrideSet

<pre>
//RenderOverrideSet is persisted user edit intent. It contains only editable
//overrides, never system commands or runtime values.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-deployment-fileoverride">cloud.v1.deployment.FileOverride</a></td>
<td><pre>
//files are user-edited replacements for editable file artifacts.<br>

json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-renderoverrideset-labelsentry">cloud.v1.deployment.RenderOverrideSet.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the override set.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the override set.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-renderoverrideset-labelsentry"></a>
### cloud.v1.deployment.RenderOverrideSet.LabelsEntry

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



<a name="cloud-v1-deployment-renderpreview"></a>
### cloud.v1.deployment.RenderPreview

<pre>
//RenderPreview is the read model shown by the wizard before a run starts.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>artifacts</td>
<td><a href="#cloud-v1-deployment-renderartifact">cloud.v1.deployment.RenderArtifact</a></td>
<td><pre>
//artifacts are render outputs and placeholders shown to the user.<br>

json_name: artifacts
go_name: Artifacts</pre></td>
</tr><tr>
<td>components</td>
<td><a href="#cloud-v1-deployment-componentrender">cloud.v1.deployment.ComponentRender</a></td>
<td><pre>
//components groups artifacts by logical component.<br>

json_name: components
go_name: Components</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-deployment-renderpreview-labelsentry">cloud.v1.deployment.RenderPreview.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the preview.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>overrides</td>
<td><a href="#cloud-v1-deployment-renderoverrideset">cloud.v1.deployment.RenderOverrideSet</a></td>
<td><pre>
//overrides are the editable user intent currently applied to this preview.<br>

json_name: overrides
go_name: Overrides</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata attached to the preview.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-renderpreview-labelsentry"></a>
### cloud.v1.deployment.RenderPreview.LabelsEntry

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



<a name="cloud-v1-deployment-yandex-disk"></a>
### cloud.v1.deployment.Yandex.Disk

<pre>
Disk describes an additional yandex_compute_disk attached to a VM.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>device_name</td>
<td>string</td>
<td><pre>
device_name becomes virtio serial visible in guest.<br>

json_name: deviceName
go_name: DeviceName</pre></td>
</tr><tr>
<td>size_gb</td>
<td>uint32</td>
<td><pre>
size_gb is disk size in GiB.<br>

json_name: sizeGb
go_name: SizeGb</pre></td>
</tr><tr>
<td>type</td>
<td>string</td>
<td><pre>
type is Yandex disk type id. Closed set: see Enums.DiskType.<br>

json_name: type
go_name: Type</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-yandex-settings"></a>
### cloud.v1.deployment.Yandex.Settings

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>assign_public_ip</td>
<td>bool</td>
<td><pre>
//assign_public_ip enables public IP assignment for provisioned VMs.<br>

json_name: assignPublicIp
go_name: AssignPublicIp</pre></td>
</tr><tr>
<td>cloud_id</td>
<td>string</td>
<td><pre>
//cloud_id is the Yandex Cloud account identifier.<br>

json_name: cloudId
go_name: CloudId</pre></td>
</tr><tr>
<td>folder_id</td>
<td>string</td>
<td><pre>
//folder_id is the Yandex Cloud folder/project resource id.<br>

json_name: folderId
go_name: FolderId</pre></td>
</tr><tr>
<td>image_id</td>
<td>string</td>
<td><pre>
//image_id is the Yandex Compute image identifier.<br>

json_name: imageId
go_name: ImageId</pre></td>
</tr><tr>
<td>network_id</td>
<td>string</td>
<td><pre>
//network_id is the VPC network resource id.<br>

json_name: networkId
go_name: NetworkId</pre></td>
</tr><tr>
<td>network_name</td>
<td>string</td>
<td><pre>
//network_name is the VPC network display name.<br>

json_name: networkName
go_name: NetworkName</pre></td>
</tr><tr>
<td>platform_id</td>
<td><a href="#cloud-v1-deployment-yandex-settings-platformid">cloud.v1.deployment.Yandex.Settings.PlatformId</a></td>
<td><pre>
//platform_id is the VM platform architecture tier.<br>

json_name: platformId
go_name: PlatformId</pre></td>
</tr><tr>
<td>software_accelerated_network</td>
<td>bool</td>
<td><pre>
//software_accelerated_network enables software packet acceleration.<br>

json_name: softwareAcceleratedNetwork
go_name: SoftwareAcceleratedNetwork</pre></td>
</tr><tr>
<td>ssh_public_key</td>
<td>string</td>
<td><pre>
//ssh_public_key is the SSH public key used for authentication.<br>

json_name: sshPublicKey
go_name: SshPublicKey</pre></td>
</tr><tr>
<td>ssh_user</td>
<td>string</td>
<td><pre>
//ssh_user is the login username used for SSH access.<br>

json_name: sshUser
go_name: SshUser</pre></td>
</tr><tr>
<td>subnet_cidr</td>
<td>string</td>
<td><pre>
//subnet_cidr is the CIDR block used for the deployment subnet.<br>

json_name: subnetCidr
go_name: SubnetCidr</pre></td>
</tr><tr>
<td>token</td>
<td>string</td>
<td><pre>
//token is the Yandex Cloud API authentication token.<br>

json_name: token
go_name: Token</pre></td>
</tr><tr>
<td>zone</td>
<td><a href="#cloud-v1-deployment-yandex-settings-zone">cloud.v1.deployment.Yandex.Settings.Zone</a></td>
<td><pre>
//zone is the default availability zone for deployment resources.<br>

json_name: zone
go_name: Zone</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-yandex-settings-platformid"></a>
### cloud.v1.deployment.Yandex.Settings.PlatformId

<pre>
//PlatformId identifies the Yandex Compute platform architecture tier.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>PLATFORM_ID_UNSPECIFIED</td>
<td><pre>
//PLATFORM_ID_UNSPECIFIED is the unset zero value.
</pre></td>
</tr><tr>
<td>PLATFORM_ID_STANDARD_V1</td>
<td><pre>
//PLATFORM_ID_STANDARD_V1 is the first-generation standard x86-64
//platform.
</pre></td>
</tr><tr>
<td>PLATFORM_ID_STANDARD_V2</td>
<td><pre>
//PLATFORM_ID_STANDARD_V2 is the second-generation standard x86-64
//platform.
</pre></td>
</tr><tr>
<td>PLATFORM_ID_STANDARD_V3</td>
<td><pre>
//PLATFORM_ID_STANDARD_V3 is the third-generation standard x86-64
//platform.
</pre></td>
</tr><tr>
<td>PLATFORM_ID_STANDARD_V4A</td>
<td><pre>
//PLATFORM_ID_STANDARD_V4A is the fourth-generation AMD-based
//standard platform.
</pre></td>
</tr><tr>
<td>PLATFORM_ID_AMD_V1</td>
<td><pre>
//PLATFORM_ID_AMD_V1 is the AMD EPYC-based platform variant.
</pre></td>
</tr><tr>
<td>PLATFORM_ID_HIGHFREQ_V3</td>
<td><pre>
//PLATFORM_ID_HIGHFREQ_V3 is the high-frequency third-generation
//x86-64 platform.
</pre></td>
</tr><tr>
<td>PLATFORM_ID_HIGHFREQ_V4A</td>
<td><pre>
//PLATFORM_ID_HIGHFREQ_V4A is the high-frequency fourth-generation
//AMD platform.
</pre></td>
</tr>
</table>

<a name="cloud-v1-deployment-yandex-settings-zone"></a>
### cloud.v1.deployment.Yandex.Settings.Zone

<pre>
//Zone identifies the Yandex Cloud availability zone for deployment
//resources.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>ZONE_UNSPECIFIED</td>
<td><pre>
//ZONE_UNSPECIFIED is the unset zero value.
</pre></td>
</tr><tr>
<td>ZONE_RU_CENTRAL1_A</td>
<td><pre>
//ZONE_RU_CENTRAL1_A is the Central Russia A zone.
</pre></td>
</tr><tr>
<td>ZONE_RU_CENTRAL1_B</td>
<td><pre>
//ZONE_RU_CENTRAL1_B is the Central Russia B zone.
</pre></td>
</tr><tr>
<td>ZONE_RU_CENTRAL1_D</td>
<td><pre>
//ZONE_RU_CENTRAL1_D is the Central Russia D zone.
</pre></td>
</tr>
</table>

<a name="cloud-v1-deployment-yandex-vm"></a>
### cloud.v1.deployment.Yandex.Vm

<pre>
Vm is the runtime yandex_compute_instance spec.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>boot_disk_gb</td>
<td>uint64</td>
<td><pre>
boot_disk_gb is boot disk size in GiB.<br>

json_name: bootDiskGb
go_name: BootDiskGb</pre></td>
</tr><tr>
<td>boot_disk_type</td>
<td>string</td>
<td><pre>
boot_disk_type is boot disk type id. Closed set: see Enums.DiskType.<br>

json_name: bootDiskType
go_name: BootDiskType</pre></td>
</tr><tr>
<td>cores</td>
<td>uint32</td>
<td><pre>
cores is VM vCPU count.<br>

json_name: cores
go_name: Cores</pre></td>
</tr><tr>
<td>internal_ip</td>
<td>string</td>
<td><pre>
internal_ip is VM private address in selected subnet.<br>

json_name: internalIp
go_name: InternalIp</pre></td>
</tr><tr>
<td>memory_gb</td>
<td>uint64</td>
<td><pre>
memory_gb is VM RAM in GiB.<br>

json_name: memoryGb
go_name: MemoryGb</pre></td>
</tr><tr>
<td>network_acceleration</td>
<td>string</td>
<td><pre>
network_acceleration selects Compute network acceleration mode.
//Closed set: "standard" | "software_accelerated". See Enums.NetworkAcceleration.<br>

json_name: networkAcceleration
go_name: NetworkAcceleration</pre></td>
</tr><tr>
<td>public_ip</td>
<td>bool</td>
<td><pre>
public_ip requests NAT public address.<br>

json_name: publicIp
go_name: PublicIp</pre></td>
</tr><tr>
<td>secondary_disks</td>
<td><a href="#cloud-v1-deployment-yandex-disk">cloud.v1.deployment.Yandex.Disk</a></td>
<td><pre>
secondary_disks contains raw attached disks.<br>

json_name: secondaryDisks
go_name: SecondaryDisks</pre></td>
</tr><tr>
<td>user_data</td>
<td>string</td>
<td><pre>
user_data is cloud-init user-data.<br>

json_name: userData
go_name: UserData</pre></td>
</tr><tr>
<td>zone</td>
<td>string</td>
<td><pre>
zone overrides Network.zone. Empty means use default zone. Closed set when set: see Enums.Zone.<br>

json_name: zone
go_name: Zone</pre></td>
</tr>
</table>



<a name="cloud-v1-deployment-yandex-vmoutput"></a>
### cloud.v1.deployment.Yandex.VmOutput

<pre>
VmOutput describes one created VM.
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
id is yandex_compute_instance id.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>internal_ip</td>
<td>string</td>
<td><pre>
internal_ip is private IP address.<br>

json_name: internalIp
go_name: InternalIp</pre></td>
</tr><tr>
<td>public_ip</td>
<td>string</td>
<td><pre>
public_ip is NAT IP address.<br>

json_name: publicIp
go_name: PublicIp</pre></td>
</tr>
</table>

