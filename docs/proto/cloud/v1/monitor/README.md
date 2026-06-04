

<a name="cloud-v1-monitor"></a>
# cloud.v1.monitor

## Table of Contents
- Messages
  - [cloud.v1.monitor.Comparison](#cloud-v1-monitor-comparison)
  - [cloud.v1.monitor.Comparison.RunSummary](#cloud-v1-monitor-comparison-runsummary)
  - [cloud.v1.monitor.Event](#cloud-v1-monitor-event)
  - [cloud.v1.monitor.Event.Kind](#cloud-v1-monitor-event-kind)
  - [cloud.v1.monitor.EventSeverity](#cloud-v1-monitor-eventseverity)
  - [cloud.v1.monitor.LogCursor](#cloud-v1-monitor-logcursor)
  - [cloud.v1.monitor.LogLine](#cloud-v1-monitor-logline)
  - [cloud.v1.monitor.LogRef](#cloud-v1-monitor-logref)
  - [cloud.v1.monitor.MetricCell](#cloud-v1-monitor-metriccell)
  - [cloud.v1.monitor.MetricRow](#cloud-v1-monitor-metricrow)
  - [cloud.v1.monitor.MetricSummary](#cloud-v1-monitor-metricsummary)
  - [cloud.v1.monitor.ObservationSource](#cloud-v1-monitor-observationsource)
  - [cloud.v1.monitor.OperationKind](#cloud-v1-monitor-operationkind)
  - [cloud.v1.monitor.OutputKind](#cloud-v1-monitor-outputkind)
  - [cloud.v1.monitor.Overview](#cloud-v1-monitor-overview)
  - [cloud.v1.monitor.PipelineNode](#cloud-v1-monitor-pipelinenode)
  - [cloud.v1.monitor.PipelineOperation](#cloud-v1-monitor-pipelineoperation)
  - [cloud.v1.monitor.PipelineOperation.LabelsEntry](#cloud-v1-monitor-pipelineoperation-labelsentry)
  - [cloud.v1.monitor.PipelineOutput](#cloud-v1-monitor-pipelineoutput)
  - [cloud.v1.monitor.PipelineOutput.LabelsEntry](#cloud-v1-monitor-pipelineoutput-labelsentry)
  - [cloud.v1.monitor.PipelineView](#cloud-v1-monitor-pipelineview)
  - [cloud.v1.monitor.RunMetrics](#cloud-v1-monitor-runmetrics)
  - [cloud.v1.monitor.Source](#cloud-v1-monitor-source)
  - [cloud.v1.monitor.Stream](#cloud-v1-monitor-stream)
  - [cloud.v1.monitor.TimeRange](#cloud-v1-monitor-timerange)
  - [cloud.v1.monitor.Verdict](#cloud-v1-monitor-verdict)
  - [cloud.v1.monitor.WorkerInfo](#cloud-v1-monitor-workerinfo)
  - [cloud.v1.monitor.WorkerPresence](#cloud-v1-monitor-workerpresence)

<a name="cloud-v1-monitor-messages"></a>
## Messages

<a name="cloud-v1-monitor-comparison"></a>
### cloud.v1.monitor.Comparison

<pre>
Comparison is the metric-by-metric diff of N runs (>= 2) against a baseline
//(run_ids[0]) with a per-run roll-up verdict.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>metrics</td>
<td><a href="#cloud-v1-monitor-metricrow">cloud.v1.monitor.MetricRow</a></td>
<td><pre>
metrics are the per-metric rows, each comparing all runs for that metric.<br>

json_name: metrics
go_name: Metrics</pre></td>
</tr><tr>
<td>range</td>
<td><a href="#cloud-v1-monitor-timerange">cloud.v1.monitor.TimeRange</a></td>
<td><pre>
range is the time window the comparison was computed over.<br>

json_name: range
go_name: Range</pre></td>
</tr><tr>
<td>run_ids</td>
<td>string</td>
<td><pre>
run_ids are the compared runs in display order; run_ids[0] is the baseline.<br>

json_name: runIds
go_name: RunIds</pre></td>
</tr><tr>
<td>summaries</td>
<td><a href="#cloud-v1-monitor-comparison-runsummary">cloud.v1.monitor.Comparison.RunSummary</a></td>
<td><pre>
summaries roll up per non-baseline run (aligned with run_ids[1:]).<br>

json_name: summaries
go_name: Summaries</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-comparison-runsummary"></a>
### cloud.v1.monitor.Comparison.RunSummary

<pre>
RunSummary rolls up one run's per-metric verdicts against the baseline.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>better</td>
<td>uint32</td>
<td><pre>
better is the count of metrics where this run beat the baseline.<br>

json_name: better
go_name: Better</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
run_id is the non-baseline run this roll-up belongs to.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>same</td>
<td>uint32</td>
<td><pre>
same is the count of metrics within the threshold (no change).<br>

json_name: same
go_name: Same</pre></td>
</tr><tr>
<td>worse</td>
<td>uint32</td>
<td><pre>
worse is the count of metrics where this run regressed.<br>

json_name: worse
go_name: Worse</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-event"></a>
### cloud.v1.monitor.Event

<pre>
Event is one item in the run's activity timeline (Temporal-derived).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
at is when the event occurred.<br>

json_name: at
go_name: At</pre></td>
</tr><tr>
<td>detail</td>
<td>string</td>
<td><pre>
detail carries optional structured-ish extra context as plain text.<br>

json_name: detail
go_name: Detail</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-monitor-event-kind">cloud.v1.monitor.Event.Kind</a></td>
<td><pre>
kind is the category of this event.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>message</td>
<td>string</td>
<td><pre>
message is the human-readable event description.<br>

json_name: message
go_name: Message</pre></td>
</tr><tr>
<td>node_execution_id</td>
<td>string</td>
<td><pre>
node_execution_id is the owning stage, when the event is about a stage.<br>

json_name: nodeExecutionId
go_name: NodeExecutionId</pre></td>
</tr><tr>
<td>sequence</td>
<td>uint64</td>
<td><pre>
sequence is a stable ordering key within one snapshot.<br>

json_name: sequence
go_name: Sequence</pre></td>
</tr><tr>
<td>severity</td>
<td><a href="#cloud-v1-monitor-eventseverity">cloud.v1.monitor.EventSeverity</a></td>
<td><pre>
severity controls frontend emphasis.<br>

json_name: severity
go_name: Severity</pre></td>
</tr><tr>
<td>source</td>
<td><a href="#cloud-v1-monitor-observationsource">cloud.v1.monitor.ObservationSource</a></td>
<td><pre>
source says where this event was derived from.<br>

json_name: source
go_name: Source</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
status is the run/stage status associated with the event, when relevant.<br>

json_name: status
go_name: Status</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-event-kind"></a>
### cloud.v1.monitor.Event.Kind

<pre>
Kind classifies what the timeline event represents.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td><pre>
KIND_UNSPECIFIED is the zero value and is never a valid kind.
</pre></td>
</tr><tr>
<td>KIND_STAGE_STARTED</td>
<td><pre>
KIND_STAGE_STARTED is a stage beginning execution.
</pre></td>
</tr><tr>
<td>KIND_STAGE_COMPLETED</td>
<td><pre>
KIND_STAGE_COMPLETED is a stage finishing successfully.
</pre></td>
</tr><tr>
<td>KIND_STAGE_FAILED</td>
<td><pre>
KIND_STAGE_FAILED is a stage ending in failure.
</pre></td>
</tr><tr>
<td>KIND_STAGE_RETRYING</td>
<td><pre>
KIND_STAGE_RETRYING is a stage being retried after a failure.
</pre></td>
</tr><tr>
<td>KIND_WORKER_ONLINE</td>
<td><pre>
KIND_WORKER_ONLINE is a worker becoming reachable.
</pre></td>
</tr><tr>
<td>KIND_WORKER_OFFLINE</td>
<td><pre>
KIND_WORKER_OFFLINE is a worker going unreachable.
</pre></td>
</tr><tr>
<td>KIND_RUN_STATUS</td>
<td><pre>
KIND_RUN_STATUS is a change in the overall run status.
</pre></td>
</tr>
</table>

<a name="cloud-v1-monitor-eventseverity"></a>
### cloud.v1.monitor.EventSeverity

<pre>
EventSeverity lets the frontend render timeline entries without guessing.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>EVENT_SEVERITY_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>EVENT_SEVERITY_INFO</td>
<td></td>
</tr><tr>
<td>EVENT_SEVERITY_WARNING</td>
<td></td>
</tr><tr>
<td>EVENT_SEVERITY_ERROR</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-monitor-logcursor"></a>
### cloud.v1.monitor.LogCursor

<pre>
//LogCursor anchors an exact line: the precise observed_at plus a per-timestamp
//sequence ordinal (provider-agnostic, stable, dedups equal timestamps).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>observed_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
observed_at is the precise timestamp the line was observed at.<br>

json_name: observedAt
go_name: ObservedAt</pre></td>
</tr><tr>
<td>seq</td>
<td>uint64</td>
<td><pre>
//seq is the per-timestamp sequence ordinal that disambiguates lines
//sharing the same observed_at, making the cursor stable and dedup-safe.<br>

json_name: seq
go_name: Seq</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-logline"></a>
### cloud.v1.monitor.LogLine

<pre>
LogLine is one unified log record returned from a query or stream.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>action</td>
<td>string</td>
<td><pre>
action is the generic action class, e.g. call_cmd/write_file/create_dir.<br>

json_name: action
go_name: Action</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
component_id is the topology component the line is about.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>cursor</td>
<td><a href="#cloud-v1-monitor-logcursor">cloud.v1.monitor.LogCursor</a></td>
<td><pre>
cursor anchors this exact line for deep-linking.<br>

json_name: cursor
go_name: Cursor</pre></td>
</tr><tr>
<td>line</td>
<td>string</td>
<td><pre>
line is the raw log text content.<br>

json_name: line
go_name: Line</pre></td>
</tr><tr>
<td>line_no</td>
<td>uint64</td>
<td><pre>
//line_no is a stable, monotonic, GLOBAL ordinal of this line within the run
//(assigned server-side at ingest). Gives a continuous numbering across the
//whole run regardless of filter, so the UI can "go to line N", show a stable
//gutter, and two users opening the same LogRef land on the same line. Cursor
//anchors the exact position; line_no is the human/scroll index.<br>

json_name: lineNo
go_name: LineNo</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
machine_id is the host the line originated on.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>mentions</td>
<td>string</td>
<td><pre>
//mentions are normalized operation tokens, e.g. vector/vmagent/postgres.
//They let the Logs API filter by rendered/executed subject without
//backend-specific step-id switches.<br>

json_name: mentions
go_name: Mentions</pre></td>
</tr><tr>
<td>node_execution_id</td>
<td>string</td>
<td><pre>
node_execution_id is the owning execution stage, when the line came from a stage op.<br>

json_name: nodeExecutionId
go_name: NodeExecutionId</pre></td>
</tr><tr>
<td>observed_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
observed_at is when the line was emitted/observed.<br>

json_name: observedAt
go_name: ObservedAt</pre></td>
</tr><tr>
<td>parent_node_execution_id</td>
<td>string</td>
<td><pre>
parent_node_execution_id links the log line to the parent pipeline stage.<br>

json_name: parentNodeExecutionId
go_name: ParentNodeExecutionId</pre></td>
</tr><tr>
<td>phase</td>
<td>string</td>
<td><pre>
phase is the top-level pipeline phase this line belongs to, when known.<br>

json_name: phase
go_name: Phase</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
run_id is the owning test run id (plain string, runtime-pure).<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>source</td>
<td><a href="#cloud-v1-monitor-source">cloud.v1.monitor.Source</a></td>
<td><pre>
source is where the line came from (command, journald, or tailed file).<br>

json_name: source
go_name: Source</pre></td>
</tr><tr>
<td>stage_name</td>
<td>string</td>
<td><pre>
stage_name is the producing stage's display name, when known.<br>

json_name: stageName
go_name: StageName</pre></td>
</tr><tr>
<td>step_id</td>
<td>string</td>
<td><pre>
step_id is the component-local deployment step id, when this line came from an agent step.<br>

json_name: stepId
go_name: StepId</pre></td>
</tr><tr>
<td>stream</td>
<td><a href="#cloud-v1-monitor-stream">cloud.v1.monitor.Stream</a></td>
<td><pre>
stream is the std stream (stdout/stderr) the line was written to.<br>

json_name: stream
go_name: Stream</pre></td>
</tr><tr>
<td>unit</td>
<td>string</td>
<td><pre>
unit is the systemd unit or file path for JOURNALD/FILE sources.<br>

json_name: unit
go_name: Unit</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-logref"></a>
### cloud.v1.monitor.LogRef

<pre>
//LogRef is a shareable handle to a log slice (the link). Resolved by the backend
//into a LogsQL query + a deep-link URL. component_id alone = a component's logs;
//node_execution_id = a stage's logs; cursor = a specific line anchor.
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
component_id, when set, narrows the slice to one topology component's logs.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>cursor</td>
<td><a href="#cloud-v1-monitor-logcursor">cloud.v1.monitor.LogCursor</a></td>
<td><pre>
cursor pins a specific line (deep-link to line).<br>

json_name: cursor
go_name: Cursor</pre></td>
</tr><tr>
<td>end</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
end, when set, bounds the slice to lines at or before this time.<br>

json_name: end
go_name: End</pre></td>
</tr><tr>
<td>node_execution_id</td>
<td>string</td>
<td><pre>
node_execution_id, when set, narrows the slice to one execution stage's logs.<br>

json_name: nodeExecutionId
go_name: NodeExecutionId</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
run_id is the owning test run the slice belongs to.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>start</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
start, when set, bounds the slice to lines at or after this time.<br>

json_name: start
go_name: Start</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-metriccell"></a>
### cloud.v1.monitor.MetricCell

<pre>
MetricCell is one run's value for a metric within an N-way Comparison row.
//diff_*_pct are relative to the baseline run (Comparison.run_ids[0]); for the
//baseline cell itself they are 0 and verdict is VERDICT_SAME.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>avg</td>
<td>double</td>
<td><pre>
avg is this run's mean value for the metric.<br>

json_name: avg
go_name: Avg</pre></td>
</tr><tr>
<td>diff_avg_pct</td>
<td>double</td>
<td><pre>
diff_avg_pct is (cell - baseline) / baseline * 100; positive means higher.<br>

json_name: diffAvgPct
go_name: DiffAvgPct</pre></td>
</tr><tr>
<td>diff_max_pct</td>
<td>double</td>
<td><pre>
diff_max_pct is the same relative diff applied to the max value.<br>

json_name: diffMaxPct
go_name: DiffMaxPct</pre></td>
</tr><tr>
<td>max</td>
<td>double</td>
<td><pre>
max is this run's peak value for the metric.<br>

json_name: max
go_name: Max</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
run_id is the run this cell belongs to (= Comparison.run_ids[index]).<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>verdict</td>
<td><a href="#cloud-v1-monitor-verdict">cloud.v1.monitor.Verdict</a></td>
<td><pre>
verdict is the better/worse/same classification of this cell vs the baseline.<br>

json_name: verdict
go_name: Verdict</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-metricrow"></a>
### cloud.v1.monitor.MetricRow

<pre>
MetricRow compares one metric across all compared runs; cells are aligned
//1:1 with Comparison.run_ids.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cells</td>
<td><a href="#cloud-v1-monitor-metriccell">cloud.v1.monitor.MetricCell</a></td>
<td><pre>
cells are this metric's per-run values, aligned 1:1 with Comparison.run_ids.<br>

json_name: cells
go_name: Cells</pre></td>
</tr><tr>
<td>group</td>
<td>string</td>
<td><pre>
group is an optional generic UI grouping label (e.g. "throughput", "latency").<br>

json_name: group
go_name: Group</pre></td>
</tr><tr>
<td>higher_is_better</td>
<td>bool</td>
<td><pre>
higher_is_better is the comparison direction as DATA (mirrors MetricSummary).<br>

json_name: higherIsBetter
go_name: HigherIsBetter</pre></td>
</tr><tr>
<td>key</td>
<td>string</td>
<td><pre>
key is the stable metric key (PromQL series identity).<br>

json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the human-readable metric name supplied by the backend.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>unit</td>
<td>string</td>
<td><pre>
unit is the metric unit, e.g. "ms", "ops/s".<br>

json_name: unit
go_name: Unit</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-metricsummary"></a>
### cloud.v1.monitor.MetricSummary

<pre>
MetricSummary is one aggregated metric series for a run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>avg</td>
<td>double</td>
<td><pre>
avg is the mean value of the series over the window.<br>

json_name: avg
go_name: Avg</pre></td>
</tr><tr>
<td>description</td>
<td>string</td>
<td><pre>
description is an optional human tooltip supplied by the backend.<br>

json_name: description
go_name: Description</pre></td>
</tr><tr>
<td>group</td>
<td>string</td>
<td><pre>
group is an optional generic UI grouping label (e.g. "throughput", "latency").<br>

json_name: group
go_name: Group</pre></td>
</tr><tr>
<td>higher_is_better</td>
<td>bool</td>
<td><pre>
higher_is_better is the comparison direction as DATA (not an enum of metric meaning).<br>

json_name: higherIsBetter
go_name: HigherIsBetter</pre></td>
</tr><tr>
<td>key</td>
<td>string</td>
<td><pre>
key is the stable metric key (PromQL series identity). Free string, not an enum.<br>

json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>last</td>
<td>double</td>
<td><pre>
last is the most recent observed value.<br>

json_name: last
go_name: Last</pre></td>
</tr><tr>
<td>max</td>
<td>double</td>
<td><pre>
max is the largest observed value over the window.<br>

json_name: max
go_name: Max</pre></td>
</tr><tr>
<td>min</td>
<td>double</td>
<td><pre>
min is the smallest observed value over the window.<br>

json_name: min
go_name: Min</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the human-readable metric name supplied by the backend.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>unit</td>
<td>string</td>
<td><pre>
unit is the metric unit, e.g. "ms", "ops/s".<br>

json_name: unit
go_name: Unit</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-observationsource"></a>
### cloud.v1.monitor.ObservationSource

<pre>
//ObservationSource tells the UI which backend source produced a field/object.
//This is intentionally part of the contract: the frontend must be able to show
//when it is looking at live Temporal state, persisted fallback, the deployment
//plan, the agent registry, or a synthetic placeholder.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>OBSERVATION_SOURCE_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>OBSERVATION_SOURCE_TEMPORAL_RUN_STATE</td>
<td></td>
</tr><tr>
<td>OBSERVATION_SOURCE_PERSISTED_RECORD</td>
<td></td>
</tr><tr>
<td>OBSERVATION_SOURCE_DEPLOYMENT_PLAN</td>
<td></td>
</tr><tr>
<td>OBSERVATION_SOURCE_AGENT_REGISTRY</td>
<td></td>
</tr><tr>
<td>OBSERVATION_SOURCE_SYNTHETIC</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-monitor-operationkind"></a>
### cloud.v1.monitor.OperationKind

<pre>
OperationKind classifies the concrete agent operation represented by a node.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>OPERATION_KIND_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>OPERATION_KIND_CREATE_DIR</td>
<td></td>
</tr><tr>
<td>OPERATION_KIND_WRITE_FILE</td>
<td></td>
</tr><tr>
<td>OPERATION_KIND_FETCH_FILE</td>
<td></td>
</tr><tr>
<td>OPERATION_KIND_CALL_CMD</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-monitor-outputkind"></a>
### cloud.v1.monitor.OutputKind

<pre>
OutputKind classifies structured information produced by a pipeline stage.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>OUTPUT_KIND_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>OUTPUT_KIND_SUMMARY</td>
<td></td>
</tr><tr>
<td>OUTPUT_KIND_RENDER_ARTIFACT</td>
<td></td>
</tr><tr>
<td>OUTPUT_KIND_DEPLOYMENT_PLAN</td>
<td></td>
</tr><tr>
<td>OUTPUT_KIND_COMMAND_RESULT</td>
<td></td>
</tr><tr>
<td>OUTPUT_KIND_FILE</td>
<td></td>
</tr><tr>
<td>OUTPUT_KIND_DIRECTORY</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-monitor-overview"></a>
### cloud.v1.monitor.Overview

<pre>
Overview is the whole Overview-tab payload for one run (snapshot or stream tick).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>degraded_reasons</td>
<td>string</td>
<td><pre>
//degraded_reasons explains missing/stale parts of the snapshot, e.g. a
//closed Temporal workflow forcing persisted-record fallback.<br>

json_name: degradedReasons
go_name: DegradedReasons</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
duration is the elapsed run time (so far, or final once finished).<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
finished_at is when the run reached a terminal state; unset while running.<br>

json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>observed_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
observed_at is the server clock time this snapshot was assembled.<br>

json_name: observedAt
go_name: ObservedAt</pre></td>
</tr><tr>
<td>pipeline</td>
<td><a href="#cloud-v1-monitor-pipelineview">cloud.v1.monitor.PipelineView</a></td>
<td><pre>
pipeline is the live pipeline view (display tree + per-node runtime + log handles).<br>

json_name: pipeline
go_name: Pipeline</pre></td>
</tr><tr>
<td>progress_pct</td>
<td>uint32</td>
<td><pre>
progress_pct is the aggregate progress (0..100), derived from the pipeline.<br>

json_name: progressPct
go_name: ProgressPct</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
run_id is the run this overview projects (plain string, runtime-pure).<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>source</td>
<td><a href="#cloud-v1-monitor-observationsource">cloud.v1.monitor.ObservationSource</a></td>
<td><pre>
source is the primary source of the top-level run status/progress.<br>

json_name: source
go_name: Source</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
started_at is when the run began executing.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
status is the overall run status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>timeline</td>
<td><a href="#cloud-v1-monitor-event">cloud.v1.monitor.Event</a></td>
<td><pre>
timeline is the Temporal-derived activity feed (what is happening / happened).<br>

json_name: timeline
go_name: Timeline</pre></td>
</tr><tr>
<td>workers</td>
<td><a href="#cloud-v1-monitor-workerinfo">cloud.v1.monitor.WorkerInfo</a></td>
<td><pre>
workers are the agents/workers participating in the run and their live state.<br>

json_name: workers
go_name: Workers</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-pipelinenode"></a>
### cloud.v1.monitor.PipelineNode

<pre>
PipelineNode is one stage in the live pipeline.
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
attempt counts a retried stage (>1 means it was retried).<br>

json_name: attempt
go_name: Attempt</pre></td>
</tr><tr>
<td>children</td>
<td><a href="#cloud-v1-monitor-pipelinenode">cloud.v1.monitor.PipelineNode</a></td>
<td><pre>
children are the nested stages under this node.<br>

json_name: children
go_name: Children</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
component_id is the topology component this node acts on, when any.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
duration is the elapsed time of this stage.<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>error_message</td>
<td>string</td>
<td><pre>
error_message is the user-facing error text when this node failed.<br>

json_name: errorMessage
go_name: ErrorMessage</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
finished_at is when this stage reached a terminal state; unset while running.<br>

json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>log_ref</td>
<td><a href="#cloud-v1-monitor-logref">cloud.v1.monitor.LogRef</a></td>
<td><pre>
//log_ref is the deep-link from this node to its logs. The UI builds the
//Logs tab filter from it. Carries the run/stage/component handles needed to
//slice the logs (node_execution_id + component_id).<br>

json_name: logRef
go_name: LogRef</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
machine_id is the machine/agent this node targets, when any.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the human label of the stage.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>node_execution_id</td>
<td>string</td>
<td><pre>
//node_execution_id is the stable id of this stage (its Temporal execution).
//It is the node's identity AND the key the LogRef/log filter uses to slice
//this stage's logs.<br>

json_name: nodeExecutionId
go_name: NodeExecutionId</pre></td>
</tr><tr>
<td>operation</td>
<td><a href="#cloud-v1-monitor-pipelineoperation">cloud.v1.monitor.PipelineOperation</a></td>
<td><pre>
//operation is the full generic projection of an executable deployment
//action. It is populated for agent step nodes and lets the UI inspect
//command/file/dir details without knowing backend-specific step ids.<br>

json_name: operation
go_name: Operation</pre></td>
</tr><tr>
<td>order</td>
<td>uint32</td>
<td><pre>
order is a stable 1-based sibling display/execution order.<br>

json_name: order
go_name: Order</pre></td>
</tr><tr>
<td>outputs</td>
<td><a href="#cloud-v1-monitor-pipelineoutput">cloud.v1.monitor.PipelineOutput</a></td>
<td><pre>
//outputs are structured artifacts/results produced by this stage, such as
//render-plan artifacts or command execution results. They are separate
//from logs: logs remain the raw stream, outputs are inspectable summaries.<br>

json_name: outputs
go_name: Outputs</pre></td>
</tr><tr>
<td>params</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
params are the baked params this stage ran with (what used to live on the static task).<br>

json_name: params
go_name: Params</pre></td>
</tr><tr>
<td>parent_node_execution_id</td>
<td>string</td>
<td><pre>
parent_node_execution_id links a nested node to its parent stage.<br>

json_name: parentNodeExecutionId
go_name: ParentNodeExecutionId</pre></td>
</tr><tr>
<td>phase</td>
<td>string</td>
<td><pre>
phase is the top-level phase this node belongs to.<br>

json_name: phase
go_name: Phase</pre></td>
</tr><tr>
<td>source</td>
<td><a href="#cloud-v1-monitor-observationsource">cloud.v1.monitor.ObservationSource</a></td>
<td><pre>
source says where this node's runtime data came from.<br>

json_name: source
go_name: Source</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
started_at is when this stage began executing.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
status is the runtime status of this stage.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>status_reason</td>
<td>string</td>
<td><pre>
status_reason is a short machine-readable explanation of the status.<br>

json_name: statusReason
go_name: StatusReason</pre></td>
</tr><tr>
<td>worker</td>
<td><a href="../domain/README.md#cloud-v1-domain-worker">cloud.v1.domain.Worker</a></td>
<td><pre>
worker is which worker ran this stage.<br>

json_name: worker
go_name: Worker</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-pipelineoperation"></a>
### cloud.v1.monitor.PipelineOperation

<pre>
PipelineOperation is a generic, inspectable projection of an AgentStep action.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>argv</td>
<td>string</td>
<td><pre>
argv is the direct exec argv when the command uses argv form.<br>

json_name: argv
go_name: Argv</pre></td>
</tr><tr>
<td>command</td>
<td><a href="../common/README.md#cloud-v1-common-cmd">cloud.v1.common.Cmd</a></td>
<td><pre>
command is the raw command payload for CALL_CMD actions.<br>

json_name: command
go_name: Command</pre></td>
</tr><tr>
<td>command_text</td>
<td>string</td>
<td><pre>
command_text is the shell script or joined argv for quick UI rendering/search.<br>

json_name: commandText
go_name: CommandText</pre></td>
</tr><tr>
<td>content_preview</td>
<td>string</td>
<td><pre>
content_preview is a bounded text preview of inline file content.<br>

json_name: contentPreview
go_name: ContentPreview</pre></td>
</tr><tr>
<td>dir</td>
<td><a href="../common/README.md#cloud-v1-common-dir">cloud.v1.common.Dir</a></td>
<td><pre>
dir is the raw directory payload for CREATE_DIR actions.<br>

json_name: dir
go_name: Dir</pre></td>
</tr><tr>
<td>elapsed</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
elapsed is the observed process runtime, when reported.<br>

json_name: elapsed
go_name: Elapsed</pre></td>
</tr><tr>
<td>exit_code</td>
<td>int32</td>
<td><pre>
exit_code is the observed process exit code.<br>

json_name: exitCode
go_name: ExitCode</pre></td>
</tr><tr>
<td>file</td>
<td><a href="../common/README.md#cloud-v1-common-file">cloud.v1.common.File</a></td>
<td><pre>
file is the raw file payload for WRITE_FILE and FETCH_FILE actions.<br>

json_name: file
go_name: File</pre></td>
</tr><tr>
<td>file_path</td>
<td>string</td>
<td><pre>
file_path is the target path for file actions.<br>

json_name: filePath
go_name: FilePath</pre></td>
</tr><tr>
<td>file_size_bytes</td>
<td>uint64</td>
<td><pre>
file_size_bytes is the inline payload size when known.<br>

json_name: fileSizeBytes
go_name: FileSizeBytes</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-monitor-operationkind">cloud.v1.monitor.OperationKind</a></td>
<td><pre>
kind is the oneof branch used by the original AgentStep.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-monitor-pipelineoperation-labelsentry">cloud.v1.monitor.PipelineOperation.LabelsEntry</a></td>
<td><pre>
labels are the original AgentStep labels, including execution/log ids.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>mentions</td>
<td>string</td>
<td><pre>
//mentions are normalized tokens extracted from the command/file target
//and payload. This is deliberately generic: the frontend can surface
//"vector", "vmagent", "postgres", etc. without backend step-id switches.<br>

json_name: mentions
go_name: Mentions</pre></td>
</tr><tr>
<td>result_available</td>
<td>bool</td>
<td><pre>
result_available is true once a CALL_CMD action has an observed result.<br>

json_name: resultAvailable
go_name: ResultAvailable</pre></td>
</tr><tr>
<td>result_summary</td>
<td>string</td>
<td><pre>
result_summary is a compact display string for the observed result.<br>

json_name: resultSummary
go_name: ResultSummary</pre></td>
</tr><tr>
<td>stderr_preview</td>
<td>string</td>
<td><pre>
stderr_preview is a bounded preview of captured stderr.<br>

json_name: stderrPreview
go_name: StderrPreview</pre></td>
</tr><tr>
<td>stdout_preview</td>
<td>string</td>
<td><pre>
stdout_preview is a bounded preview of captured stdout.<br>

json_name: stdoutPreview
go_name: StdoutPreview</pre></td>
</tr><tr>
<td>step_id</td>
<td>string</td>
<td><pre>
step_id is the stable component-local step id.<br>

json_name: stepId
go_name: StepId</pre></td>
</tr><tr>
<td>step_order</td>
<td>uint32</td>
<td><pre>
step_order is the component-local execution order from the deployment plan.<br>

json_name: stepOrder
go_name: StepOrder</pre></td>
</tr><tr>
<td>summary</td>
<td>string</td>
<td><pre>
summary is a compact display string derived from the concrete action.<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>target</td>
<td>string</td>
<td><pre>
target is the main file path, directory path, command executable, or script headline.<br>

json_name: target
go_name: Target</pre></td>
</tr><tr>
<td>timed_out</td>
<td>bool</td>
<td><pre>
timed_out is true when the executor terminated the process by timeout.<br>

json_name: timedOut
go_name: TimedOut</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-pipelineoperation-labelsentry"></a>
### cloud.v1.monitor.PipelineOperation.LabelsEntry

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



<a name="cloud-v1-monitor-pipelineoutput"></a>
### cloud.v1.monitor.PipelineOutput

<pre>
PipelineOutput is one structured artifact/result produced by a stage.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>action</td>
<td>string</td>
<td><pre>
action is the generic action class, e.g. call_cmd/write_file/create_dir.<br>

json_name: action
go_name: Action</pre></td>
</tr><tr>
<td>command_text</td>
<td>string</td>
<td><pre>
command_text carries a bounded command/script preview when this output is command-like.<br>

json_name: commandText
go_name: CommandText</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
component_id is the topology component this output belongs to, when any.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>content_preview</td>
<td>string</td>
<td><pre>
content_preview carries a bounded file/result preview.<br>

json_name: contentPreview
go_name: ContentPreview</pre></td>
</tr><tr>
<td>count</td>
<td>uint32</td>
<td><pre>
count is a generic count for summary outputs.<br>

json_name: count
go_name: Count</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
id is a stable output id within the producing stage.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-monitor-outputkind">cloud.v1.monitor.OutputKind</a></td>
<td><pre>
kind is the broad output class.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-monitor-pipelineoutput-labelsentry">cloud.v1.monitor.PipelineOutput.LabelsEntry</a></td>
<td><pre>
labels carries renderer/action metadata for UI filtering/debugging.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
machine_id is the target machine/agent id, when any.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the display label.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>size_bytes</td>
<td>uint64</td>
<td><pre>
size_bytes is an inline content/result size when known.<br>

json_name: sizeBytes
go_name: SizeBytes</pre></td>
</tr><tr>
<td>step_id</td>
<td>string</td>
<td><pre>
step_id is the component-local deployment step id, when any.<br>

json_name: stepId
go_name: StepId</pre></td>
</tr><tr>
<td>summary</td>
<td>string</td>
<td><pre>
summary is a compact human-readable description.<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>target</td>
<td>string</td>
<td><pre>
target is the file path, directory path, command target, or rendered object id.<br>

json_name: target
go_name: Target</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-pipelineoutput-labelsentry"></a>
### cloud.v1.monitor.PipelineOutput.LabelsEntry

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



<a name="cloud-v1-monitor-pipelineview"></a>
### cloud.v1.monitor.PipelineView

<pre>
//PipelineView is the ONE pipeline representation: the live run pipeline derived
//from Temporal. There is no separate static "blueprint" — for a not-yet-started
//run the server emits the same tree with PENDING statuses.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>roots</td>
<td><a href="#cloud-v1-monitor-pipelinenode">cloud.v1.monitor.PipelineNode</a></td>
<td><pre>
roots are the top-level pipeline stages; each may nest children.<br>

json_name: roots
go_name: Roots</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-runmetrics"></a>
### cloud.v1.monitor.RunMetrics

<pre>
RunMetrics is the collected metric summary set for one run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>metrics</td>
<td><a href="#cloud-v1-monitor-metricsummary">cloud.v1.monitor.MetricSummary</a></td>
<td><pre>
metrics is the per-series aggregated summary set for this run.<br>

json_name: metrics
go_name: Metrics</pre></td>
</tr><tr>
<td>range</td>
<td><a href="#cloud-v1-monitor-timerange">cloud.v1.monitor.TimeRange</a></td>
<td><pre>
range is the time window the metrics were computed over.<br>

json_name: range
go_name: Range</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
run_id is the plain run identity (string, keeps runtime models-independent).<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-source"></a>
### cloud.v1.monitor.Source

<pre>
Source is where a log line came from.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>SOURCE_UNSPECIFIED</td>
<td><pre>
SOURCE_UNSPECIFIED is the zero value and is never a valid source.
</pre></td>
</tr><tr>
<td>SOURCE_COMMAND</td>
<td><pre>
SOURCE_COMMAND is agent command stdout/stderr (an execution-stage op).
</pre></td>
</tr><tr>
<td>SOURCE_JOURNALD</td>
<td><pre>
SOURCE_JOURNALD is a systemd-managed service's stdout/stderr.
</pre></td>
</tr><tr>
<td>SOURCE_FILE</td>
<td><pre>
SOURCE_FILE is a tailed log file (e.g. a postgresql log under /var/log/postgresql).
</pre></td>
</tr>
</table>

<a name="cloud-v1-monitor-stream"></a>
### cloud.v1.monitor.Stream

<pre>
Stream is the std stream the line was written to.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>STREAM_UNSPECIFIED</td>
<td><pre>
STREAM_UNSPECIFIED is the zero value and is never a valid stream.
</pre></td>
</tr><tr>
<td>STREAM_STDOUT</td>
<td><pre>
STREAM_STDOUT is the standard output stream.
</pre></td>
</tr><tr>
<td>STREAM_STDERR</td>
<td><pre>
STREAM_STDERR is the standard error stream.
</pre></td>
</tr>
</table>

<a name="cloud-v1-monitor-timerange"></a>
### cloud.v1.monitor.TimeRange

<pre>
TimeRange is the [start, end] window the metrics cover.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>end</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
end is the inclusive upper bound of the window.<br>

json_name: end
go_name: End</pre></td>
</tr><tr>
<td>start</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
start is the inclusive lower bound of the window.<br>

json_name: start
go_name: Start</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-verdict"></a>
### cloud.v1.monitor.Verdict

<pre>
Verdict classifies a per-metric change of one run against the baseline,
//honouring the metric's higher_is_better direction and the request threshold.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>VERDICT_UNSPECIFIED</td>
<td><pre>
VERDICT_UNSPECIFIED is the zero value and is never a valid verdict.
</pre></td>
</tr><tr>
<td>VERDICT_BETTER</td>
<td><pre>
VERDICT_BETTER means the run improved on the baseline (per direction/threshold).
</pre></td>
</tr><tr>
<td>VERDICT_WORSE</td>
<td><pre>
VERDICT_WORSE means the run regressed against the baseline.
</pre></td>
</tr><tr>
<td>VERDICT_SAME</td>
<td><pre>
VERDICT_SAME means the change was within the threshold (no meaningful difference).
</pre></td>
</tr>
</table>

<a name="cloud-v1-monitor-workerinfo"></a>
### cloud.v1.monitor.WorkerInfo

<pre>
WorkerInfo is a master/agent worker and its live state.
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
agent_version is the build/version reported by the agent.<br>

json_name: agentVersion
go_name: AgentVersion</pre></td>
</tr><tr>
<td>current_node_execution_id</td>
<td>string</td>
<td><pre>
current_node_execution_id is the stage the worker is executing, if any.<br>

json_name: currentNodeExecutionId
go_name: CurrentNodeExecutionId</pre></td>
</tr><tr>
<td>heartbeat_interval_seconds</td>
<td>uint32</td>
<td><pre>
heartbeat_interval_seconds is the cadence the server expects.<br>

json_name: heartbeatIntervalSeconds
go_name: HeartbeatIntervalSeconds</pre></td>
</tr><tr>
<td>host</td>
<td>string</td>
<td><pre>
host is the worker's network host/address.<br>

json_name: host
go_name: Host</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
id is the worker's identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-worker-kind">cloud.v1.domain.Worker.Kind</a></td>
<td><pre>
kind is whether this worker is the MASTER or an AGENT.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>last_seen_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
last_seen_at is the last successful register/heartbeat timestamp.<br>

json_name: lastSeenAt
go_name: LastSeenAt</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
machine_id is the host this worker runs on.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>online</td>
<td>bool</td>
<td><pre>
online is true if the agent is currently reachable/polling.<br>

json_name: online
go_name: Online</pre></td>
</tr><tr>
<td>presence</td>
<td><a href="#cloud-v1-monitor-workerpresence">cloud.v1.monitor.WorkerPresence</a></td>
<td><pre>
presence is registry-based liveness; this is the canonical online state.<br>

json_name: presence
go_name: Presence</pre></td>
</tr><tr>
<td>registered_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
registered_at is when the agent registered for this run, if seen.<br>

json_name: registeredAt
go_name: RegisteredAt</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
run_id is the run the agent registry sample is scoped to.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>source</td>
<td><a href="#cloud-v1-monitor-observationsource">cloud.v1.monitor.ObservationSource</a></td>
<td><pre>
source says where the liveness fields came from.<br>

json_name: source
go_name: Source</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
status is the worker's live status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>status_reason</td>
<td>string</td>
<td><pre>
status_reason explains presence/status, especially stale/offline/unknown.<br>

json_name: statusReason
go_name: StatusReason</pre></td>
</tr>
</table>



<a name="cloud-v1-monitor-workerpresence"></a>
### cloud.v1.monitor.WorkerPresence

<pre>
WorkerPresence is registry-based liveness, separate from lifecycle status.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>WORKER_PRESENCE_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>WORKER_PRESENCE_UNKNOWN</td>
<td></td>
</tr><tr>
<td>WORKER_PRESENCE_ONLINE</td>
<td></td>
</tr><tr>
<td>WORKER_PRESENCE_STALE</td>
<td></td>
</tr><tr>
<td>WORKER_PRESENCE_OFFLINE</td>
<td></td>
</tr><tr>
<td>WORKER_PRESENCE_TERMINATED</td>
<td></td>
</tr>
</table>