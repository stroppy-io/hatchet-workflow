

<a name="cloud-v1-models"></a>
# cloud.v1.models

## Table of Contents
- Messages
  - [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-databasepresetrecord)
  - [cloud.v1.models.FavoriteRecord](#cloud-v1-models-favoriterecord)
  - [cloud.v1.models.PackageRecord](#cloud-v1-models-packagerecord)
  - [cloud.v1.models.PackageRecord.Format](#cloud-v1-models-packagerecord-format)
  - [cloud.v1.models.PackageRecord.Status](#cloud-v1-models-packagerecord-status)
  - [cloud.v1.models.ShareRecord](#cloud-v1-models-sharerecord)
  - [cloud.v1.models.ShareRecord.Snapshot](#cloud-v1-models-sharerecord-snapshot)
  - [cloud.v1.models.ShareRecord.Target](#cloud-v1-models-sharerecord-target)
  - [cloud.v1.models.ShareRecord.Target.Kind](#cloud-v1-models-sharerecord-target-kind)
  - [cloud.v1.models.SharedSuiteRun](#cloud-v1-models-sharedsuiterun)
  - [cloud.v1.models.SharedTestRun](#cloud-v1-models-sharedtestrun)
  - [cloud.v1.models.SuiteRecord](#cloud-v1-models-suiterecord)
  - [cloud.v1.models.SuiteRecord.Summary](#cloud-v1-models-suiterecord-summary)
  - [cloud.v1.models.SuiteRunRecord](#cloud-v1-models-suiterunrecord)
  - [cloud.v1.models.SuiteRunRecord.Summary](#cloud-v1-models-suiterunrecord-summary)
  - [cloud.v1.models.SuiteWizardDraftRecord](#cloud-v1-models-suitewizarddraftrecord)
  - [cloud.v1.models.SuiteWizardDraftRecord.Cell](#cloud-v1-models-suitewizarddraftrecord-cell)
  - [cloud.v1.models.TenantSettingsRecord](#cloud-v1-models-tenantsettingsrecord)
  - [cloud.v1.models.TestPresetRecord](#cloud-v1-models-testpresetrecord)
  - [cloud.v1.models.TestRunRecord](#cloud-v1-models-testrunrecord)
  - [cloud.v1.models.TestRunRecord.Summary](#cloud-v1-models-testrunrecord-summary)
  - [cloud.v1.models.TestWizardDraftRecord](#cloud-v1-models-testwizarddraftrecord)
  - [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-workloadpresetrecord)

<a name="cloud-v1-models-messages"></a>
## Messages

<a name="cloud-v1-models-databasepresetrecord"></a>
### cloud.v1.models.DatabasePresetRecord

<pre>
//DatabasePresetRecord table: a reusable, provider-agnostic database
//configuration that can be referenced by tests and suites.
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
//database is the baked, params-only database configuration payload.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
//entity is the storage envelope (id, tenant_id, name, description,
//timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>is_system</td>
<td>bool</td>
<td><pre>
//is_system marks a platform-seeded preset. System presets are READ-ONLY:
//the service rejects Update and Delete on them. To customize one, Clone it
//into a new editable tenant preset (is_system = false). Server-managed;
//clients cannot set it true.<br>

json_name: isSystem
go_name: IsSystem</pre></td>
</tr>
</table>



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



<a name="cloud-v1-models-packagerecord"></a>
### cloud.v1.models.PackageRecord

<pre>
//PackageRecord is a tenant-uploaded custom package — e.g. a .deb (apt) or a raw
//binary — used to install a custom database build instead of the stock version.
//The blob lives in object storage (S3/MinIO); this row holds only metadata + the
//storage key. Upload is via presigned PUT (see api/package.proto). At install
//time the agent fetches the blob by a file reference (resolved to a presigned
//download). Tenant-private.
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
//sha256 is the expected content hash (hex), verified on CompleteUpload.<br>

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



<a name="cloud-v1-models-suiterecord"></a>
### cloud.v1.models.SuiteRecord

<pre>
//SuiteRecord is a persisted suite DEFINITION (reusable template). It is created
//by the suite wizard's finish and is the thing SuiteAPI.Start expands into a
//SuiteRunRecord.
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
//entity is the storage envelope (id, tenant_id, name, description,
//timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>spec</td>
<td><a href="../domain/README.md#cloud-v1-domain-suite">cloud.v1.domain.Suite</a></td>
<td><pre>
//spec is the suite definition payload that SuiteAPI.Start expands into a
//SuiteRunRecord.<br>

json_name: spec
go_name: Spec</pre></td>
</tr><tr>
<td>summary</td>
<td><a href="#cloud-v1-models-suiterecord-summary">cloud.v1.models.SuiteRecord.Summary</a></td>
<td><pre>
//summary holds denormalized facets for the suites table (schedule state +
//last-run info).<br>

json_name: summary
go_name: Summary</pre></td>
</tr>
</table>



<a name="cloud-v1-models-suiterecord-summary"></a>
### cloud.v1.models.SuiteRecord.Summary

<pre>
//Summary is the flat, indexed projection of the suite definition used by
//the suites table.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cron</td>
<td>string</td>
<td><pre>
//cron mirrors spec.schedule.cron.<br>

json_name: cron
go_name: Cron</pre></td>
</tr><tr>
<td>last_run_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//last_run_at is the last time the suite was started (manual or cron).<br>

json_name: lastRunAt
go_name: LastRunAt</pre></td>
</tr><tr>
<td>last_run_status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//last_run_status is the status of the last suite run.<br>

json_name: lastRunStatus
go_name: LastRunStatus</pre></td>
</tr><tr>
<td>next_run_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//next_run_at is the next planned auto-run computed from the cron;
//unset if scheduling is disabled.<br>

json_name: nextRunAt
go_name: NextRunAt</pre></td>
</tr><tr>
<td>run_count</td>
<td>uint32</td>
<td><pre>
//run_count is how many suite runs this definition has spawned.<br>

json_name: runCount
go_name: RunCount</pre></td>
</tr><tr>
<td>schedule_enabled</td>
<td>bool</td>
<td><pre>
//schedule_enabled mirrors spec.schedule.enabled for fast filter/sort.<br>

json_name: scheduleEnabled
go_name: ScheduleEnabled</pre></td>
</tr>
</table>



<a name="cloud-v1-models-suiterunrecord"></a>
### cloud.v1.models.SuiteRunRecord

<pre>
//SuiteRunRecord is a persisted suite EXECUTION. It references its expanded child
//runs by id (each is a first-class TestRunRecord whose suite_run_id points back
//here), so suite children list / track / show logs+metrics like any other run.
//SuiteWorkflow receives a domain.SuiteRun assembled from the children at start.
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
<td>max_parallel</td>
<td>uint32</td>
<td><pre>
//max_parallel is the max number of concurrent child TestWorkflows.
//0 = unlimited.<br>

json_name: maxParallel
go_name: MaxParallel</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the lifecycle status of the suite run as a whole.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>suite_id</td>
<td>string</td>
<td><pre>
//suite_id references the SuiteRecord definition this run came from.<br>

json_name: suiteId
go_name: SuiteId</pre></td>
</tr><tr>
<td>summary</td>
<td><a href="#cloud-v1-models-suiterunrecord-summary">cloud.v1.models.SuiteRunRecord.Summary</a></td>
<td><pre>
//summary holds denormalized, queryable facets for the suite-runs table
//(fewer than a test run: mostly child-count aggregates + provider +
//timing).<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>test_run_ids</td>
<td>string</td>
<td><pre>
//test_run_ids are the ids of the child TestRunRecord rows this suite run
//expanded into.<br>

json_name: testRunIds
go_name: TestRunIds</pre></td>
</tr><tr>
<td>trigger</td>
<td><a href="../common/README.md#cloud-v1-common-trigger">cloud.v1.common.Trigger</a></td>
<td><pre>
//trigger records how this run was triggered (MANUAL / CRON / API).<br>

json_name: trigger
go_name: Trigger</pre></td>
</tr>
</table>



<a name="cloud-v1-models-suiterunrecord-summary"></a>
### cloud.v1.models.SuiteRunRecord.Summary

<pre>
//Summary is the flat, indexed projection of the suite run used by the
//suite-runs table.
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
//db_kinds are the distinct database kinds exercised by the suite (for
//filtering).<br>

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
<td>pending</td>
<td>uint32</td>
<td><pre>
//pending is the number of child runs not yet started.<br>

json_name: pending
go_name: Pending</pre></td>
</tr><tr>
<td>progress_pct</td>
<td>uint32</td>
<td><pre>
//progress_pct is the aggregate progress (0..100), derived on the
//backend from the children.<br>

json_name: progressPct
go_name: ProgressPct</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the single provider the whole suite ran on.<br>

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
<td>suite_name</td>
<td>string</td>
<td><pre>
//suite_name is the name of the originating suite definition.<br>

json_name: suiteName
go_name: SuiteName</pre></td>
</tr><tr>
<td>total</td>
<td>uint32</td>
<td><pre>
//total is the total number of child runs.<br>

json_name: total
go_name: Total</pre></td>
</tr>
</table>



<a name="cloud-v1-models-suitewizarddraftrecord"></a>
### cloud.v1.models.SuiteWizardDraftRecord

<pre>
//SuiteWizardDraft is the server-held, mutable state of a SUITE wizard.

//Big-schema model, like the test wizard: the whole suite form is ONE conditional
//schemapb schema in `form`. It carries the selections (db / workload / test
//preset ids), the single provider, the per-topology provider settings (one
//branch per selected db preset, gated/emitted by the server), and max_parallel.
//Presets already hold baked db/workload params, so the suite wizard does NOT
//re-fill those — it only composes presets and fills the provider settings that
//differ per topology. The server validates the form, prunes the matrix to
//workload<->db compatible pairs (a root CEL rule), expands the preview and
//recomputes readiness on every patch.

//Persistence: own table (tenant-scoped via Entity) + in-memory cache. On finish
//it bakes into a domain.SuiteRun (the full N*M TestRuns).
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
//entity is the storage envelope (tenant-scoped: id, tenant_id, name,
//timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>errors</td>
<td><a href="../../../schemapb/README.md#schemapb-fielderror">schemapb.FieldError</a></td>
<td><pre>
//errors are the authoritative validation errors (recomputed on every
//patch); paths group by section in the UI.<br>

json_name: errors
go_name: Errors</pre></td>
</tr><tr>
<td>form</td>
<td><a href="../../../schemapb/README.md#schemapb-filled">schemapb.Filled</a></td>
<td><pre>
//form is the whole suite form: selections + provider + per-topology
//provider settings + max_parallel, as one conditional schema + values.<br>

json_name: form
go_name: Form</pre></td>
</tr><tr>
<td>preview</td>
<td><a href="#cloud-v1-models-suitewizarddraftrecord-cell">cloud.v1.models.SuiteWizardDraftRecord.Cell</a></td>
<td><pre>
//preview holds the server-computed expanded, compatible cells (recomputed
//on every patch). These are lightweight summaries; full TestRuns are baked
//only at finish to avoid generating a topology per cell here.<br>

json_name: preview
go_name: Preview</pre></td>
</tr><tr>
<td>ready</td>
<td>bool</td>
<td><pre>
//ready is true when there is >=1 compatible cell, every involved db preset
//has its provider settings filled, and the form validates.<br>

json_name: ready
go_name: Ready</pre></td>
</tr>
</table>



<a name="cloud-v1-models-suitewizarddraftrecord-cell"></a>
### cloud.v1.models.SuiteWizardDraftRecord.Cell

<pre>
//Cell is one resolved (db, workload) pair the suite will run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>compatible</td>
<td>bool</td>
<td><pre>
//compatible is true when the workload is compatible with the database
//kind.<br>

json_name: compatible
go_name: Compatible</pre></td>
</tr><tr>
<td>db_preset_id</td>
<td>string</td>
<td><pre>
//db_preset_id is the database preset from a db x workload matrix pair
//(paired with workload_preset_id).<br>

json_name: dbPresetId
go_name: DbPresetId</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the display name of the resulting run.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>ready</td>
<td>bool</td>
<td><pre>
//ready is true when everything this cell needs (incl. its provider
//settings) is present.<br>

json_name: ready
go_name: Ready</pre></td>
</tr><tr>
<td>test_preset_id</td>
<td>string</td>
<td><pre>
//test_preset_id is set when the cell came from a full TestPreset
//instead of a matrix pair (then db/workload preset ids are empty).<br>

json_name: testPresetId
go_name: TestPresetId</pre></td>
</tr><tr>
<td>workload_preset_id</td>
<td>string</td>
<td><pre>
//workload_preset_id is the workload preset from a db x workload matrix
//pair (paired with db_preset_id).<br>

json_name: workloadPresetId
go_name: WorkloadPresetId</pre></td>
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
<td>providers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-providersettings">cloud.v1.deployment.ProviderSettings</a></td>
<td><pre>
//Per-provider configuration for this tenant: one ProviderSettings per
//provider the tenant has set up (credentials / region / global sizing
//policy, as baked values against the provider's settings schema). This is
//what the wizards/deploy use as the provider base. SENSITIVE: the baked
//values carry secrets (mark secret fields in the schema); guard reads.
//At most one entry per Provider.<br>

json_name: providers
go_name: Providers</pre></td>
</tr><tr>
<td>run_retention_days</td>
<td>uint32</td>
<td><pre>
//run_retention_days is the retention for finished runs, in days.
//0 = keep forever.<br>

json_name: runRetentionDays
go_name: RunRetentionDays</pre></td>
</tr>
</table>



<a name="cloud-v1-models-testpresetrecord"></a>
### cloud.v1.models.TestPresetRecord

<pre>
//TestPresetRecord table: a reusable database + workload combo bundling both
//sides into a single preset.
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
//entity is the storage envelope (id, tenant_id, name, description,
//timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>is_system</td>
<td>bool</td>
<td><pre>
//is_system marks a platform-seeded preset. System presets are READ-ONLY:
//the service rejects Update and Delete on them. To customize one, Clone it
//into a new editable tenant preset (is_system = false). Server-managed;
//clients cannot set it true.<br>

json_name: isSystem
go_name: IsSystem</pre></td>
</tr><tr>
<td>test</td>
<td><a href="../domain/README.md#cloud-v1-domain-test">cloud.v1.domain.Test</a></td>
<td><pre>
//test is the baked database + workload combination payload.<br>

json_name: test
go_name: Test</pre></td>
</tr>
</table>



<a name="cloud-v1-models-testrunrecord"></a>
### cloud.v1.models.TestRunRecord

<pre>
//TestRunRecord is a persisted test execution (one table). The baked spec is the
//input to TestWorkflow; status tracks the run lifecycle; suite_run_id links it to
//a parent SuiteRunRecord when the run is part of a suite (empty = standalone).

//`spec` is a baked Struct and is NOT queryable. For the runs table (filter / sort
/// display of db, workload, preset, topology, progress, duration, ...) the server
//DENORMALIZES those into flat columns in `summary`, filled at Start and updated
//as the run progresses.

//Runtime observations (logs/metrics) are keyed by the run id directly (no dag
//id) — see monitor/logs.proto, monitor/metrics.proto.
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
<td>in_global_rating</td>
<td>bool</td>
<td><pre>
//in_global_rating is the global-leaderboard membership, set at creation.
//Defaults FALSE: publishing to the cross-system / public leaderboard is
//explicit opt-in. Both global views (public + system-wide private) key off
//this flag.<br>

json_name: inGlobalRating
go_name: InGlobalRating</pre></td>
</tr><tr>
<td>in_tenant_rating</td>
<td>bool</td>
<td><pre>
//in_tenant_rating is the tenant-leaderboard membership, set at creation
//(any path: manual/wizard/suite/cron). Defaults TRUE: the run counts in
//this tenant's leaderboard.<br>

json_name: inTenantRating
go_name: InTenantRating</pre></td>
</tr><tr>
<td>spec</td>
<td><a href="../domain/README.md#cloud-v1-domain-testrun">cloud.v1.domain.TestRun</a></td>
<td><pre>
//spec is the baked run spec — the TestWorkflow input (for details/relaunch,
//NOT for queries).<br>

json_name: spec
go_name: Spec</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the run lifecycle status
//(PENDING/RUNNING/COMPLETED/FAILED/CANCELLING/CANCELLED/...).<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>suite_run_id</td>
<td>string</td>
<td><pre>
//suite_run_id is the owning suite run; empty for a standalone run.<br>

json_name: suiteRunId
go_name: SuiteRunId</pre></td>
</tr><tr>
<td>summary</td>
<td><a href="#cloud-v1-models-testrunrecord-summary">cloud.v1.models.TestRunRecord.Summary</a></td>
<td><pre>
//summary holds denormalized, queryable facets for the runs table (incl.
//progress_pct, which the backend derives from the live pipeline /
//Temporal).<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>trigger</td>
<td><a href="../common/README.md#cloud-v1-common-trigger">cloud.v1.common.Trigger</a></td>
<td><pre>
//trigger is the root cause of the run: MANUAL / CRON / API. For a suite
//child it carries the PARENT suite run's trigger (e.g. CRON), while suite
//membership is shown by suite_run_id. So "cron + from suite" = trigger=CRON
//&& suite_run_id set.<br>

json_name: trigger
go_name: Trigger</pre></td>
</tr>
</table>



<a name="cloud-v1-models-testrunrecord-summary"></a>
### cloud.v1.models.TestRunRecord.Summary

<pre>
//Summary is the flat, indexed projection of the run used by the table:
//every field is filterable and sortable without touching the baked spec.
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
//db_kind is the database engine the run targets (database facet).<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>db_preset_id</td>
<td>string</td>
<td><pre>
//db_preset_id is the id of the database preset used.<br>

json_name: dbPresetId
go_name: DbPresetId</pre></td>
</tr><tr>
<td>db_preset_name</td>
<td>string</td>
<td><pre>
//db_preset_name is the display name of the database preset used.<br>

json_name: dbPresetName
go_name: DbPresetName</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//duration is finished_at - started_at, or the live elapsed time while
//running.<br>

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
//progress_pct (0..100) is the run progress (runtime facet), derived on
//the backend from the pipeline / Temporal.<br>

json_name: progressPct
go_name: ProgressPct</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the deployment provider the run ran on (provider facet).<br>

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
//topology_label is the human-readable topology summary (topology
//facet), e.g. "PG HA x3".<br>

json_name: topologyLabel
go_name: TopologyLabel</pre></td>
</tr><tr>
<td>workload_name</td>
<td>string</td>
<td><pre>
//workload_name is the display name of the workload.<br>

json_name: workloadName
go_name: WorkloadName</pre></td>
</tr><tr>
<td>workload_preset_id</td>
<td>string</td>
<td><pre>
//workload_preset_id is the id of the workload preset used (workload
//facet).<br>

json_name: workloadPresetId
go_name: WorkloadPresetId</pre></td>
</tr>
</table>



<a name="cloud-v1-models-testwizarddraftrecord"></a>
### cloud.v1.models.TestWizardDraftRecord

<pre>
//TestWizardDraft is the server-held, mutable state of a TEST wizard.

//Big-schema model: the whole test form is ONE conditional schemapb schema,
//carried in `form` (a Filled = schema + values). Database kind, database params,
//workload, provider and provider settings all live under their paths in
//form.values; conditional branches (e.g. provider sizing per topology) are gated
//by schemapb `when` (validated/shown only when their CEL condition holds). The
//server builds the schema, validates the whole form authoritatively, regenerates
//the topology and recomputes readiness on every patch. The frontend renders the
//form straight from `form` (schemapb ts sdk + cel-es for live UX) and sends back
//a patched Filled.

//Persistence: own table (tenant-scoped via Entity) + in-memory cache. On finish
//it bakes into a domain.TestRun.
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
//entity is the storage envelope (tenant-scoped: id, tenant_id, name,
//timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>errors</td>
<td><a href="../../../schemapb/README.md#schemapb-fielderror">schemapb.FieldError</a></td>
<td><pre>
//errors are the current authoritative validation errors (recomputed on
//every patch); FieldError.field carries the path so the UI can group by
//section (database.*, workload.*, provider.*).<br>

json_name: errors
go_name: Errors</pre></td>
</tr><tr>
<td>form</td>
<td><a href="../../../schemapb/README.md#schemapb-filled">schemapb.Filled</a></td>
<td><pre>
//form is the whole test form: one big conditional schema + its current
//values (a Filled = schema + values).<br>

json_name: form
go_name: Form</pre></td>
</tr><tr>
<td>ready</td>
<td>bool</td>
<td><pre>
//ready is true when the whole form validates and FinishTestWizard is
//allowed.<br>

json_name: ready
go_name: Ready</pre></td>
</tr><tr>
<td>test_preset_id</td>
<td>string</td>
<td><pre>
//test_preset_id is the test preset the wizard was seeded from, if it
//started from one.<br>

json_name: testPresetId
go_name: TestPresetId</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the server-computed topology generated from the current form
//values (recomputed on every patch): abstract machines, with provider_parms
//filled once provider settings are valid.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>



<a name="cloud-v1-models-workloadpresetrecord"></a>
### cloud.v1.models.WorkloadPresetRecord

<pre>
//WorkloadPresetRecord table: a reusable stroppy workload configuration.
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
//entity is the storage envelope (id, tenant_id, name, description,
//timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>is_system</td>
<td>bool</td>
<td><pre>
//is_system marks a platform-seeded preset. System presets are READ-ONLY:
//the service rejects Update and Delete on them. To customize one, Clone it
//into a new editable tenant preset (is_system = false). Server-managed;
//clients cannot set it true.<br>

json_name: isSystem
go_name: IsSystem</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the baked, params-only stroppy workload configuration
//payload.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>

