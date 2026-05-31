# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [cloud/v1/agent/logs.proto](#cloud_v1_agent_logs-proto)
    - [LogBatch](#cloud-v1-agent-LogBatch)
    - [ShipLogsAck](#cloud-v1-agent-ShipLogsAck)
  
    - [AgentLogService](#cloud-v1-agent-AgentLogService)
  
- [cloud/v1/agent/registry.proto](#cloud_v1_agent_registry-proto)
    - [AgentInfo](#cloud-v1-agent-AgentInfo)
    - [HeartbeatRequest](#cloud-v1-agent-HeartbeatRequest)
    - [HeartbeatResponse](#cloud-v1-agent-HeartbeatResponse)
    - [RegisterRequest](#cloud-v1-agent-RegisterRequest)
    - [RegisterResponse](#cloud-v1-agent-RegisterResponse)
  
    - [AgentRegistryService](#cloud-v1-agent-AgentRegistryService)
  
- [cloud/v1/agent/shell.proto](#cloud_v1_agent_shell-proto)
    - [AgentShellMsg](#cloud-v1-agent-AgentShellMsg)
    - [OpenShell](#cloud-v1-agent-OpenShell)
    - [Register](#cloud-v1-agent-Register)
    - [ServerShellMsg](#cloud-v1-agent-ServerShellMsg)
    - [ShellExit](#cloud-v1-agent-ShellExit)
    - [ShellResize](#cloud-v1-agent-ShellResize)
  
    - [AgentShellAgentService](#cloud-v1-agent-AgentShellAgentService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="cloud_v1_agent_logs-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/agent/logs.proto



<a name="cloud-v1-agent-LogBatch"></a>

### LogBatch
LogBatch is a chunk of lines the agent flushes together (by count or time).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lines | [cloud.v1.monitor.LogLine](#cloud-v1-monitor-LogLine) | repeated | lines is the batch of log lines to ship; capped at 10000 per batch so a single message stays bounded. |






<a name="cloud-v1-agent-ShipLogsAck"></a>

### ShipLogsAck
ShipLogsAck is the server&#39;s running acknowledgement back to the agent on the
log stream.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| accepted | [uint64](#uint64) |  | accepted is how many lines the server accepted across this stream so far (used for backpressure). |





 

 

 


<a name="cloud-v1-agent-AgentLogService"></a>

### AgentLogService
AgentLogService is the agent-facing log ingestion endpoint that forwards
shipped lines into VictoriaLogs.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ShipLogs | [LogBatch](#cloud-v1-agent-LogBatch) stream | [ShipLogsAck](#cloud-v1-agent-ShipLogsAck) | ShipLogs is a client stream of batches: the agent flushes LogBatch chunks continuously; the server writes them to VictoriaLogs and acks counts. |

 



<a name="cloud_v1_agent_registry-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/agent/registry.proto



<a name="cloud-v1-agent-AgentInfo"></a>

### AgentInfo
AgentInfo identifies an agent host.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| machine_id | [string](#string) |  | machine_id is the stable per-host identifier the agent registers under. |
| host | [string](#string) |  | host is the agent&#39;s hostname / network address (informational). |
| agent_version | [string](#string) |  | agent_version is the running agent build version (for compatibility/audit). |
| run_id | [string](#string) |  | run_id is the run this agent is provisioned for (scoping/audit), if any. |






<a name="cloud-v1-agent-HeartbeatRequest"></a>

### HeartbeatRequest
HeartbeatRequest keeps an already-registered agent marked online.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| machine_id | [string](#string) |  | machine_id is the host whose liveness is being refreshed. |






<a name="cloud-v1-agent-HeartbeatResponse"></a>

### HeartbeatResponse
HeartbeatResponse is the empty server acknowledgement of a heartbeat.






<a name="cloud-v1-agent-RegisterRequest"></a>

### RegisterRequest
RegisterRequest announces an agent coming online to the server.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| info | [AgentInfo](#cloud-v1-agent-AgentInfo) |  | info is the identifying details of the agent host coming online. |






<a name="cloud-v1-agent-RegisterResponse"></a>

### RegisterResponse
RegisterResponse tells the agent it is registered and how often to heartbeat.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| registered_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | registered_at is the server clock time the agent was recorded online. |
| heartbeat_interval_seconds | [uint32](#uint32) |  | heartbeat_interval_seconds is the heartbeat cadence the server expects; the agent is considered offline after a missed window. |





 

 

 


<a name="cloud-v1-agent-AgentRegistryService"></a>

### AgentRegistryService
AgentRegistryService is the agent-facing presence channel: agents register
and heartbeat so the server knows which agents are online.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Register | [RegisterRequest](#cloud-v1-agent-RegisterRequest) | [RegisterResponse](#cloud-v1-agent-RegisterResponse) | Register announces an agent coming online. |
| Heartbeat | [HeartbeatRequest](#cloud-v1-agent-HeartbeatRequest) | [HeartbeatResponse](#cloud-v1-agent-HeartbeatResponse) | Heartbeat keeps the agent marked online. |

 



<a name="cloud_v1_agent_shell-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/agent/shell.proto



<a name="cloud-v1-agent-AgentShellMsg"></a>

### AgentShellMsg
AgentShellMsg is agent -&gt; server over the control stream.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id is the session this frame belongs to (empty on the first register frame). |
| register | [Register](#cloud-v1-agent-Register) |  | register is the first frame identifying the host; session_id empty. |
| stdout | [bytes](#bytes) |  | stdout is PTY standard-output bytes for the session. |
| stderr | [bytes](#bytes) |  | stderr is PTY standard-error bytes for the session. |
| exit | [ShellExit](#cloud-v1-agent-ShellExit) |  | exit reports the session&#39;s process ending. |






<a name="cloud-v1-agent-OpenShell"></a>

### OpenShell
OpenShell asks the agent to spawn a PTY for a new session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run_id | [string](#string) |  | run_id runs this shell in the context of a run (for audit/scoping); may be empty. |
| component_id | [string](#string) |  | component_id is the target component on the host, optional. |
| cols | [uint32](#uint32) |  | cols is the initial terminal width in columns. |
| rows | [uint32](#uint32) |  | rows is the initial terminal height in rows. |
| shell | [string](#string) |  | shell is the shell binary to launch (e.g. &#34;/bin/bash&#34;); empty -&gt; agent default. |






<a name="cloud-v1-agent-Register"></a>

### Register
Register is the agent&#39;s first AgentShellMsg, identifying which host it is.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| machine_id | [string](#string) |  | machine_id is the host identifier the control stream belongs to. |






<a name="cloud-v1-agent-ServerShellMsg"></a>

### ServerShellMsg
ServerShellMsg is server -&gt; agent over the control stream.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id is the session this frame belongs to (server-assigned on open). |
| open | [OpenShell](#cloud-v1-agent-OpenShell) |  | open requests spawning a new PTY session. |
| stdin | [bytes](#bytes) |  | stdin is keystroke input bytes for the session&#39;s PTY. |
| resize | [ShellResize](#cloud-v1-agent-ShellResize) |  | resize updates the session&#39;s terminal window size. |
| close | [google.protobuf.Empty](#google-protobuf-Empty) |  | close terminates the session. |






<a name="cloud-v1-agent-ShellExit"></a>

### ShellExit
ShellExit reports a session ending.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [int32](#int32) |  | code is the process exit code of the shell session. |
| error | [string](#string) |  | error is an optional error message describing why the session ended. |






<a name="cloud-v1-agent-ShellResize"></a>

### ShellResize
ShellResize updates the PTY window size.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cols | [uint32](#uint32) |  | cols is the new terminal width in columns. |
| rows | [uint32](#uint32) |  | rows is the new terminal height in rows. |





 

 

 


<a name="cloud-v1-agent-AgentShellAgentService"></a>

### AgentShellAgentService
AgentShellAgentApi is dialed by the agent (not by users). Auth is the agent
token, NOT the iam RBAC interceptor — hence no (cloud.v1.iam.auth) annotation.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Connect | [AgentShellMsg](#cloud-v1-agent-AgentShellMsg) stream | [ServerShellMsg](#cloud-v1-agent-ServerShellMsg) stream | Connect opens the long-lived, multiplexed control stream. |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

