

<a name="cloud-v1-domain"></a>
# cloud.v1.domain

## Table of Contents
- Messages
  - [cloud.v1.domain.Database.Kind](#cloud-v1-domain-database-kind)
  - [cloud.v1.domain.Worker](#cloud-v1-domain-worker)
  - [cloud.v1.domain.Worker.Kind](#cloud-v1-domain-worker-kind)
  - [cloud.v1.domain.Workload.Protocol](#cloud-v1-domain-workload-protocol)

<a name="cloud-v1-domain-messages"></a>
## Messages

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
</tr><tr>
<td>KIND_ORIOLEDB</td>
<td><pre>
KIND_ORIOLEDB is OrioleDB (docker-only patched-Postgres storage engine).
</pre></td>
</tr><tr>
<td>KIND_NOOP</td>
<td><pre>
KIND_NOOP is the no-database machine benchmark: stroppy runs with its
//internal noop driver, deploying NO database, to measure the max row
//generation rate a single runner machine can produce.
</pre></td>
</tr><tr>
<td>KIND_PG_NOOP</td>
<td><pre>
KIND_PG_NOOP is the pg-noop blackhole: a single static binary that
//speaks the PostgreSQL wire protocol and discards everything, to measure
//the max rate stroppy can deliver over the wire (the delivery ceiling).
</pre></td>
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
</tr><tr>
<td>PROTOCOL_NOOP</td>
<td><pre>
NOOP is stroppy's internal no-database driver: rows are generated and
//discarded, no connection is made (used by the KIND_NOOP machine
//benchmark).
</pre></td>
</tr>
</table>