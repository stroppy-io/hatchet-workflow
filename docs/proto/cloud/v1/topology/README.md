

<a name="cloud-v1-topology"></a>
# cloud.v1.topology

## Table of Contents
- Messages
  - [cloud.v1.topology.Component](#cloud-v1-topology-component)
  - [cloud.v1.topology.Component.Kind](#cloud-v1-topology-component-kind)
  - [cloud.v1.topology.Component.Strategy](#cloud-v1-topology-component-strategy)
  - [cloud.v1.topology.Connection](#cloud-v1-topology-connection)
  - [cloud.v1.topology.Connection.Kind](#cloud-v1-topology-connection-kind)
  - [cloud.v1.topology.Connection.Mode](#cloud-v1-topology-connection-mode)
  - [cloud.v1.topology.Connection.Protocol](#cloud-v1-topology-connection-protocol)
  - [cloud.v1.topology.Topology](#cloud-v1-topology-topology)
  - [cloud.v1.topology.Topology.Instance](#cloud-v1-topology-topology-instance)

<a name="cloud-v1-topology-messages"></a>
## Messages

<a name="cloud-v1-topology-component"></a>
### cloud.v1.topology.Component

<pre>
//Component is one logical node in the topology graph.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>allocated_on_instance_id</td>
<td>string</td>
<td><pre>
//allocated_on_instance_id is the instance this component was placed on;
//calculated (deployment).<br>

json_name: allocatedOnInstanceId
go_name: AllocatedOnInstanceId</pre></td>
</tr><tr>
<td>deployment_strategy</td>
<td><a href="#cloud-v1-topology-component-strategy">cloud.v1.topology.Component.Strategy</a></td>
<td><pre>
//deployment_strategy is the deploy recipe; calculated (build).<br>

json_name: deploymentStrategy
go_name: DeploymentStrategy</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the unique identifier of the component within the topology.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-topology-component-kind">cloud.v1.topology.Component.Kind</a></td>
<td><pre>
//kind is the role this component plays; must be a defined, non-zero value.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>provider_parms</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
//provider_parms are the baked provider parameters; calculated (wisard
//actual for non-owr components).<br>

json_name: providerParms
go_name: ProviderParms</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the current runtime status of the component.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary key/value labels attached to the component.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-component-kind"></a>
### cloud.v1.topology.Component.Kind

<pre>
//Kind enumerates the role a component plays in the topology.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td><pre>
//KIND_UNSPECIFIED is the unset zero value (rejected by validation).
</pre></td>
</tr><tr>
<td>KIND_AGENT</td>
<td><pre>
//KIND_AGENT is a stroppy agent host.
</pre></td>
</tr><tr>
<td>KIND_MONITOR</td>
<td><pre>
//KIND_MONITOR is a monitoring/metrics component.
</pre></td>
</tr><tr>
<td>KIND_DATABASE</td>
<td><pre>
//KIND_DATABASE is a primary database node.
</pre></td>
</tr><tr>
<td>KIND_REPLICA</td>
<td><pre>
//KIND_REPLICA is a database replica node.
</pre></td>
</tr><tr>
<td>KIND_PROXY</td>
<td><pre>
//KIND_PROXY is a connection proxy/pooler.
</pre></td>
</tr><tr>
<td>KIND_WORKLOAD</td>
<td><pre>
//KIND_WORKLOAD is a workload/load-generator runner.
</pre></td>
</tr><tr>
<td>KIND_COORDINATOR</td>
<td><pre>
//KIND_COORDINATOR is a cluster coordinator/control node.
</pre></td>
</tr><tr>
<td>KIND_ADDON</td>
<td><pre>
//KIND_ADDON is a supporting add-on component.
</pre></td>
</tr><tr>
<td>KIND_EXTERNAL</td>
<td><pre>
//KIND_EXTERNAL is a component provided externally (e.g. a managed service).
</pre></td>
</tr>
</table>

<a name="cloud-v1-topology-component-strategy"></a>
### cloud.v1.topology.Component.Strategy

<pre>
//Strategy is the recipe used to deploy a component: configuration files to
//lay down and commands to run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>configuration_files</td>
<td><a href="../common/README.md#cloud-v1-common-bakedfile">cloud.v1.common.BakedFile</a></td>
<td><pre>
//configuration_files are the rendered config files to place on the host.<br>

json_name: configurationFiles
go_name: ConfigurationFiles</pre></td>
</tr><tr>
<td>deployment_commands</td>
<td><a href="../common/README.md#cloud-v1-common-cmd">cloud.v1.common.Cmd</a></td>
<td><pre>
//deployment_commands are the commands to run to bring the component up.<br>

json_name: deploymentCommands
go_name: DeploymentCommands</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-connection"></a>
### cloud.v1.topology.Connection

<pre>
//Connection is a directed edge between two components in the topology.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>from</td>
<td>string</td>
<td><pre>
//from is the source component id of the edge.<br>

json_name: from
go_name: From</pre></td>
</tr><tr>
<td>inner</td>
<td>bool</td>
<td><pre>
//inner is true when both endpoints live on one physical VM.<br>

json_name: inner
go_name: Inner</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-topology-connection-kind">cloud.v1.topology.Connection.Kind</a></td>
<td><pre>
//kind is the semantic relationship of the edge.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>mode</td>
<td><a href="#cloud-v1-topology-connection-mode">cloud.v1.topology.Connection.Mode</a></td>
<td><pre>
//mode is the traffic character of the edge.<br>

json_name: mode
go_name: Mode</pre></td>
</tr><tr>
<td>port</td>
<td>uint32</td>
<td><pre>
//port is the destination port, when applicable (<= 65535).<br>

json_name: port
go_name: Port</pre></td>
</tr><tr>
<td>protocol</td>
<td><a href="#cloud-v1-topology-connection-protocol">cloud.v1.topology.Connection.Protocol</a></td>
<td><pre>
//protocol is the wire protocol carried on the edge.<br>

json_name: protocol
go_name: Protocol</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary key/value labels attached to the connection.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>to</td>
<td>string</td>
<td><pre>
//to is the destination component id of the edge.<br>

json_name: to
go_name: To</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-connection-kind"></a>
### cloud.v1.topology.Connection.Kind

<pre>
//Kind is the semantic relationship an edge represents.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td><pre>
//KIND_UNSPECIFIED is the unset zero value.
</pre></td>
</tr><tr>
<td>KIND_FLOW</td>
<td><pre>
//KIND_FLOW is a normal data/application flow.
</pre></td>
</tr><tr>
<td>KIND_PROXY</td>
<td><pre>
//KIND_PROXY is traffic routed through a proxy/pooler.
</pre></td>
</tr><tr>
<td>KIND_REPLICATION</td>
<td><pre>
//KIND_REPLICATION is database replication traffic.
</pre></td>
</tr><tr>
<td>KIND_COORDINATION</td>
<td><pre>
//KIND_COORDINATION is cluster coordination/control traffic.
</pre></td>
</tr><tr>
<td>KIND_OBSERVATION</td>
<td><pre>
//KIND_OBSERVATION is monitoring/metrics observation traffic.
</pre></td>
</tr><tr>
<td>KIND_SUPPORT</td>
<td><pre>
//KIND_SUPPORT is auxiliary/supporting traffic.
</pre></td>
</tr>
</table>

<a name="cloud-v1-topology-connection-mode"></a>
### cloud.v1.topology.Connection.Mode

<pre>
Mode is the traffic character on an edge.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>MODE_UNSPECIFIED</td>
<td><pre>
//MODE_UNSPECIFIED is the unset zero value.
</pre></td>
</tr><tr>
<td>MODE_REQUEST</td>
<td><pre>
REQUEST is request/response.
</pre></td>
</tr><tr>
<td>MODE_STREAM</td>
<td><pre>
STREAM is a continuous one-way data stream.
</pre></td>
</tr><tr>
<td>MODE_SYNC</td>
<td><pre>
SYNC is bidirectional state synchronization.
</pre></td>
</tr><tr>
<td>MODE_HEARTBEAT</td>
<td><pre>
HEARTBEAT is periodic liveness/keepalive.
</pre></td>
</tr><tr>
<td>MODE_BROADCAST</td>
<td><pre>
BROADCAST is one-to-many fanout.
</pre></td>
</tr>
</table>

<a name="cloud-v1-topology-connection-protocol"></a>
### cloud.v1.topology.Connection.Protocol

<pre>
Protocol is the wire format on an edge.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>PROTOCOL_UNSPECIFIED</td>
<td><pre>
//PROTOCOL_UNSPECIFIED is the unset zero value.
</pre></td>
</tr><tr>
<td>PROTOCOL_TCP</td>
<td><pre>
TCP is plain TCP/IP traffic.
</pre></td>
</tr><tr>
<td>PROTOCOL_GRPC</td>
<td><pre>
GRPC is gRPC.
</pre></td>
</tr><tr>
<td>PROTOCOL_HTTP</td>
<td><pre>
HTTP is HTTP/HTTPS.
</pre></td>
</tr><tr>
<td>PROTOCOL_REPLICATION</td>
<td><pre>
REPLICATION is a DB-engine replication stream.
</pre></td>
</tr><tr>
<td>PROTOCOL_POOL</td>
<td><pre>
POOL is a pooled connection (pgbouncer, proxysql).
</pre></td>
</tr><tr>
<td>PROTOCOL_CONTROL</td>
<td><pre>
CONTROL is a control-plane protocol (DCS, raft).
</pre></td>
</tr><tr>
<td>PROTOCOL_OTLP</td>
<td><pre>
OTLP is a metrics/logs/traces scrape.
</pre></td>
</tr><tr>
<td>PROTOCOL_PROMETHEUS_REMOTE_WRITE</td>
<td><pre>
PROTOCOL_PROMETHEUS_REMOTE_WRITE is a Prometheus remote-write metrics push.
</pre></td>
</tr><tr>
<td>PROTOCOL_PROMETHEUS_PULL</td>
<td><pre>
PROTOCOL_PROMETHEUS_PULL is a Prometheus pull/scrape of metrics.
</pre></td>
</tr>
</table>

<a name="cloud-v1-topology-topology"></a>
### cloud.v1.topology.Topology

<pre>
//Topology is the complete description of a benchmark deployment: its
//instances, the edges between components, and any external components.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>connections</td>
<td><a href="#cloud-v1-topology-connection">cloud.v1.topology.Connection</a></td>
<td><pre>
//connections are the edges between components; at least one is required.<br>

json_name: connections
go_name: Connections</pre></td>
</tr><tr>
<td>external_components</td>
<td><a href="#cloud-v1-topology-component">cloud.v1.topology.Component</a></td>
<td><pre>
//Here we can add something like managed database or another sevice from prvider
//Responsibility of this is RenderTerraformVariablesWorkflow|RenderDockerInputWorkflow<br>

json_name: externalComponents
go_name: ExternalComponents</pre></td>
</tr><tr>
<td>instances</td>
<td><a href="#cloud-v1-topology-topology-instance">cloud.v1.topology.Topology.Instance</a></td>
<td><pre>
//instances are the physical machines making up the topology; at least one
//is required.<br>

json_name: instances
go_name: Instances</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary key/value labels attached to the whole topology.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-topology-instance"></a>
### cloud.v1.topology.Topology.Instance

<pre>
//Instance is one physical machine (VM) in the topology onto which
//components are allocated.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>allocated_quotas</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-allocation">cloud.v1.deployment.Quota.Allocation</a></td>
<td><pre>
//allocated_quotas are the quotas actually granted to this instance;
//calculated (deployment).<br>

json_name: allocatedQuotas
go_name: AllocatedQuotas</pre></td>
</tr><tr>
<td>deployment_parms</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
//deployment_parms are the baked deployment parameters; calculated
//(deployment).<br>

json_name: deploymentParms
go_name: DeploymentParms</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the unique identifier of the instance within the topology.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>machine_info</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-machineinfo">cloud.v1.deployment.MachineInfo</a></td>
<td><pre>
//machine_info describes the requested machine shape/specs.<br>

json_name: machineInfo
go_name: MachineInfo</pre></td>
</tr><tr>
<td>provider_parms</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
//provider_parms are the baked provider parameters; calculated (wisard).<br>

json_name: providerParms
go_name: ProviderParms</pre></td>
</tr><tr>
<td>quota_requests</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-request">cloud.v1.deployment.Quota.Request</a></td>
<td><pre>
//quota_requests are the resource quotas requested for this instance;
//calculated (deployment).<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the current runtime status of the instance.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary key/value labels attached to the instance.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>

