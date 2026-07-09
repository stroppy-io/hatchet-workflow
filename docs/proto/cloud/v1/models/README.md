

<a name="cloud-v1-models"></a>
# cloud.v1.models

## Table of Contents
- Messages
  - [cloud.v1.models.FavoriteRecord](#cloud-v1-models-favoriterecord)
  - [cloud.v1.models.ObservabilityRefs](#cloud-v1-models-observabilityrefs)
  - [cloud.v1.models.PackageRecord](#cloud-v1-models-packagerecord)
  - [cloud.v1.models.PackageRecord.Format](#cloud-v1-models-packagerecord-format)
  - [cloud.v1.models.PackageRecord.Status](#cloud-v1-models-packagerecord-status)
  - [cloud.v1.models.RecipeBundle](#cloud-v1-models-recipebundle)
  - [cloud.v1.models.RecipeBundle.FilesEntry](#cloud-v1-models-recipebundle-filesentry)
  - [cloud.v1.models.RecipeRecord](#cloud-v1-models-reciperecord)
  - [cloud.v1.models.RecipeRecord.Summary](#cloud-v1-models-reciperecord-summary)
  - [cloud.v1.models.Run](#cloud-v1-models-run)
  - [cloud.v1.models.Run.Summary](#cloud-v1-models-run-summary)
  - [cloud.v1.models.RunTopology](#cloud-v1-models-runtopology)
  - [cloud.v1.models.RunTopology.MachineNode](#cloud-v1-models-runtopology-machinenode)
  - [cloud.v1.models.RunTopology.MachineNode.LabelsEntry](#cloud-v1-models-runtopology-machinenode-labelsentry)
  - [cloud.v1.models.RunTopology.ServiceNode](#cloud-v1-models-runtopology-servicenode)
  - [cloud.v1.models.ShareRecord](#cloud-v1-models-sharerecord)
  - [cloud.v1.models.ShareRecord.Snapshot](#cloud-v1-models-sharerecord-snapshot)
  - [cloud.v1.models.ShareRecord.Target](#cloud-v1-models-sharerecord-target)
  - [cloud.v1.models.ShareRecord.Target.Kind](#cloud-v1-models-sharerecord-target-kind)
  - [cloud.v1.models.SharedSuiteRun](#cloud-v1-models-sharedsuiterun)
  - [cloud.v1.models.SharedTestRun](#cloud-v1-models-sharedtestrun)
  - [cloud.v1.models.TenantSettingsRecord](#cloud-v1-models-tenantsettingsrecord)

<a name="cloud-v1-models-messages"></a>
## Messages

<a name="cloud-v1-models-favoriterecord"></a>
### cloud.v1.models.FavoriteRecord

<pre>
//FavoriteRecord is one user's favorite of one row (a join row in its own table).

//The favoriting user is entity.author_id; the tenant is entity.tenant_id. The
//target is (kind, target_id). There MUST be at most one FavoriteRecord per
//(author_id, kind, target_id) — enforce a unique index on those three; AddFavorite
//is idempotent against it.

//This table is the source of truth for two derived things on other reads:
//- common.Entity.is_favorite (per-caller computed flag), and
//- common.EntityFilter.favorites_only / EntitySortField.FAVORITE,
//which are computed by joining the queried rows against this table for the
//requesting caller. Nothing else stores a favorite flag.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
//entity is the storage envelope: it carries the row id, the favoriting
//user (entity.author_id) and the tenant (entity.tenant_id).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="../common/README.md#cloud-v1-common-favoritekind">cloud.v1.common.FavoriteKind</a></td>
<td><pre>
//kind names which table the favorite points at. Must be a defined,
//non-zero FavoriteKind.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>target_id</td>
<td>string</td>
<td><pre>
//target_id is the id of the favorited row, in the table named by kind.<br>

json_name: targetId
go_name: TargetId</pre></td>
</tr>
</table>



<a name="cloud-v1-models-observabilityrefs"></a>
### cloud.v1.models.ObservabilityRefs

<pre>
//ObservabilityRefs makes the "runtime observations keyed by run id"
//convention (see logs.proto/metrics.proto doc comments) an explicit
//contract on Run instead of an implicit one on TestRunRecord. v1: every
//field is filled with entity.id (metrics/logs) or the relay's existing
//hardcoded uid (grafana) at mint time (Task 3) — SP-F gives these real
//per-provider variance.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>grafana_dashboard_uid</td>
<td>string</td>
<td><pre>
json_name: grafanaDashboardUid
go_name: GrafanaDashboardUid</pre></td>
</tr><tr>
<td>logs_query_key</td>
<td>string</td>
<td><pre>
json_name: logsQueryKey
go_name: LogsQueryKey</pre></td>
</tr><tr>
<td>metrics_query_key</td>
<td>string</td>
<td><pre>
json_name: metricsQueryKey
go_name: MetricsQueryKey</pre></td>
</tr>
</table>



<a name="cloud-v1-models-packagerecord"></a>
### cloud.v1.models.PackageRecord

<pre>
//PackageRecord is a package catalog row — either a tenant-uploaded custom
//package, e.g. a .deb (apt) or a raw binary, or a server-defined built-in
//package for the stock install path. Uploaded package blobs live in object
//storage (S3/MinIO); this row holds only metadata + the storage key. Upload is
//via presigned PUT (see api/package.proto). At install time the agent fetches
//the blob by a file reference (resolved to a presigned download). Tenant-private
//except for built-in rows, which are projected into each tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>arch</td>
<td>string</td>
<td><pre>
//arch is the target CPU architecture, e.g. "amd64".<br>

json_name: arch
go_name: Arch</pre></td>
</tr><tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
//entity is the storage envelope (id, tenant_id, name, timings). Packages
//are tenant-private.<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>format</td>
<td><a href="#cloud-v1-models-packagerecord-format">cloud.v1.models.PackageRecord.Format</a></td>
<td><pre>
//format is the on-disk kind of the blob (deb or raw binary). Must be a
//defined Format value.<br>

json_name: format
go_name: Format</pre></td>
</tr><tr>
<td>is_builtin</td>
<td>bool</td>
<td><pre>
//is_builtin marks server-defined package choices. Built-in package records
//are listed for discoverability but are immutable and have no uploaded blob
//owned by the tenant.<br>

json_name: isBuiltin
go_name: IsBuiltin</pre></td>
</tr><tr>
<td>os</td>
<td>string</td>
<td><pre>
//os is a target host descriptor for informational / matching purposes,
//e.g. "ubuntu-22.04".<br>

json_name: os
go_name: Os</pre></td>
</tr><tr>
<td>sha256</td>
<td>string</td>
<td><pre>
//sha256 is the expected content hash (hex), verified on CompleteUpload.
//Built-in package records do not have a tenant-uploaded blob, so the field
//may be empty for them.<br>

json_name: sha256
go_name: Sha256</pre></td>
</tr><tr>
<td>size_bytes</td>
<td>uint64</td>
<td><pre>
//size_bytes is the size of the uploaded blob in bytes.<br>

json_name: sizeBytes
go_name: SizeBytes</pre></td>
</tr><tr>
<td>status</td>
<td><a href="#cloud-v1-models-packagerecord-status">cloud.v1.models.PackageRecord.Status</a></td>
<td><pre>
//status is the current upload + verification lifecycle state.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>storage_uri</td>
<td>string</td>
<td><pre>
//storage_uri is the object-storage key for the blob. Server-assigned, not
//client-set.<br>

json_name: storageUri
go_name: StorageUri</pre></td>
</tr><tr>
<td>target_db_kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//target_db_kind is which database engine this build targets.<br>

json_name: targetDbKind
go_name: TargetDbKind</pre></td>
</tr><tr>
<td>version</td>
<td>string</td>
<td><pre>
//version is the package version label (free text).<br>

json_name: version
go_name: Version</pre></td>
</tr>
</table>



<a name="cloud-v1-models-packagerecord-format"></a>
### cloud.v1.models.PackageRecord.Format

<pre>
//Format is the on-disk kind of the uploaded package blob.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>FORMAT_UNSPECIFIED</td>
<td><pre>
//FORMAT_UNSPECIFIED is the unset default.
</pre></td>
</tr><tr>
<td>FORMAT_DEB</td>
<td><pre>
//FORMAT_DEB is an apt / dpkg package.
</pre></td>
</tr><tr>
<td>FORMAT_BINARY</td>
<td><pre>
//FORMAT_BINARY is a raw executable / archive run by install commands.
</pre></td>
</tr>
</table>

<a name="cloud-v1-models-packagerecord-status"></a>
### cloud.v1.models.PackageRecord.Status

<pre>
//Status tracks the upload + verification lifecycle of the blob.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>STATUS_UNSPECIFIED</td>
<td><pre>
//STATUS_UNSPECIFIED is the unset default.
</pre></td>
</tr><tr>
<td>STATUS_UPLOADING</td>
<td><pre>
//STATUS_UPLOADING means the record is created but the blob is not yet
//uploaded/verified.
</pre></td>
</tr><tr>
<td>STATUS_READY</td>
<td><pre>
//STATUS_READY means the blob is uploaded and its checksum/size are
//verified.
</pre></td>
</tr><tr>
<td>STATUS_FAILED</td>
<td><pre>
//STATUS_FAILED means verification failed.
</pre></td>
</tr>
</table>

<a name="cloud-v1-models-recipebundle"></a>
### cloud.v1.models.RecipeBundle

<pre>
//RecipeBundle is the recipe's file set: slash-path -> file bytes. Keys are
//the same paths internal/dsl include.Sources uses (cluster.yaml,
//workflow.yaml, components/**, providers/**).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-models-recipebundle-filesentry">cloud.v1.models.RecipeBundle.FilesEntry</a></td>
<td><pre>
//files maps a slash-path within the bundle to its raw file bytes.<br>

json_name: files
go_name: Files</pre></td>
</tr>
</table>



<a name="cloud-v1-models-recipebundle-filesentry"></a>
### cloud.v1.models.RecipeBundle.FilesEntry

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
<td>bytes</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-models-reciperecord"></a>
### cloud.v1.models.RecipeRecord

<pre>
//RecipeRecord is a persisted DSL recipe bundle.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>bundle</td>
<td><a href="#cloud-v1-models-recipebundle">cloud.v1.models.RecipeBundle</a></td>
<td><pre>
//bundle holds the recipe's files (cluster.yaml, workflow.yaml,
//components/**, providers/**).<br>

json_name: bundle
go_name: Bundle</pre></td>
</tr><tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
//entity is the storage envelope (id, tenant_id, name, description,
//timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>summary</td>
<td><a href="#cloud-v1-models-reciperecord-summary">cloud.v1.models.RecipeRecord.Summary</a></td>
<td><pre>
//summary holds denormalized facets for the recipes table (provider,
//machine-group/service counts, last known Check result).<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>version</td>
<td>uint32</td>
<td><pre>
//version is the immutable version number of this bundle. The triple
//(tenant_id, name, version) is unique.<br>

json_name: version
go_name: Version</pre></td>
</tr>
</table>



<a name="cloud-v1-models-reciperecord-summary"></a>
### cloud.v1.models.RecipeRecord.Summary

<pre>
//Summary is the flat, indexed projection of the recipe used by the
//recipes table.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>compiles</td>
<td>bool</td>
<td><pre>
//compiles is the last known Check result: true when the bundle
//compiled with no error diagnostics.<br>

json_name: compiles
go_name: Compiles</pre></td>
</tr><tr>
<td>machine_group_count</td>
<td>uint32</td>
<td><pre>
//machine_group_count is the number of machine groups declared by
//the bundle's cluster.yaml.<br>

json_name: machineGroupCount
go_name: MachineGroupCount</pre></td>
</tr><tr>
<td>provider</td>
<td>string</td>
<td><pre>
//provider is the deployment provider name declared by the bundle.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>service_count</td>
<td>uint32</td>
<td><pre>
//service_count is the number of services declared by the bundle's
//cluster.yaml.<br>

json_name: serviceCount
go_name: ServiceCount</pre></td>
</tr>
</table>



<a name="cloud-v1-models-run"></a>
### cloud.v1.models.Run

<pre>
//Run is a persisted recipe run (RunRecipeWorkflow's only live path — see
//package doc). Replaces TestRunRecord (above): every field here is
//something RunRecipeWorkflow actually produces (compile -> CompiledPlan +
//Baked identity; provision -> topology; execute -> per-job status via
//workflow.RunState; always -> observability refs), unlike TestRunRecord
//whose spec/infrastructure_state/deployment_plan fields a recipe run
//always leaves empty (see TestRunRecord's doc and StartRun's own comment).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>baked</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
baked is the sealed launch-form snapshot (SP-A's schemapb.Bake
output). nil until SP-D's generated-form launch path exists.<br>

json_name: baked
go_name: Baked</pre></td>
</tr><tr>
<td>compiled_plan</td>
<td><a href="../dsl/README.md#cloud-v1-dsl-compiledplan">cloud.v1.dsl.CompiledPlan</a></td>
<td><pre>
compiled_plan is the compiled DSL plan RunRecipeWorkflow executed.
NEW relative to TestRunRecord: today the plan lives only in workflow
memory and is never persisted; see persistRunCompiledPlan (Task 3).<br>

json_name: compiledPlan
go_name: CompiledPlan</pre></td>
</tr><tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>in_global_rating</td>
<td>bool</td>
<td><pre>
json_name: inGlobalRating
go_name: InGlobalRating</pre></td>
</tr><tr>
<td>in_tenant_rating</td>
<td>bool</td>
<td><pre>
json_name: inTenantRating
go_name: InTenantRating</pre></td>
</tr><tr>
<td>observability</td>
<td><a href="#cloud-v1-models-observabilityrefs">cloud.v1.models.ObservabilityRefs</a></td>
<td><pre>
json_name: observability
go_name: Observability</pre></td>
</tr><tr>
<td>runtime_state</td>
<td><a href="../workflow/README.md#cloud-v1-workflow-runstate">cloud.v1.workflow.RunState</a></td>
<td><pre>
runtime_state is the last workflow.RunState RunRecipeWorkflow
persisted (unchanged type/semantics from TestRunRecord.runtime_state).<br>

json_name: runtimeState
go_name: RuntimeState</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>summary</td>
<td><a href="#cloud-v1-models-run-summary">cloud.v1.models.Run.Summary</a></td>
<td><pre>
summary: same 15 fields as TestRunRecord.Summary (denormalized,
queryable facets for the runs table). See Run.Summary below.<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="#cloud-v1-models-runtopology">cloud.v1.models.RunTopology</a></td>
<td><pre>
topology is the provisioned-machine snapshot (renamed
RecipeTopologySnapshot -> RunTopology, see below).<br>

json_name: topology
go_name: Topology</pre></td>
</tr><tr>
<td>trigger</td>
<td><a href="../common/README.md#cloud-v1-common-trigger">cloud.v1.common.Trigger</a></td>
<td><pre>
json_name: trigger
go_name: Trigger</pre></td>
</tr><tr>
<td>workflow_id</td>
<td>string</td>
<td><pre>
workflow_id is the originating models.RecipeRecord.entity.id (ex
recipe_id) that StartRun launched this run from. Stamped once, never
changed. Reserves the name for SP-B's catalog Workflow — until then,
its value is exactly what recipe_id carries on TestRunRecord today.<br>

json_name: workflowId
go_name: WorkflowId</pre></td>
</tr><tr>
<td>workflow_version</td>
<td>string</td>
<td><pre>
workflow_version identifies the bundle version at launch time:
strconv.FormatUint(RecipeRecord.version, 10) (see recipe.go StartRun).<br>

json_name: workflowVersion
go_name: WorkflowVersion</pre></td>
</tr>
</table>



<a name="cloud-v1-models-run-summary"></a>
### cloud.v1.models.Run.Summary

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>db_kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>db_preset_id</td>
<td>string</td>
<td><pre>
json_name: dbPresetId
go_name: DbPresetId</pre></td>
</tr><tr>
<td>db_preset_name</td>
<td>string</td>
<td><pre>
json_name: dbPresetName
go_name: DbPresetName</pre></td>
</tr><tr>
<td>db_version</td>
<td>string</td>
<td><pre>
db_version is the database engine's version, read from the same
DB ServiceSpec image tag db_kind is inferred from (e.g.
"postgres:16" -> "16") — parity with ref's runs table, which
carried a db version the DSL-era table dropped (see
internal/workflows/runrecipe_summary.go's inferDbKind/imageTag).<br>

json_name: dbVersion
go_name: DbVersion</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>node_count</td>
<td>uint32</td>
<td><pre>
json_name: nodeCount
go_name: NodeCount</pre></td>
</tr><tr>
<td>progress_pct</td>
<td>uint32</td>
<td><pre>
json_name: progressPct
go_name: ProgressPct</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr><tr>
<td>test_preset_id</td>
<td>string</td>
<td><pre>
json_name: testPresetId
go_name: TestPresetId</pre></td>
</tr><tr>
<td>test_preset_name</td>
<td>string</td>
<td><pre>
json_name: testPresetName
go_name: TestPresetName</pre></td>
</tr><tr>
<td>topology_label</td>
<td>string</td>
<td><pre>
json_name: topologyLabel
go_name: TopologyLabel</pre></td>
</tr><tr>
<td>workload_name</td>
<td>string</td>
<td><pre>
json_name: workloadName
go_name: WorkloadName</pre></td>
</tr><tr>
<td>workload_preset_id</td>
<td>string</td>
<td><pre>
json_name: workloadPresetId
go_name: WorkloadPresetId</pre></td>
</tr><tr>
<td>workload_protocol</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload-protocol">cloud.v1.domain.Workload.Protocol</a></td>
<td><pre>
json_name: workloadProtocol
go_name: WorkloadProtocol</pre></td>
</tr>
</table>



<a name="cloud-v1-models-runtopology"></a>
### cloud.v1.models.RunTopology

<pre>
//RunTopology replaces RecipeTopologySnapshot (see that message's doc,
//above) — same shape, generalized name: every live run is now a
//recipe/workflow run, so the "Recipe" prefix no longer distinguishes
//anything.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>nodes</td>
<td><a href="#cloud-v1-models-runtopology-machinenode">cloud.v1.models.RunTopology.MachineNode</a></td>
<td><pre>
json_name: nodes
go_name: Nodes</pre></td>
</tr><tr>
<td>provider</td>
<td>string</td>
<td><pre>
json_name: provider
go_name: Provider</pre></td>
</tr>
</table>



<a name="cloud-v1-models-runtopology-machinenode"></a>
### cloud.v1.models.RunTopology.MachineNode

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>group</td>
<td>string</td>
<td><pre>
json_name: group
go_name: Group</pre></td>
</tr><tr>
<td>ip</td>
<td>string</td>
<td><pre>
json_name: ip
go_name: Ip</pre></td>
</tr><tr>
<td>labels</td>
<td><a href="#cloud-v1-models-runtopology-machinenode-labelsentry">cloud.v1.models.RunTopology.MachineNode.LabelsEntry</a></td>
<td><pre>
json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
json_name: nodeId
go_name: NodeId</pre></td>
</tr><tr>
<td>services</td>
<td><a href="#cloud-v1-models-runtopology-servicenode">cloud.v1.models.RunTopology.ServiceNode</a></td>
<td><pre>
json_name: services
go_name: Services</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
json_name: status
go_name: Status</pre></td>
</tr>
</table>



<a name="cloud-v1-models-runtopology-machinenode-labelsentry"></a>
### cloud.v1.models.RunTopology.MachineNode.LabelsEntry

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



<a name="cloud-v1-models-runtopology-servicenode"></a>
### cloud.v1.models.RunTopology.ServiceNode

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>image</td>
<td>string</td>
<td><pre>
json_name: image
go_name: Image</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
json_name: name
go_name: Name</pre></td>
</tr>
</table>



<a name="cloud-v1-models-sharerecord"></a>
### cloud.v1.models.ShareRecord

<pre>
//ShareRecord is a public, read-only share link for a run (test or suite). It
//exposes a LIMITED snapshot to people OUTSIDE the system — no account, no tenant
//membership, just an unguessable token.

//Snapshot model (NOT live): the limited view is FROZEN into `snapshot` and kept
//fresh by a BACKGROUND job that updates the row in the DB while the share is
//alive (so an in-progress run's numbers can still move). The public endpoint
//serves the stored snapshot only — it never reaches into the live system on
//behalf of an anonymous caller, and the share survives the run being deleted.

//Security: token is unguessable (>=128 bit). The snapshot carries ONLY basic
//info + our metrics — never the baked spec, params, secrets, raw logs, shell, or
//tenant data. Revocable + (by default) time-limited.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
//entity is the storage envelope (id, tenant_id, name, timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>expires_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//expires_at is when the share stops working. Unset = NEVER expires —
//insecure and it keeps the background refresh running indefinitely; only
//allow with an explicit warning. Default when creating: 1 week.<br>

json_name: expiresAt
go_name: ExpiresAt</pre></td>
</tr><tr>
<td>revoked</td>
<td>bool</td>
<td><pre>
//revoked, when true, marks the share as manually revoked: the public
//endpoint returns gone regardless of expiry.<br>

json_name: revoked
go_name: Revoked</pre></td>
</tr><tr>
<td>snapshot</td>
<td><a href="#cloud-v1-models-sharerecord-snapshot">cloud.v1.models.ShareRecord.Snapshot</a></td>
<td><pre>
//snapshot is the frozen, background-refreshed limited view served
//publicly.<br>

json_name: snapshot
go_name: Snapshot</pre></td>
</tr><tr>
<td>target</td>
<td><a href="#cloud-v1-models-sharerecord-target">cloud.v1.models.ShareRecord.Target</a></td>
<td><pre>
//target identifies what is shared (which run this link points at).<br>

json_name: target
go_name: Target</pre></td>
</tr><tr>
<td>token</td>
<td>string</td>
<td><pre>
//token is the public, unguessable access token carried in the share URL
//(>=128 bit of entropy).<br>

json_name: token
go_name: Token</pre></td>
</tr>
</table>



<a name="cloud-v1-models-sharerecord-snapshot"></a>
### cloud.v1.models.ShareRecord.Snapshot

<pre>
//Snapshot is the limited public view, refreshed in the background.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>captured_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//captured_at is when the background job last refreshed this snapshot.<br>

json_name: capturedAt
go_name: CapturedAt</pre></td>
</tr><tr>
<td>suite_run</td>
<td><a href="#cloud-v1-models-sharedsuiterun">cloud.v1.models.SharedSuiteRun</a></td>
<td><pre>
//suite_run is the limited projection for a suite run.<br>

json_name: suiteRun
go_name: SuiteRun</pre></td>
</tr><tr>
<td>test_run</td>
<td><a href="#cloud-v1-models-sharedtestrun">cloud.v1.models.SharedTestRun</a></td>
<td><pre>
//test_run is the limited projection for a single test run.<br>

json_name: testRun
go_name: TestRun</pre></td>
</tr>
</table>



<a name="cloud-v1-models-sharerecord-target"></a>
### cloud.v1.models.ShareRecord.Target

<pre>
//Target picks the run this share points at.
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
//id is the id of the targeted TestRunRecord / SuiteRunRecord.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-models-sharerecord-target-kind">cloud.v1.models.ShareRecord.Target.Kind</a></td>
<td><pre>
//kind selects whether id refers to a test run or a suite run. Must be
//a defined, non-zero value.<br>

json_name: kind
go_name: Kind</pre></td>
</tr>
</table>



<a name="cloud-v1-models-sharerecord-target-kind"></a>
### cloud.v1.models.ShareRecord.Target.Kind

<pre>
//Kind names which kind of run the share targets.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td><pre>
//KIND_UNSPECIFIED is the unset default (rejected).
</pre></td>
</tr><tr>
<td>KIND_TEST_RUN</td>
<td><pre>
//KIND_TEST_RUN targets a single TestRunRecord.
</pre></td>
</tr><tr>
<td>KIND_SUITE_RUN</td>
<td><pre>
//KIND_SUITE_RUN targets a SuiteRunRecord.
</pre></td>
</tr>
</table>

<a name="cloud-v1-models-sharedsuiterun"></a>
### cloud.v1.models.SharedSuiteRun

<pre>
//SharedSuiteRun is the LIMITED public projection of a suite run: aggregates +
//the per-test limited views (so an interesting matrix can be shown off).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>completed</td>
<td>uint32</td>
<td><pre>
//completed is the number of child runs that finished successfully.<br>

json_name: completed
go_name: Completed</pre></td>
</tr><tr>
<td>db_kinds</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//db_kinds are the distinct database engines exercised across the suite.<br>

json_name: dbKinds
go_name: DbKinds</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//duration is the suite run's elapsed time.<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>failed</td>
<td>uint32</td>
<td><pre>
//failed is the number of child runs that failed.<br>

json_name: failed
go_name: Failed</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//finished_at is when the suite run finished (unset while running).<br>

json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the suite run's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>progress_pct</td>
<td>uint32</td>
<td><pre>
//progress_pct is the aggregate suite progress as a percentage (0..100).<br>

json_name: progressPct
go_name: ProgressPct</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the single deployment provider the whole suite ran on.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>running</td>
<td>uint32</td>
<td><pre>
//running is the number of child runs currently in progress.<br>

json_name: running
go_name: Running</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_at is when the suite run started.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the suite run's lifecycle status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>tests</td>
<td><a href="#cloud-v1-models-sharedtestrun">cloud.v1.models.SharedTestRun</a></td>
<td><pre>
//tests are the limited child test views.<br>

json_name: tests
go_name: Tests</pre></td>
</tr><tr>
<td>total</td>
<td>uint32</td>
<td><pre>
//total is the total number of child runs.<br>

json_name: total
go_name: Total</pre></td>
</tr>
</table>



<a name="cloud-v1-models-sharedtestrun"></a>
### cloud.v1.models.SharedTestRun

<pre>
//SharedTestRun is the LIMITED public projection of a test run: basic info + our
//metrics (NOT Grafana). No spec/params/secrets/logs.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>db_kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//db_kind is the database engine the run exercised.<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>db_name</td>
<td>string</td>
<td><pre>
//db_name is the preset display label for the database (safe to expose).<br>

json_name: dbName
go_name: DbName</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//duration is the run's elapsed time.<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>finished_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//finished_at is when the run finished (unset while running).<br>

json_name: finishedAt
go_name: FinishedAt</pre></td>
</tr><tr>
<td>metrics</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-runmetrics">cloud.v1.monitor.RunMetrics</a></td>
<td><pre>
//metrics is our computed metrics snapshot (NOT a Grafana link).<br>

json_name: metrics
go_name: Metrics</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the run's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>node_count</td>
<td>uint32</td>
<td><pre>
//node_count is the number of nodes in the topology.<br>

json_name: nodeCount
go_name: NodeCount</pre></td>
</tr><tr>
<td>progress_pct</td>
<td>uint32</td>
<td><pre>
//progress_pct is the run's progress as a percentage (0..100).<br>

json_name: progressPct
go_name: ProgressPct</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the deployment provider the run ran on.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_at is when the run started.<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the run's lifecycle status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
//stroppy_version is the stroppy build that ran the workload.<br>

json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr><tr>
<td>topology_label</td>
<td>string</td>
<td><pre>
//topology_label is the human-readable topology summary.<br>

json_name: topologyLabel
go_name: TopologyLabel</pre></td>
</tr><tr>
<td>workload_name</td>
<td>string</td>
<td><pre>
//workload_name is the display name of the workload.<br>

json_name: workloadName
go_name: WorkloadName</pre></td>
</tr>
</table>



<a name="cloud-v1-models-tenantsettingsrecord"></a>
### cloud.v1.models.TenantSettingsRecord

<pre>
//TenantSettingsRecord is the per-tenant configuration — exactly one row per
//tenant (entity.tenant_id identifies it). Distinct from the global control-plane
//api.PlatformSettings. Starter field set; extend as features need it.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>default_in_global_rating</td>
<td>bool</td>
<td><pre>
//default_in_global_rating is the default global-rating membership for new
//runs. Optional: unset = platform default (global false).<br>

json_name: defaultInGlobalRating
go_name: DefaultInGlobalRating</pre></td>
</tr><tr>
<td>default_in_tenant_rating</td>
<td>bool</td>
<td><pre>
//default_in_tenant_rating is the default tenant-rating membership for new
//runs (see TestRunRecord). Optional: unset = platform default (tenant
//true).<br>

json_name: defaultInTenantRating
go_name: DefaultInTenantRating</pre></td>
</tr><tr>
<td>default_max_parallel</td>
<td>uint32</td>
<td><pre>
//default_max_parallel is the default suite parallelism. 0 = unlimited.<br>

json_name: defaultMaxParallel
go_name: DefaultMaxParallel</pre></td>
</tr><tr>
<td>default_provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//default_provider is the deployment provider preselected in the wizards
//for this tenant.<br>

json_name: defaultProvider
go_name: DefaultProvider</pre></td>
</tr><tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
//entity is the storage envelope; entity.tenant_id identifies which tenant
//these settings belong to (exactly one row per tenant).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>run_retention_days</td>
<td>uint32</td>
<td><pre>
//run_retention_days is the retention for finished runs, in days.
//0 = keep forever.<br>

json_name: runRetentionDays
go_name: RunRetentionDays</pre></td>
</tr><tr>
<td>yandex_settings</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-yandex-settings">cloud.v1.deployment.Yandex.Settings</a></td>
<td><pre>
//Per-provider configuration for this tenant: one ProviderSettings per
//provider the tenant has set up (credentials / region / global sizing
//policy, as baked values against the provider's settings schema). This is
//what the wizards/deploy use as the provider base. SENSITIVE: the baked
//values carry secrets (mark secret fields in the schema); guard reads.
//At most one entry per Provider.<br>

json_name: yandexSettings
go_name: YandexSettings</pre></td>
</tr>
</table>

