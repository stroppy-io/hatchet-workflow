

<a name="cloud-v1-models"></a>
# cloud.v1.models

## Table of Contents
- Messages
  - [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-databasepresetrecord)
  - [cloud.v1.models.DatabasePresetRecord.Summary](#cloud-v1-models-databasepresetrecord-summary)
  - [cloud.v1.models.FavoriteRecord](#cloud-v1-models-favoriterecord)
  - [cloud.v1.models.PackageRecord](#cloud-v1-models-packagerecord)
  - [cloud.v1.models.PackageRecord.Format](#cloud-v1-models-packagerecord-format)
  - [cloud.v1.models.PackageRecord.Status](#cloud-v1-models-packagerecord-status)
  - [cloud.v1.models.ShareRecord](#cloud-v1-models-sharerecord)
  - [cloud.v1.models.ShareRecord.Snapshot](#cloud-v1-models-sharerecord-snapshot)
  - [cloud.v1.models.ShareRecord.Target](#cloud-v1-models-sharerecord-target)
  - [cloud.v1.models.ShareRecord.Target.Kind](#cloud-v1-models-sharerecord-target-kind)
  - [cloud.v1.models.SharedDatabase](#cloud-v1-models-shareddatabase)
  - [cloud.v1.models.SharedDatabase.Setting](#cloud-v1-models-shareddatabase-setting)
  - [cloud.v1.models.SharedMachine](#cloud-v1-models-sharedmachine)
  - [cloud.v1.models.SharedMachine.Disk](#cloud-v1-models-sharedmachine-disk)
  - [cloud.v1.models.SharedSuiteRun](#cloud-v1-models-sharedsuiterun)
  - [cloud.v1.models.SharedTestRun](#cloud-v1-models-sharedtestrun)
  - [cloud.v1.models.SharedWorkloadSegment](#cloud-v1-models-sharedworkloadsegment)
  - [cloud.v1.models.SuiteRecord](#cloud-v1-models-suiterecord)
  - [cloud.v1.models.SuiteRecord.Summary](#cloud-v1-models-suiterecord-summary)
  - [cloud.v1.models.SuiteRunRecord](#cloud-v1-models-suiterunrecord)
  - [cloud.v1.models.SuiteRunRecord.ChildRun](#cloud-v1-models-suiterunrecord-childrun)
  - [cloud.v1.models.SuiteRunRecord.Summary](#cloud-v1-models-suiterunrecord-summary)
  - [cloud.v1.models.SuiteWizardDraftRecord](#cloud-v1-models-suitewizarddraftrecord)
  - [cloud.v1.models.SuiteWizardDraftRecord.Cell](#cloud-v1-models-suitewizarddraftrecord-cell)
  - [cloud.v1.models.TenantSettingsRecord](#cloud-v1-models-tenantsettingsrecord)
  - [cloud.v1.models.TestPresetRecord](#cloud-v1-models-testpresetrecord)
  - [cloud.v1.models.TestPresetRecord.Summary](#cloud-v1-models-testpresetrecord-summary)
  - [cloud.v1.models.TestRunRecord](#cloud-v1-models-testrunrecord)
  - [cloud.v1.models.TestRunRecord.Summary](#cloud-v1-models-testrunrecord-summary)
  - [cloud.v1.models.TestWizardDraftRecord](#cloud-v1-models-testwizarddraftrecord)
  - [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-workloadpresetrecord)
  - [cloud.v1.models.WorkloadPresetRecord.Summary](#cloud-v1-models-workloadpresetrecord-summary)

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
</tr><tr>
<td>summary</td>
<td><a href="#cloud-v1-models-databasepresetrecord-summary">cloud.v1.models.DatabasePresetRecord.Summary</a></td>
<td><pre>
//summary is the denormalized projection used by preset pickers and suite
//matrix screens without decoding the whole database body.<br>

json_name: summary
go_name: Summary</pre></td>
</tr>
</table>



<a name="cloud-v1-models-databasepresetrecord-summary"></a>
### cloud.v1.models.DatabasePresetRecord.Summary

<pre>
//Summary is filled by the server from `database`.
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
//db_kind is the database engine kind.<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>external</td>
<td>bool</td>
<td><pre>
//external is true when the preset targets an already-running database.<br>

json_name: external
go_name: External</pre></td>
</tr><tr>
<td>version</td>
<td>string</td>
<td><pre>
//version is the self-deploy engine version, when present.<br>

json_name: version
go_name: Version</pre></td>
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

<a name="cloud-v1-models-shareddatabase"></a>
### cloud.v1.models.SharedDatabase

<pre>
//SharedDatabase is the SAFE projection of the database under test.

//`settings` carries engine-specific sizing and tuning, but every key is chosen
//server-side from a TYPED field of the engine's params (replica counts, pdisk
//counts, shared_buffers_mb, …). The free-form `*_options` maps — postgresql.conf,
//haproxy.cfg, patroni.yml and friends — are deliberately NOT projected: they
//are whatever the author typed, and may hold credentials.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="#cloud-v1-models-shareddatabase-setting">cloud.v1.models.SharedDatabase.Setting</a></td>
<td><pre>
settings are typed, server-selected sizing/tuning knobs, in display order.<br>

json_name: settings
go_name: Settings</pre></td>
</tr><tr>
<td>version</td>
<td>string</td>
<td><pre>
version is the engine version (e.g. "17", "pg17").<br>

json_name: version
go_name: Version</pre></td>
</tr>
</table>



<a name="cloud-v1-models-shareddatabase-setting"></a>
### cloud.v1.models.SharedDatabase.Setting

<pre>
//Setting is one server-selected knob. Both key and value originate from a
//typed proto field — never from a user-supplied map entry.
</pre>

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



<a name="cloud-v1-models-sharedmachine"></a>
### cloud.v1.models.SharedMachine

<pre>
//SharedMachine is the SAFE projection of one provisioned VM's hardware.

//Every field is a typed, validated value from the run's infrastructure plan
//(cores/memory/disk/zone are all `> 0` / closed-set enforced). The plan's
//provider `settings` — which sit right next to this and carry the cloud TOKEN,
//ssh public key and cloud/folder ids — are NEVER read here; only the typed
//per-VM sizing and the platform/zone enums.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>boot_disk_gb</td>
<td>uint64</td>
<td><pre>
boot_disk_gb is the boot disk size in GiB.<br>

json_name: bootDiskGb
go_name: BootDiskGb</pre></td>
</tr><tr>
<td>boot_disk_type</td>
<td>string</td>
<td><pre>
boot_disk_type is the boot disk class (network-ssd, …).<br>

json_name: bootDiskType
go_name: BootDiskType</pre></td>
</tr><tr>
<td>cores</td>
<td>uint32</td>
<td><pre>
cores is the vCPU count.<br>

json_name: cores
go_name: Cores</pre></td>
</tr><tr>
<td>memory_gb</td>
<td>uint64</td>
<td><pre>
memory_gb is RAM in GiB.<br>

json_name: memoryGb
go_name: MemoryGb</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
node_id is the machine's role name (e.g. "postgres-master").<br>

json_name: nodeId
go_name: NodeId</pre></td>
</tr><tr>
<td>platform</td>
<td>string</td>
<td><pre>
platform is the compute platform tier (from the plan's provider settings).<br>

json_name: platform
go_name: Platform</pre></td>
</tr><tr>
<td>secondary_disks</td>
<td><a href="#cloud-v1-models-sharedmachine-disk">cloud.v1.models.SharedMachine.Disk</a></td>
<td><pre>
secondary_disks are extra attached data disks.<br>

json_name: secondaryDisks
go_name: SecondaryDisks</pre></td>
</tr><tr>
<td>zone</td>
<td>string</td>
<td><pre>
zone is the availability zone the VM ran in.<br>

json_name: zone
go_name: Zone</pre></td>
</tr>
</table>



<a name="cloud-v1-models-sharedmachine-disk"></a>
### cloud.v1.models.SharedMachine.Disk

<pre>
//Disk is one attached data disk's size + class.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>size_gb</td>
<td>uint32</td>
<td><pre>
json_name: sizeGb
go_name: SizeGb</pre></td>
</tr><tr>
<td>type</td>
<td>string</td>
<td><pre>
json_name: type
go_name: Type</pre></td>
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
<td>database</td>
<td><a href="#cloud-v1-models-shareddatabase">cloud.v1.models.SharedDatabase</a></td>
<td><pre>
//database is the sizing + typed tuning of the database under test.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
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
<td>machines</td>
<td><a href="#cloud-v1-models-sharedmachine">cloud.v1.models.SharedMachine</a></td>
<td><pre>
//machines are the per-VM hardware the run was provisioned on, so a shared
//result carries the iron it ran on.<br>

json_name: machines
go_name: Machines</pre></td>
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
</tr><tr>
<td>workload_segments</td>
<td><a href="#cloud-v1-models-sharedworkloadsegment">cloud.v1.models.SharedWorkloadSegment</a></td>
<td><pre>
//workload_segments are the stroppy launch knobs, so a shared result is
//reproducible without handing over the run spec.<br>

json_name: workloadSegments
go_name: WorkloadSegments</pre></td>
</tr>
</table>



<a name="cloud-v1-models-sharedworkloadsegment"></a>
### cloud.v1.models.SharedWorkloadSegment

<pre>
//SharedWorkloadSegment is the SAFE projection of one stroppy workload segment:
//the knobs that explain a number, and nothing else.

//Deliberately absent, and never to be added: `parameters.env` (a map that
//routinely carries PGPASSWORD and API tokens), `segment.sql` and
//`segment.files` (private schemas and queries) and `execution.extra_args`
//(arbitrary flags, e.g. `--token=`). Those are free-form: the system cannot
//know they are safe, so a public view must not carry them.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>bulk_size</td>
<td>uint32</td>
<td><pre>
bulk_size is rows per bulk INSERT (only meaningful for plain_bulk).<br>

json_name: bulkSize
go_name: BulkSize</pre></td>
</tr><tr>
<td>duration</td>
<td>string</td>
<td><pre>
duration bounds a time-bounded segment (empty when iteration-bounded).<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>insert_method</td>
<td>string</td>
<td><pre>
insert_method is the row-insertion mode ("native", "plain_bulk", …).<br>

json_name: insertMethod
go_name: InsertMethod</pre></td>
</tr><tr>
<td>iterations</td>
<td>uint32</td>
<td><pre>
iterations bounds an iteration-bounded segment.<br>

json_name: iterations
go_name: Iterations</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the segment's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>no_steps</td>
<td>string</td>
<td><pre>
no_steps are the explicitly disabled steps.<br>

json_name: noSteps
go_name: NoSteps</pre></td>
</tr><tr>
<td>no_thresholds</td>
<td>bool</td>
<td><pre>
no_thresholds disables k6 threshold checks.<br>

json_name: noThresholds
go_name: NoThresholds</pre></td>
</tr><tr>
<td>pool_size</td>
<td>uint32</td>
<td><pre>
pool_size is stroppy's DB connection pool size.<br>

json_name: poolSize
go_name: PoolSize</pre></td>
</tr><tr>
<td>quiet</td>
<td>bool</td>
<td><pre>
quiet suppresses stroppy's per-iteration output.<br>

json_name: quiet
go_name: Quiet</pre></td>
</tr><tr>
<td>scale_factor</td>
<td>double</td>
<td><pre>
scale_factor is the dataset scale.<br>

json_name: scaleFactor
go_name: ScaleFactor</pre></td>
</tr><tr>
<td>script</td>
<td>string</td>
<td><pre>
script is the stroppy script the segment ran (e.g. "tpcc/tx").<br>

json_name: script
go_name: Script</pre></td>
</tr><tr>
<td>steps</td>
<td>string</td>
<td><pre>
steps are the enabled stroppy steps; empty means all of them.<br>

json_name: steps
go_name: Steps</pre></td>
</tr><tr>
<td>vus</td>
<td>uint32</td>
<td><pre>
vus is the k6 virtual-user count.<br>

json_name: vus
go_name: Vus</pre></td>
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
//summary holds denormalized facets for the suites table (cell count,
//schedule state, last-run info).<br>

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
<td>cell_count</td>
<td>uint32</td>
<td><pre>
//cell_count is the number of enabled cells in the suite definition.<br>

json_name: cellCount
go_name: CellCount</pre></td>
</tr><tr>
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
//SuiteWorkflow receives workflow.RunConfig entries assembled from those
//children at start.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>children</td>
<td><a href="#cloud-v1-models-suiterunrecord-childrun">cloud.v1.models.SuiteRunRecord.ChildRun</a></td>
<td><pre>
//children are child TestRunRecord ids keyed back to the originating suite
//cell. These rows are still listed through TestRunAPI.ListTestRuns with
//suite_run_id.<br>

json_name: children
go_name: Children</pre></td>
</tr><tr>
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
<td>trigger</td>
<td><a href="../common/README.md#cloud-v1-common-trigger">cloud.v1.common.Trigger</a></td>
<td><pre>
//trigger records how this run was triggered (MANUAL / CRON / API).<br>

json_name: trigger
go_name: Trigger</pre></td>
</tr>
</table>



<a name="cloud-v1-models-suiterunrecord-childrun"></a>
### cloud.v1.models.SuiteRunRecord.ChildRun

<pre>
//ChildRun links one suite cell to one persisted TestRunRecord.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the child display label.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the latest known child run status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>suite_cell_id</td>
<td>string</td>
<td><pre>
//suite_cell_id points to domain.SuiteCell.id.<br>

json_name: suiteCellId
go_name: SuiteCellId</pre></td>
</tr><tr>
<td>test_run_id</td>
<td>string</td>
<td><pre>
//test_run_id is the child TestRunRecord entity id.<br>

json_name: testRunId
go_name: TestRunId</pre></td>
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
//SuiteWizardDraftRecord is the server-held mutable state of a SUITE wizard.

//It mirrors the typed test wizard model, but per cell:
//SuiteCell source -> resolved database/workload
//database/workload -> topology_spec
//topology_spec + provider/defaults/machine overrides -> infrastructure_plan
//topology + runtime placeholders -> render_preview

//The draft never stores provider account settings. Provider settings are resolved
//from tenant settings only when the suite is started, and runtime facts are
//produced by workflow stages.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cells</td>
<td><a href="#cloud-v1-models-suitewizarddraftrecord-cell">cloud.v1.models.SuiteWizardDraftRecord.Cell</a></td>
<td><pre>
//cells are the editable matrix entries and their server-derived previews.<br>

json_name: cells
go_name: Cells</pre></td>
</tr><tr>
<td>default_in_global_rating</td>
<td>bool</td>
<td><pre>
//default_in_global_rating is persisted to Suite.default_in_global_rating.<br>

json_name: defaultInGlobalRating
go_name: DefaultInGlobalRating</pre></td>
</tr><tr>
<td>default_in_tenant_rating</td>
<td>bool</td>
<td><pre>
//default_in_tenant_rating is persisted to Suite.default_in_tenant_rating.<br>

json_name: defaultInTenantRating
go_name: DefaultInTenantRating</pre></td>
</tr><tr>
<td>entity</td>
<td><a href="../common/README.md#cloud-v1-common-entity">cloud.v1.common.Entity</a></td>
<td><pre>
//entity is the storage envelope.<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>errors</td>
<td><a href="../../../schemapb/README.md#schemapb-fielderror">schemapb.FieldError</a></td>
<td><pre>
//errors are draft-level validation errors.<br>

json_name: errors
go_name: Errors</pre></td>
</tr><tr>
<td>max_parallel</td>
<td>uint32</td>
<td><pre>
//max_parallel is the suite default concurrency selected in the wizard.
//0 = unlimited.<br>

json_name: maxParallel
go_name: MaxParallel</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider selects one deployment backend for every cell.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>ready</td>
<td>bool</td>
<td><pre>
//ready is true when at least one enabled cell is ready and the suite-level
//settings validate.<br>

json_name: ready
go_name: Ready</pre></td>
</tr><tr>
<td>schedule</td>
<td><a href="../domain/README.md#cloud-v1-domain-schedule">cloud.v1.domain.Schedule</a></td>
<td><pre>
//schedule is the optional cron schedule edited in the wizard.<br>

json_name: schedule
go_name: Schedule</pre></td>
</tr><tr>
<td>suite_id</td>
<td>string</td>
<td><pre>
//suite_id is set when the draft was seeded from an existing suite.<br>

json_name: suiteId
go_name: SuiteId</pre></td>
</tr>
</table>



<a name="cloud-v1-models-suitewizarddraftrecord-cell"></a>
### cloud.v1.models.SuiteWizardDraftRecord.Cell

<pre>
//Cell is one selected suite cell plus the server-derived preview artifacts
//shown to the user.
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
//compatible is true when database/workload compatibility rules pass.<br>

json_name: compatible
go_name: Compatible</pre></td>
</tr><tr>
<td>database</td>
<td><a href="../domain/README.md#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
//database is the resolved database payload for this cell.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>errors</td>
<td><a href="../../../schemapb/README.md#schemapb-fielderror">schemapb.FieldError</a></td>
<td><pre>
//errors are per-cell validation, capacity, and render errors.<br>

json_name: errors
go_name: Errors</pre></td>
</tr><tr>
<td>infrastructure_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//infrastructure_plan is provider-specific machine intent. Provider
//account settings are omitted/redacted in wizard drafts.<br>

json_name: infrastructurePlan
go_name: InfrastructurePlan</pre></td>
</tr><tr>
<td>ready</td>
<td>bool</td>
<td><pre>
//ready is true when this enabled cell can be baked into a TestRun.<br>

json_name: ready
go_name: Ready</pre></td>
</tr><tr>
<td>render_preview</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderpreview">cloud.v1.deployment.RenderPreview</a></td>
<td><pre>
//render_preview is what generated files/commands/dirs would look like
//before runtime-only values are known.<br>

json_name: renderPreview
go_name: RenderPreview</pre></td>
</tr><tr>
<td>spec</td>
<td><a href="../domain/README.md#cloud-v1-domain-suitecell">cloud.v1.domain.SuiteCell</a></td>
<td><pre>
//spec is the editable cell definition: source, enabled flag, machine
//overrides and render overrides.<br>

json_name: spec
go_name: Spec</pre></td>
</tr><tr>
<td>topology_spec</td>
<td><a href="../topology/README.md#cloud-v1-topology-topologyspec">cloud.v1.topology.TopologySpec</a></td>
<td><pre>
//topology_spec is the server-derived provider-agnostic graph.<br>

json_name: topologySpec
go_name: TopologySpec</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the resolved workload payload for this cell.<br>

json_name: workload
go_name: Workload</pre></td>
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
<td>summary</td>
<td><a href="#cloud-v1-models-testpresetrecord-summary">cloud.v1.models.TestPresetRecord.Summary</a></td>
<td><pre>
//summary is the denormalized projection used by preset pickers and suite
//matrix screens without decoding the whole test body.<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>test</td>
<td><a href="../domain/README.md#cloud-v1-domain-test">cloud.v1.domain.Test</a></td>
<td><pre>
//test is the baked database + workload combination payload.<br>

json_name: test
go_name: Test</pre></td>
</tr>
</table>



<a name="cloud-v1-models-testpresetrecord-summary"></a>
### cloud.v1.models.TestPresetRecord.Summary

<pre>
//Summary is filled by the server from `test`.
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
//db_kind is the database engine kind.<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>protocol</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload-protocol">cloud.v1.domain.Workload.Protocol</a></td>
<td><pre>
//protocol is the workload wire protocol.<br>

json_name: protocol
go_name: Protocol</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
//stroppy_version is the stroppy binary version/tag.<br>

json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr>
</table>



<a name="cloud-v1-models-testrunrecord"></a>
### cloud.v1.models.TestRunRecord

<pre>
//TestRunRecord is a persisted test execution. spec is the immutable workflow
//input. infrastructure_state and deployment_plan are staged workflow artifacts
//filled as the run progresses.

//For the runs table (filter / sort / display of db, workload, preset,
//topology, progress, duration, ...) the server DENORMALIZES queryable facets
//into flat columns in `summary`, filled at Start and updated as the run
//progresses.

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
<td>deployment_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-deploymentplan">cloud.v1.deployment.DeploymentPlan</a></td>
<td><pre>
//deployment_plan is the rendered agent execution plan, then updated with
//execution statuses.<br>

json_name: deploymentPlan
go_name: DeploymentPlan</pre></td>
</tr><tr>
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
<td>infrastructure_state</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructurestate">cloud.v1.deployment.InfrastructureState</a></td>
<td><pre>
//infrastructure_state is provider output (resource ids, IPs/endpoints,
//allocated quotas), filled after infrastructure deployment.<br>

json_name: infrastructureState
go_name: InfrastructureState</pre></td>
</tr><tr>
<td>runtime_state</td>
<td><a href="../workflow/README.md#cloud-v1-workflow-runstate">cloud.v1.workflow.RunState</a></td>
<td><pre>
//runtime_state is the last TestWorkflow RunState persisted by the
//workflow itself. Overview uses it as the durable projection when the
//Temporal workflow is already closed and no longer queryable.<br>

json_name: runtimeState
go_name: RuntimeState</pre></td>
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
<td>suite_cell_id</td>
<td>string</td>
<td><pre>
//suite_cell_id is the originating SuiteCell.id inside suite_run_id. Empty
//for standalone runs and ad-hoc suite children without a stable cell id.<br>

json_name: suiteCellId
go_name: SuiteCellId</pre></td>
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
<td>test_preset_id</td>
<td>string</td>
<td><pre>
//test_preset_id is the complete test preset source, when one was used.<br>

json_name: testPresetId
go_name: TestPresetId</pre></td>
</tr><tr>
<td>test_preset_name</td>
<td>string</td>
<td><pre>
//test_preset_name is the display name of the test preset source.<br>

json_name: testPresetName
go_name: TestPresetName</pre></td>
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
</tr><tr>
<td>workload_protocol</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload-protocol">cloud.v1.domain.Workload.Protocol</a></td>
<td><pre>
//workload_protocol is the wire protocol exercised by the workload.<br>

json_name: workloadProtocol
go_name: WorkloadProtocol</pre></td>
</tr>
</table>



<a name="cloud-v1-models-testwizarddraftrecord"></a>
### cloud.v1.models.TestWizardDraftRecord

<pre>
//TestWizardDraft is the server-held, mutable state of a TEST wizard.

//The server derives:
//database + workload -> topology_spec
//topology_spec + provider + explicit machine_overrides -> infrastructure_plan

//Provider account settings come from tenant settings at bake/start time. The
//draft stores provider choice and per-node machine overrides separately from
//the derived infrastructure_plan preview so generated resource values never
//become launch intent unless the user explicitly confirms or edits them.
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
//database is the typed, provider-agnostic database under test (engine kind +
//DatabaseParams: logical node counts, HA flags, options). Replaces the old
//schemapb database form half.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
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
//errors are the current authoritative errors (recomputed on every patch):
//schema validation errors PLUS the bake-time capacity/sanity errors
//(RAM/quota/zones). FieldError.field carries the path so the UI can group
//by section (database.*, workload.*, provider_type).<br>

json_name: errors
go_name: Errors</pre></td>
</tr><tr>
<td>infrastructure_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
//infrastructure_plan is the provider-specific resource preview derived
//from topology_spec, provider and machine_overrides.<br>

json_name: infrastructurePlan
go_name: InfrastructurePlan</pre></td>
</tr><tr>
<td>machine_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-machineplan">cloud.v1.deployment.MachinePlan</a></td>
<td><pre>
//machine_overrides are the explicit per-node provider machine settings the
//user confirmed or edited. They are merged into infrastructure_plan and
//later baked into the TestRun.<br>

json_name: machineOverrides
go_name: MachineOverrides</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider selects the deployment backend (docker/yandex).<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>ready</td>
<td>bool</td>
<td><pre>
//ready is true when the whole form validates and FinishTestWizard is
//allowed.<br>

json_name: ready
go_name: Ready</pre></td>
</tr><tr>
<td>render_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderoverrideset">cloud.v1.deployment.RenderOverrideSet</a></td>
<td><pre>
//render_overrides are user edits to editable render artifacts. They are
//merged into render_preview and later into the runtime deployment plan.<br>

json_name: renderOverrides
go_name: RenderOverrides</pre></td>
</tr><tr>
<td>render_preview</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderpreview">cloud.v1.deployment.RenderPreview</a></td>
<td><pre>
//render_preview is the server-rendered wizard view of generated files,
//commands, directories, and runtime-only placeholders.<br>

json_name: renderPreview
go_name: RenderPreview</pre></td>
</tr><tr>
<td>test_preset_id</td>
<td>string</td>
<td><pre>
//test_preset_id is the test preset the wizard was seeded from, if it
//started from one.<br>

json_name: testPresetId
go_name: TestPresetId</pre></td>
</tr><tr>
<td>topology_spec</td>
<td><a href="../topology/README.md#cloud-v1-topology-topologyspec">cloud.v1.topology.TopologySpec</a></td>
<td><pre>
//topology_spec is the server-derived provider-agnostic graph. Node roles
//and counts are never hand-entered; they come from database/workload.<br>

json_name: topologySpec
go_name: TopologySpec</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
//workload is the typed stroppy workload (the "how to load" half).<br>

json_name: workload
go_name: Workload</pre></td>
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
<td>summary</td>
<td><a href="#cloud-v1-models-workloadpresetrecord-summary">cloud.v1.models.WorkloadPresetRecord.Summary</a></td>
<td><pre>
//summary is the denormalized projection used by preset pickers and suite
//matrix screens without decoding the whole workload body.<br>

json_name: summary
go_name: Summary</pre></td>
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



<a name="cloud-v1-models-workloadpresetrecord-summary"></a>
### cloud.v1.models.WorkloadPresetRecord.Summary

<pre>
//Summary is filled by the server from `workload`.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>protocol</td>
<td><a href="../domain/README.md#cloud-v1-domain-workload-protocol">cloud.v1.domain.Workload.Protocol</a></td>
<td><pre>
//protocol is the workload wire protocol.<br>

json_name: protocol
go_name: Protocol</pre></td>
</tr><tr>
<td>script</td>
<td>string</td>
<td><pre>
//script is the workload script/preset/path label.<br>

json_name: script
go_name: Script</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
//stroppy_version is the stroppy binary version/tag.<br>

json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr>
</table>

