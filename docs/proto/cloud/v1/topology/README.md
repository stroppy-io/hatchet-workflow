

<a name="cloud-v1-topology"></a>
# cloud.v1.topology

## Table of Contents
- Messages
  - [cloud.v1.topology.Component](#cloud-v1-topology-component)
  - [cloud.v1.topology.Component.Kind](#cloud-v1-topology-component-kind)
  - [cloud.v1.topology.Component.LabelsEntry](#cloud-v1-topology-component-labelsentry)
  - [cloud.v1.topology.Connection](#cloud-v1-topology-connection)
  - [cloud.v1.topology.Connection.Kind](#cloud-v1-topology-connection-kind)
  - [cloud.v1.topology.Connection.Mode](#cloud-v1-topology-connection-mode)
  - [cloud.v1.topology.Connection.Protocol](#cloud-v1-topology-connection-protocol)
  - [cloud.v1.topology.Node](#cloud-v1-topology-node)
  - [cloud.v1.topology.Node.LabelsEntry](#cloud-v1-topology-node-labelsentry)
  - [cloud.v1.topology.Topology](#cloud-v1-topology-topology)
  - [cloud.v1.topology.Topology.State](#cloud-v1-topology-topology-state)
  - [cloud.v1.topology.TopologySpec](#cloud-v1-topology-topologyspec)
  - [cloud.v1.topology.TopologySpec.LabelsEntry](#cloud-v1-topology-topologyspec-labelsentry)

<a name="cloud-v1-topology-messages"></a>
## Messages

<a name="cloud-v1-topology-component"></a>
### cloud.v1.topology.Component

<pre>
//Component is one logical service role in the benchmark graph.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>engine</td>
<td>string</td>
<td><pre>
//engine identifies the owning product or subsystem, e.g. postgres, ydb,
//stroppy, victoriametrics. It is data rather than an enum so new engines
//do not require changing topology proto.<br>

json_name: engine
go_name: Engine</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable logical component id within the topology spec.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-topology-component-kind">cloud.v1.topology.Component.Kind</a></td>
<td><pre>
//kind is the broad component family.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-topology-component-labelsentry">cloud.v1.topology.Component.LabelsEntry</a></td>
<td><pre>
//labels are structured role metadata used by renderers and UI.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>role</td>
<td>string</td>
<td><pre>
//role identifies the concrete role inside the engine, e.g. master,
//replica, haproxy, pgbouncer, patroni, etcd.<br>

json_name: role
go_name: Role</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary user/system tags.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-component-kind"></a>
### cloud.v1.topology.Component.Kind

<pre>
//Kind enumerates the broad role family a component belongs to.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td><pre>
//KIND_UNSPECIFIED is the unset zero value.
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
//KIND_DATABASE is a primary database service component.
</pre></td>
</tr><tr>
<td>KIND_REPLICA</td>
<td><pre>
//KIND_REPLICA is a database replica service component.
</pre></td>
</tr><tr>
<td>KIND_PROXY</td>
<td><pre>
//KIND_PROXY is a proxy, load balancer, or connection pooler.
</pre></td>
</tr><tr>
<td>KIND_WORKLOAD</td>
<td><pre>
//KIND_WORKLOAD is a workload/load-generator runner.
</pre></td>
</tr><tr>
<td>KIND_COORDINATOR</td>
<td><pre>
//KIND_COORDINATOR is a cluster coordinator/control-plane component.
</pre></td>
</tr><tr>
<td>KIND_ADDON</td>
<td><pre>
//KIND_ADDON is a supporting add-on component.
</pre></td>
</tr><tr>
<td>KIND_EXTERNAL</td>
<td><pre>
//KIND_EXTERNAL is provided outside this deployment.
</pre></td>
</tr>
</table>

<a name="cloud-v1-topology-component-labelsentry"></a>
### cloud.v1.topology.Component.LabelsEntry

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



<a name="cloud-v1-topology-connection"></a>
### cloud.v1.topology.Connection

<pre>
//Connection is a directed logical edge from one component to another.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>colocated</td>
<td>bool</td>
<td><pre>
//colocated is true when this edge intentionally stays inside one node.<br>

json_name: colocated
go_name: Colocated</pre></td>
</tr><tr>
<td>endpoint_name</td>
<td>string</td>
<td><pre>
//endpoint_name optionally selects a named destination endpoint, e.g.
//postgres, pgbouncer, patroni_rest, etcd_peer. Empty means renderer
//chooses the role default.<br>

json_name: endpointName
go_name: EndpointName</pre></td>
</tr><tr>
<td>from_component_id</td>
<td>string</td>
<td><pre>
//from_component_id is the source component id.<br>

json_name: fromComponentId
go_name: FromComponentId</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-topology-connection-kind">cloud.v1.topology.Connection.Kind</a></td>
<td><pre>
//kind is the semantic relationship.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>mode</td>
<td><a href="#cloud-v1-topology-connection-mode">cloud.v1.topology.Connection.Mode</a></td>
<td><pre>
//mode is the traffic character.<br>

json_name: mode
go_name: Mode</pre></td>
</tr><tr>
<td>port</td>
<td>uint32</td>
<td><pre>
//port is the destination port when it is known at spec time.<br>

json_name: port
go_name: Port</pre></td>
</tr><tr>
<td>protocol</td>
<td><a href="#cloud-v1-topology-connection-protocol">cloud.v1.topology.Connection.Protocol</a></td>
<td><pre>
//protocol is the protocol family.<br>

json_name: protocol
go_name: Protocol</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata on the edge.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>to_component_id</td>
<td>string</td>
<td><pre>
//to_component_id is the destination component id.<br>

json_name: toComponentId
go_name: ToComponentId</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-connection-kind"></a>
### cloud.v1.topology.Connection.Kind

<pre>
//Kind is the semantic relationship represented by the edge.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>KIND_FLOW</td>
<td></td>
</tr><tr>
<td>KIND_PROXY</td>
<td></td>
</tr><tr>
<td>KIND_REPLICATION</td>
<td></td>
</tr><tr>
<td>KIND_COORDINATION</td>
<td></td>
</tr><tr>
<td>KIND_OBSERVATION</td>
<td></td>
</tr><tr>
<td>KIND_SUPPORT</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-topology-connection-mode"></a>
### cloud.v1.topology.Connection.Mode

<pre>
//Mode is the traffic character of the edge.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>MODE_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>MODE_REQUEST</td>
<td></td>
</tr><tr>
<td>MODE_STREAM</td>
<td></td>
</tr><tr>
<td>MODE_SYNC</td>
<td></td>
</tr><tr>
<td>MODE_HEARTBEAT</td>
<td></td>
</tr><tr>
<td>MODE_BROADCAST</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-topology-connection-protocol"></a>
### cloud.v1.topology.Connection.Protocol

<pre>
//Protocol is the wire/protocol family carried over the edge.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>PROTOCOL_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>PROTOCOL_TCP</td>
<td></td>
</tr><tr>
<td>PROTOCOL_GRPC</td>
<td></td>
</tr><tr>
<td>PROTOCOL_HTTP</td>
<td></td>
</tr><tr>
<td>PROTOCOL_REPLICATION</td>
<td></td>
</tr><tr>
<td>PROTOCOL_POOL</td>
<td></td>
</tr><tr>
<td>PROTOCOL_CONTROL</td>
<td></td>
</tr><tr>
<td>PROTOCOL_OTLP</td>
<td></td>
</tr><tr>
<td>PROTOCOL_PROMETHEUS_REMOTE_WRITE</td>
<td></td>
</tr><tr>
<td>PROTOCOL_PROMETHEUS_PULL</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-topology-node"></a>
### cloud.v1.topology.Node

<pre>
//Node is a logical placement unit. A node may become a VM, a Docker container,
//or a managed/external provider resource after the infrastructure stage.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>component_ids</td>
<td>string</td>
<td><pre>
//component_ids are the components intentionally colocated on this node.<br>

json_name: componentIds
go_name: ComponentIds</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable logical node id.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-topology-node-labelsentry">cloud.v1.topology.Node.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata used by planners and UI.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary user/system tags.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-node-labelsentry"></a>
### cloud.v1.topology.Node.LabelsEntry

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



<a name="cloud-v1-topology-topology"></a>
### cloud.v1.topology.Topology

<pre>
//Topology is the full staged object for a run or wizard draft.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan is the agent execution plan. Present from
//STATE_DEPLOYMENT_PLANNED.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//infrastructure_plan is provider input derived from spec + provider
//choices. Present from STATE_INFRASTRUCTURE_PLANNED.<br>

json_name: infrastructurePlan
go_name: InfrastructurePlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is provider output: ids, addresses, allocated
//quotas. Present from STATE_INFRASTRUCTURE_DEPLOYED.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>spec</td>
<td><a href="#cloud-v1-topology-topologyspec">cloud.v1.topology.TopologySpec</a></td>
<td><pre>
//spec is the logical graph.<br>

json_name: spec
go_name: Spec</pre></td>
</tr><tr>
<td>state</td>
<td><a href="#cloud-v1-topology-topology-state">cloud.v1.topology.Topology.State</a></td>
<td><pre>
//state records the current stage.<br>

json_name: state
go_name: State</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata on the envelope.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-topology-state"></a>
### cloud.v1.topology.Topology.State

<pre>
//State records the furthest stage represented by the envelope.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>STATE_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>STATE_SPEC</td>
<td></td>
</tr><tr>
<td>STATE_INFRASTRUCTURE_PLANNED</td>
<td></td>
</tr><tr>
<td>STATE_INFRASTRUCTURE_DEPLOYED</td>
<td></td>
</tr><tr>
<td>STATE_DEPLOYMENT_PLANNED</td>
<td></td>
</tr><tr>
<td>STATE_DEPLOYED</td>
<td></td>
</tr><tr>
<td>STATE_UNDEPLOYED</td>
<td></td>
</tr><tr>
<td>STATE_ARCHIVE</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-topology-topologyspec"></a>
### cloud.v1.topology.TopologySpec

<pre>
//TopologySpec is the pure logical graph produced from domain params.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>components</td>
<td><a href="#cloud-v1-topology-component">cloud.v1.topology.Component</a></td>
<td><pre>
//components are logical services/roles.<br>

json_name: components
go_name: Components</pre></td>
</tr><tr>
<td>connections</td>
<td><a href="#cloud-v1-topology-connection">cloud.v1.topology.Connection</a></td>
<td><pre>
//connections are logical edges between components. Single-node topologies
//may legitimately have no edges.<br>

json_name: connections
go_name: Connections</pre></td>
</tr><tr>
<td>external_components</td>
<td><a href="#cloud-v1-topology-component">cloud.v1.topology.Component</a></td>
<td><pre>
//external_components are logical components not deployed by this run.<br>

json_name: externalComponents
go_name: ExternalComponents</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-topology-topologyspec-labelsentry">cloud.v1.topology.TopologySpec.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata attached to the spec.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>nodes</td>
<td><a href="#cloud-v1-topology-node">cloud.v1.topology.Node</a></td>
<td><pre>
//nodes are logical placement units.<br>

json_name: nodes
go_name: Nodes</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary user/system tags.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-topologyspec-labelsentry"></a>
### cloud.v1.topology.TopologySpec.LabelsEntry

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

