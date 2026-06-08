

<a name="cloud-v1-workflow"></a>
# cloud.v1.workflow

## Table of Contents
- [cloud.v1.workflow.AgentCommandService](#cloud-v1-workflow-agentcommandservice)
  - [Activities](#cloud-v1-workflow-agentcommandservice-activities)
    - [cloud.v1.workflow.AgentCommandService.CallCmdActivity](#cloud-v1-workflow-agentcommandservice-callcmdactivity-activity)
    - [cloud.v1.workflow.AgentCommandService.CreateDirActivity](#cloud-v1-workflow-agentcommandservice-creatediractivity-activity)
    - [cloud.v1.workflow.AgentCommandService.CreateTempDirActivity](#cloud-v1-workflow-agentcommandservice-createtempdiractivity-activity)
    - [cloud.v1.workflow.AgentCommandService.EnsureAgentOnlineActivity](#cloud-v1-workflow-agentcommandservice-ensureagentonlineactivity-activity)
    - [cloud.v1.workflow.AgentCommandService.FetchFileActivity](#cloud-v1-workflow-agentcommandservice-fetchfileactivity-activity)
    - [cloud.v1.workflow.AgentCommandService.WriteFileActivity](#cloud-v1-workflow-agentcommandservice-writefileactivity-activity)
- [cloud.v1.workflow.DeploymentService](#cloud-v1-workflow-deploymentservice)
  - [Workflows](#cloud-v1-workflow-deploymentservice-workflows)
    - [CalculateQuotasWorkflow](#calculatequotasworkflow-workflow)
    - [ExecuteDeploymentPlanWorkflow](#executedeploymentplanworkflow-workflow)
    - [ProcessInfrastructureWorkflow](#processinfrastructureworkflow-workflow)
    - [RenderDeploymentPlanWorkflow](#renderdeploymentplanworkflow-workflow)
    - [RenderDockerInputWorkflow](#renderdockerinputworkflow-workflow)
    - [RenderTerraformVariablesWorkflow](#renderterraformvariablesworkflow-workflow)
  - [Activities](#cloud-v1-workflow-deploymentservice-activities)
    - [cloud.v1.workflow.DeploymentService.AcquireNetworkActivity](#cloud-v1-workflow-deploymentservice-acquirenetworkactivity-activity)
    - [cloud.v1.workflow.DeploymentService.AcquireQuotasActivity](#cloud-v1-workflow-deploymentservice-acquirequotasactivity-activity)
    - [cloud.v1.workflow.DeploymentService.CommitNetworkActivity](#cloud-v1-workflow-deploymentservice-commitnetworkactivity-activity)
    - [cloud.v1.workflow.DeploymentService.CommitQuotasActivity](#cloud-v1-workflow-deploymentservice-commitquotasactivity-activity)
    - [cloud.v1.workflow.DeploymentService.DockerDownActivity](#cloud-v1-workflow-deploymentservice-dockerdownactivity-activity)
    - [cloud.v1.workflow.DeploymentService.DockerPullActivity](#cloud-v1-workflow-deploymentservice-dockerpullactivity-activity)
    - [cloud.v1.workflow.DeploymentService.DockerUpActivity](#cloud-v1-workflow-deploymentservice-dockerupactivity-activity)
    - [cloud.v1.workflow.DeploymentService.ReleaseNetworkActivity](#cloud-v1-workflow-deploymentservice-releasenetworkactivity-activity)
    - [cloud.v1.workflow.DeploymentService.ReleaseQuotasActivity](#cloud-v1-workflow-deploymentservice-releasequotasactivity-activity)
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
  - [Queries](#cloud-v1-workflow-testservice-queries)
    - [GetRunState](#getrunstate-query)
  - [Signals](#cloud-v1-workflow-testservice-signals)
    - [UpdateStage](#updatestage-signal)
- Messages
  - [cloud.v1.workflow.AcquireNetworkActivityRequest](#cloud-v1-workflow-acquirenetworkactivityrequest)
  - [cloud.v1.workflow.AcquireNetworkActivityResponse](#cloud-v1-workflow-acquirenetworkactivityresponse)
  - [cloud.v1.workflow.AcquireQuotasActivityRequest](#cloud-v1-workflow-acquirequotasactivityrequest)
  - [cloud.v1.workflow.AcquireQuotasActivityResponse](#cloud-v1-workflow-acquirequotasactivityresponse)
  - [cloud.v1.workflow.AgentBootstrap](#cloud-v1-workflow-agentbootstrap)
  - [cloud.v1.workflow.AgentBootstrap.AgentTaskQueuesEntry](#cloud-v1-workflow-agentbootstrap-agenttaskqueuesentry)
  - [cloud.v1.workflow.AgentBootstrap.AgentTokensEntry](#cloud-v1-workflow-agentbootstrap-agenttokensentry)
  - [cloud.v1.workflow.AgentBootstrap.ExtraEnvEntry](#cloud-v1-workflow-agentbootstrap-extraenventry)
  - [cloud.v1.workflow.CalculateQuotasWorkflowRequest](#cloud-v1-workflow-calculatequotasworkflowrequest)
  - [cloud.v1.workflow.CalculateQuotasWorkflowResponse](#cloud-v1-workflow-calculatequotasworkflowresponse)
  - [cloud.v1.workflow.CommitNetworkActivityRequest](#cloud-v1-workflow-commitnetworkactivityrequest)
  - [cloud.v1.workflow.CommitNetworkActivityResponse](#cloud-v1-workflow-commitnetworkactivityresponse)
  - [cloud.v1.workflow.CommitQuotasActivityRequest](#cloud-v1-workflow-commitquotasactivityrequest)
  - [cloud.v1.workflow.CommitQuotasActivityResponse](#cloud-v1-workflow-commitquotasactivityresponse)
  - [cloud.v1.workflow.ExecuteDeploymentPlanWorkflowRequest](#cloud-v1-workflow-executedeploymentplanworkflowrequest)
  - [cloud.v1.workflow.ExecuteDeploymentPlanWorkflowResponse](#cloud-v1-workflow-executedeploymentplanworkflowresponse)
  - [cloud.v1.workflow.InstallDatabaseWorkflowRequest](#cloud-v1-workflow-installdatabaseworkflowrequest)
  - [cloud.v1.workflow.InstallDatabaseWorkflowResponse](#cloud-v1-workflow-installdatabaseworkflowresponse)
  - [cloud.v1.workflow.InstallStroppyWorkflowRequest](#cloud-v1-workflow-installstroppyworkflowrequest)
  - [cloud.v1.workflow.InstallStroppyWorkflowResponse](#cloud-v1-workflow-installstroppyworkflowresponse)
  - [cloud.v1.workflow.ProcessInfrastructureWorkflowRequest](#cloud-v1-workflow-processinfrastructureworkflowrequest)
  - [cloud.v1.workflow.ProcessInfrastructureWorkflowResponse](#cloud-v1-workflow-processinfrastructureworkflowresponse)
  - [cloud.v1.workflow.QuotaAllocationRef](#cloud-v1-workflow-quotaallocationref)
  - [cloud.v1.workflow.QuotaRequestRef](#cloud-v1-workflow-quotarequestref)
  - [cloud.v1.workflow.ReleaseNetworkActivityRequest](#cloud-v1-workflow-releasenetworkactivityrequest)
  - [cloud.v1.workflow.ReleaseNetworkActivityResponse](#cloud-v1-workflow-releasenetworkactivityresponse)
  - [cloud.v1.workflow.ReleaseQuotasActivityRequest](#cloud-v1-workflow-releasequotasactivityrequest)
  - [cloud.v1.workflow.ReleaseQuotasActivityResponse](#cloud-v1-workflow-releasequotasactivityresponse)
  - [cloud.v1.workflow.RenderDeploymentPlanWorkflowRequest](#cloud-v1-workflow-renderdeploymentplanworkflowrequest)
  - [cloud.v1.workflow.RenderDeploymentPlanWorkflowResponse](#cloud-v1-workflow-renderdeploymentplanworkflowresponse)
  - [cloud.v1.workflow.RenderDockerInputWorkflowRequest](#cloud-v1-workflow-renderdockerinputworkflowrequest)
  - [cloud.v1.workflow.RenderTerraformVariablesWorkflowRequest](#cloud-v1-workflow-renderterraformvariablesworkflowrequest)
  - [cloud.v1.workflow.RunConfig](#cloud-v1-workflow-runconfig)
  - [cloud.v1.workflow.RunState](#cloud-v1-workflow-runstate)
  - [cloud.v1.workflow.RunWorkloadWorkflowRequest](#cloud-v1-workflow-runworkloadworkflowrequest)
  - [cloud.v1.workflow.RunWorkloadWorkflowResponse](#cloud-v1-workflow-runworkloadworkflowresponse)
  - [cloud.v1.workflow.Stage](#cloud-v1-workflow-stage)
  - [cloud.v1.workflow.StageUpdate](#cloud-v1-workflow-stageupdate)
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
<a name="cloud-v1-workflow-agentcommandservice-fetchfileactivity-activity"></a>
### cloud.v1.workflow.AgentCommandService.FetchFileActivity

<pre>
//FetchFileActivity downloads a file/binary by reference (URL / S3 minio) to
//the agent host at File.info.path and caches it by File.AsRef.checksum (e.g.
//the stroppy binary, packages). Idempotent given the checksum => retryable.
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
<tr><td>heartbeat_timeout</td><td>1 minute</td></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>5 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>3</td></tr>
<tr><td>schedule_to_close_timeout</td><td>1 minute</td></tr>
<tr><td>start_to_close_timeout</td><td>10 minutes</td></tr>
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
//DeploymentService groups the staged deployment workflows and provider
//activities.
</pre>

<a name="cloud-v1-workflow-deploymentservice-workflows"></a>
### Workflows

---
<a name="calculatequotasworkflow-workflow"></a>
### CalculateQuotasWorkflow

<pre>
//CalculateQuotasWorkflow computes quota requests from an infrastructure
//plan.
</pre>

**Input:** [cloud.v1.workflow.CalculateQuotasWorkflowRequest](#cloud-v1-workflow-calculatequotasworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the infrastructure plan to compute quotas for.<br>

json_name: plan
go_name: Plan</pre></td>
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
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the plan the quotas were computed for.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-quotarequestref">cloud.v1.workflow.QuotaRequestRef</a></td>
<td><pre>
//quota_requests are requested quotas with topology node ids.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
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
<a name="executedeploymentplanworkflow-workflow"></a>
### ExecuteDeploymentPlanWorkflow

<pre>
//ExecuteDeploymentPlanWorkflow executes rendered agent steps.
</pre>

**Input:** [cloud.v1.workflow.ExecuteDeploymentPlanWorkflowRequest](#cloud-v1-workflow-executedeploymentplanworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap carries the per-node task queue map used to route agent
//activities to the queue rendered into each node's bootstrap.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan is the plan to execute.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is used to route steps to node agents.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the persisted test run id. It lets the child workflow persist
//live deployment-plan action statuses and stamp log correlation labels.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.ExecuteDeploymentPlanWorkflowResponse](#cloud-v1-workflow-executedeploymentplanworkflowresponse)

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
//deployment_plan is the executed plan with statuses/results filled.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
</table>

---
<a name="processinfrastructureworkflow-workflow"></a>
### ProcessInfrastructureWorkflow

<pre>
//ProcessInfrastructureWorkflow provisions provider infrastructure.
</pre>

**Input:** [cloud.v1.workflow.ProcessInfrastructureWorkflowRequest](#cloud-v1-workflow-processinfrastructureworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is runtime control-plane data delivered to every
//provisioned agent. Providers only choose the carrier: Docker env file,
//cloud-init user-data, or local process env.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the provider-specific infrastructure plan to materialize.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the stable deployment/run identifier used for provider
//resource names and Terraform workdir recovery.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.ProcessInfrastructureWorkflowResponse](#cloud-v1-workflow-processinfrastructureworkflowresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//state is the runtime provider state after apply/up.<br>

json_name: state
go_name: State</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
</table>

---
<a name="renderdeploymentplanworkflow-workflow"></a>
### RenderDeploymentPlanWorkflow

<pre>
//RenderDeploymentPlanWorkflow renders package/config/agent steps.
</pre>

**Input:** [cloud.v1.workflow.RenderDeploymentPlanWorkflowRequest](#cloud-v1-workflow-renderdeploymentplanworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap carries per-node agent tokens for rendering monitor and
//workload OTLP bearer credentials without leaking them through topology
//labels.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>database</td>
<td><a href="../domain/README.md#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
//database is the engine input used by package resolution and config
//renderers. It is not topology because renderers need version/package and
//editable engine config values.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>infrastructure_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//infrastructure_plan is provider input, including OS/image choices.<br>

json_name: infrastructurePlan
go_name: InfrastructurePlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state carries runtime facts such as addresses. It may be
//partially filled when a renderer can use logical hostnames instead.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>render_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderoverrideset">cloud.v1.deployment.RenderOverrideSet</a></td>
<td><pre>
//render_overrides are user edits to editable render artifacts.<br>

json_name: renderOverrides
go_name: RenderOverrides</pre></td>
</tr><tr>
<td>topology_spec</td>
<td><a href="../topology/README.md#cloud-v1-topology-topologyspec">cloud.v1.topology.TopologySpec</a></td>
<td><pre>
//topology_spec is the provider-agnostic logical graph.<br>

json_name: topologySpec
go_name: TopologySpec</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the stroppy workload input used by workload-runner renderers.
//It is optional for pure database render previews and required when the
//topology contains a stroppy workload component.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.RenderDeploymentPlanWorkflowResponse](#cloud-v1-workflow-renderdeploymentplanworkflowresponse)

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
//deployment_plan is the agent-executable plan.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
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
<a name="renderdockerinputworkflow-workflow"></a>
### RenderDockerInputWorkflow

<pre>
//RenderDockerInputWorkflow renders infrastructure plan to Docker input.
</pre>

**Input:** [cloud.v1.workflow.RenderDockerInputWorkflowRequest](#cloud-v1-workflow-renderdockerinputworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is rendered into the Docker agent env file.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the Docker infrastructure plan.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is used as the default Docker network suffix when the plan does
//not pin a network name.<br>

json_name: runId
go_name: RunId</pre></td>
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
<td>log_context</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-logcontext">cloud.v1.deployment.Docker.Input.LogContext</a></td>
<td><pre>
log_context scopes Docker stderr/progress into run logs.<br>

json_name: logContext
go_name: LogContext</pre></td>
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
//RenderTerraformVariablesWorkflow renders infrastructure plan to Terraform
//input.
</pre>

**Input:** [cloud.v1.workflow.RenderTerraformVariablesWorkflowRequest](#cloud-v1-workflow-renderterraformvariablesworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>action</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-terraform-action">cloud.v1.deployment.Terraform.Action</a></td>
<td><pre>
//action selects plan/apply/destroy. Empty means apply.<br>

json_name: action
go_name: Action</pre></td>
</tr><tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is rendered into Yandex cloud-init user-data.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the Terraform-backed infrastructure plan.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the stable Terraform workdir id.<br>

json_name: runId
go_name: RunId</pre></td>
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
//AcquireNetworkActivity acquires a provider network.
</pre>

**Input:** [cloud.v1.workflow.AcquireNetworkActivityRequest](#cloud-v1-workflow-acquirenetworkactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the infrastructure plan whose provider settings drive network
//acquisition.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
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
<td>network_cidr</td>
<td>string</td>
<td><pre>
//network_cidr is the run CIDR reserved for provider subnets.<br>

json_name: networkCidr
go_name: NetworkCidr</pre></td>
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
//AcquireQuotasActivity acquires requested quotas.
</pre>

**Input:** [cloud.v1.workflow.AcquireQuotasActivityRequest](#cloud-v1-workflow-acquirequotasactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-quotarequestref">cloud.v1.workflow.QuotaRequestRef</a></td>
<td><pre>
//quota_requests are requested quotas with topology node ids.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
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
<td>quota_allocations</td>
<td><a href="#cloud-v1-workflow-quotaallocationref">cloud.v1.workflow.QuotaAllocationRef</a></td>
<td><pre>
//quota_allocations are granted allocations with topology node ids.<br>

json_name: quotaAllocations
go_name: QuotaAllocations</pre></td>
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
<a name="cloud-v1-workflow-deploymentservice-commitnetworkactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.CommitNetworkActivity

<pre>
//CommitNetworkActivity marks a successful network reservation as allocated.
</pre>

**Input:** [cloud.v1.workflow.CommitNetworkActivityRequest](#cloud-v1-workflow-commitnetworkactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.CommitNetworkActivityResponse](#cloud-v1-workflow-commitnetworkactivityresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>network_cidr</td>
<td>string</td>
<td><pre>
json_name: networkCidr
go_name: NetworkCidr</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>2 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>5</td></tr>
<tr><td>start_to_close_timeout</td><td>1 minute</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-commitquotasactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.CommitQuotasActivity

<pre>
//CommitQuotasActivity marks a successful reservation as allocated.
</pre>

**Input:** [cloud.v1.workflow.CommitQuotasActivityRequest](#cloud-v1-workflow-commitquotasactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.CommitQuotasActivityResponse](#cloud-v1-workflow-commitquotasactivityresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_allocations</td>
<td><a href="#cloud-v1-workflow-quotaallocationref">cloud.v1.workflow.QuotaAllocationRef</a></td>
<td><pre>
json_name: quotaAllocations
go_name: QuotaAllocations</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>2 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>5</td></tr>
<tr><td>start_to_close_timeout</td><td>1 minute</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-dockerdownactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.DockerDownActivity

<pre>
//DockerDownActivity tears down the Docker topology.
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
<td>log_context</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-logcontext">cloud.v1.deployment.Docker.Input.LogContext</a></td>
<td><pre>
log_context scopes Docker stderr/progress into run logs.<br>

json_name: logContext
go_name: LogContext</pre></td>
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
//DockerPullActivity pulls container images.
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
<td>log_context</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-logcontext">cloud.v1.deployment.Docker.Input.LogContext</a></td>
<td><pre>
log_context scopes Docker stderr/progress into run logs.<br>

json_name: logContext
go_name: LogContext</pre></td>
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
//DockerUpActivity starts the Docker topology.
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
<td>log_context</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-docker-input-logcontext">cloud.v1.deployment.Docker.Input.LogContext</a></td>
<td><pre>
log_context scopes Docker stderr/progress into run logs.<br>

json_name: logContext
go_name: LogContext</pre></td>
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
<a name="cloud-v1-workflow-deploymentservice-releasenetworkactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.ReleaseNetworkActivity

<pre>
//ReleaseNetworkActivity releases a run's pre-deploy network reservations.
</pre>

**Input:** [cloud.v1.workflow.ReleaseNetworkActivityRequest](#cloud-v1-workflow-releasenetworkactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.ReleaseNetworkActivityResponse](#cloud-v1-workflow-releasenetworkactivityresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>released</td>
<td>uint32</td>
<td><pre>
json_name: released
go_name: Released</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>2 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>5</td></tr>
<tr><td>start_to_close_timeout</td><td>1 minute</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-releasequotasactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.ReleaseQuotasActivity

<pre>
//ReleaseQuotasActivity releases a run's pre-deploy reservations.
</pre>

**Input:** [cloud.v1.workflow.ReleaseQuotasActivityRequest](#cloud-v1-workflow-releasequotasactivityrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.ReleaseQuotasActivityResponse](#cloud-v1-workflow-releasequotasactivityresponse)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>released</td>
<td>uint32</td>
<td><pre>
json_name: released
go_name: Released</pre></td>
</tr>
</table>

**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>retry_policy.backoff_coefficient</td><td>2</td></tr>
<tr><td>retry_policy.initial_interval</td><td>2 seconds</td></tr>
<tr><td>retry_policy.max_attempts</td><td>5</td></tr>
<tr><td>start_to_close_timeout</td><td>1 minute</td></tr>
</table> 

---
<a name="cloud-v1-workflow-deploymentservice-terraformapplyactivity-activity"></a>
### cloud.v1.workflow.DeploymentService.TerraformApplyActivity

<pre>
//TerraformApplyActivity runs terraform apply.
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
//TerraformDestroyActivity runs terraform destroy.
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
//TerraformPlanActivity runs terraform plan.
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
//SuiteWorkflowService runs a child TestWorkflow per RunConfig, honoring
//max_parallel.
</pre>

<a name="cloud-v1-workflow-suiteworkflowservice-workflows"></a>
### Workflows

---
<a name="suiteworkflow-workflow"></a>
### SuiteWorkflow

<pre>
//SuiteWorkflow fans out a child TestWorkflow per run in the suite,
//deduplicated by a deterministic id derived from suite_run_id.
</pre>

**Input:** [cloud.v1.workflow.SuiteWorkflowRequest](#cloud-v1-workflow-suiteworkflowrequest)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>max_parallel</td>
<td>uint32</td>
<td><pre>
//max_parallel caps concurrent child TestWorkflow executions. 0 =
//unlimited.<br>

json_name: maxParallel
go_name: MaxParallel</pre></td>
</tr><tr>
<td>runs</td>
<td><a href="#cloud-v1-workflow-runconfig">cloud.v1.workflow.RunConfig</a></td>
<td><pre>
//runs are the child run workflow inputs.<br>

json_name: runs
go_name: Runs</pre></td>
</tr><tr>
<td>suite_run_id</td>
<td>string</td>
<td><pre>
//suite_run_id is the persisted SuiteRunRecord id.<br>

json_name: suiteRunId
go_name: SuiteRunId</pre></td>
</tr>
</table>

**Output:** [cloud.v1.workflow.SuiteWorkflowResponse](#cloud-v1-workflow-suiteworkflowresponse)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id</td><td><pre><code>suite-run/${! suiteRunId }</code></pre></td></tr>
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
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan contains the database-related agent steps.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is used to route steps to node agents.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
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
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan contains the stroppy-related agent steps.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is used to route steps to node agents.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
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
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap carries the per-node Temporal task queues used to reach
//the workload runner agent.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan is the materialized plan containing the workload-runner
//component and its rendered config path.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state carries runtime machine state for route validation.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the stable run identifier used for stage/log correlation.<br>

json_name: runId
go_name: RunId</pre></td>
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
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is runtime control-plane data delivered to provisioned
//agents through the provider-specific carrier.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes quota reservations and provider settings lookup.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
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
<tr><td>id</td><td><pre><code>test-run/${! testRun.id }</code></pre></td></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
</table>

**Queries:**

<table>
<tr><th>Query</th></tr>
<tr><td><a href="#cloud-v1-workflow-testservice-getrunstate-query">cloud.v1.workflow.TestService.GetRunState</a></td></tr>
</table>

**Signals:**

<table>
<tr><th>Signal</th><th>Start</th></tr>
<tr><td><a href="#cloud-v1-workflow-testservice-updatestage-signal">cloud.v1.workflow.TestService.UpdateStage</a></td><td>false</td></tr>
</table>  

<a name="cloud-v1-workflow-testservice-queries"></a>
### Queries

---
<a name="getrunstate-query"></a>
### GetRunState

<pre>
//GetRunState is a Temporal query against a running TestWorkflow returning
//the live RunState (overall status + per-stage breakdown) for the run
//Overview. Read-only; takes no input.
</pre>

**Output:** [cloud.v1.workflow.RunState](#cloud-v1-workflow-runstate)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>stages</td>
<td><a href="#cloud-v1-workflow-stage">cloud.v1.workflow.Stage</a></td>
<td><pre>
//stages is the per-stage breakdown of the run.<br>

json_name: stages
go_name: Stages</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the overall status of the run.<br>

json_name: status
go_name: Status</pre></td>
</tr>
</table>  

<a name="cloud-v1-workflow-testservice-signals"></a>
### Signals

---
<a name="updatestage-signal"></a>
### UpdateStage

<pre>
//UpdateStage is a Temporal signal used by child workflows to update one
//concrete runtime stage inside the parent TestWorkflow RunState.
</pre>

**Input:** [cloud.v1.workflow.StageUpdate](#cloud-v1-workflow-stageupdate)

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>stage</td>
<td><a href="#cloud-v1-workflow-stage">cloud.v1.workflow.Stage</a></td>
<td><pre>
//stage is the complete current snapshot for one runtime stage.<br>

json_name: stage
go_name: Stage</pre></td>
</tr>
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
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the infrastructure plan whose provider settings drive network
//acquisition.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
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
<td>network_cidr</td>
<td>string</td>
<td><pre>
//network_cidr is the run CIDR reserved for provider subnets.<br>

json_name: networkCidr
go_name: NetworkCidr</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-acquirequotasactivityrequest"></a>
### cloud.v1.workflow.AcquireQuotasActivityRequest

<pre>
//AcquireQuotasActivityRequest asks the provider to acquire requested quotas.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-quotarequestref">cloud.v1.workflow.QuotaRequestRef</a></td>
<td><pre>
//quota_requests are requested quotas with topology node ids.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-acquirequotasactivityresponse"></a>
### cloud.v1.workflow.AcquireQuotasActivityResponse

<pre>
//AcquireQuotasActivityResponse returns granted allocations with node ids.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_allocations</td>
<td><a href="#cloud-v1-workflow-quotaallocationref">cloud.v1.workflow.QuotaAllocationRef</a></td>
<td><pre>
//quota_allocations are granted allocations with topology node ids.<br>

json_name: quotaAllocations
go_name: QuotaAllocations</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-agentbootstrap"></a>
### cloud.v1.workflow.AgentBootstrap

<pre>
//AgentBootstrap is the provider-independent startup contract for every agent.
//It is runtime control-plane data, not topology and not provider settings.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_task_queues</td>
<td><a href="#cloud-v1-workflow-agentbootstrap-agenttaskqueuesentry">cloud.v1.workflow.AgentBootstrap.AgentTaskQueuesEntry</a></td>
<td><pre>
//agent_task_queues carries the per-node Temporal task queue name. It is
//generated with a per-run secret suffix and rendered only to the matching
//node so a valid agent token alone is not enough to poll another node's
//work.<br>

json_name: agentTaskQueues
go_name: AgentTaskQueues</pre></td>
</tr><tr>
<td>agent_tokens</td>
<td><a href="#cloud-v1-workflow-agentbootstrap-agenttokensentry">cloud.v1.workflow.AgentBootstrap.AgentTokensEntry</a></td>
<td><pre>
//agent_tokens carries per-node bearer tokens. The renderer injects only
//the token matching the current node into that node's env as
//STROPPY_AGENT_TOKEN; it must not be rendered as generic extra env.<br>

json_name: agentTokens
go_name: AgentTokens</pre></td>
</tr><tr>
<td>binary_url</td>
<td>string</td>
<td><pre>
//binary_url overrides the agent binary URL. Empty means
//server_addr + "/agent/binary".<br>

json_name: binaryUrl
go_name: BinaryUrl</pre></td>
</tr><tr>
<td>extra_env</td>
<td><a href="#cloud-v1-workflow-agentbootstrap-extraenventry">cloud.v1.workflow.AgentBootstrap.ExtraEnvEntry</a></td>
<td><pre>
//extra_env is appended to the agent env file. Core STROPPY and AGENT
//fields are still rendered by the system and win over this map.<br>

json_name: extraEnv
go_name: ExtraEnv</pre></td>
</tr><tr>
<td>server_addr</td>
<td>string</td>
<td><pre>
//server_addr is the public/base control-plane address agents use to reach
//the server and Temporal proxy, e.g. http://10.0.0.10:8080.<br>

json_name: serverAddr
go_name: ServerAddr</pre></td>
</tr><tr>
<td>temporal_namespace</td>
<td>string</td>
<td><pre>
//temporal_namespace is the namespace agents use when registering their
//Temporal worker. Empty means default.<br>

json_name: temporalNamespace
go_name: TemporalNamespace</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-agentbootstrap-agenttaskqueuesentry"></a>
### cloud.v1.workflow.AgentBootstrap.AgentTaskQueuesEntry

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



<a name="cloud-v1-workflow-agentbootstrap-agenttokensentry"></a>
### cloud.v1.workflow.AgentBootstrap.AgentTokensEntry

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



<a name="cloud-v1-workflow-agentbootstrap-extraenventry"></a>
### cloud.v1.workflow.AgentBootstrap.ExtraEnvEntry

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



<a name="cloud-v1-workflow-calculatequotasworkflowrequest"></a>
### cloud.v1.workflow.CalculateQuotasWorkflowRequest

<pre>
//CalculateQuotasWorkflowRequest asks to compute quota requests for a plan.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the infrastructure plan to compute quotas for.<br>

json_name: plan
go_name: Plan</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-calculatequotasworkflowresponse"></a>
### cloud.v1.workflow.CalculateQuotasWorkflowResponse

<pre>
//CalculateQuotasWorkflowResponse returns all quota requests with node ids.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the plan the quotas were computed for.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>quota_requests</td>
<td><a href="#cloud-v1-workflow-quotarequestref">cloud.v1.workflow.QuotaRequestRef</a></td>
<td><pre>
//quota_requests are requested quotas with topology node ids.<br>

json_name: quotaRequests
go_name: QuotaRequests</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-commitnetworkactivityrequest"></a>
### cloud.v1.workflow.CommitNetworkActivityRequest

<pre>
//CommitNetworkActivityRequest marks a successful network reservation as
//provider-backed after infrastructure was provisioned.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-commitnetworkactivityresponse"></a>
### cloud.v1.workflow.CommitNetworkActivityResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>network_cidr</td>
<td>string</td>
<td><pre>
json_name: networkCidr
go_name: NetworkCidr</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-commitquotasactivityrequest"></a>
### cloud.v1.workflow.CommitQuotasActivityRequest

<pre>
//CommitQuotasActivityRequest marks a run's reserved quotas as provider-backed
//allocations after infrastructure was successfully provisioned.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-commitquotasactivityresponse"></a>
### cloud.v1.workflow.CommitQuotasActivityResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quota_allocations</td>
<td><a href="#cloud-v1-workflow-quotaallocationref">cloud.v1.workflow.QuotaAllocationRef</a></td>
<td><pre>
json_name: quotaAllocations
go_name: QuotaAllocations</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-executedeploymentplanworkflowrequest"></a>
### cloud.v1.workflow.ExecuteDeploymentPlanWorkflowRequest

<pre>
//ExecuteDeploymentPlanWorkflowRequest asks to execute an agent plan.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap carries the per-node task queue map used to route agent
//activities to the queue rendered into each node's bootstrap.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan is the plan to execute.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is used to route steps to node agents.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the persisted test run id. It lets the child workflow persist
//live deployment-plan action statuses and stamp log correlation labels.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-executedeploymentplanworkflowresponse"></a>
### cloud.v1.workflow.ExecuteDeploymentPlanWorkflowResponse

<pre>
//ExecuteDeploymentPlanWorkflowResponse returns the executed plan with statuses.
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
//deployment_plan is the executed plan with statuses/results filled.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
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
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan contains the database-related agent steps.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is used to route steps to node agents.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
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
//InstallStroppyWorkflowRequest asks to execute stroppy installation steps on
//runner nodes.
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
//deployment_plan contains the stroppy-related agent steps.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is used to route steps to node agents.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-installstroppyworkflowresponse"></a>
### cloud.v1.workflow.InstallStroppyWorkflowResponse

<pre>
//InstallStroppyWorkflowResponse is the empty result of installing stroppy.
</pre>



<a name="cloud-v1-workflow-processinfrastructureworkflowrequest"></a>
### cloud.v1.workflow.ProcessInfrastructureWorkflowRequest

<pre>
//ProcessInfrastructureWorkflowRequest is the input to provider provisioning.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is runtime control-plane data delivered to every
//provisioned agent. Providers only choose the carrier: Docker env file,
//cloud-init user-data, or local process env.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the provider-specific infrastructure plan to materialize.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the stable deployment/run identifier used for provider
//resource names and Terraform workdir recovery.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-processinfrastructureworkflowresponse"></a>
### cloud.v1.workflow.ProcessInfrastructureWorkflowResponse

<pre>
//ProcessInfrastructureWorkflowResponse is provider runtime output.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//state is the runtime provider state after apply/up.<br>

json_name: state
go_name: State</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-quotaallocationref"></a>
### cloud.v1.workflow.QuotaAllocationRef

<pre>
//QuotaAllocationRef keeps the topology node id together with one granted
//quota allocation.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>allocation</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-allocation">cloud.v1.deployment.Quota.Allocation</a></td>
<td><pre>
json_name: allocation
go_name: Allocation</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
json_name: nodeId
go_name: NodeId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-quotarequestref"></a>
### cloud.v1.workflow.QuotaRequestRef

<pre>
//QuotaRequestRef keeps the topology node id together with one quota request.
//One node usually asks for several quotas (instances, CPU, memory, disk), so
//this must be repeated rather than map<node_id, request>.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>node_id</td>
<td>string</td>
<td><pre>
json_name: nodeId
go_name: NodeId</pre></td>
</tr><tr>
<td>request</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-request">cloud.v1.deployment.Quota.Request</a></td>
<td><pre>
json_name: request
go_name: Request</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-releasenetworkactivityrequest"></a>
### cloud.v1.workflow.ReleaseNetworkActivityRequest

<pre>
//ReleaseNetworkActivityRequest releases pre-deploy network reservations.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-releasenetworkactivityresponse"></a>
### cloud.v1.workflow.ReleaseNetworkActivityResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>released</td>
<td>uint32</td>
<td><pre>
json_name: released
go_name: Released</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-releasequotasactivityrequest"></a>
### cloud.v1.workflow.ReleaseQuotasActivityRequest

<pre>
//ReleaseQuotasActivityRequest releases pre-deploy reservations for a run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-releasequotasactivityresponse"></a>
### cloud.v1.workflow.ReleaseQuotasActivityResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>released</td>
<td>uint32</td>
<td><pre>
json_name: released
go_name: Released</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-renderdeploymentplanworkflowrequest"></a>
### cloud.v1.workflow.RenderDeploymentPlanWorkflowRequest

<pre>
//RenderDeploymentPlanWorkflowRequest asks to render install/config agent steps.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap carries per-node agent tokens for rendering monitor and
//workload OTLP bearer credentials without leaking them through topology
//labels.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>database</td>
<td><a href="../domain/README.md#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
//database is the engine input used by package resolution and config
//renderers. It is not topology because renderers need version/package and
//editable engine config values.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>infrastructure_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//infrastructure_plan is provider input, including OS/image choices.<br>

json_name: infrastructurePlan
go_name: InfrastructurePlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state carries runtime facts such as addresses. It may be
//partially filled when a renderer can use logical hostnames instead.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>render_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderoverrideset">cloud.v1.deployment.RenderOverrideSet</a></td>
<td><pre>
//render_overrides are user edits to editable render artifacts.<br>

json_name: renderOverrides
go_name: RenderOverrides</pre></td>
</tr><tr>
<td>topology_spec</td>
<td><a href="../topology/README.md#cloud-v1-topology-topologyspec">cloud.v1.topology.TopologySpec</a></td>
<td><pre>
//topology_spec is the provider-agnostic logical graph.<br>

json_name: topologySpec
go_name: TopologySpec</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the stroppy workload input used by workload-runner renderers.
//It is optional for pure database render previews and required when the
//topology contains a stroppy workload component.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-renderdeploymentplanworkflowresponse"></a>
### cloud.v1.workflow.RenderDeploymentPlanWorkflowResponse

<pre>
//RenderDeploymentPlanWorkflowResponse returns an executable deployment plan.
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
//deployment_plan is the agent-executable plan.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-renderdockerinputworkflowrequest"></a>
### cloud.v1.workflow.RenderDockerInputWorkflowRequest

<pre>
//RenderDockerInputWorkflowRequest asks to render Docker daemon input from an
//infrastructure plan.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is rendered into the Docker agent env file.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the Docker infrastructure plan.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is used as the default Docker network suffix when the plan does
//not pin a network name.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-renderterraformvariablesworkflowrequest"></a>
### cloud.v1.workflow.RenderTerraformVariablesWorkflowRequest

<pre>
//RenderTerraformVariablesWorkflowRequest asks to render Terraform operation
//input from an infrastructure plan.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>action</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-terraform-action">cloud.v1.deployment.Terraform.Action</a></td>
<td><pre>
//action selects plan/apply/destroy. Empty means apply.<br>

json_name: action
go_name: Action</pre></td>
</tr><tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is rendered into Yandex cloud-init user-data.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//plan is the Terraform-backed infrastructure plan.<br>

json_name: plan
go_name: Plan</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the stable Terraform workdir id.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-runconfig"></a>
### cloud.v1.workflow.RunConfig

<pre>
//RunConfig is the durable baked input for one benchmark run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is runtime control-plane data delivered to provisioned
//agents through the provider-specific carrier.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>database</td>
<td><a href="../domain/README.md#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
//database is the database under test.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan is filled after package/config rendering. It may be empty
//at workflow start and carried forward by the workflow.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable run identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>infrastructure_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//infrastructure_plan is the provider-specific machine/resource intent.<br>

json_name: infrastructurePlan
go_name: InfrastructurePlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is filled after provider provisioning. It may be
//empty at workflow start and carried forward by the workflow.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>render_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderoverrideset">cloud.v1.deployment.RenderOverrideSet</a></td>
<td><pre>
//render_overrides are user edits to editable render artifacts. Workflow
//renderers apply them when producing deployment_plan.<br>

json_name: renderOverrides
go_name: RenderOverrides</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes quota reservations and provider settings lookup.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>topology_spec</td>
<td><a href="../topology/README.md#cloud-v1-topology-topologyspec">cloud.v1.topology.TopologySpec</a></td>
<td><pre>
//topology_spec is the provider-agnostic logical graph.<br>

json_name: topologySpec
go_name: TopologySpec</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the workload to run against the database.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-runstate"></a>
### cloud.v1.workflow.RunState

<pre>
//RunState is the live overview of a test run, returned by the GetRunState
//Temporal query against a running TestWorkflow.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>stages</td>
<td><a href="#cloud-v1-workflow-stage">cloud.v1.workflow.Stage</a></td>
<td><pre>
//stages is the per-stage breakdown of the run.<br>

json_name: stages
go_name: Stages</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the overall status of the run.<br>

json_name: status
go_name: Status</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-runworkloadworkflowrequest"></a>
### cloud.v1.workflow.RunWorkloadWorkflowRequest

<pre>
//RunWorkloadWorkflowRequest asks to run the already-rendered workload via
//the agent. Deployment rendering writes stroppy-config.json and monitor
//collectors first; this workflow executes the real stroppy load and relies on
//the OTLP exporter in that config for metrics.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap carries the per-node Temporal task queues used to reach
//the workload runner agent.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan is the materialized plan containing the workload-runner
//component and its rendered config path.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state carries runtime machine state for route validation.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the stable run identifier used for stage/log correlation.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-runworkloadworkflowresponse"></a>
### cloud.v1.workflow.RunWorkloadWorkflowResponse

<pre>
//RunWorkloadWorkflowResponse is the empty result of a workload run (results go
//to metrics).
</pre>



<a name="cloud-v1-workflow-stage"></a>
### cloud.v1.workflow.Stage

<pre>
//Stage is one step of a test run (a child workflow / activity execution) as
//surfaced in the run Overview.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>attempt</td>
<td>uint32</td>
<td><pre>
//attempt is the current attempt number for this stage.<br>

json_name: attempt
go_name: Attempt</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id is the deployment/topology component this stage acts on.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>error_message</td>
<td>string</td>
<td><pre>
//error_message is the user-facing error text for failed stages.<br>

json_name: errorMessage
go_name: ErrorMessage</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//finished_at is when the stage finished (unset while running).<br>

json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
//machine_id is the target machine/agent id for agent-side stages.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the human-readable stage name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>node_execution_id</td>
<td>string</td>
<td><pre>
//node_execution_id identifies the underlying node execution.<br>

json_name: nodeExecutionId
go_name: NodeExecutionId</pre></td>
</tr><tr>
<td>operation</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-pipelineoperation">cloud.v1.monitor.PipelineOperation</a></td>
<td><pre>
//operation is the executable operation payload for agent deployment stages.<br>

json_name: operation
go_name: Operation</pre></td>
</tr><tr>
<td>order</td>
<td>uint32</td>
<td><pre>
//order is the stable 1-based sibling execution/display order.<br>

json_name: order
go_name: Order</pre></td>
</tr><tr>
<td>outputs</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-pipelineoutput">cloud.v1.monitor.PipelineOutput</a></td>
<td><pre>
//outputs are structured artifacts/results produced by this stage. They are
//carried in RunState so the Overview projection can render stage details
//without reverse-engineering stored deployment plans.<br>

json_name: outputs
go_name: Outputs</pre></td>
</tr><tr>
<td>parent_node_execution_id</td>
<td>string</td>
<td><pre>
//parent_node_execution_id links nested stages to their parent stage.<br>

json_name: parentNodeExecutionId
go_name: ParentNodeExecutionId</pre></td>
</tr><tr>
<td>phase</td>
<td>string</td>
<td><pre>
//phase is the top-level phase this stage belongs to.<br>

json_name: phase
go_name: Phase</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_at is when the stage started.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the status of this stage.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>status_reason</td>
<td>string</td>
<td><pre>
//status_reason is a short machine-readable explanation of the status.<br>

json_name: statusReason
go_name: StatusReason</pre></td>
</tr><tr>
<td>worker</td>
<td><a href="../domain/README.md#cloud-v1-domain-worker">cloud.v1.domain.Worker</a></td>
<td><pre>
//worker is the Temporal worker identity that executes this stage.<br>

json_name: worker
go_name: Worker</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-stageupdate"></a>
### cloud.v1.workflow.StageUpdate

<pre>
//StageUpdate is emitted as a Temporal signal by child workflows/activities
//whenever a concrete runtime stage changes status.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>stage</td>
<td><a href="#cloud-v1-workflow-stage">cloud.v1.workflow.Stage</a></td>
<td><pre>
//stage is the complete current snapshot for one runtime stage.<br>

json_name: stage
go_name: Stage</pre></td>
</tr>
</table>



<a name="cloud-v1-workflow-suiteworkflowrequest"></a>
### cloud.v1.workflow.SuiteWorkflowRequest

<pre>
//SuiteWorkflowRequest is the input to a suite run. The API/start layer expands
//the suite definition into persisted child TestRunRecords and then builds one
//RunConfig per child with provider settings and agent bootstrap resolved.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>max_parallel</td>
<td>uint32</td>
<td><pre>
//max_parallel caps concurrent child TestWorkflow executions. 0 =
//unlimited.<br>

json_name: maxParallel
go_name: MaxParallel</pre></td>
</tr><tr>
<td>runs</td>
<td><a href="#cloud-v1-workflow-runconfig">cloud.v1.workflow.RunConfig</a></td>
<td><pre>
//runs are the child run workflow inputs.<br>

json_name: runs
go_name: Runs</pre></td>
</tr><tr>
<td>suite_run_id</td>
<td>string</td>
<td><pre>
//suite_run_id is the persisted SuiteRunRecord id.<br>

json_name: suiteRunId
go_name: SuiteRunId</pre></td>
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
<td>agent_bootstrap</td>
<td><a href="#cloud-v1-workflow-agentbootstrap">cloud.v1.workflow.AgentBootstrap</a></td>
<td><pre>
//agent_bootstrap is runtime control-plane data delivered to provisioned
//agents through the provider-specific carrier.<br>

json_name: agentBootstrap
go_name: AgentBootstrap</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes quota reservations and provider settings lookup.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
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

