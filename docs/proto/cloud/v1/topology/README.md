

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
  - [cloud.v1.topology.RuntimeConnection](#cloud-v1-topology-runtimeconnection)
  - [cloud.v1.topology.RuntimeConnection.LabelsEntry](#cloud-v1-topology-runtimeconnection-labelsentry)
  - [cloud.v1.topology.RuntimeNode](#cloud-v1-topology-runtimenode)
  - [cloud.v1.topology.RuntimeNode.Kind](#cloud-v1-topology-runtimenode-kind)
  - [cloud.v1.topology.RuntimeNode.LabelsEntry](#cloud-v1-topology-runtimenode-labelsentry)
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



<a name="cloud-v1-topology-runtimeconnection"></a>
### cloud.v1.topology.RuntimeConnection

<pre>
//RuntimeConnection is one concrete directed interaction visible during a run:
//agent-control, binary download, component traffic, metrics scrape, logs
//shipping, local exporter reads, dependency/placement, or a Temporal-backed
//agent action. It carries endpoint/protocol/status/timing so the UI can show
//who talks to whom and when.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>endpoint_name</td>
<td>string</td>
<td><pre>
//endpoint_name is the destination endpoint/path/job/action name.<br>

json_name: endpointName
go_name: EndpointName</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//finished_at is when the backing runtime action/stage finished, if known.<br>

json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>from_node_id</td>
<td>string</td>
<td><pre>
//from_node_id is the source RuntimeNode.id.<br>

json_name: fromNodeId
go_name: FromNodeId</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is a stable graph edge id.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-topology-connection-kind">cloud.v1.topology.Connection.Kind</a></td>
<td><pre>
//kind is the semantic relationship, reusing logical Connection.Kind.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-topology-runtimeconnection-labelsentry">cloud.v1.topology.RuntimeConnection.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata for UI/debugging.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>mode</td>
<td><a href="#cloud-v1-topology-connection-mode">cloud.v1.topology.Connection.Mode</a></td>
<td><pre>
//mode is the traffic character, reusing logical Connection.Mode.<br>

json_name: mode
go_name: Mode</pre></td>
</tr><tr>
<td>node_execution_id</td>
<td>string</td>
<td><pre>
//node_execution_id links this edge to the pipeline/Temporal stage that
//produced or last updated it.<br>

json_name: nodeExecutionId
go_name: NodeExecutionId</pre></td>
</tr><tr>
<td>phase</td>
<td>string</td>
<td><pre>
//phase is the top-level pipeline phase this connection belongs to.<br>

json_name: phase
go_name: Phase</pre></td>
</tr><tr>
<td>port</td>
<td>uint32</td>
<td><pre>
//port is the destination port when known.<br>

json_name: port
go_name: Port</pre></td>
</tr><tr>
<td>protocol</td>
<td><a href="#cloud-v1-topology-connection-protocol">cloud.v1.topology.Connection.Protocol</a></td>
<td><pre>
//protocol is the wire/protocol family, reusing logical Connection.Protocol.<br>

json_name: protocol
go_name: Protocol</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_at is when the backing runtime action/stage started, if known.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is this connection's current/last-known lifecycle status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>status_reason</td>
<td>string</td>
<td><pre>
//status_reason is a short machine-readable explanation of status.<br>

json_name: statusReason
go_name: StatusReason</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata on the runtime connection.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>to_node_id</td>
<td>string</td>
<td><pre>
//to_node_id is the destination RuntimeNode.id.<br>

json_name: toNodeId
go_name: ToNodeId</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-runtimeconnection-labelsentry"></a>
### cloud.v1.topology.RuntimeConnection.LabelsEntry

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



<a name="cloud-v1-topology-runtimenode"></a>
### cloud.v1.topology.RuntimeNode

<pre>
//RuntimeNode is one concrete thing that exists or participates during a run:
//the control-plane server, a provider machine, an agent, a deployed component,
//a monitor daemon/exporter, a workload runner, or an external endpoint. Unlike
//TopologySpec.Component, this is a runtime fact with placement, status and
//timing/provenance.
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
//address is the known host/IP/base URL for this node.<br>

json_name: address
go_name: Address</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id links this node to TopologySpec.Component / DeploymentPlan.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>engine</td>
<td>string</td>
<td><pre>
//engine identifies the product/subsystem, e.g. stroppy, postgres,
//victoriametrics, vector, node_exporter.<br>

json_name: engine
go_name: Engine</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//finished_at is when the backing runtime action/stage finished, if known.<br>

json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is a stable graph id, e.g. control-plane, machine/node-1,
//agent/node-1, component/postgres-master, monitor/node-1/vmagent.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-topology-runtimenode-kind">cloud.v1.topology.RuntimeNode.Kind</a></td>
<td><pre>
//kind is the concrete runtime node family.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>label</td>
<td>string</td>
<td><pre>
//label is a short display label.<br>

json_name: label
go_name: Label</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-topology-runtimenode-labelsentry">cloud.v1.topology.RuntimeNode.LabelsEntry</a></td>
<td><pre>
//labels are structured metadata for UI/debugging.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
//machine_id links this node to TopologySpec.Node / MachineState.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>node_execution_id</td>
<td>string</td>
<td><pre>
//node_execution_id links this node to the pipeline/Temporal stage that
//materialized or last touched it.<br>

json_name: nodeExecutionId
go_name: NodeExecutionId</pre></td>
</tr><tr>
<td>port</td>
<td>uint32</td>
<td><pre>
//port is the known listening port for this node, when applicable.<br>

json_name: port
go_name: Port</pre></td>
</tr><tr>
<td>role</td>
<td>string</td>
<td><pre>
//role identifies the concrete runtime role inside the engine.<br>

json_name: role
go_name: Role</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_at is when the backing runtime action/stage started, if known.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is this runtime node's current/last-known lifecycle status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>status_reason</td>
<td>string</td>
<td><pre>
//status_reason is a short machine-readable explanation of status.<br>

json_name: statusReason
go_name: StatusReason</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are arbitrary metadata on the runtime node.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-topology-runtimenode-kind"></a>
### cloud.v1.topology.RuntimeNode.Kind

<pre>
//Kind classifies the concrete runtime node.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>KIND_CONTROL_PLANE</td>
<td></td>
</tr><tr>
<td>KIND_MACHINE</td>
<td></td>
</tr><tr>
<td>KIND_AGENT</td>
<td></td>
</tr><tr>
<td>KIND_COMPONENT</td>
<td></td>
</tr><tr>
<td>KIND_MONITOR</td>
<td></td>
</tr><tr>
<td>KIND_EXPORTER</td>
<td></td>
</tr><tr>
<td>KIND_WORKLOAD</td>
<td></td>
</tr><tr>
<td>KIND_EXTERNAL</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-topology-runtimenode-labelsentry"></a>
### cloud.v1.topology.RuntimeNode.LabelsEntry

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
<td>runtime_connections</td>
<td><a href="#cloud-v1-topology-runtimeconnection">cloud.v1.topology.RuntimeConnection</a></td>
<td><pre>
//runtime_connections are concrete directed interactions between
//runtime_nodes, with endpoint/protocol/status/timing metadata.<br>

json_name: runtimeConnections
go_name: RuntimeConnections</pre></td>
</tr><tr>
<td>runtime_nodes</td>
<td><a href="#cloud-v1-topology-runtimenode">cloud.v1.topology.RuntimeNode</a></td>
<td><pre>
//runtime_nodes are concrete nodes known for this run, including the
//control-plane server, machines, agents, components, monitor daemons and
//external endpoints.<br>

json_name: runtimeNodes
go_name: RuntimeNodes</pre></td>
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

