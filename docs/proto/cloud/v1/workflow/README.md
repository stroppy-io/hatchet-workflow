

<a name="cloud-v1-workflow"></a>
# cloud.v1.workflow

## Table of Contents
- [cloud.v1.workflow.AgentCommandService](#cloud-v1-workflow-agentcommandservice)
  - [Activities](#cloud-v1-workflow-agentcommandservice-activities)
    - [cloud.v1.workflow.AgentCommandService.CallCmdActivity](#cloud-v1-workflow-agentcommandservice-callcmdactivity-activity)
    - [cloud.v1.workflow.AgentCommandService.CreateDirActivity](#cloud-v1-workflow-agentcommandservice-creatediractivity-activity)
    - [cloud.v1.workflow.AgentCommandService.CreateTempDirActivity](#cloud-v1-workflow-agentcommandservice-createtempdiractivity-activity)
    - [cloud.v1.workflow.AgentCommandService.EnsureAgentOnlineActivity](#cloud-v1-workflow-agentcommandservice-ensureagentonlineactivity-activity)
    - [cloud.v1.workflow.AgentCommandService.WriteFileActivity](#cloud-v1-workflow-agentcommandservice-writefileactivity-activity)
- [cloud.v1.workflow.DeploymentService](#cloud-v1-workflow-deploymentservice)
  - [Workflows](#cloud-v1-workflow-deploymentservice-workflows)
    - [CalculateQuotasWorkflow](#calculatequotasworkflow-workflow)
    - [ProcessDeploymentWorkflow](#processdeploymentworkflow-workflow)
    - [RenderDockerInputWorkflow](#renderdockerinputworkflow-workflow)
    - [RenderTerraformVariablesWorkflow](#renderterraformvariablesworkflow-workflow)
  - [Activities](#cloud-v1-workflow-deploymentservice-activities)
    - [cloud.v1.workflow.DeploymentService.AcquireNetworkActivity](#cloud-v1-workflow-deploymentservice-acquirenetworkactivity-activity)
    - [cloud.v1.workflow.DeploymentService.AcquireQuotasActivity](#cloud-v1-workflow-deploymentservice-acquirequotasactivity-activity)
    - [cloud.v1.workflow.DeploymentService.DockerDownActivity](#cloud-v1-workflow-deploymentservice-dockerdownactivity-activity)
    - [cloud.v1.workflow.DeploymentService.DockerPullActivity](#cloud-v1-workflow-deploymentservice-dockerpullactivity-activity)
    - [cloud.v1.workflow.DeploymentService.DockerUpActivity](#cloud-v1-workflow-deploymentservice-dockerupactivity-activity)
    - [cloud.v1.workflow.DeploymentService.TerraformApplyActivity](#cloud-v1-workflow-deploymentservice-terraformapplyactivity-activity)
    - [cloud.v1.workflow.DeploymentService.TerraformDestroyActivity](#cloud-v1-workflow-deploymentservice-terraformdestroyactivity-activity)
    - [cloud.v1.workflow.DeploymentService.TerraformPlanActivity](#cloud-v1-workflow-deploymentservice-terraformplanactivity-activity)
- [cloud.v1.workflow.SuiteWorkflowService](#cloud-v1-workflow-suiteworkflowservice)
  - [Workflows](#cloud-v1-workflow-suiteworkflowservice-workflows)
    - [SuiteWorkflow](#suiteworkflow-workflow)
- [cloud.v1.workflow.TestService](#cloud-v1-workflow-testservice)
  - [Workflows](#cloud-v1-workflow-testservice-workflows)
    - [InstallDatabaseWorkflow](#installdatabaseworkflow-workflow)
    - [InstallStroppyWorkflow](#installstroppyworkflow-workflow)
    - [RunWorkloadWorkflow](#runworkloadworkflow-workflow)
    - [TestWorkflow](#testworkflow-workflow)
- Messages
  - [cloud.v1.workflow.AcquireNetworkActivityRequest](#cloud-v1-workflow-acquirenetworkactivityrequest)
  - [cloud.v1.workflow.AcquireNetworkActivityResponse](#cloud-v1-workflow-acquirenetworkactivityresponse)
  - [cloud.v1.workflow.AcquireQuotasActivityRequest](#cloud-v1-workflow-acquirequotasactivityrequest)
  - [cloud.v1.workflow.AcquireQuotasActivityRequest.QuotaRequestsEntry](#cloud-v1-workflow-acquirequotasactivityrequest-quotarequestsentry)
  - [cloud.v1.workflow.AcquireQuotasActivityResponse](#cloud-v1-workflow-acquirequotasactivityresponse)
  - [cloud.v1.workflow.AcquireQuotasActivityResponse.QuotaAllocationEntry](#cloud-v1-workflow-acquirequotasactivityresponse-quotaallocationentry)
  - [cloud.v1.workflow.CalculateQuotasWorkflowRequest](#cloud-v1-workflow-calculatequotasworkflowrequest)
  - [cloud.v1.workflow.CalculateQuotasWorkflowResponse](#cloud-v1-workflow-calculatequotasworkflowresponse)
  - [cloud.v1.workflow.CalculateQuotasWorkflowResponse.QuotaRequestsEntry](#cloud-v1-workflow-calculatequotasworkflowresponse-quotarequestsentry)
  - [cloud.v1.workflow.InstallDatabaseWorkflowRequest](#cloud-v1-workflow-installdatabaseworkflowrequest)
  - [cloud.v1.workflow.InstallDatabaseWorkflowResponse](#cloud-v1-workflow-installdatabaseworkflowresponse)
  - [cloud.v1.workflow.InstallStroppyWorkflowRequest](#cloud-v1-workflow-installstroppyworkflowrequest)
  - [cloud.v1.workflow.InstallStroppyWorkflowResponse](#cloud-v1-workflow-installstroppyworkflowresponse)
  - [cloud.v1.workflow.ProcessDeploymentWorkflowRequest](#cloud-v1-workflow-processdeploymentworkflowrequest)
  - [cloud.v1.workflow.ProcessDeploymentWorkflowResponse](#cloud-v1-workflow-processdeploymentworkflowresponse)
  - [cloud.v1.workflow.RunWorkloadWorkflowRequest](#cloud-v1-workflow-runworkloadworkflowrequest)
  - [cloud.v1.workflow.RunWorkloadWorkflowResponse](#cloud-v1-workflow-runworkloadworkflowresponse)
  - [cloud.v1.workflow.SuiteWorkflowRequest](#cloud-v1-workflow-suiteworkflowrequest)
  - [cloud.v1.workflow.SuiteWorkflowResponse](#cloud-v1-workflow-suiteworkflowresponse)
  - [cloud.v1.workflow.TestWorkflowRequest](#cloud-v1-workflow-testworkflowrequest)
  - [cloud.v1.workflow.TestWorkflowResponse](#cloud-v1-workflow-testworkflowresponse)

<a name="cloud-v1-workflow-services"></a>
## Services

<a name="cloud-v1-workflow-agentcommandservice"></a>
## cloud.v1.workflow.AgentCommandService

<pre>
//AgentCommandService groups the agent-facing Temporal activities that perform
//filesystem and command work on an agent host.
</pre>   

<a name="cloud-v1-workflow-agentcommandservice-activities"></a>
### Activities

---
<a name="cloud-v1-workflow-agentcommandservice-callcmdactivity-activity"></a>
### cloud.v1.workflow.AgentCommandService.CallCmdActivity

<pre>
//CallCmdActivity runs a command on the agent host (including the long
//stroppy load) and returns its result, heartbeating while it runs.
</pre>

**Input:** [cloud.v1.common.Cmd](../common/README.md#cloud-v1-common-cmd)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>result</td>
<td><a href="../common/README.md#cloud-v1-common-cmd-result">cloud.v1.common.Cmd.Result</a></td>
<td><pre>
result contains observed execution output after the command finishes.<br>

json_name: result
go_name: Result</pre></td>
</tr><tr>
<td>spec</td>
<td><a href="../common/README.md#cloud-v1-common-cmd-spec">cloud.v1.common.Cmd.Spec</a></td>
<td><pre>
spec describes the command to execute.<br>

json_name: spec
go_name: Spec</pre></td>
</tr>
</table>

**Output:** [cloud.v1.common.Cmd.Result](../common/README.md#cloud-v1-common-cmd-result)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>elapsed</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
elapsed is observed wall-clock runtime.<br>

json_name: elapsed
go_name: Elapsed</pre></td>
</tr><tr>
<td>exit_code</td>
<td>int32</td>
<td><pre>
exit_code is the process exit code.<br>

json_name: exitCode
go_name: ExitCode</pre></td>
</tr><tr>
<td>stderr</td>
<td>bytes</td>
<td><pre>
stderr contains captured standard error.<br>

json_name: stderr
go_name: Stderr</pre></td>
</tr><tr>
<td>stdout</td>
<td>bytes</td>
<td><pre>
stdout contains captured standard output.<br>

json_name: stdout
go_name: Stdout</pre></td>
</tr><tr>
<td>timed_out</td>
<td>bool</td>
<td><pre>
timed_out is true when the executor terminated the command by timeout.<br>

json_name: timedOut
go_name: TimedOut</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>heartbeat_timeout</td><td>1 minute</td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
<tr><td>schedule_to_close_timeout</td><td>1 minute</td></tr>
<tr><td>start_to_close_timeout</td><td>4 weeks 2 days</td></tr>
</table> 

---
<a name="cloud-v1-workflow-agentcommandservice-creatediractivity-activity"></a>
### cloud.v1.workflow.AgentCommandService.CreateDirActivity

<pre>
//CreateDirActivity creates a directory on the agent host (mkdir -p
//semantics).
</pre>

**Input:** [cloud.v1.common.Dir](../common/README.md#cloud-v1-common-dir)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>create_parents</td>
<td>bool</td>
<td><pre>
create_parents creates missing parent directories.<br>

json_name: createParents
go_name: CreateParents</pre></td>
</tr><tr>
<td>info</td>
<td><a href="../common/README.md#cloud-v1-common-dir-info">cloud.v1.common.Dir.Info</a></td>
<td><pre>
info contains path, permissions, owner, and group.<br>

json_name: info
go_name: Info</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.initial_interval</td><td>2 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>start_to_close_timeout</td><td>30 seconds</td></tr>
</table> 

---
<a name="cloud-v1-workflow-agentcommandservice-createtempdiractivity-activity"></a>
### cloud.v1.workflow.AgentCommandService.CreateTempDirActivity

<pre>
//CreateTempDirActivity creates a fresh temporary directory on the agent
//host and returns its path.
</pre>

**Input:** [cloud.v1.common.Dir.Temp](#cloud-v1-common-dir-temp)



**Output:** [cloud.v1.common.Dir](../common/README.md#cloud-v1-common-dir)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>create_parents</td>
<td>bool</td>
<td><pre>
create_parents creates missing parent directories.<br>

json_name: createParents
go_name: CreateParents</pre></td>
</tr><tr>
<td>info</td>
<td><a href="../common/README.md#cloud-v1-common-dir-info">cloud.v1.common.Dir.Info</a></td>
<td><pre>
info contains path, permissions, owner, and group.<br>

json_name: info
go_name: Info</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
<tr><td>start_to_close_timeout</td><td>30 seconds</td></tr>
</table> 

---
<a name="cloud-v1-workflow-agentcommandservice-ensureagentonlineactivity-activity"></a>
### cloud.v1.workflow.AgentCommandService.EnsureAgentOnlineActivity

<pre>
//EnsureAgentOnlineActivity blocks until the target agent reports online,
//heartbeating while it polls.
</pre>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>heartbeat_timeout</td><td>1 minute</td></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_interval</td><td>1 minute</td></tr>
<tr><td>schedule_to_close_timeout</td><td>1 minute</td></tr>
<tr><td>start_to_close_timeout</td><td>15 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-agentcommandservice-writefileactivity-activity"></a>
### cloud.v1.workflow.AgentCommandService.WriteFileActivity

<pre>
//WriteFileActivity writes a file's full contents to the agent host.
</pre>

**Input:** [cloud.v1.common.File](../common/README.md#cloud-v1-common-file)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>append</td>
<td>bool</td>
<td><pre>
append, when true, appends content to an existing file instead of overwriting it.<br>

json_name: append
go_name: Append</pre></td>
</tr><tr>
<td>as_ref</td>
<td><a href="../common/README.md#cloud-v1-common-file-asref">cloud.v1.common.File.AsRef</a></td>
<td><pre>
as_ref contains referenced file content.<br>

json_name: asRef
go_name: AsRef</pre></td>
</tr><tr>
<td>bytes</td>
<td>bytes</td>
<td><pre>
bytes contains binary file content.<br>

json_name: bytes
go_name: Bytes</pre></td>
</tr><tr>
<td>info</td>
<td><a href="../common/README.md#cloud-v1-common-file-info">cloud.v1.common.File.Info</a></td>
<td><pre>
info contains path, permissions, owner, and group.<br>

json_name: info
go_name: Info</pre></td>
</tr><tr>
<td>text</td>
<td>string</td>
<td><pre>
text contains UTF-8 or text-like file content.<br>

json_name: text
go_name: Text</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.initial_interval</td><td>2 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>start_to_close_timeout</td><td>30 seconds</td></tr>
</table>   

<a name="cloud-v1-workflow-deploymentservice"></a>
## cloud.v1.workflow.DeploymentService

<pre>
//DeploymentService groups the Temporal workflows and activities that provision
//and tear down a topology's infrastructure.
</pre>

<a name="cloud-v1-workflow-deploymentservice-workflows"></a>
### Workflows

---
<a name="calculatequotasworkflow-workflow"></a>
### CalculateQuotasWorkflow

<pre>
//CalculateQuotasWorkflow computes resource quota requests from a topology
//(pure computation, retryable).
</pre>

**Input:** [cloud.v1.workflow.CalculateQuotasWorkflowRequest](#cloud-v1-workflow-calculatequotasworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology to compute quota requests for.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.CalculateQuotasWorkflowResponse](#cloud-v1-workflow-calculatequotasworkflowresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-calculatequotasworkflowresponse-quotarequestsentry">cloud.v1.workflow.CalculateQuotasWorkflowResponse.QuotaRequestsEntry</a></td>
<td><pre>
//quota_requests are the computed requests keyed by component.id.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the (unchanged) topology the quotas were computed for.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>run_timeout</td><td>5 minutes</td></tr>
</table>

---
<a name="processdeploymentworkflow-workflow"></a>
### ProcessDeploymentWorkflow

<pre>
//ProcessDeploymentWorkflow provisions a topology end to end; always a
//child of TestWorkflow and never auto-retried as a whole.
</pre>

**Input:** [cloud.v1.workflow.ProcessDeploymentWorkflowRequest](#cloud-v1-workflow-processdeploymentworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the target cloud/provider to deploy onto.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology to provision; instances MUST already carry their
//provider_parms.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.ProcessDeploymentWorkflowResponse](#cloud-v1-workflow-processdeploymentworkflowresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>deployed_topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//deployed_topology is the full deployed topology with all runtime params
//filled in.<br>

json_name: deployedTopology
go_name: DeployedTopology</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the provider the topology was deployed onto.<br>

json_name: provider
go_name: Provider</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
</table>

---
<a name="renderdockerinputworkflow-workflow"></a>
### RenderDockerInputWorkflow

<pre>
//RenderDockerInputWorkflow renders a topology into Docker compose input
//(pure render, retryable).
</pre>

**Input:** [cloud.v1.topology.Topology](../topology/README.md#cloud-v1-topology-topology)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>connections</td>
<td><a href="../topology/README.md#cloud-v1-topology-connection">cloud.v1.topology.Connection</a></td>
<td><pre>
//connections are the edges between components; at least one is required.<br>

json_name: connections
go_name: Connections</pre></td>
</tr><tr>
<td>external_components</td>
<td><a href="../topology/README.md#cloud-v1-topology-component">cloud.v1.topology.Component</a></td>
<td><pre>
//Here we can add something like managed database or another sevice from prvider
//Responsibility of this is RenderTerraformVariablesWorkflow|RenderDockerInputWorkflow<br>

json_name: externalComponents
go_name: ExternalComponents</pre></td>
</tr><tr>
<td>instances</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology-instance">cloud.v1.topology.Topology.Instance</a></td>
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

**Output:** [cloud.v1.deployment.Docker.Input](../deployment/README.md#cloud-v1-deployment-docker-input)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-containersentry">cloud.v1.deployment.Docker.Input.ContainersEntry</a></td>
<td><pre>
containers contains runtime container specs keyed by stable name.<br>

json_name: containers
go_name: Containers</pre></td>
</tr><tr>
<td>network</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-network">cloud.v1.deployment.Docker.Network</a></td>
<td><pre>
network describes the Docker network and DNS settings.<br>

json_name: network
go_name: Network</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>run_timeout</td><td>1 minute</td></tr>
</table>

---
<a name="renderterraformvariablesworkflow-workflow"></a>
### RenderTerraformVariablesWorkflow

<pre>
//RenderTerraformVariablesWorkflow renders a topology into Terraform
//variables input (pure render, retryable).
</pre>

**Input:** [cloud.v1.topology.Topology](../topology/README.md#cloud-v1-topology-topology)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>connections</td>
<td><a href="../topology/README.md#cloud-v1-topology-connection">cloud.v1.topology.Connection</a></td>
<td><pre>
//connections are the edges between components; at least one is required.<br>

json_name: connections
go_name: Connections</pre></td>
</tr><tr>
<td>external_components</td>
<td><a href="../topology/README.md#cloud-v1-topology-component">cloud.v1.topology.Component</a></td>
<td><pre>
//Here we can add something like managed database or another sevice from prvider
//Responsibility of this is RenderTerraformVariablesWorkflow|RenderDockerInputWorkflow<br>

json_name: externalComponents
go_name: ExternalComponents</pre></td>
</tr><tr>
<td>instances</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology-instance">cloud.v1.topology.Topology.Instance</a></td>
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

**Output:** [cloud.v1.deployment.Terraform.Input](#cloud-v1-deployment-terraform-input)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>run_timeout</td><td>1 minute</td></tr>
</table>    

<a name="cloud-v1-workflow-deploymentservice-activities"></a>
### Activities

---
<a name="cloud-v1-workflow-deploymentservice-acquirenetworkactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.AcquireNetworkActivity

<pre>
//AcquireNetworkActivity acquires a network from the provider (deduped by
//name, retried on transient errors).
</pre>

**Input:** [cloud.v1.workflow.AcquireNetworkActivityRequest](#cloud-v1-workflow-acquirenetworkactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-providersettings">cloud.v1.deployment.ProviderSettings</a></td>
<td><pre>
//settings are the provider-specific settings used to acquire the network.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.AcquireNetworkActivityResponse](#cloud-v1-workflow-acquirenetworkactivityresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>net</td>
<td><a href="../common/README.md#cloud-v1-common-net">cloud.v1.common.Net</a></td>
<td><pre>
//net is the network acquired from the provider.<br>

json_name: net
go_name: Net</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>start_to_close_timeout</td><td>5 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-acquirequotasactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.AcquireQuotasActivity

<pre>
//AcquireQuotasActivity acquires the requested quotas from the provider
//(retried with backoff).
</pre>

**Input:** [cloud.v1.workflow.AcquireQuotasActivityRequest](#cloud-v1-workflow-acquirequotasactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-acquirequotasactivityrequest-quotarequestsentry">cloud.v1.workflow.AcquireQuotasActivityRequest.QuotaRequestsEntry</a></td>
<td><pre>
//quota_requests are the requests to acquire, keyed by component.id.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.AcquireQuotasActivityResponse](#cloud-v1-workflow-acquirequotasactivityresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_allocation</td>
<td><a href="#cloud-v1-workflow-acquirequotasactivityresponse-quotaallocationentry">cloud.v1.workflow.AcquireQuotasActivityResponse.QuotaAllocationEntry</a></td>
<td><pre>
//quota_allocation is the granted allocation keyed by component.id.<br>

json_name: quotaAllocation
go_name: QuotaAllocation</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>start_to_close_timeout</td><td>5 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-dockerdownactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.DockerDownActivity

<pre>
//DockerDownActivity tears the compose stack down (idempotent, retryable).
</pre>

**Input:** [cloud.v1.deployment.Docker.Input](../deployment/README.md#cloud-v1-deployment-docker-input)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-containersentry">cloud.v1.deployment.Docker.Input.ContainersEntry</a></td>
<td><pre>
containers contains runtime container specs keyed by stable name.<br>

json_name: containers
go_name: Containers</pre></td>
</tr><tr>
<td>network</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-network">cloud.v1.deployment.Docker.Network</a></td>
<td><pre>
network describes the Docker network and DNS settings.<br>

json_name: network
go_name: Network</pre></td>
</tr>
</table>

**Output:** [cloud.v1.deployment.Docker.Output](../deployment/README.md#cloud-v1-deployment-docker-output)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-output-containersentry">cloud.v1.deployment.Docker.Output.ContainersEntry</a></td>
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

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>start_to_close_timeout</td><td>5 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-dockerpullactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.DockerPullActivity

<pre>
//DockerPullActivity pulls the required container images (idempotent,
//retried with backoff).
</pre>

**Input:** [cloud.v1.deployment.Docker.Input](../deployment/README.md#cloud-v1-deployment-docker-input)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-containersentry">cloud.v1.deployment.Docker.Input.ContainersEntry</a></td>
<td><pre>
containers contains runtime container specs keyed by stable name.<br>

json_name: containers
go_name: Containers</pre></td>
</tr><tr>
<td>network</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-network">cloud.v1.deployment.Docker.Network</a></td>
<td><pre>
network describes the Docker network and DNS settings.<br>

json_name: network
go_name: Network</pre></td>
</tr>
</table>

**Output:** [cloud.v1.deployment.Docker.Output](../deployment/README.md#cloud-v1-deployment-docker-output)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-output-containersentry">cloud.v1.deployment.Docker.Output.ContainersEntry</a></td>
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

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>start_to_close_timeout</td><td>10 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-dockerupactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.DockerUpActivity

<pre>
//DockerUpActivity brings the compose stack up (idempotent/converges,
//heartbeats while starting).
</pre>

**Input:** [cloud.v1.deployment.Docker.Input](../deployment/README.md#cloud-v1-deployment-docker-input)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-containersentry">cloud.v1.deployment.Docker.Input.ContainersEntry</a></td>
<td><pre>
containers contains runtime container specs keyed by stable name.<br>

json_name: containers
go_name: Containers</pre></td>
</tr><tr>
<td>network</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-network">cloud.v1.deployment.Docker.Network</a></td>
<td><pre>
network describes the Docker network and DNS settings.<br>

json_name: network
go_name: Network</pre></td>
</tr>
</table>

**Output:** [cloud.v1.deployment.Docker.Output](../deployment/README.md#cloud-v1-deployment-docker-output)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>containers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-output-containersentry">cloud.v1.deployment.Docker.Output.ContainersEntry</a></td>
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

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>heartbeat_timeout</td><td>1 minute</td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>schedule_to_close_timeout</td><td>1 minute</td></tr>
<tr><td>start_to_close_timeout</td><td>10 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-terraformapplyactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.TerraformApplyActivity

<pre>
//TerraformApplyActivity runs terraform apply to provision resources
//(mutating; retried sparingly under the state lock).
</pre>

**Input:** [cloud.v1.deployment.Terraform.Input](#cloud-v1-deployment-terraform-input)



**Output:** [cloud.v1.deployment.Terraform.Output](#cloud-v1-deployment-terraform-output)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>heartbeat_timeout</td><td>1 minute</td></tr>
<tr><td>retry_policy.initial_interval</td><td>10 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>2</td></tr>
<tr><td>schedule_to_close_timeout</td><td>1 minute</td></tr>
<tr><td>start_to_close_timeout</td><td>30 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-terraformdestroyactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.TerraformDestroyActivity

<pre>
//TerraformDestroyActivity runs terraform destroy to tear down all
//resources (idempotent/converges, retried to avoid leaks).
</pre>

**Input:** [cloud.v1.deployment.Terraform.Input](#cloud-v1-deployment-terraform-input)



**Output:** [cloud.v1.deployment.Terraform.Output](#cloud-v1-deployment-terraform-output)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>heartbeat_timeout</td><td>1 minute</td></tr>
<tr><td>retry_policy.initial_interval</td><td>10 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>schedule_to_close_timeout</td><td>1 minute</td></tr>
<tr><td>start_to_close_timeout</td><td>30 minutes</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-terraformplanactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.TerraformPlanActivity

<pre>
//TerraformPlanActivity runs terraform plan against the provider (read-only,
//retryable).
</pre>

**Input:** [cloud.v1.deployment.Terraform.Input](#cloud-v1-deployment-terraform-input)



**Output:** [cloud.v1.deployment.Terraform.Output](#cloud-v1-deployment-terraform-output)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>heartbeat_timeout</td><td>1 minute</td></tr>
<tr><td>retry_policy.initial_interval</td><td>10 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>2</td></tr>
<tr><td>schedule_to_close_timeout</td><td>1 minute</td></tr>
<tr><td>start_to_close_timeout</td><td>15 minutes</td></tr>
</table>   

<a name="cloud-v1-workflow-suiteworkflowservice"></a>
## cloud.v1.workflow.SuiteWorkflowService

<pre>
//SuiteWorkflowService runs a child TestWorkflow per test_run, honoring
//max_parallel.
</pre>

<a name="cloud-v1-workflow-suiteworkflowservice-workflows"></a>
### Workflows

---
<a name="suiteworkflow-workflow"></a>
### SuiteWorkflow

<pre>
//SuiteWorkflow fans out a child TestWorkflow per test run in the suite,
//deduplicated by a deterministic id derived from SuiteRun.id.
</pre>

**Input:** [cloud.v1.workflow.SuiteWorkflowRequest](#cloud-v1-workflow-suiteworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>suite_run</td>
<td><a href="../domain/README.md#cloud-v1-domain-suiterun">cloud.v1.domain.SuiteRun</a></td>
<td><pre>
//suite_run is the full description of the suite run to execute.<br>

json_name: suiteRun
go_name: SuiteRun</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.SuiteWorkflowResponse](#cloud-v1-workflow-suiteworkflowresponse)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id</td><td><pre><code>suite-run/${! suite_run.id }</code></pre></td></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
</table>     

<a name="cloud-v1-workflow-testservice"></a>
## cloud.v1.workflow.TestService

<pre>
//TestService orchestrates one full test cycle:
//deploy -> install stroppy -> [install database unless external] -> run workload -> teardown.
//It composes child workflows; topology stays baked, only runtime info from the
//deployment is carried forward.
</pre>

<a name="cloud-v1-workflow-testservice-workflows"></a>
### Workflows

---
<a name="installdatabaseworkflow-workflow"></a>
### InstallDatabaseWorkflow

<pre>
//InstallDatabaseWorkflow brings up / provisions the database (child of
//TestWorkflow; idempotent and retryable).
</pre>

**Input:** [cloud.v1.workflow.InstallDatabaseWorkflowRequest](#cloud-v1-workflow-installdatabaseworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>database</td>
<td><a href="../domain/README.md#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
//database is the database definition to provision.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology the database is brought up within.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.InstallDatabaseWorkflowResponse](#cloud-v1-workflow-installdatabaseworkflowresponse)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>run_timeout</td><td>30 minutes</td></tr>
</table>

---
<a name="installstroppyworkflow-workflow"></a>
### InstallStroppyWorkflow

<pre>
//InstallStroppyWorkflow installs stroppy onto the runner machines (child
//of TestWorkflow; idempotent and retryable).
</pre>

**Input:** [cloud.v1.workflow.InstallStroppyWorkflowRequest](#cloud-v1-workflow-installstroppyworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the (read-only) topology whose runner machines get stroppy.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.InstallStroppyWorkflowResponse](#cloud-v1-workflow-installstroppyworkflowresponse)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>run_timeout</td><td>30 minutes</td></tr>
</table>

---
<a name="runworkloadworkflow-workflow"></a>
### RunWorkloadWorkflow

<pre>
//RunWorkloadWorkflow runs the workload via the agent (child of
//TestWorkflow; unbounded, never retried to avoid double load).
</pre>

**Input:** [cloud.v1.workflow.RunWorkloadWorkflowRequest](#cloud-v1-workflow-runworkloadworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology the workload runs against.<br>

json_name: topology
go_name: Topology</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the workload definition to execute.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.RunWorkloadWorkflowResponse](#cloud-v1-workflow-runworkloadworkflowresponse)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
</table>

---
<a name="testworkflow-workflow"></a>
### TestWorkflow

<pre>
//TestWorkflow runs one full test cycle, deduplicated by a deterministic id
//derived from TestRun.id and never auto-retried as a whole.
</pre>

**Input:** [cloud.v1.workflow.TestWorkflowRequest](#cloud-v1-workflow-testworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>test_run</td>
<td><a href="../domain/README.md#cloud-v1-domain-testrun">cloud.v1.domain.TestRun</a></td>
<td><pre>
//test_run is the full description of the test run to execute.<br>

json_name: testRun
go_name: TestRun</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.TestWorkflowResponse](#cloud-v1-workflow-testworkflowresponse)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id</td><td><pre><code>test-run/${! test_run.id }</code></pre></td></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
</table>     

<a name="cloud-v1-workflow-messages"></a>
## Messages

<a name="cloud-v1-workflow-acquirenetworkactivityrequest"></a>
### cloud.v1.workflow.AcquireNetworkActivityRequest

<pre>
//AcquireNetworkActivityRequest asks the provider to acquire a network.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-providersettings">cloud.v1.deployment.ProviderSettings</a></td>
<td><pre>
//settings are the provider-specific settings used to acquire the network.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-acquirenetworkactivityresponse"></a>
### cloud.v1.workflow.AcquireNetworkActivityResponse

<pre>
//AcquireNetworkActivityResponse returns the acquired network.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>net</td>
<td><a href="../common/README.md#cloud-v1-common-net">cloud.v1.common.Net</a></td>
<td><pre>
//net is the network acquired from the provider.<br>

json_name: net
go_name: Net</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-acquirequotasactivityrequest"></a>
### cloud.v1.workflow.AcquireQuotasActivityRequest

<pre>
//AcquireQuotasActivityRequest asks the provider to acquire the requested
//quotas.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-acquirequotasactivityrequest-quotarequestsentry">cloud.v1.workflow.AcquireQuotasActivityRequest.QuotaRequestsEntry</a></td>
<td><pre>
//quota_requests are the requests to acquire, keyed by component.id.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-acquirequotasactivityrequest-quotarequestsentry"></a>
### cloud.v1.workflow.AcquireQuotasActivityRequest.QuotaRequestsEntry

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
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-request">cloud.v1.deployment.Quota.Request</a></td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-acquirequotasactivityresponse"></a>
### cloud.v1.workflow.AcquireQuotasActivityResponse

<pre>
//AcquireQuotasActivityResponse returns the quotas the provider allocated.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_allocation</td>
<td><a href="#cloud-v1-workflow-acquirequotasactivityresponse-quotaallocationentry">cloud.v1.workflow.AcquireQuotasActivityResponse.QuotaAllocationEntry</a></td>
<td><pre>
//quota_allocation is the granted allocation keyed by component.id.<br>

json_name: quotaAllocation
go_name: QuotaAllocation</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-acquirequotasactivityresponse-quotaallocationentry"></a>
### cloud.v1.workflow.AcquireQuotasActivityResponse.QuotaAllocationEntry

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
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-allocation">cloud.v1.deployment.Quota.Allocation</a></td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-calculatequotasworkflowrequest"></a>
### cloud.v1.workflow.CalculateQuotasWorkflowRequest

<pre>
//CalculateQuotasWorkflowRequest asks the workflow to compute resource quotas
//for a topology.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology to compute quota requests for.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-calculatequotasworkflowresponse"></a>
### cloud.v1.workflow.CalculateQuotasWorkflowResponse

<pre>
//CalculateQuotasWorkflowResponse returns the topology along with the computed
//quota requests.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-calculatequotasworkflowresponse-quotarequestsentry">cloud.v1.workflow.CalculateQuotasWorkflowResponse.QuotaRequestsEntry</a></td>
<td><pre>
//quota_requests are the computed requests keyed by component.id.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the (unchanged) topology the quotas were computed for.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-calculatequotasworkflowresponse-quotarequestsentry"></a>
### cloud.v1.workflow.CalculateQuotasWorkflowResponse.QuotaRequestsEntry

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
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-request">cloud.v1.deployment.Quota.Request</a></td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-installdatabaseworkflowrequest"></a>
### cloud.v1.workflow.InstallDatabaseWorkflowRequest

<pre>
//InstallDatabaseWorkflowRequest asks to bring up / provision the database
//(self-deploy or managed). Our responsibility for every kind except external;
//skipped for external dbs.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>database</td>
<td><a href="../domain/README.md#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
//database is the database definition to provision.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology the database is brought up within.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-installdatabaseworkflowresponse"></a>
### cloud.v1.workflow.InstallDatabaseWorkflowResponse

<pre>
//InstallDatabaseWorkflowResponse is the empty result of installing the database.
</pre>



<a name="cloud-v1-workflow-installstroppyworkflowrequest"></a>
### cloud.v1.workflow.InstallStroppyWorkflowRequest

<pre>
//InstallStroppyWorkflowRequest asks to install stroppy on the runner
//instances. Runner machines are always deployed, so this always runs.
//Topology is read-only here (baked + runtime).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the (read-only) topology whose runner machines get stroppy.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-installstroppyworkflowresponse"></a>
### cloud.v1.workflow.InstallStroppyWorkflowResponse

<pre>
//InstallStroppyWorkflowResponse is the empty result of installing stroppy.
</pre>



<a name="cloud-v1-workflow-processdeploymentworkflowrequest"></a>
### cloud.v1.workflow.ProcessDeploymentWorkflowRequest

<pre>
//ProcessDeploymentWorkflowRequest is the input to the top-level deployment
//workflow: which provider to use and the topology to provision.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the target cloud/provider to deploy onto.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology to provision; instances MUST already carry their
//provider_parms.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-processdeploymentworkflowresponse"></a>
### cloud.v1.workflow.ProcessDeploymentWorkflowResponse

<pre>
//ProcessDeploymentWorkflowResponse is the result of the deployment workflow:
//the provider used and the fully deployed topology.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>deployed_topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//deployed_topology is the full deployed topology with all runtime params
//filled in.<br>

json_name: deployedTopology
go_name: DeployedTopology</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the provider the topology was deployed onto.<br>

json_name: provider
go_name: Provider</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-runworkloadworkflowrequest"></a>
### cloud.v1.workflow.RunWorkloadWorkflowRequest

<pre>
//RunWorkloadWorkflowRequest asks to run the workload via the agent (write
//stroppy config + call stroppy). Results land in metrics, not in the response.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the topology the workload runs against.<br>

json_name: topology
go_name: Topology</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the workload definition to execute.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-runworkloadworkflowresponse"></a>
### cloud.v1.workflow.RunWorkloadWorkflowResponse

<pre>
//RunWorkloadWorkflowResponse is the empty result of a workload run (results go
//to metrics).
</pre>



<a name="cloud-v1-workflow-suiteworkflowrequest"></a>
### cloud.v1.workflow.SuiteWorkflowRequest

<pre>
//SuiteWorkflowRequest is the input to a suite run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>suite_run</td>
<td><a href="../domain/README.md#cloud-v1-domain-suiterun">cloud.v1.domain.SuiteRun</a></td>
<td><pre>
//suite_run is the full description of the suite run to execute.<br>

json_name: suiteRun
go_name: SuiteRun</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-suiteworkflowresponse"></a>
### cloud.v1.workflow.SuiteWorkflowResponse

<pre>
//SuiteWorkflowResponse is the empty result of a completed suite run.
</pre>



<a name="cloud-v1-workflow-testworkflowrequest"></a>
### cloud.v1.workflow.TestWorkflowRequest

<pre>
//TestWorkflowRequest is the input to a single test run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>test_run</td>
<td><a href="../domain/README.md#cloud-v1-domain-testrun">cloud.v1.domain.TestRun</a></td>
<td><pre>
//test_run is the full description of the test run to execute.<br>

json_name: testRun
go_name: TestRun</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-testworkflowresponse"></a>
### cloud.v1.workflow.TestWorkflowResponse

<pre>
//TestWorkflowResponse is the empty result of a completed test run.
</pre>

