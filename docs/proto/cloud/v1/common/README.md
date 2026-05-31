

<a name="cloud-v1-common"></a>
# cloud.v1.common

## Table of Contents
- Messages
  - [cloud.v1.common.BakedFile](#cloud-v1-common-bakedfile)
  - [cloud.v1.common.Cidr](#cloud-v1-common-cidr)
  - [cloud.v1.common.Cmd](#cloud-v1-common-cmd)
  - [cloud.v1.common.Cmd.Argv](#cloud-v1-common-cmd-argv)
  - [cloud.v1.common.Cmd.Result](#cloud-v1-common-cmd-result)
  - [cloud.v1.common.Cmd.Script](#cloud-v1-common-cmd-script)
  - [cloud.v1.common.Cmd.Spec](#cloud-v1-common-cmd-spec)
  - [cloud.v1.common.Cmd.Spec.EnvEntry](#cloud-v1-common-cmd-spec-enventry)
  - [cloud.v1.common.Cmd.Streams](#cloud-v1-common-cmd-streams)
  - [cloud.v1.common.Cmd.Streams.Mode](#cloud-v1-common-cmd-streams-mode)
  - [cloud.v1.common.Dir](#cloud-v1-common-dir)
  - [cloud.v1.common.Dir.Info](#cloud-v1-common-dir-info)
  - [cloud.v1.common.Dir.Temp](#cloud-v1-common-dir-temp)
  - [cloud.v1.common.Entity](#cloud-v1-common-entity)
  - [cloud.v1.common.EntityFilter](#cloud-v1-common-entityfilter)
  - [cloud.v1.common.EntitySort](#cloud-v1-common-entitysort)
  - [cloud.v1.common.EntitySortField](#cloud-v1-common-entitysortfield)
  - [cloud.v1.common.FavoriteKind](#cloud-v1-common-favoritekind)
  - [cloud.v1.common.File](#cloud-v1-common-file)
  - [cloud.v1.common.File.AsRef](#cloud-v1-common-file-asref)
  - [cloud.v1.common.File.Info](#cloud-v1-common-file-info)
  - [cloud.v1.common.IpAddress](#cloud-v1-common-ipaddress)
  - [cloud.v1.common.IpAddress.Family](#cloud-v1-common-ipaddress-family)
  - [cloud.v1.common.Net](#cloud-v1-common-net)
  - [cloud.v1.common.Page](#cloud-v1-common-page)
  - [cloud.v1.common.Status](#cloud-v1-common-status)
  - [cloud.v1.common.Tags](#cloud-v1-common-tags)
  - [cloud.v1.common.Tags.LabelsEntry](#cloud-v1-common-tags-labelsentry)
  - [cloud.v1.common.Timings](#cloud-v1-common-timings)
  - [cloud.v1.common.Trigger](#cloud-v1-common-trigger)

<a name="cloud-v1-common-messages"></a>
## Messages

<a name="cloud-v1-common-bakedfile"></a>
### cloud.v1.common.BakedFile

<pre>
//BakedFile pairs a File with the resolver that produced it and the
//schema-validated data used to render it, capturing a fully materialized file.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>data</td>
<td><a href="../../../schemapb/README.md#schemapb-baked">schemapb.Baked</a></td>
<td><pre>
data is the schema-validated input baked into the file.<br>

json_name: data
go_name: Data</pre></td>
</tr><tr>
<td>file</td>
<td><a href="#cloud-v1-common-file">cloud.v1.common.File</a></td>
<td><pre>
file is the materialized file description.<br>

json_name: file
go_name: File</pre></td>
</tr><tr>
<td>resolver_name</td>
<td>string</td>
<td><pre>
resolver_name identifies the resolver that produced the file content.<br>

json_name: resolverName
go_name: ResolverName</pre></td>
</tr>
</table>



<a name="cloud-v1-common-cidr"></a>
### cloud.v1.common.Cidr

<pre>
Cidr describes a network block in CIDR notation.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>addresses</td>
<td><a href="#cloud-v1-common-ipaddress">cloud.v1.common.IpAddress</a></td>
<td><pre>
addresses optionally denormalizes the addresses covered by this block.<br>

json_name: addresses
go_name: Addresses</pre></td>
</tr><tr>
<td>value</td>
<td>string</td>
<td><pre>
value is the CIDR string, e.g. 10.0.0.0/24.<br>

json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-common-cmd"></a>
### cloud.v1.common.Cmd

<pre>
//Cmd describes a command execution request and its observable result.
//It is a reusable system model, not an operation. Runtime ops decide how to
//execute it, capture output, enforce timeout, and interpret exit codes.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>result</td>
<td><a href="#cloud-v1-common-cmd-result">cloud.v1.common.Cmd.Result</a></td>
<td><pre>
result contains observed execution output after the command finishes.<br>

json_name: result
go_name: Result</pre></td>
</tr><tr>
<td>spec</td>
<td><a href="#cloud-v1-common-cmd-spec">cloud.v1.common.Cmd.Spec</a></td>
<td><pre>
spec describes the command to execute.<br>

json_name: spec
go_name: Spec</pre></td>
</tr>
</table>



<a name="cloud-v1-common-cmd-argv"></a>
### cloud.v1.common.Cmd.Argv

<pre>
Argv describes direct exec-style command arguments.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>args</td>
<td>string</td>
<td><pre>
args contains executable path and arguments.<br>

json_name: args
go_name: Args</pre></td>
</tr>
</table>



<a name="cloud-v1-common-cmd-result"></a>
### cloud.v1.common.Cmd.Result

<pre>
Result describes process output observed after execution.
</pre>

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



<a name="cloud-v1-common-cmd-script"></a>
### cloud.v1.common.Cmd.Script

<pre>
Script describes shell-style command execution.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>shell</td>
<td>string</td>
<td><pre>
shell contains shell executable path. Empty means executor default.<br>

json_name: shell
go_name: Shell</pre></td>
</tr><tr>
<td>text</td>
<td>string</td>
<td><pre>
text contains shell script content.<br>

json_name: text
go_name: Text</pre></td>
</tr>
</table>



<a name="cloud-v1-common-cmd-spec"></a>
### cloud.v1.common.Cmd.Spec

<pre>
Spec describes how a process should be executed.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>argv</td>
<td><a href="#cloud-v1-common-cmd-argv">cloud.v1.common.Cmd.Argv</a></td>
<td><pre>
argv executes a command directly without shell parsing.<br>

json_name: argv
go_name: Argv</pre></td>
</tr><tr>
<td>cwd</td>
<td>string</td>
<td><pre>
cwd is the working directory for the command. Empty means executor default.<br>

json_name: cwd
go_name: Cwd</pre></td>
</tr><tr>
<td>env</td>
<td><a href="#cloud-v1-common-cmd-spec-enventry">cloud.v1.common.Cmd.Spec.EnvEntry</a></td>
<td><pre>
env contains environment variables added or overridden for the command.<br>

json_name: env
go_name: Env</pre></td>
</tr><tr>
<td>expected_exit_codes</td>
<td>int32</td>
<td><pre>
expected_exit_codes lists successful exit codes. Empty means executor default.<br>

json_name: expectedExitCodes
go_name: ExpectedExitCodes</pre></td>
</tr><tr>
<td>script</td>
<td><a href="#cloud-v1-common-cmd-script">cloud.v1.common.Cmd.Script</a></td>
<td><pre>
script executes text through a shell.<br>

json_name: script
go_name: Script</pre></td>
</tr><tr>
<td>stdin</td>
<td>bytes</td>
<td><pre>
stdin contains bytes written to process stdin.<br>

json_name: stdin
go_name: Stdin</pre></td>
</tr><tr>
<td>streams</td>
<td><a href="#cloud-v1-common-cmd-streams">cloud.v1.common.Cmd.Streams</a></td>
<td><pre>
streams controls stdout and stderr handling.<br>

json_name: streams
go_name: Streams</pre></td>
</tr><tr>
<td>timeout</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
timeout limits command runtime. Zero means executor default.<br>

json_name: timeout
go_name: Timeout</pre></td>
</tr>
</table>



<a name="cloud-v1-common-cmd-spec-enventry"></a>
### cloud.v1.common.Cmd.Spec.EnvEntry

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



<a name="cloud-v1-common-cmd-streams"></a>
### cloud.v1.common.Cmd.Streams

<pre>
Streams describes stdout/stderr handling requested by the caller.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>stderr</td>
<td><a href="#cloud-v1-common-cmd-streams-mode">cloud.v1.common.Cmd.Streams.Mode</a></td>
<td><pre>
stderr controls standard error handling.<br>

json_name: stderr
go_name: Stderr</pre></td>
</tr><tr>
<td>stdout</td>
<td><a href="#cloud-v1-common-cmd-streams-mode">cloud.v1.common.Cmd.Streams.Mode</a></td>
<td><pre>
stdout controls standard output handling.<br>

json_name: stdout
go_name: Stdout</pre></td>
</tr>
</table>



<a name="cloud-v1-common-cmd-streams-mode"></a>
### cloud.v1.common.Cmd.Streams.Mode

<pre>
Mode selects how one process stream is handled.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>MODE_UNSPECIFIED</td>
<td><pre>
UNSPECIFIED means executor default.
</pre></td>
</tr><tr>
<td>MODE_INHERIT</td>
<td><pre>
INHERIT attaches the stream to executor output.
</pre></td>
</tr><tr>
<td>MODE_CAPTURE</td>
<td><pre>
CAPTURE stores the stream in Cmd.Result.
</pre></td>
</tr><tr>
<td>MODE_DISCARD</td>
<td><pre>
DISCARD drops the stream.
</pre></td>
</tr><tr>
<td>MODE_STDERR_TO_STDOUT</td>
<td><pre>
STDERR_TO_STDOUT redirects stderr into stdout.
</pre></td>
</tr>
</table>

<a name="cloud-v1-common-dir"></a>
### cloud.v1.common.Dir

<pre>
Dir describes a Linux directory.
</pre>

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
<td><a href="#cloud-v1-common-dir-info">cloud.v1.common.Dir.Info</a></td>
<td><pre>
info contains path, permissions, owner, and group.<br>

json_name: info
go_name: Info</pre></td>
</tr>
</table>



<a name="cloud-v1-common-dir-info"></a>
### cloud.v1.common.Dir.Info

<pre>
Info describes filesystem metadata for a Linux directory.
</pre>

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
group is the desired Unix group name. Empty means executor default.<br>

json_name: group
go_name: Group</pre></td>
</tr><tr>
<td>mode</td>
<td>uint32</td>
<td><pre>
mode contains Unix permission bits, e.g. 0755. Zero means executor default.<br>

json_name: mode
go_name: Mode</pre></td>
</tr><tr>
<td>owner</td>
<td>string</td>
<td><pre>
owner is the desired Unix user name. Empty means executor default.<br>

json_name: owner
go_name: Owner</pre></td>
</tr><tr>
<td>path</td>
<td>string</td>
<td><pre>
path is an absolute or executor-relative directory path.<br>

json_name: path
go_name: Path</pre></td>
</tr>
</table>



<a name="cloud-v1-common-dir-temp"></a>
### cloud.v1.common.Dir.Temp



<a name="cloud-v1-common-entity"></a>
### cloud.v1.common.Entity

<pre>
Entity is the common storage envelope shared by every DB-persisted model.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>author_id</td>
<td>string</td>
<td><pre>
//author_id is the Account that created the row. Server-assigned from the
//caller; immutable afterwards.<br>

json_name: authorId
go_name: AuthorId</pre></td>
</tr><tr>
<td>description</td>
<td>string</td>
<td><pre>
description is optional free-text describing the row.<br>

json_name: description
go_name: Description</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
id is the stable, server-assigned unique row identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>is_favorite</td>
<td>bool</td>
<td><pre>
//is_favorite is a COMPUTED, PER-CALLER flag — NOT persisted on the row.
//On every read the server fills it from the favorites of the REQUESTING
//account (a FavoriteRecord with author = caller, kind = this entity's kind,
//target_id = this entity's id). Different callers see different values.
//Writers MUST ignore any client-supplied value.

//Implementation note: when serving a list/get, left-join the rows against
//the caller's FavoriteRecords and set this true where a match exists.<br>

json_name: isFavorite
go_name: IsFavorite</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
name is the human-facing display name of the row.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
tenant_id scopes the row to its owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>timings</td>
<td><a href="#cloud-v1-common-timings">cloud.v1.common.Timings</a></td>
<td><pre>
timings carries the created/updated/deleted audit timestamps.<br>

json_name: timings
go_name: Timings</pre></td>
</tr>
</table>



<a name="cloud-v1-common-entityfilter"></a>
### cloud.v1.common.EntityFilter

<pre>
//EntityFilter is the ready-made filter over the common Entity fields. Any
//model list endpoint can embed it. Every field is optional — an unset field is
//not applied. Tenant scoping is NOT here: it comes from the request tenant_id
//+ the auth interceptor, never from a client-supplied filter.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>author_ids</td>
<td>string</td>
<td><pre>
author_ids restricts results to rows authored by these Accounts (empty = no author filter).<br>

json_name: authorIds
go_name: AuthorIds</pre></td>
</tr><tr>
<td>created_after</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
created_after is the lower bound of the created-at window (timings.created_at).<br>

json_name: createdAfter
go_name: CreatedAfter</pre></td>
</tr><tr>
<td>created_before</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
created_before is the upper bound of the created-at window (timings.created_at).<br>

json_name: createdBefore
go_name: CreatedBefore</pre></td>
</tr><tr>
<td>favorites_only</td>
<td>bool</td>
<td><pre>
//favorites_only: when true, return only rows the REQUESTING caller has
//favorited (rows with a matching FavoriteRecord for caller+kind+id). Unset
//= no favorite filter. Implementation: inner-join against the caller's
//FavoriteRecords instead of left-join.<br>

json_name: favoritesOnly
go_name: FavoritesOnly</pre></td>
</tr><tr>
<td>ids</td>
<td>string</td>
<td><pre>
ids restricts results to these exact ids (empty = no id filter).<br>

json_name: ids
go_name: Ids</pre></td>
</tr><tr>
<td>include_deleted</td>
<td>bool</td>
<td><pre>
include_deleted, when true, includes soft-deleted rows (timings.deleted_at set). Default false.<br>

json_name: includeDeleted
go_name: IncludeDeleted</pre></td>
</tr><tr>
<td>search</td>
<td>string</td>
<td><pre>
search is free-text matched over name + description (substring / ILIKE).<br>

json_name: search
go_name: Search</pre></td>
</tr><tr>
<td>updated_after</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
updated_after is the lower bound of the updated-at window (timings.updated_at).<br>

json_name: updatedAfter
go_name: UpdatedAfter</pre></td>
</tr><tr>
<td>updated_before</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
updated_before is the upper bound of the updated-at window (timings.updated_at).<br>

json_name: updatedBefore
go_name: UpdatedBefore</pre></td>
</tr>
</table>



<a name="cloud-v1-common-entitysort"></a>
### cloud.v1.common.EntitySort

<pre>
EntitySort is the ready-made ordering over common Entity fields.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>desc</td>
<td>bool</td>
<td><pre>
desc selects descending order when true, ascending otherwise.<br>

json_name: desc
go_name: Desc</pre></td>
</tr><tr>
<td>field</td>
<td><a href="#cloud-v1-common-entitysortfield">cloud.v1.common.EntitySortField</a></td>
<td><pre>
field is the Entity column to order by.<br>

json_name: field
go_name: Field</pre></td>
</tr>
</table>



<a name="cloud-v1-common-entitysortfield"></a>
### cloud.v1.common.EntitySortField

<pre>
EntitySortField names the common Entity columns a list can be ordered by.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>ENTITY_SORT_FIELD_UNSPECIFIED</td>
<td><pre>
ENTITY_SORT_FIELD_UNSPECIFIED applies the server default ordering.
</pre></td>
</tr><tr>
<td>ENTITY_SORT_FIELD_NAME</td>
<td><pre>
ENTITY_SORT_FIELD_NAME orders by the display name.
</pre></td>
</tr><tr>
<td>ENTITY_SORT_FIELD_CREATED_AT</td>
<td><pre>
ENTITY_SORT_FIELD_CREATED_AT orders by creation time.
</pre></td>
</tr><tr>
<td>ENTITY_SORT_FIELD_UPDATED_AT</td>
<td><pre>
ENTITY_SORT_FIELD_UPDATED_AT orders by last-mutation time.
</pre></td>
</tr><tr>
<td>ENTITY_SORT_FIELD_AUTHOR_ID</td>
<td><pre>
ENTITY_SORT_FIELD_AUTHOR_ID orders by the authoring Account.
</pre></td>
</tr><tr>
<td>ENTITY_SORT_FIELD_FAVORITE</td>
<td><pre>
ENTITY_SORT_FIELD_FAVORITE orders by the caller's favorite flag (favorited rows first when desc).
</pre></td>
</tr>
</table>

<a name="cloud-v1-common-favoritekind"></a>
### cloud.v1.common.FavoriteKind

<pre>
//FavoriteKind names the table a favorite points at. Presets are split (the RBAC
//RESOURCE_PRESET lumps the three preset tables together, but a favorite must
//address one exact row, so the kind has to distinguish them). (kind, target_id)
//together identify the favorited row.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>FAVORITE_KIND_UNSPECIFIED</td>
<td><pre>
FAVORITE_KIND_UNSPECIFIED is the unset/invalid default.
</pre></td>
</tr><tr>
<td>FAVORITE_KIND_DATABASE_PRESET</td>
<td><pre>
FAVORITE_KIND_DATABASE_PRESET targets a database preset row.
</pre></td>
</tr><tr>
<td>FAVORITE_KIND_WORKLOAD_PRESET</td>
<td><pre>
FAVORITE_KIND_WORKLOAD_PRESET targets a workload preset row.
</pre></td>
</tr><tr>
<td>FAVORITE_KIND_TEST_PRESET</td>
<td><pre>
FAVORITE_KIND_TEST_PRESET targets a test preset row.
</pre></td>
</tr><tr>
<td>FAVORITE_KIND_TEST_RUN</td>
<td><pre>
FAVORITE_KIND_TEST_RUN targets a test run row.
</pre></td>
</tr><tr>
<td>FAVORITE_KIND_SUITE</td>
<td><pre>
FAVORITE_KIND_SUITE targets a suite row.
</pre></td>
</tr><tr>
<td>FAVORITE_KIND_SUITE_RUN</td>
<td><pre>
FAVORITE_KIND_SUITE_RUN targets a suite run row.
</pre></td>
</tr>
</table>

<a name="cloud-v1-common-file"></a>
### cloud.v1.common.File

<pre>
//File is a Linux file description that can be rendered, previewed, stored, or
//later applied by runtime ops. It intentionally contains only file data and
//filesystem metadata, without domain concepts such as database role, target,
//preview state, or override state.
</pre>

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
<td><a href="#cloud-v1-common-file-asref">cloud.v1.common.File.AsRef</a></td>
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
<td><a href="#cloud-v1-common-file-info">cloud.v1.common.File.Info</a></td>
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



<a name="cloud-v1-common-file-asref"></a>
### cloud.v1.common.File.AsRef

<pre>
//AsRef identifies file content stored outside the current message. The
//resolver can map refs to object storage, artifact storage, uploaded
//files, or generated outputs without changing the file metadata model.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>checksum</td>
<td>string</td>
<td><pre>
checksum optionally verifies external file content.<br>

json_name: checksum
go_name: Checksum</pre></td>
</tr><tr>
<td>uri</td>
<td>string</td>
<td><pre>
uri locates the external file content (e.g. object/artifact storage).<br>

json_name: uri
go_name: Uri</pre></td>
</tr>
</table>



<a name="cloud-v1-common-file-info"></a>
### cloud.v1.common.File.Info

<pre>
Info describes filesystem metadata for a Linux file.
</pre>

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
//create_parents creates missing parent directories before writing
//(mkdir -p semantics). Matches Dir.create_parents; covers the current
//agent's WriteFileConfig.MkdirParents (defaults on).<br>

json_name: createParents
go_name: CreateParents</pre></td>
</tr><tr>
<td>group</td>
<td>string</td>
<td><pre>
group is the desired Unix group name. Empty means executor default.<br>

json_name: group
go_name: Group</pre></td>
</tr><tr>
<td>mode</td>
<td>uint32</td>
<td><pre>
mode contains Unix permission bits, e.g. 0644. Zero means executor default.<br>

json_name: mode
go_name: Mode</pre></td>
</tr><tr>
<td>owner</td>
<td>string</td>
<td><pre>
owner is the desired Unix user name. Empty means executor default.<br>

json_name: owner
go_name: Owner</pre></td>
</tr><tr>
<td>path</td>
<td>string</td>
<td><pre>
path is an absolute or executor-relative file path.<br>

json_name: path
go_name: Path</pre></td>
</tr>
</table>



<a name="cloud-v1-common-ipaddress"></a>
### cloud.v1.common.IpAddress

<pre>
IpAddress describes an address assigned to a network interface.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>address</td>
<td>string</td>
<td><pre>
address is the textual IP address.<br>

json_name: address
go_name: Address</pre></td>
</tr><tr>
<td>family</td>
<td><a href="#cloud-v1-common-ipaddress-family">cloud.v1.common.IpAddress.Family</a></td>
<td><pre>
family identifies IP protocol version.<br>

json_name: family
go_name: Family</pre></td>
</tr><tr>
<td>prefix_len</td>
<td>uint32</td>
<td><pre>
prefix_len is CIDR prefix length.<br>

json_name: prefixLen
go_name: PrefixLen</pre></td>
</tr><tr>
<td>scope</td>
<td>string</td>
<td><pre>
scope is address scope.<br>

json_name: scope
go_name: Scope</pre></td>
</tr>
</table>



<a name="cloud-v1-common-ipaddress-family"></a>
### cloud.v1.common.IpAddress.Family

<pre>
Family identifies IP protocol version.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>FAMILY_UNSPECIFIED</td>
<td><pre>
UNSPECIFIED means unknown address family.
</pre></td>
</tr><tr>
<td>FAMILY_IPV4</td>
<td><pre>
IPV4 is an IPv4 address.
</pre></td>
</tr><tr>
<td>FAMILY_IPV6</td>
<td><pre>
IPV6 is an IPv6 address.
</pre></td>
</tr>
</table>

<a name="cloud-v1-common-net"></a>
### cloud.v1.common.Net

<pre>
Net groups one or more CIDR blocks together with their member addresses.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>addresses</td>
<td><a href="#cloud-v1-common-ipaddress">cloud.v1.common.IpAddress</a></td>
<td><pre>
addresses lists the IP addresses belonging to this network.<br>

json_name: addresses
go_name: Addresses</pre></td>
</tr><tr>
<td>cidrs</td>
<td><a href="#cloud-v1-common-cidr">cloud.v1.common.Cidr</a></td>
<td><pre>
cidrs lists the CIDR blocks that make up this network.<br>

json_name: cidrs
go_name: Cidrs</pre></td>
</tr>
</table>



<a name="cloud-v1-common-page"></a>
### cloud.v1.common.Page

<pre>
Page is the reusable request-side pagination cursor.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>size</td>
<td>uint32</td>
<td><pre>
size caps returned rows; 0 means server default.<br>

json_name: size
go_name: Size</pre></td>
</tr><tr>
<td>token</td>
<td>string</td>
<td><pre>
token is the opaque cursor from a previous response (empty = first page).<br>

json_name: token
go_name: Token</pre></td>
</tr>
</table>



<a name="cloud-v1-common-status"></a>
### cloud.v1.common.Status

<pre>
Status is the lifecycle state of an executable node.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>STATUS_UNSPECIFIED</td>
<td><pre>
UNSPECIFIED is invalid.
</pre></td>
</tr><tr>
<td>STATUS_PENDING</td>
<td><pre>
PENDING means the node is waiting for dependencies or admission.
</pre></td>
</tr><tr>
<td>STATUS_RUNNING</td>
<td><pre>
RUNNING means execution is in progress.
</pre></td>
</tr><tr>
<td>STATUS_RETRY_WAIT</td>
<td><pre>
RETRY_WAIT means the node failed and waits until retry.next_run_at.
</pre></td>
</tr><tr>
<td>STATUS_COMPLETED</td>
<td><pre>
COMPLETED means execution finished successfully.
</pre></td>
</tr><tr>
<td>STATUS_FAILED</td>
<td><pre>
FAILED means execution finished unsuccessfully.
</pre></td>
</tr><tr>
<td>STATUS_SKIPPED</td>
<td><pre>
SKIPPED means the node was intentionally not executed.
</pre></td>
</tr><tr>
<td>STATUS_CANCELLING</td>
<td><pre>
CANCELLING means cancellation has been requested.
</pre></td>
</tr><tr>
<td>STATUS_CANCELLED</td>
<td><pre>
CANCELLED means cancellation completed.
</pre></td>
</tr><tr>
<td>STATUS_ALLOCATED</td>
<td><pre>
ALLOCATED means infrastructure resources have been reserved for the node.
</pre></td>
</tr><tr>
<td>STATUS_DEPLOYMENT</td>
<td><pre>
DEPLOYMENT means deployment of the node is in progress.
</pre></td>
</tr><tr>
<td>STATUS_DEPLOYED</td>
<td><pre>
DEPLOYED means the node has been successfully deployed.
</pre></td>
</tr>
</table>

<a name="cloud-v1-common-tags"></a>
### cloud.v1.common.Tags

<pre>
Tags carries a flat label set and arbitrary key/value metadata for a model.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>labels</td>
<td><a href="#cloud-v1-common-tags-labelsentry">cloud.v1.common.Tags.LabelsEntry</a></td>
<td><pre>
//labels is arbitrary key/value metadata (reconcile/observability) — free to
//extend since Tags is always serialized as a single column.<br>

json_name: labels
go_name: Labels</pre></td>
</tr><tr>
<td>tags</td>
<td>string</td>
<td><pre>
tags is a flat label set.<br>

json_name: tags
go_name: Tags</pre></td>
</tr>
</table>



<a name="cloud-v1-common-tags-labelsentry"></a>
### cloud.v1.common.Tags.LabelsEntry

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



<a name="cloud-v1-common-timings"></a>
### cloud.v1.common.Timings

<pre>
//Timings is the audit timestamp set carried by every persisted row.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
created_at is when the row was first persisted (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>deleted_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
deleted_at, when set, marks the row as soft-deleted.<br>

json_name: deletedAt
go_name: DeletedAt</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
updated_at is when the row was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-common-trigger"></a>
### cloud.v1.common.Trigger

<pre>
//Trigger records the ROOT cause of a run (test or suite) — who/what set it off.
//"Part of a suite" is an ORTHOGONAL axis, not a trigger value: a suite child has
//its parent suite run's trigger propagated, and its suite membership is shown by
//a non-empty suite_run_id. So a child of a cron suite has trigger=CRON AND
//suite_run_id set; a manual standalone has trigger=MANUAL and no suite_run_id.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>TRIGGER_UNSPECIFIED</td>
<td><pre>
TRIGGER_UNSPECIFIED is the unset/invalid default.
</pre></td>
</tr><tr>
<td>TRIGGER_MANUAL</td>
<td><pre>
TRIGGER_MANUAL means the run was started manually by a user.
</pre></td>
</tr><tr>
<td>TRIGGER_CRON</td>
<td><pre>
TRIGGER_CRON means the run was auto-started by a suite's cron schedule.
</pre></td>
</tr><tr>
<td>TRIGGER_API</td>
<td><pre>
TRIGGER_API means the run was started by an api token / automation.
</pre></td>
</tr>
</table>