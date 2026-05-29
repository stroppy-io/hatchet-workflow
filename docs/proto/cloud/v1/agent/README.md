

<a name="cloud-v1-agent"></a>
# cloud.v1.agent

## Table of Contents
- Messages
  - [cloud.v1.agent.AgentInfo](#cloud-v1-agent-agentinfo)
  - [cloud.v1.agent.AgentShellMsg](#cloud-v1-agent-agentshellmsg)
  - [cloud.v1.agent.HeartbeatRequest](#cloud-v1-agent-heartbeatrequest)
  - [cloud.v1.agent.HeartbeatResponse](#cloud-v1-agent-heartbeatresponse)
  - [cloud.v1.agent.LogBatch](#cloud-v1-agent-logbatch)
  - [cloud.v1.agent.OpenShell](#cloud-v1-agent-openshell)
  - [cloud.v1.agent.Register](#cloud-v1-agent-register)
  - [cloud.v1.agent.RegisterRequest](#cloud-v1-agent-registerrequest)
  - [cloud.v1.agent.RegisterResponse](#cloud-v1-agent-registerresponse)
  - [cloud.v1.agent.ServerShellMsg](#cloud-v1-agent-servershellmsg)
  - [cloud.v1.agent.ShellExit](#cloud-v1-agent-shellexit)
  - [cloud.v1.agent.ShellResize](#cloud-v1-agent-shellresize)
  - [cloud.v1.agent.ShipLogsAck](#cloud-v1-agent-shiplogsack)

<a name="cloud-v1-agent-services"></a>
## Services

<a name="cloud-v1-agent-messages"></a>
## Messages

<a name="cloud-v1-agent-agentinfo"></a>
### cloud.v1.agent.AgentInfo

<pre>
//AgentInfo identifies an agent host.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent_version</td>
<td>string</td>
<td><pre>
//agent_version is the running agent build version (for compatibility/audit).<br>

json_name: agentVersion
go_name: AgentVersion</pre></td>
</tr><tr>
<td>host</td>
<td>string</td>
<td><pre>
//host is the agent's hostname / network address (informational).<br>

json_name: host
go_name: Host</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
//machine_id is the stable per-host identifier the agent registers under.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the run this agent is provisioned for (scoping/audit), if any.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-agentshellmsg"></a>
### cloud.v1.agent.AgentShellMsg

<pre>
//AgentShellMsg is agent -> server over the control stream.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>exit</td>
<td><a href="#cloud-v1-agent-shellexit">cloud.v1.agent.ShellExit</a></td>
<td><pre>
//exit reports the session's process ending.<br>

json_name: exit
go_name: Exit</pre></td>
</tr><tr>
<td>register</td>
<td><a href="#cloud-v1-agent-register">cloud.v1.agent.Register</a></td>
<td><pre>
//register is the first frame identifying the host; session_id empty.<br>

json_name: register
go_name: Register</pre></td>
</tr><tr>
<td>session_id</td>
<td>string</td>
<td><pre>
//session_id is the session this frame belongs to (empty on the first
//register frame).<br>

json_name: sessionId
go_name: SessionId</pre></td>
</tr><tr>
<td>stderr</td>
<td>bytes</td>
<td><pre>
//stderr is PTY standard-error bytes for the session.<br>

json_name: stderr
go_name: Stderr</pre></td>
</tr><tr>
<td>stdout</td>
<td>bytes</td>
<td><pre>
//stdout is PTY standard-output bytes for the session.<br>

json_name: stdout
go_name: Stdout</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-heartbeatrequest"></a>
### cloud.v1.agent.HeartbeatRequest

<pre>
//HeartbeatRequest keeps an already-registered agent marked online.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
//machine_id is the host whose liveness is being refreshed.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-heartbeatresponse"></a>
### cloud.v1.agent.HeartbeatResponse

<pre>
//HeartbeatResponse is the empty server acknowledgement of a heartbeat.
</pre>



<a name="cloud-v1-agent-logbatch"></a>
### cloud.v1.agent.LogBatch

<pre>
//LogBatch is a chunk of lines the agent flushes together (by count or time).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>lines</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logline">cloud.v1.monitor.LogLine</a></td>
<td><pre>
//lines is the batch of log lines to ship; capped at 10000 per batch so a
//single message stays bounded.<br>

json_name: lines
go_name: Lines</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-openshell"></a>
### cloud.v1.agent.OpenShell

<pre>
//OpenShell asks the agent to spawn a PTY for a new session.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cols</td>
<td>uint32</td>
<td><pre>
//cols is the initial terminal width in columns.<br>

json_name: cols
go_name: Cols</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id is the target component on the host, optional.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>rows</td>
<td>uint32</td>
<td><pre>
//rows is the initial terminal height in rows.<br>

json_name: rows
go_name: Rows</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id runs this shell in the context of a run (for audit/scoping); may
//be empty.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>shell</td>
<td>string</td>
<td><pre>
//shell is the shell binary to launch (e.g. "/bin/bash"); empty -> agent
//default.<br>

json_name: shell
go_name: Shell</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-register"></a>
### cloud.v1.agent.Register

<pre>
//Register is the agent's first AgentShellMsg, identifying which host it is.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
//machine_id is the host identifier the control stream belongs to.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-registerrequest"></a>
### cloud.v1.agent.RegisterRequest

<pre>
//RegisterRequest announces an agent coming online to the server.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>info</td>
<td><a href="#cloud-v1-agent-agentinfo">cloud.v1.agent.AgentInfo</a></td>
<td><pre>
//info is the identifying details of the agent host coming online.<br>

json_name: info
go_name: Info</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-registerresponse"></a>
### cloud.v1.agent.RegisterResponse

<pre>
//RegisterResponse tells the agent it is registered and how often to heartbeat.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>heartbeat_interval_seconds</td>
<td>uint32</td>
<td><pre>
//heartbeat_interval_seconds is the heartbeat cadence the server expects;
//the agent is considered offline after a missed window.<br>

json_name: heartbeatIntervalSeconds
go_name: HeartbeatIntervalSeconds</pre></td>
</tr><tr>
<td>registered_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//registered_at is the server clock time the agent was recorded online.<br>

json_name: registeredAt
go_name: RegisteredAt</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-servershellmsg"></a>
### cloud.v1.agent.ServerShellMsg

<pre>
//ServerShellMsg is server -> agent over the control stream.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>close</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-empty">google.protobuf.Empty</a></td>
<td><pre>
//close terminates the session.<br>

json_name: close
go_name: Close</pre></td>
</tr><tr>
<td>open</td>
<td><a href="#cloud-v1-agent-openshell">cloud.v1.agent.OpenShell</a></td>
<td><pre>
//open requests spawning a new PTY session.<br>

json_name: open
go_name: Open</pre></td>
</tr><tr>
<td>resize</td>
<td><a href="#cloud-v1-agent-shellresize">cloud.v1.agent.ShellResize</a></td>
<td><pre>
//resize updates the session's terminal window size.<br>

json_name: resize
go_name: Resize</pre></td>
</tr><tr>
<td>session_id</td>
<td>string</td>
<td><pre>
//session_id is the session this frame belongs to (server-assigned on open).<br>

json_name: sessionId
go_name: SessionId</pre></td>
</tr><tr>
<td>stdin</td>
<td>bytes</td>
<td><pre>
//stdin is keystroke input bytes for the session's PTY.<br>

json_name: stdin
go_name: Stdin</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-shellexit"></a>
### cloud.v1.agent.ShellExit

<pre>
//ShellExit reports a session ending.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>code</td>
<td>int32</td>
<td><pre>
//code is the process exit code of the shell session.<br>

json_name: code
go_name: Code</pre></td>
</tr><tr>
<td>error</td>
<td>string</td>
<td><pre>
//error is an optional error message describing why the session ended.<br>

json_name: error
go_name: Error</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-shellresize"></a>
### cloud.v1.agent.ShellResize

<pre>
//ShellResize updates the PTY window size.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cols</td>
<td>uint32</td>
<td><pre>
//cols is the new terminal width in columns.<br>

json_name: cols
go_name: Cols</pre></td>
</tr><tr>
<td>rows</td>
<td>uint32</td>
<td><pre>
//rows is the new terminal height in rows.<br>

json_name: rows
go_name: Rows</pre></td>
</tr>
</table>



<a name="cloud-v1-agent-shiplogsack"></a>
### cloud.v1.agent.ShipLogsAck

<pre>
//ShipLogsAck is the server's running acknowledgement back to the agent on the
//log stream.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>accepted</td>
<td>uint64</td>
<td><pre>
//accepted is how many lines the server accepted across this stream so far
//(used for backpressure).<br>

json_name: accepted
go_name: Accepted</pre></td>
</tr>
</table>

