

<a name="cloud-v1-domain"></a>
# cloud.v1.domain

## Table of Contents
- Messages
  - [cloud.v1.domain.Database](#cloud-v1-domain-database)
  - [cloud.v1.domain.Database.External](#cloud-v1-domain-database-external)
  - [cloud.v1.domain.Database.Kind](#cloud-v1-domain-database-kind)
  - [cloud.v1.domain.Database.PresetId](#cloud-v1-domain-database-presetid)
  - [cloud.v1.domain.Schedule](#cloud-v1-domain-schedule)
  - [cloud.v1.domain.Suite](#cloud-v1-domain-suite)
  - [cloud.v1.domain.SuiteRun](#cloud-v1-domain-suiterun)
  - [cloud.v1.domain.Test](#cloud-v1-domain-test)
  - [cloud.v1.domain.TestRun](#cloud-v1-domain-testrun)
  - [cloud.v1.domain.Worker](#cloud-v1-domain-worker)
  - [cloud.v1.domain.Worker.Kind](#cloud-v1-domain-worker-kind)
  - [cloud.v1.domain.Workload](#cloud-v1-domain-workload)

<a name="cloud-v1-domain-messages"></a>
## Messages

<a name="cloud-v1-domain-database"></a>
### cloud.v1.domain.Database

<pre>
//Database is the database-under-test definition for a test run. It selects a
//Kind and provides it through exactly one source variant, optionally carrying
//the params schema and free-form tags.
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
<td>params</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
params is the self-deploy variant: filled values of prams_schema.
//TestWorkflow deploys a DB instance into the topology and tears it
//down at the end.<br>

json_name: params
go_name: Params</pre></td>
</tr><tr>
<td>prams_schema</td>
<td><a href="../../../schemapb/README.md#schemapb-schema">schemapb.Schema</a></td>
<td><pre>
prams_schema is the schema (the form) describing self-deploy params for
//this kind.<br>

json_name: pramsSchema
go_name: PramsSchema</pre></td>
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
//PresetId is a reference to a stored database preset. Resolved
//server-side into one of the inline `source` variants (params for
//self-deploy, or external).
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



<a name="cloud-v1-domain-schedule"></a>
### cloud.v1.domain.Schedule

<pre>
//Schedule is an optional cron trigger for a suite. When enabled with a cron
//expression, the platform auto-starts the suite on that cadence; `enabled`
//gates it so a configured schedule can be paused without losing the cron.
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
cron is a standard cron expression (e.g. "0 2 * * *"). Validated
//server-side.<br>

json_name: cron
go_name: Cron</pre></td>
</tr><tr>
<td>enabled</td>
<td>bool</td>
<td><pre>
enabled gates the schedule: false = paused (never auto-runs).<br>

json_name: enabled
go_name: Enabled</pre></td>
</tr><tr>
<td>timezone</td>
<td>string</td>
<td><pre>
timezone is the IANA timezone for the cron (e.g. "Europe/Moscow");
//empty = UTC.<br>

json_name: timezone
go_name: Timezone</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-suite"></a>
### cloud.v1.domain.Suite

<pre>
//Suite is a reusable test bundle. It references provider-agnostic,
//params-only presets; the provider is applied once here, and the wizard bakes
//provider_parms when expanding presets into TestRuns.
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
default_in_global_rating is the global-rating default propagated to every
//child TestRun the suite spawns (incl. cron runs). Same semantics as
//TestRunRecord: global defaults false (opt-in). Optional so unset =
//platform default.<br>

json_name: defaultInGlobalRating
go_name: DefaultInGlobalRating</pre></td>
</tr><tr>
<td>default_in_tenant_rating</td>
<td>bool</td>
<td><pre>
default_in_tenant_rating is the tenant-rating default propagated to every
//child TestRun the suite spawns (incl. cron runs). Same semantics as
//TestRunRecord: tenant defaults true. Optional so unset = platform default.<br>

json_name: defaultInTenantRating
go_name: DefaultInTenantRating</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
id is the stable suite identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>preset_ids</td>
<td>string</td>
<td><pre>
preset_ids are the presets composing the suite (Database / Workload /
//Test presets).<br>

json_name: presetIds
go_name: PresetIds</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
provider is the single deployment provider for now. Multi-provider
//(cross-product to compare clouds) is planned: this becomes
//`repeated Provider providers` and expansion does presets x providers.
//Deferred to avoid the exponential bake cost for now.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>schedule</td>
<td><a href="#cloud-v1-domain-schedule">cloud.v1.domain.Schedule</a></td>
<td><pre>
schedule is an optional cron schedule that auto-starts this suite.
//Absent / disabled = the suite only runs when started manually.<br>

json_name: schedule
go_name: Schedule</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
tags are free-form metadata attached to the suite.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-suiterun"></a>
### cloud.v1.domain.SuiteRun

<pre>
//SuiteRun is a materialized suite execution: the Suite's preset_ids expanded
//into concrete TestRuns, executed with a bounded degree of parallelism.
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
id is the stable suite-run identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>max_parallel</td>
<td>uint32</td>
<td><pre>
max_parallel is the max concurrent TestWorkflows. 0 = unlimited.<br>

json_name: maxParallel
go_name: MaxParallel</pre></td>
</tr><tr>
<td>suite_id</td>
<td>string</td>
<td><pre>
suite_id references the Suite this run was expanded from.<br>

json_name: suiteId
go_name: SuiteId</pre></td>
</tr><tr>
<td>test_runs</td>
<td><a href="#cloud-v1-domain-testrun">cloud.v1.domain.TestRun</a></td>
<td><pre>
test_runs are the expanded runs (one per resolved preset).<br>

json_name: testRuns
go_name: TestRuns</pre></td>
</tr>
</table>



<a name="cloud-v1-domain-test"></a>
### cloud.v1.domain.Test

<pre>
//Test is an abstract, provider-agnostic test definition: what to test.
//Topology is derived from params (e.g. database replica count) in code; the
//provider is chosen later, and the wizard fills provider_parms into a TestRun.
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
//TestRun is a single, fully-baked test execution. All fields are baked at
//creation time; TestWorkflow does not mutate the topology, only carries
//runtime info returned by the deployment.
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
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-providersettings">cloud.v1.deployment.ProviderSettings</a></td>
<td><pre>
provider says where/how to provision the stand: backend + baked provider
//settings.<br>

json_name: provider
go_name: Provider</pre></td>
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
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
topology is the baked topology: stroppy runner instances (+ db instances
//when self-deploy), and external_components when the database is external.<br>

json_name: topology
go_name: Topology</pre></td>
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
//Workload describes the stroppy load to run against the database under test:
//the stroppy binary version plus its sealed, schema-backed parameters.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>params</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
params is the sealed, schema-backed stroppy workload configuration.<br>

json_name: params
go_name: Params</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
stroppy_version is the stroppy binary version to run.<br>

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

