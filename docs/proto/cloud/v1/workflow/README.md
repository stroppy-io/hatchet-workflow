

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
- [cloud.v1.workflow.TestService](#cloud-v1-workflow-testservice)
  - [Workflows](#cloud-v1-workflow-testservice-workflows)
    - [PlaceholderWorkflow](#placeholderworkflow-workflow)
  - [Queries](#cloud-v1-workflow-testservice-queries)
    - [GetRunState](#getrunstate-query)
  - [Signals](#cloud-v1-workflow-testservice-signals)
    - [UpdateStage](#updatestage-signal)
- Messages
  - [cloud.v1.workflow.PlaceholderWorkflowRequest](#cloud-v1-workflow-placeholderworkflowrequest)
  - [cloud.v1.workflow.PlaceholderWorkflowResponse](#cloud-v1-workflow-placeholderworkflowresponse)
  - [cloud.v1.workflow.RunState](#cloud-v1-workflow-runstate)
  - [cloud.v1.workflow.Stage](#cloud-v1-workflow-stage)
  - [cloud.v1.workflow.StageUpdate](#cloud-v1-workflow-stageupdate)

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

<a name="cloud-v1-workflow-testservice"></a>
## cloud.v1.workflow.TestService

<pre>
//TestService exposes the live run-state query/signal surface a running test
//workflow answers. It composed the full test-run orchestration workflows;
//that orchestration is now driven by the generic DSL interpreter workflow
//(RunRecipeWorkflow, internal/workflows/runrecipe.go), which answers the
//same GetRunState query / UpdateStage signal directly against the SDK
//(see runrecipe.go's GetRunState/GetRunStateQueryName) without going
//through this generated service.
</pre>

<a name="cloud-v1-workflow-testservice-workflows"></a>
### Workflows

---
<a name="placeholderworkflow-workflow"></a>
### PlaceholderWorkflow

<pre>
//PlaceholderWorkflow: see the message doc above — required by the
//code generator, never started.
</pre>

**Input:** [cloud.v1.workflow.PlaceholderWorkflowRequest](#cloud-v1-workflow-placeholderworkflowrequest)



**Output:** [cloud.v1.workflow.PlaceholderWorkflowResponse](#cloud-v1-workflow-placeholderworkflowresponse)



**Defaults:**

<table>
<tr><th>Name</th><th>Value</th></tr>
<tr><td>id_reuse_policy</td><td><pre><code>WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED</code></pre></td></tr>
<tr><td>retry_policy.max_attempts</td><td>1</td></tr>
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

<a name="cloud-v1-workflow-placeholderworkflowrequest"></a>
### cloud.v1.workflow.PlaceholderWorkflowRequest

<pre>
//PlaceholderWorkflow is NOT a real, startable workflow — nothing registers
//or starts it. It exists only so protoc-gen-go-temporal emits a valid
//TestService: the plugin's generated TestClient (testsuite-environment
//helper) unconditionally references a "TestServiceWorkflows" interface,
//which the plugin itself only generates when the service declares at
//least one (temporal.v1.workflow) rpc — a service with only
//query/signal rpcs (GetRunState/UpdateStage, needed by the live run
//Overview and by RunRecipeWorkflow) hits that gap and fails to compile
//without one. Do not add real fields or logic here; if this whole
//workaround ever becomes unnecessary (e.g. a fixed plugin version), delete
//this message + rpc first.
</pre>



<a name="cloud-v1-workflow-placeholderworkflowresponse"></a>
### cloud.v1.workflow.PlaceholderWorkflowResponse



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

