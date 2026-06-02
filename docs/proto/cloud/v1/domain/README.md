

<a name="cloud-v1-domain"></a>
# cloud.v1.domain

## Table of Contents
- Messages
  - [cloud.v1.domain.CockroachParams](#cloud-v1-domain-cockroachparams)
  - [cloud.v1.domain.CockroachParams.OptionsEntry](#cloud-v1-domain-cockroachparams-optionsentry)
  - [cloud.v1.domain.Database](#cloud-v1-domain-database)
  - [cloud.v1.domain.Database.External](#cloud-v1-domain-database-external)
  - [cloud.v1.domain.Database.Kind](#cloud-v1-domain-database-kind)
  - [cloud.v1.domain.Database.PresetId](#cloud-v1-domain-database-presetid)
  - [cloud.v1.domain.DatabaseParams](#cloud-v1-domain-databaseparams)
  - [cloud.v1.domain.MySqlParams](#cloud-v1-domain-mysqlparams)
  - [cloud.v1.domain.MySqlParams.PrimaryOptionsEntry](#cloud-v1-domain-mysqlparams-primaryoptionsentry)
  - [cloud.v1.domain.MySqlParams.ProxysqlOptionsEntry](#cloud-v1-domain-mysqlparams-proxysqloptionsentry)
  - [cloud.v1.domain.MySqlParams.ReplicaOptionsEntry](#cloud-v1-domain-mysqlparams-replicaoptionsentry)
  - [cloud.v1.domain.Package](#cloud-v1-domain-package)
  - [cloud.v1.domain.PicodataParams](#cloud-v1-domain-picodataparams)
  - [cloud.v1.domain.PicodataParams.HaproxyOptionsEntry](#cloud-v1-domain-picodataparams-haproxyoptionsentry)
  - [cloud.v1.domain.PicodataParams.InstanceOptionsEntry](#cloud-v1-domain-picodataparams-instanceoptionsentry)
  - [cloud.v1.domain.PicodataTier](#cloud-v1-domain-picodatatier)
  - [cloud.v1.domain.PostgresParams](#cloud-v1-domain-postgresparams)
  - [cloud.v1.domain.PostgresParams.EtcdOptionsEntry](#cloud-v1-domain-postgresparams-etcdoptionsentry)
  - [cloud.v1.domain.PostgresParams.HaproxyOptionsEntry](#cloud-v1-domain-postgresparams-haproxyoptionsentry)
  - [cloud.v1.domain.PostgresParams.MasterOptionsEntry](#cloud-v1-domain-postgresparams-masteroptionsentry)
  - [cloud.v1.domain.PostgresParams.PatroniOptionsEntry](#cloud-v1-domain-postgresparams-patronioptionsentry)
  - [cloud.v1.domain.PostgresParams.PgbouncerOptionsEntry](#cloud-v1-domain-postgresparams-pgbounceroptionsentry)
  - [cloud.v1.domain.PostgresParams.ReplicaOptionsEntry](#cloud-v1-domain-postgresparams-replicaoptionsentry)
  - [cloud.v1.domain.Schedule](#cloud-v1-domain-schedule)
  - [cloud.v1.domain.Suite](#cloud-v1-domain-suite)
  - [cloud.v1.domain.SuiteCell](#cloud-v1-domain-suitecell)
  - [cloud.v1.domain.SuiteCell.PresetPair](#cloud-v1-domain-suitecell-presetpair)
  - [cloud.v1.domain.Test](#cloud-v1-domain-test)
  - [cloud.v1.domain.TestRun](#cloud-v1-domain-testrun)
  - [cloud.v1.domain.Worker](#cloud-v1-domain-worker)
  - [cloud.v1.domain.Worker.Kind](#cloud-v1-domain-worker-kind)
  - [cloud.v1.domain.Workload](#cloud-v1-domain-workload)
  - [cloud.v1.domain.Workload.Execution](#cloud-v1-domain-workload-execution)
  - [cloud.v1.domain.Workload.Parameters](#cloud-v1-domain-workload-parameters)
  - [cloud.v1.domain.Workload.Parameters.EnvEntry](#cloud-v1-domain-workload-parameters-enventry)
  - [cloud.v1.domain.Workload.Protocol](#cloud-v1-domain-workload-protocol)
  - [cloud.v1.domain.Workload.WorkloadFile](#cloud-v1-domain-workload-workloadfile)
  - [cloud.v1.domain.YdbManagedParams](#cloud-v1-domain-ydbmanagedparams)
  - [cloud.v1.domain.YdbManagedParams.AutoScale](#cloud-v1-domain-ydbmanagedparams-autoscale)
  - [cloud.v1.domain.YdbManagedParams.ComputeType](#cloud-v1-domain-ydbmanagedparams-computetype)
  - [cloud.v1.domain.YdbManagedParams.Type](#cloud-v1-domain-ydbmanagedparams-type)
  - [cloud.v1.domain.YdbParams](#cloud-v1-domain-ydbparams)
  - [cloud.v1.domain.YdbParams.DatabaseOptionsEntry](#cloud-v1-domain-ydbparams-databaseoptionsentry)
  - [cloud.v1.domain.YdbParams.DiskType](#cloud-v1-domain-ydbparams-disktype)
  - [cloud.v1.domain.YdbParams.FailureDomain](#cloud-v1-domain-ydbparams-failuredomain)
  - [cloud.v1.domain.YdbParams.FaultTolerance](#cloud-v1-domain-ydbparams-faulttolerance)
  - [cloud.v1.domain.YdbParams.HaproxyOptionsEntry](#cloud-v1-domain-ydbparams-haproxyoptionsentry)
  - [cloud.v1.domain.YdbParams.StorageOptionsEntry](#cloud-v1-domain-ydbparams-storageoptionsentry)

<a name="cloud-v1-domain-messages"></a>
## Messages

<a name="cloud-v1-domain-cockroachparams"></a>
### cloud.v1.domain.CockroachParams

<pre>
CockroachParams is the engine topology for CockroachDB (homogeneous nodes).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>nodes</td>
<td>uint32</td>
<td><pre>
nodes is the homogeneous node count.<br>

json_name: nodes
go_name: Nodes</pre></td>
</tr><tr>
<td>options</td>
<td><a href="#cloud-v1-domain-cockroachparams-optionsentry">cloud.v1.domain.CockroachParams.OptionsEntry</a></td>
<td><pre>
options are applied post-init as `SET CLUSTER SETTING k='v'`, or as a startup
//flag when the key is prefixed `flag:`.<br>

json_name: options
go_name: Options</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-cockroachparams-optionsentry"></a>
### cloud.v1.domain.CockroachParams.OptionsEntry

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



<a name="cloud-v1-domain-database"></a>
### cloud.v1.domain.Database

<pre>
//Database is the database-under-test definition for a test run. It selects a
//Kind and provides it through exactly one source variant, optionally carrying
//free-form tags.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>database_preset_id</td>
<td><a href="#cloud-v1-domain-database-presetid">cloud.v1.domain.Database.PresetId</a></td>
<td><pre>
database_preset_id references a preset, resolved into params|external
//server-side.<br>

json_name: databasePresetId
go_name: DatabasePresetId</pre></td>
</tr><tr>
<td>external</td>
<td><a href="#cloud-v1-domain-database-external">cloud.v1.domain.Database.External</a></td>
<td><pre>
external is the external variant: connect to an existing endpoint,
//skip deploy/teardown.<br>

json_name: external
go_name: External</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
kind selects which database engine is under test.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>package_id</td>
<td>string</td>
<td><pre>
package_id selects the install package (domain.Package) for self-deploy
//kinds; empty resolves the builtin default for kind + params.version. Ignored
//for external / managed sources.<br>

json_name: packageId
go_name: PackageId</pre></td>
</tr><tr>
<td>params</td>
<td><a href="#cloud-v1-domain-databaseparams">cloud.v1.domain.DatabaseParams</a></td>
<td><pre>
params is the typed self-deploy variant: deploy a DB instance into the
//topology and tear it down at the end.<br>

json_name: params
go_name: Params</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
tags are free-form metadata attached to the database definition.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-database-external"></a>
### cloud.v1.domain.Database.External

<pre>
//External is an already-running database. TestWorkflow does NOT deploy or
//tear it down, it only connects to `dsn` and runs the workload on top.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>dsn</td>
<td>string</td>
<td><pre>
dsn is the connection string of the external database.<br>

json_name: dsn
go_name: Dsn</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
tags carry extra info about the external db (version, region, owner...).<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-database-kind"></a>
### cloud.v1.domain.Database.Kind

<pre>
Kind enumerates the supported database engines under test.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td><pre>
KIND_UNSPECIFIED is the unset zero value; never a valid engine.
</pre></td>
</tr><tr>
<td>KIND_POSTGRES</td>
<td><pre>
KIND_POSTGRES is PostgreSQL.
</pre></td>
</tr><tr>
<td>KIND_MYSQL</td>
<td><pre>
KIND_MYSQL is MySQL.
</pre></td>
</tr><tr>
<td>KIND_MARIADB</td>
<td><pre>
KIND_MARIADB is MariaDB.
</pre></td>
</tr><tr>
<td>KIND_YDB</td>
<td><pre>
KIND_YDB is self-deployed YDB.
</pre></td>
</tr><tr>
<td>KIND_YDB_MANAGED</td>
<td><pre>
KIND_YDB_MANAGED is managed (cloud-provided) YDB.
</pre></td>
</tr><tr>
<td>KIND_COCKROACH</td>
<td><pre>
KIND_COCKROACH is CockroachDB.
</pre></td>
</tr><tr>
<td>KIND_PICODATA</td>
<td><pre>
KIND_PICODATA is Picodata.
</pre></td>
</tr><tr>
<td>KIND_EXTERNAL</td>
<td><pre>
KIND_EXTERNAL is an external / unmanaged database addressed only by
//dsn. Use with source.external.
</pre></td>
</tr>
</table>

<a name="cloud-v1-domain-database-presetid"></a>
### cloud.v1.domain.Database.PresetId

<pre>
//PresetId is a reference to a stored database preset. Resolved server-side
//into one of the inline `source` variants (params for self-deploy, or
//external).
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
id is the stored database preset identifier to resolve.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-databaseparams"></a>
### cloud.v1.domain.DatabaseParams

<pre>
//DatabaseParams is the typed self-deploy variant (replaces the former
//schemapb.Baked). It carries the common version + config overrides and exactly
//one engine-specific params message. The selected engine MUST match Database.kind.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cockroach</td>
<td><a href="#cloud-v1-domain-cockroachparams">cloud.v1.domain.CockroachParams</a></td>
<td><pre>
json_name: cockroach
go_name: Cockroach</pre></td>
</tr><tr>
<td>mariadb</td>
<td><a href="#cloud-v1-domain-mysqlparams">cloud.v1.domain.MySqlParams</a></td>
<td><pre>
mariadb reuses the MySQL params shape.<br>

json_name: mariadb
go_name: Mariadb</pre></td>
</tr><tr>
<td>mysql</td>
<td><a href="#cloud-v1-domain-mysqlparams">cloud.v1.domain.MySqlParams</a></td>
<td><pre>
json_name: mysql
go_name: Mysql</pre></td>
</tr><tr>
<td>package</td>
<td><a href="#cloud-v1-domain-package">cloud.v1.domain.Package</a></td>
<td><pre>
json_name: package
go_name: Package</pre></td>
</tr><tr>
<td>picodata</td>
<td><a href="#cloud-v1-domain-picodataparams">cloud.v1.domain.PicodataParams</a></td>
<td><pre>
json_name: picodata
go_name: Picodata</pre></td>
</tr><tr>
<td>postgres</td>
<td><a href="#cloud-v1-domain-postgresparams">cloud.v1.domain.PostgresParams</a></td>
<td><pre>
json_name: postgres
go_name: Postgres</pre></td>
</tr><tr>
<td>version</td>
<td>string</td>
<td><pre>
version is the engine version label (e.g. "16", "8.0", "25.2"). Empty uses
//the engine default.<br>

json_name: version
go_name: Version</pre></td>
</tr><tr>
<td>ydb</td>
<td><a href="#cloud-v1-domain-ydbparams">cloud.v1.domain.YdbParams</a></td>
<td><pre>
json_name: ydb
go_name: Ydb</pre></td>
</tr><tr>
<td>ydb_managed</td>
<td><a href="#cloud-v1-domain-ydbmanagedparams">cloud.v1.domain.YdbManagedParams</a></td>
<td><pre>
json_name: ydbManaged
go_name: YdbManaged</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-mysqlparams"></a>
### cloud.v1.domain.MySqlParams

<pre>
//MySqlParams is the engine topology for MySQL. MariaDB reuses this exact
//message (same shape); only the engine Kind / install package differs.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>group_replication</td>
<td>bool</td>
<td><pre>
group_replication enables MySQL Group Replication.<br>

json_name: groupReplication
go_name: GroupReplication</pre></td>
</tr><tr>
<td>primary_options</td>
<td><a href="#cloud-v1-domain-mysqlparams-primaryoptionsentry">cloud.v1.domain.MySqlParams.PrimaryOptionsEntry</a></td>
<td><pre>
primary_options is my.cnf for the primary.<br>

json_name: primaryOptions
go_name: PrimaryOptions</pre></td>
</tr><tr>
<td>proxysql</td>
<td>uint32</td>
<td><pre>
proxysql is the dedicated ProxySQL node count (0 = none).<br>

json_name: proxysql
go_name: Proxysql</pre></td>
</tr><tr>
<td>proxysql_options</td>
<td><a href="#cloud-v1-domain-mysqlparams-proxysqloptionsentry">cloud.v1.domain.MySqlParams.ProxysqlOptionsEntry</a></td>
<td><pre>
proxysql_options tunes proxysql.cnf (e.g. threads, max_connections).<br>

json_name: proxysqlOptions
go_name: ProxysqlOptions</pre></td>
</tr><tr>
<td>replica_options</td>
<td><a href="#cloud-v1-domain-mysqlparams-replicaoptionsentry">cloud.v1.domain.MySqlParams.ReplicaOptionsEntry</a></td>
<td><pre>
replica_options is my.cnf for replicas.<br>

json_name: replicaOptions
go_name: ReplicaOptions</pre></td>
</tr><tr>
<td>replicas</td>
<td>uint32</td>
<td><pre>
replicas is the replica node count (0 = no replicas).<br>

json_name: replicas
go_name: Replicas</pre></td>
</tr><tr>
<td>semi_sync</td>
<td>bool</td>
<td><pre>
semi_sync enables semi-synchronous replication (used when group_replication is off).<br>

json_name: semiSync
go_name: SemiSync</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-mysqlparams-primaryoptionsentry"></a>
### cloud.v1.domain.MySqlParams.PrimaryOptionsEntry

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



<a name="cloud-v1-domain-mysqlparams-proxysqloptionsentry"></a>
### cloud.v1.domain.MySqlParams.ProxysqlOptionsEntry

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



<a name="cloud-v1-domain-mysqlparams-replicaoptionsentry"></a>
### cloud.v1.domain.MySqlParams.ReplicaOptionsEntry

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



<a name="cloud-v1-domain-package"></a>
### cloud.v1.domain.Package

<pre>
//Package is how to install one database engine version on a host. Selected on a
//Database via package_id; when empty the backend resolves the builtin default for
//the database's kind + version.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>apt_packages</td>
<td>string</td>
<td><pre>
apt_packages are the apt package names to install (empty for download-only
//engines like YDB/Cockroach that pull a binary directly).<br>

json_name: aptPackages
go_name: AptPackages</pre></td>
</tr><tr>
<td>custom_repo</td>
<td>string</td>
<td><pre>
custom_repo is an extra apt repo line to add before install.<br>

json_name: customRepo
go_name: CustomRepo</pre></td>
</tr><tr>
<td>custom_repo_key</td>
<td>string</td>
<td><pre>
custom_repo_key is the gpg key (url or inline) for custom_repo.<br>

json_name: customRepoKey
go_name: CustomRepoKey</pre></td>
</tr><tr>
<td>db_kind</td>
<td><a href="#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
db_kind is the database engine this package installs.<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>db_version</td>
<td>string</td>
<td><pre>
db_version is the engine version this package installs, e.g. "16".<br>

json_name: dbVersion
go_name: DbVersion</pre></td>
</tr><tr>
<td>deb_filename</td>
<td>string</td>
<td><pre>
deb_filename is the download URL (or filename) of a .deb to install; for a
//custom package it is filled by the launcher (server addr + auth token).<br>

json_name: debFilename
go_name: DebFilename</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
id is the stable package identifier (a stored row id, or a builtin id).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>is_builtin</td>
<td>bool</td>
<td><pre>
is_builtin marks a stock, server-seeded package (vs a tenant custom one).<br>

json_name: isBuiltin
go_name: IsBuiltin</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the package's display name, e.g. "PostgreSQL 16".<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>package_record_id</td>
<td>string</td>
<td><pre>
package_record_id links to an uploaded models.PackageRecord blob (custom
//builds); empty for builtins.<br>

json_name: packageRecordId
go_name: PackageRecordId</pre></td>
</tr><tr>
<td>pre_install</td>
<td>string</td>
<td><pre>
pre_install are shell commands run before the install (add repo, import gpg
//key, apt-get update).<br>

json_name: preInstall
go_name: PreInstall</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-picodataparams"></a>
### cloud.v1.domain.PicodataParams

<pre>
PicodataParams is the engine topology for Picodata.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>haproxy</td>
<td>uint32</td>
<td><pre>
haproxy is the HAProxy LB node count for the pgproto port (0 = none).<br>

json_name: haproxy
go_name: Haproxy</pre></td>
</tr><tr>
<td>haproxy_options</td>
<td><a href="#cloud-v1-domain-picodataparams-haproxyoptionsentry">cloud.v1.domain.PicodataParams.HaproxyOptionsEntry</a></td>
<td><pre>
haproxy_options tunes haproxy.cfg.<br>

json_name: haproxyOptions
go_name: HaproxyOptions</pre></td>
</tr><tr>
<td>instance_options</td>
<td><a href="#cloud-v1-domain-picodataparams-instanceoptionsentry">cloud.v1.domain.PicodataParams.InstanceOptionsEntry</a></td>
<td><pre>
instance_options tunes picodata.yaml (e.g. memtx_memory, vinyl_memory, log_level).<br>

json_name: instanceOptions
go_name: InstanceOptions</pre></td>
</tr><tr>
<td>instances</td>
<td>uint32</td>
<td><pre>
instances is the instance node count (single-tier).<br>

json_name: instances
go_name: Instances</pre></td>
</tr><tr>
<td>replication_factor</td>
<td>uint32</td>
<td><pre>
replication_factor is the cluster replication factor (single-tier).<br>

json_name: replicationFactor
go_name: ReplicationFactor</pre></td>
</tr><tr>
<td>shards</td>
<td>uint32</td>
<td><pre>
shards is the shard count (single-tier).<br>

json_name: shards
go_name: Shards</pre></td>
</tr><tr>
<td>tiers</td>
<td><a href="#cloud-v1-domain-picodatatier">cloud.v1.domain.PicodataTier</a></td>
<td><pre>
tiers optionally defines a multi-tier layout (overrides single-tier rf/shards/instances).<br>

json_name: tiers
go_name: Tiers</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-picodataparams-haproxyoptionsentry"></a>
### cloud.v1.domain.PicodataParams.HaproxyOptionsEntry

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



<a name="cloud-v1-domain-picodataparams-instanceoptionsentry"></a>
### cloud.v1.domain.PicodataParams.InstanceOptionsEntry

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



<a name="cloud-v1-domain-picodatatier"></a>
### cloud.v1.domain.PicodataTier

<pre>
PicodataTier is one tier of a multi-tier Picodata cluster.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>can_vote</td>
<td>bool</td>
<td><pre>
can_vote marks the tier as raft-voting.<br>

json_name: canVote
go_name: CanVote</pre></td>
</tr><tr>
<td>count</td>
<td>uint32</td>
<td><pre>
count is the instance count in this tier.<br>

json_name: count
go_name: Count</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the tier name (e.g. "compute", "storage").<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>replication_factor</td>
<td>uint32</td>
<td><pre>
replication_factor is the per-tier replication factor.<br>

json_name: replicationFactor
go_name: ReplicationFactor</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-postgresparams"></a>
### cloud.v1.domain.PostgresParams

<pre>
PostgresParams is the engine topology for PostgreSQL (master is always 1).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>etcd</td>
<td>bool</td>
<td><pre>
etcd colocates etcd on the PG nodes (capped at the first 3).<br>

json_name: etcd
go_name: Etcd</pre></td>
</tr><tr>
<td>etcd_options</td>
<td><a href="#cloud-v1-domain-postgresparams-etcdoptionsentry">cloud.v1.domain.PostgresParams.EtcdOptionsEntry</a></td>
<td><pre>
etcd_options tunes etcd.<br>

json_name: etcdOptions
go_name: EtcdOptions</pre></td>
</tr><tr>
<td>haproxy</td>
<td>uint32</td>
<td><pre>
haproxy is the dedicated HAProxy LB node count (0 = none).<br>

json_name: haproxy
go_name: Haproxy</pre></td>
</tr><tr>
<td>haproxy_options</td>
<td><a href="#cloud-v1-domain-postgresparams-haproxyoptionsentry">cloud.v1.domain.PostgresParams.HaproxyOptionsEntry</a></td>
<td><pre>
haproxy_options tunes haproxy.cfg.<br>

json_name: haproxyOptions
go_name: HaproxyOptions</pre></td>
</tr><tr>
<td>master_options</td>
<td><a href="#cloud-v1-domain-postgresparams-masteroptionsentry">cloud.v1.domain.PostgresParams.MasterOptionsEntry</a></td>
<td><pre>
master_options is postgresql.conf for the master.<br>

json_name: masterOptions
go_name: MasterOptions</pre></td>
</tr><tr>
<td>patroni</td>
<td>bool</td>
<td><pre>
patroni enables Patroni-managed HA (needs etcd).<br>

json_name: patroni
go_name: Patroni</pre></td>
</tr><tr>
<td>patroni_options</td>
<td><a href="#cloud-v1-domain-postgresparams-patronioptionsentry">cloud.v1.domain.PostgresParams.PatroniOptionsEntry</a></td>
<td><pre>
patroni_options tunes patroni.yml (e.g. ttl, loop_wait, retry_timeout).<br>

json_name: patroniOptions
go_name: PatroniOptions</pre></td>
</tr><tr>
<td>pgbouncer</td>
<td>bool</td>
<td><pre>
pgbouncer colocates PgBouncer on each PG node.<br>

json_name: pgbouncer
go_name: Pgbouncer</pre></td>
</tr><tr>
<td>pgbouncer_options</td>
<td><a href="#cloud-v1-domain-postgresparams-pgbounceroptionsentry">cloud.v1.domain.PostgresParams.PgbouncerOptionsEntry</a></td>
<td><pre>
pgbouncer_options tunes pgbouncer.ini (e.g. auth_type, pool_mode).<br>

json_name: pgbouncerOptions
go_name: PgbouncerOptions</pre></td>
</tr><tr>
<td>replica_options</td>
<td><a href="#cloud-v1-domain-postgresparams-replicaoptionsentry">cloud.v1.domain.PostgresParams.ReplicaOptionsEntry</a></td>
<td><pre>
replica_options is postgresql.conf for replicas.<br>

json_name: replicaOptions
go_name: ReplicaOptions</pre></td>
</tr><tr>
<td>replicas</td>
<td>uint32</td>
<td><pre>
replicas is the streaming-replica node count (0 = no replicas).<br>

json_name: replicas
go_name: Replicas</pre></td>
</tr><tr>
<td>sync_replicas</td>
<td>uint32</td>
<td><pre>
sync_replicas is the synchronous standby count; >0 turns on Patroni sync mode.<br>

json_name: syncReplicas
go_name: SyncReplicas</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-postgresparams-etcdoptionsentry"></a>
### cloud.v1.domain.PostgresParams.EtcdOptionsEntry

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



<a name="cloud-v1-domain-postgresparams-haproxyoptionsentry"></a>
### cloud.v1.domain.PostgresParams.HaproxyOptionsEntry

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



<a name="cloud-v1-domain-postgresparams-masteroptionsentry"></a>
### cloud.v1.domain.PostgresParams.MasterOptionsEntry

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



<a name="cloud-v1-domain-postgresparams-patronioptionsentry"></a>
### cloud.v1.domain.PostgresParams.PatroniOptionsEntry

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



<a name="cloud-v1-domain-postgresparams-pgbounceroptionsentry"></a>
### cloud.v1.domain.PostgresParams.PgbouncerOptionsEntry

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



<a name="cloud-v1-domain-postgresparams-replicaoptionsentry"></a>
### cloud.v1.domain.PostgresParams.ReplicaOptionsEntry

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



<a name="cloud-v1-domain-schedule"></a>
### cloud.v1.domain.Schedule

<pre>
//Schedule is an optional cron trigger for a suite.
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
//cron is a standard cron expression, validated server-side.<br>

json_name: cron
go_name: Cron</pre></td>
</tr><tr>
<td>enabled</td>
<td>bool</td>
<td><pre>
//enabled gates the schedule: false = paused.<br>

json_name: enabled
go_name: Enabled</pre></td>
</tr><tr>
<td>timezone</td>
<td>string</td>
<td><pre>
//timezone is the IANA timezone for cron evaluation. Empty means UTC.<br>

json_name: timezone
go_name: Timezone</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-suite"></a>
### cloud.v1.domain.Suite

<pre>
//Suite is a reusable test bundle. It stores only stable user intent: provider,
//schedule, and enabled cells. Starting it produces a SuiteRun with fully baked
//TestRuns.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cells</td>
<td><a href="#cloud-v1-domain-suitecell">cloud.v1.domain.SuiteCell</a></td>
<td><pre>
//cells are the runnable entries composing the suite.<br>

json_name: cells
go_name: Cells</pre></td>
</tr><tr>
<td>default_in_global_rating</td>
<td>bool</td>
<td><pre>
//default_in_global_rating is propagated to child TestRuns when StartSuite
//does not override it. Unset means tenant/platform default.<br>

json_name: defaultInGlobalRating
go_name: DefaultInGlobalRating</pre></td>
</tr><tr>
<td>default_in_tenant_rating</td>
<td>bool</td>
<td><pre>
//default_in_tenant_rating is propagated to child TestRuns when StartSuite
//does not override it. Unset means tenant/platform default.<br>

json_name: defaultInTenantRating
go_name: DefaultInTenantRating</pre></td>
</tr><tr>
<td>default_max_parallel</td>
<td>uint32</td>
<td><pre>
//default_max_parallel caps concurrent child run workflows. 0 means no suite
//definition override; the start request or tenant default decides.<br>

json_name: defaultMaxParallel
go_name: DefaultMaxParallel</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable suite identifier. For persisted suites this mirrors the
//SuiteRecord entity id; for inline API suites it may be client-supplied.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the single deployment provider for every cell in this suite.
//Multi-provider comparison should be modeled as repeated suite starts or a
//future providers[] expansion layer, not by mixing providers inside a cell.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>schedule</td>
<td><a href="#cloud-v1-domain-schedule">cloud.v1.domain.Schedule</a></td>
<td><pre>
//schedule is an optional cron schedule that auto-starts this suite. Absent
//or disabled means the suite only runs manually/API.<br>

json_name: schedule
go_name: Schedule</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are free-form metadata attached to the suite.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-suitecell"></a>
### cloud.v1.domain.SuiteCell

<pre>
//SuiteCell is one runnable entry inside a suite definition.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>enabled</td>
<td>bool</td>
<td><pre>
//enabled gates this cell without deleting its overrides. Disabled cells are
//not baked into SuiteRun.<br>

json_name: enabled
go_name: Enabled</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is stable within the suite and is copied into SuiteRunCell.suite_cell_id.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>inline_test</td>
<td><a href="#cloud-v1-domain-test">cloud.v1.domain.Test</a></td>
<td><pre>
//inline_test is for CLI/API automation that wants a suite without first
//creating presets. Persisted UI-created suites should prefer presets.<br>

json_name: inlineTest
go_name: InlineTest</pre></td>
</tr><tr>
<td>machine_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-machineplan">cloud.v1.deployment.MachinePlan</a></td>
<td><pre>
//machine_overrides are user edits to provider-specific machine intent. They
//are merged into the derived InfrastructurePlan by node_id at start time.
//Provider account settings are not stored here.<br>

json_name: machineOverrides
go_name: MachineOverrides</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the display label for this cell. Empty means server derives one
//from the presets/test.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>preset_pair</td>
<td><a href="#cloud-v1-domain-suitecell-presetpair">cloud.v1.domain.SuiteCell.PresetPair</a></td>
<td><pre>
//preset_pair expands database preset x workload preset.<br>

json_name: presetPair
go_name: PresetPair</pre></td>
</tr><tr>
<td>render_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderoverrideset">cloud.v1.deployment.RenderOverrideSet</a></td>
<td><pre>
//render_overrides are user edits to editable generated config artifacts for
//this cell.<br>

json_name: renderOverrides
go_name: RenderOverrides</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags are free-form metadata attached to this cell and propagated to child
//TestRun tags.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>test_preset_id</td>
<td>string</td>
<td><pre>
//test_preset_id resolves a complete database+workload preset.<br>

json_name: testPresetId
go_name: TestPresetId</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-suitecell-presetpair"></a>
### cloud.v1.domain.SuiteCell.PresetPair

<pre>
//PresetPair references a database preset and a workload preset. The server
//resolves both and validates compatibility before baking a TestRun.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>db_preset_id</td>
<td>string</td>
<td><pre>
//db_preset_id is the database preset to resolve.<br>

json_name: dbPresetId
go_name: DbPresetId</pre></td>
</tr><tr>
<td>workload_preset_id</td>
<td>string</td>
<td><pre>
//workload_preset_id is the workload preset to resolve.<br>

json_name: workloadPresetId
go_name: WorkloadPresetId</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-test"></a>
### cloud.v1.domain.Test

<pre>
//Test is an abstract, provider-agnostic test definition: what to test.
//TopologySpec is derived from params later; provider infrastructure is chosen
//only when creating a TestRun.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>database</td>
<td><a href="#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
database is the database under test.<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
tags are free-form metadata attached to the test definition.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
workload is the stroppy workload to run against the database.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-testrun"></a>
### cloud.v1.domain.TestRun

<pre>
//TestRun is the durable input for a concrete execution. It is fully specified
//up to provider infrastructure intent. Runtime facts (IPs/resource ids) and
//rendered agent steps are produced by workflow stages and stored in the run
//record, not in this domain object.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>database</td>
<td><a href="#cloud-v1-domain-database">cloud.v1.domain.Database</a></td>
<td><pre>
database is the database under test (self-deploy / managed / external).<br>

json_name: database
go_name: Database</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
id is the stable test-run identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>infrastructure_plan</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-infrastructureplan">cloud.v1.deployment.InfrastructurePlan</a></td>
<td><pre>
infrastructure_plan is the provider-specific resource intent.<br>

json_name: infrastructurePlan
go_name: InfrastructurePlan</pre></td>
</tr><tr>
<td>render_overrides</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-renderoverrideset">cloud.v1.deployment.RenderOverrideSet</a></td>
<td><pre>
render_overrides are user edits to editable render artifacts.<br>

json_name: renderOverrides
go_name: RenderOverrides</pre></td>
</tr><tr>
<td>suite_id</td>
<td>string</td>
<td><pre>
suite_id is the owning suite. Empty for a standalone (non-suite) run.<br>

json_name: suiteId
go_name: SuiteId</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
tags are free-form metadata attached to the test run.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>topology_spec</td>
<td><a href="../topology/README.md#cloud-v1-topology-topologyspec">cloud.v1.topology.TopologySpec</a></td>
<td><pre>
topology_spec is the provider-agnostic logical graph.<br>

json_name: topologySpec
go_name: TopologySpec</pre></td>
</tr><tr>
<td>workload</td>
<td><a href="#cloud-v1-domain-workload">cloud.v1.domain.Workload</a></td>
<td><pre>
workload is the stroppy workload to run on top of the database.<br>

json_name: workload
go_name: Workload</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-worker"></a>
### cloud.v1.domain.Worker

<pre>
//Worker is a unit that executes pipeline stages: the master (orchestrator) or
//an agent (on a host). The live pipeline / worker info lives in
//monitor/overview.proto.
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
id is the stable worker identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-domain-worker-kind">cloud.v1.domain.Worker.Kind</a></td>
<td><pre>
kind says whether this worker is the master or an agent.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
tags are free-form metadata attached to the worker.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-worker-kind"></a>
### cloud.v1.domain.Worker.Kind

<pre>
Kind distinguishes the orchestrator from a host agent.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td><pre>
KIND_UNSPECIFIED is the unset zero value.
</pre></td>
</tr><tr>
<td>KIND_MASTER</td>
<td><pre>
KIND_MASTER is the orchestrator worker.
</pre></td>
</tr><tr>
<td>KIND_AGENT</td>
<td><pre>
KIND_AGENT is a worker running on a host.
</pre></td>
</tr>
</table>

<a name="cloud-v1-domain-workload"></a>
### cloud.v1.domain.Workload

<pre>
//Workload is the cloud-facing workload DTO sent by the wizard — ONLY the load
//(script, protocol, k6 profile, parameters, run-scoped files). DB engine
//install/packages belong to the Database intent, not here; stroppy_version is
//just a selector for which stroppy the load needs. The backend renders this into
//stroppy's RunConfig protojson before launching.

//These are the params the user supplies AROUND the probe: the wizard sends the
//base fields (version, script, sql, files, protocol, scale_factor, pool_size) to
//`stroppy probe`; the frontend renders the discovered structure (env
//declarations, steps, sql sections, driver setups) itself; the user's choices
//land back in Parameters (env values, steps) + Execution. The probe OUTPUT is
//intentionally NOT modelled in proto — it is stroppy-version-specific and
//rendered client-side.

//protocol x engine support and the (kind, protocol, script) compatibility matrix
//stay backend data + validation, enforced when a preset binds Database.Kind to
//Workload.Protocol — intentionally not modeled in proto.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>execution</td>
<td><a href="#cloud-v1-domain-workload-execution">cloud.v1.domain.Workload.Execution</a></td>
<td><pre>
execution is the k6 execution profile.<br>

json_name: execution
go_name: Execution</pre></td>
</tr><tr>
<td>files</td>
<td><a href="#cloud-v1-domain-workload-workloadfile">cloud.v1.domain.Workload.WorkloadFile</a></td>
<td><pre>
files are run-scoped files staged next to stroppy-config.json.<br>

json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>parameters</td>
<td><a href="#cloud-v1-domain-workload-parameters">cloud.v1.domain.Workload.Parameters</a></td>
<td><pre>
parameters are workload/script parameters.<br>

json_name: parameters
go_name: Parameters</pre></td>
</tr><tr>
<td>protocol</td>
<td><a href="#cloud-v1-domain-workload-protocol">cloud.v1.domain.Workload.Protocol</a></td>
<td><pre>
protocol selects wire format. UNSPECIFIED means backend default for Database.Kind.<br>

json_name: protocol
go_name: Protocol</pre></td>
</tr><tr>
<td>script</td>
<td>string</td>
<td><pre>
script is a stroppy-accepted script/preset/path/inline SQL,
//e.g. "tpcc/tx", "tpcds", "./bench.ts", "queries.sql".<br>

json_name: script
go_name: Script</pre></td>
</tr><tr>
<td>sql</td>
<td>string</td>
<td><pre>
sql is an optional second stroppy positional arg, e.g. an SQL probe file.<br>

json_name: sql
go_name: Sql</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
stroppy_version is the stroppy binary version/tag the load needs.<br>

json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
tags are free-form metadata attached to the workload.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-workload-execution"></a>
### cloud.v1.domain.Workload.Execution

<pre>
//Execution is the k6 execution profile. Limit is exclusive: duration OR
//iterations.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>duration</td>
<td>string</td>
<td><pre>
duration maps to k6 --duration, format like "10m", "1h30m".<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>iterations</td>
<td>uint32</td>
<td><pre>
iterations maps to k6 --iterations, fixed iteration count.<br>

json_name: iterations
go_name: Iterations</pre></td>
</tr><tr>
<td>no_thresholds</td>
<td>bool</td>
<td><pre>
no_thresholds maps to k6 --no-thresholds.<br>

json_name: noThresholds
go_name: NoThresholds</pre></td>
</tr><tr>
<td>quiet</td>
<td>bool</td>
<td><pre>
quiet maps to k6 -q.<br>

json_name: quiet
go_name: Quiet</pre></td>
</tr><tr>
<td>vus</td>
<td>uint32</td>
<td><pre>
vus is virtual users (k6 --vus).<br>

json_name: vus
go_name: Vus</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-workload-parameters"></a>
### cloud.v1.domain.Workload.Parameters

<pre>
//Parameters are workload/script parameters rendered into stroppy env, driver
//config, and step filters.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>default_insert_method</td>
<td>string</td>
<td><pre>
default_insert_method overrides insert method. "native" by default.<br>

json_name: defaultInsertMethod
go_name: DefaultInsertMethod</pre></td>
</tr><tr>
<td>env</td>
<td><a href="#cloud-v1-domain-workload-parameters-enventry">cloud.v1.domain.Workload.Parameters.EnvEntry</a></td>
<td><pre>
env are script-specific env overrides (POSIX-style key) — the values the
//user filled for the env declarations the probe reported.<br>

json_name: env
go_name: Env</pre></td>
</tr><tr>
<td>no_steps</td>
<td>string</td>
<td><pre>
no_steps is a phase blocklist. Mutually exclusive with steps.<br>

json_name: noSteps
go_name: NoSteps</pre></td>
</tr><tr>
<td>pool_size</td>
<td>uint32</td>
<td><pre>
pool_size is DB connection pool size on the stroppy side.<br>

json_name: poolSize
go_name: PoolSize</pre></td>
</tr><tr>
<td>scale_factor</td>
<td>double</td>
<td><pre>
scale_factor is TPC-C warehouses / TPC-B branches / TPC-H scale factor.
//Fractional values are valid for smoke tests, e.g. TPCH SCALE_FACTOR=0.01.<br>

json_name: scaleFactor
go_name: ScaleFactor</pre></td>
</tr><tr>
<td>steps</td>
<td>string</td>
<td><pre>
steps is a phase allowlist (e.g. create_schema, load_data, workload).
//Mutually exclusive with no_steps — backend enforces.<br>

json_name: steps
go_name: Steps</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-workload-parameters-enventry"></a>
### cloud.v1.domain.Workload.Parameters.EnvEntry

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



<a name="cloud-v1-domain-workload-protocol"></a>
### cloud.v1.domain.Workload.Protocol

<pre>
//Protocol is the cloud-side wire-format selector. The backend maps this plus
//Database.Kind into stroppy driverType/url in stroppy-config.json.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>PROTOCOL_UNSPECIFIED</td>
<td><pre>
UNSPECIFIED defers to backend default per Database.Kind.
</pre></td>
</tr><tr>
<td>PROTOCOL_PG</td>
<td><pre>
PG is Postgres / CockroachDB / Yugabyte pg-wire.
</pre></td>
</tr><tr>
<td>PROTOCOL_MYSQL</td>
<td><pre>
MYSQL is MySQL / MariaDB / Percona / Vitess.
</pre></td>
</tr><tr>
<td>PROTOCOL_PICODATA</td>
<td><pre>
PICODATA is Picodata-aware pg-wire routing.
</pre></td>
</tr><tr>
<td>PROTOCOL_YDB_GRPC</td>
<td><pre>
YDB_GRPC is YDB native gRPC.
</pre></td>
</tr><tr>
<td>PROTOCOL_YDB_GRPCS</td>
<td><pre>
YDB_GRPCS is YDB native gRPC over TLS.
</pre></td>
</tr><tr>
<td>PROTOCOL_COCKROACH</td>
<td><pre>
COCKROACH is CockroachDB pg-wire on its own default port.
</pre></td>
</tr>
</table>

<a name="cloud-v1-domain-workload-workloadfile"></a>
### cloud.v1.domain.Workload.WorkloadFile

<pre>
WorkloadFile is a run-scoped file staged for stroppy (SQL probes, support files).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>content</td>
<td>string</td>
<td><pre>
content is the raw file content.<br>

json_name: content
go_name: Content</pre></td>
</tr><tr>
<td>kind</td>
<td>string</td>
<td><pre>
kind tags file usage, e.g. "sql", "config".<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the file basename inside the run dir.<br>

json_name: name
go_name: Name</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-ydbmanagedparams"></a>
### cloud.v1.domain.YdbManagedParams

<pre>
//YdbManagedParams is cloud-managed (provider-hosted) YDB. The managed DB itself
//has no node sizing; the stroppy client node is a separate topology instance.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>auto_scale</td>
<td><a href="#cloud-v1-domain-ydbmanagedparams-autoscale">cloud.v1.domain.YdbManagedParams.AutoScale</a></td>
<td><pre>
auto_scale switches dedicated to autoscaling.<br>

json_name: autoScale
go_name: AutoScale</pre></td>
</tr><tr>
<td>compute_type</td>
<td><a href="#cloud-v1-domain-ydbmanagedparams-computetype">cloud.v1.domain.YdbManagedParams.ComputeType</a></td>
<td><pre>
compute_type is the UI workload class (oltp/olap).<br>

json_name: computeType
go_name: ComputeType</pre></td>
</tr><tr>
<td>node_count</td>
<td>uint32</td>
<td><pre>
node_count is the dedicated fixed-scale node count (ignored when auto_scale set).<br>

json_name: nodeCount
go_name: NodeCount</pre></td>
</tr><tr>
<td>resource_preset_id</td>
<td>string</td>
<td><pre>
resource_preset_id is the dedicated node preset id (ignored for serverless).<br>

json_name: resourcePresetId
go_name: ResourcePresetId</pre></td>
</tr><tr>
<td>storage_groups</td>
<td>uint32</td>
<td><pre>
storage_groups is the dedicated storage group count (ignored for serverless).<br>

json_name: storageGroups
go_name: StorageGroups</pre></td>
</tr><tr>
<td>storage_type</td>
<td>string</td>
<td><pre>
storage_type is the dedicated disk type id, e.g. "ssd" (ignored for serverless).<br>

json_name: storageType
go_name: StorageType</pre></td>
</tr><tr>
<td>throttling_rcus</td>
<td>uint32</td>
<td><pre>
throttling_rcus is the serverless throttling RCU limit (0 = provider default).<br>

json_name: throttlingRcus
go_name: ThrottlingRcus</pre></td>
</tr><tr>
<td>type</td>
<td><a href="#cloud-v1-domain-ydbmanagedparams-type">cloud.v1.domain.YdbManagedParams.Type</a></td>
<td><pre>
type selects serverless vs dedicated.<br>

json_name: type
go_name: Type</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-ydbmanagedparams-autoscale"></a>
### cloud.v1.domain.YdbManagedParams.AutoScale

<pre>
AutoScale switches a dedicated database to autoscaling (replaces node_count).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cpu_utilization_percent</td>
<td>uint32</td>
<td><pre>
cpu_utilization_percent is the target-tracking CPU threshold (default 70).<br>

json_name: cpuUtilizationPercent
go_name: CpuUtilizationPercent</pre></td>
</tr><tr>
<td>max_size</td>
<td>uint32</td>
<td><pre>
max_size is the maximum node count.<br>

json_name: maxSize
go_name: MaxSize</pre></td>
</tr><tr>
<td>min_size</td>
<td>uint32</td>
<td><pre>
min_size is the minimum node count.<br>

json_name: minSize
go_name: MinSize</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-ydbmanagedparams-computetype"></a>
### cloud.v1.domain.YdbManagedParams.ComputeType

<pre>
ComputeType is a UI workload class that filters the preset catalog; it is
//NOT sent to terraform.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>COMPUTE_TYPE_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>COMPUTE_TYPE_OLTP</td>
<td></td>
</tr><tr>
<td>COMPUTE_TYPE_OLAP</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-domain-ydbmanagedparams-type"></a>
### cloud.v1.domain.YdbManagedParams.Type

<pre>
Type is the managed YDB flavor.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>TYPE_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>TYPE_SERVERLESS</td>
<td><pre>
serverless (pay-per-request).
</pre></td>
</tr><tr>
<td>TYPE_DEDICATED</td>
<td><pre>
dedicated (provisioned nodes).
</pre></td>
</tr>
</table>

<a name="cloud-v1-domain-ydbparams"></a>
### cloud.v1.domain.YdbParams

<pre>
YdbParams is the self-deployed (IaaS) YDB engine topology.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>auto_size_pdisks</td>
<td>bool</td>
<td><pre>
auto_size_pdisks lets a dry-run resize the pdisks from the workload.<br>

json_name: autoSizePdisks
go_name: AutoSizePdisks</pre></td>
</tr><tr>
<td>database_nodes</td>
<td>uint32</td>
<td><pre>
database_nodes is the dynamic (compute) node count; 0 = combined mode
//(compute runs on the storage nodes).<br>

json_name: databaseNodes
go_name: DatabaseNodes</pre></td>
</tr><tr>
<td>database_options</td>
<td><a href="#cloud-v1-domain-ydbparams-databaseoptionsentry">cloud.v1.domain.YdbParams.DatabaseOptionsEntry</a></td>
<td><pre>
database_options is the database config passthrough.<br>

json_name: databaseOptions
go_name: DatabaseOptions</pre></td>
</tr><tr>
<td>database_path</td>
<td>string</td>
<td><pre>
database_path is the tenant DB path (default "/Root/testdb").<br>

json_name: databasePath
go_name: DatabasePath</pre></td>
</tr><tr>
<td>default_disk_type</td>
<td><a href="#cloud-v1-domain-ydbparams-disktype">cloud.v1.domain.YdbParams.DiskType</a></td>
<td><pre>
default_disk_type is the storage-pool disk kind.<br>

json_name: defaultDiskType
go_name: DefaultDiskType</pre></td>
</tr><tr>
<td>failure_domain_type</td>
<td><a href="#cloud-v1-domain-ydbparams-failuredomain">cloud.v1.domain.YdbParams.FailureDomain</a></td>
<td><pre>
failure_domain_type is the failure-domain granularity.<br>

json_name: failureDomainType
go_name: FailureDomainType</pre></td>
</tr><tr>
<td>fault_tolerance</td>
<td><a href="#cloud-v1-domain-ydbparams-faulttolerance">cloud.v1.domain.YdbParams.FaultTolerance</a></td>
<td><pre>
fault_tolerance is the erasure mode.<br>

json_name: faultTolerance
go_name: FaultTolerance</pre></td>
</tr><tr>
<td>haproxy</td>
<td>uint32</td>
<td><pre>
haproxy is the HAProxy LB node count for the gRPC port (0 = none).<br>

json_name: haproxy
go_name: Haproxy</pre></td>
</tr><tr>
<td>haproxy_options</td>
<td><a href="#cloud-v1-domain-ydbparams-haproxyoptionsentry">cloud.v1.domain.YdbParams.HaproxyOptionsEntry</a></td>
<td><pre>
haproxy_options tunes haproxy.cfg.<br>

json_name: haproxyOptions
go_name: HaproxyOptions</pre></td>
</tr><tr>
<td>pdisks_per_storage_node</td>
<td>uint32</td>
<td><pre>
pdisks_per_storage_node is the number of data pdisks per storage node
//(logical: mirror-3-dc needs >=3). Their SIZE is a topology/provider concern.<br>

json_name: pdisksPerStorageNode
go_name: PdisksPerStorageNode</pre></td>
</tr><tr>
<td>storage_groups</td>
<td>uint32</td>
<td><pre>
storage_groups is the pool group count for `database create` (0 → 1).<br>

json_name: storageGroups
go_name: StorageGroups</pre></td>
</tr><tr>
<td>storage_nodes</td>
<td>uint32</td>
<td><pre>
storage_nodes is the static (storage) node count.<br>

json_name: storageNodes
go_name: StorageNodes</pre></td>
</tr><tr>
<td>storage_options</td>
<td><a href="#cloud-v1-domain-ydbparams-storageoptionsentry">cloud.v1.domain.YdbParams.StorageOptionsEntry</a></td>
<td><pre>
storage_options is the storage config passthrough.<br>

json_name: storageOptions
go_name: StorageOptions</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-ydbparams-databaseoptionsentry"></a>
### cloud.v1.domain.YdbParams.DatabaseOptionsEntry

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



<a name="cloud-v1-domain-ydbparams-disktype"></a>
### cloud.v1.domain.YdbParams.DiskType

<pre>
DiskType is the YDB storage-pool kind.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>DISK_TYPE_UNSPECIFIED</td>
<td><pre>
unset → "ssd".
</pre></td>
</tr><tr>
<td>DISK_TYPE_SSD</td>
<td></td>
</tr><tr>
<td>DISK_TYPE_NVME</td>
<td></td>
</tr><tr>
<td>DISK_TYPE_ROT</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-domain-ydbparams-failuredomain"></a>
### cloud.v1.domain.YdbParams.FailureDomain

<pre>
FailureDomain is the failure-domain granularity.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>FAILURE_DOMAIN_UNSPECIFIED</td>
<td><pre>
unset → default.
</pre></td>
</tr><tr>
<td>FAILURE_DOMAIN_DISK</td>
<td><pre>
maps to "disk".
</pre></td>
</tr>
</table>

<a name="cloud-v1-domain-ydbparams-faulttolerance"></a>
### cloud.v1.domain.YdbParams.FaultTolerance

<pre>
FaultTolerance is the YDB erasure / fault-tolerance mode.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>FAULT_TOLERANCE_UNSPECIFIED</td>
<td><pre>
unset → "none".
</pre></td>
</tr><tr>
<td>FAULT_TOLERANCE_NONE</td>
<td><pre>
maps to "none".
</pre></td>
</tr><tr>
<td>FAULT_TOLERANCE_BLOCK_4_2</td>
<td><pre>
maps to "block-4-2".
</pre></td>
</tr><tr>
<td>FAULT_TOLERANCE_MIRROR_3_DC</td>
<td><pre>
maps to "mirror-3-dc" (needs >=3 storage nodes + >=3 pdisks each, disk failure domain).
</pre></td>
</tr>
</table>

<a name="cloud-v1-domain-ydbparams-haproxyoptionsentry"></a>
### cloud.v1.domain.YdbParams.HaproxyOptionsEntry

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



<a name="cloud-v1-domain-ydbparams-storageoptionsentry"></a>
### cloud.v1.domain.YdbParams.StorageOptionsEntry

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

