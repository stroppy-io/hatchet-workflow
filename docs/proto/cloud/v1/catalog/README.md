

<a name="cloud-v1-catalog"></a>
# cloud.v1.catalog

## Table of Contents
- Messages
  - [cloud.v1.catalog.CatalogEntry](#cloud-v1-catalog-catalogentry)
  - [cloud.v1.catalog.CatalogEntry.Summary](#cloud-v1-catalog-catalogentry-summary)
  - [cloud.v1.catalog.CheckCatalogProviderRequest](#cloud-v1-catalog-checkcatalogproviderrequest)
  - [cloud.v1.catalog.CheckCatalogProviderRequest.FilesEntry](#cloud-v1-catalog-checkcatalogproviderrequest-filesentry)
  - [cloud.v1.catalog.CheckCatalogProviderResponse](#cloud-v1-catalog-checkcatalogproviderresponse)
  - [cloud.v1.catalog.CheckCatalogWorkflowRequest](#cloud-v1-catalog-checkcatalogworkflowrequest)
  - [cloud.v1.catalog.CheckCatalogWorkflowRequest.FilesEntry](#cloud-v1-catalog-checkcatalogworkflowrequest-filesentry)
  - [cloud.v1.catalog.CheckCatalogWorkflowResponse](#cloud-v1-catalog-checkcatalogworkflowresponse)
  - [cloud.v1.catalog.CreateInstanceEntryRequest](#cloud-v1-catalog-createinstanceentryrequest)
  - [cloud.v1.catalog.CreateInstanceEntryRequest.FilesEntry](#cloud-v1-catalog-createinstanceentryrequest-filesentry)
  - [cloud.v1.catalog.CreateInstanceEntryResponse](#cloud-v1-catalog-createinstanceentryresponse)
  - [cloud.v1.catalog.CreateOrgProviderRequest](#cloud-v1-catalog-createorgproviderrequest)
  - [cloud.v1.catalog.CreateOrgProviderRequest.FilesEntry](#cloud-v1-catalog-createorgproviderrequest-filesentry)
  - [cloud.v1.catalog.CreateOrgProviderResponse](#cloud-v1-catalog-createorgproviderresponse)
  - [cloud.v1.catalog.CreateOrgWorkflowRequest](#cloud-v1-catalog-createorgworkflowrequest)
  - [cloud.v1.catalog.CreateOrgWorkflowRequest.FilesEntry](#cloud-v1-catalog-createorgworkflowrequest-filesentry)
  - [cloud.v1.catalog.CreateOrgWorkflowResponse](#cloud-v1-catalog-createorgworkflowresponse)
  - [cloud.v1.catalog.DeleteInstanceEntryRequest](#cloud-v1-catalog-deleteinstanceentryrequest)
  - [cloud.v1.catalog.DeleteInstanceEntryResponse](#cloud-v1-catalog-deleteinstanceentryresponse)
  - [cloud.v1.catalog.DeleteOrgProviderRequest](#cloud-v1-catalog-deleteorgproviderrequest)
  - [cloud.v1.catalog.DeleteOrgProviderResponse](#cloud-v1-catalog-deleteorgproviderresponse)
  - [cloud.v1.catalog.DeleteOrgWorkflowRequest](#cloud-v1-catalog-deleteorgworkflowrequest)
  - [cloud.v1.catalog.DeleteOrgWorkflowResponse](#cloud-v1-catalog-deleteorgworkflowresponse)
  - [cloud.v1.catalog.GetInstanceEntryRequest](#cloud-v1-catalog-getinstanceentryrequest)
  - [cloud.v1.catalog.GetInstanceEntryResponse](#cloud-v1-catalog-getinstanceentryresponse)
  - [cloud.v1.catalog.GetOrgProviderRequest](#cloud-v1-catalog-getorgproviderrequest)
  - [cloud.v1.catalog.GetOrgProviderResponse](#cloud-v1-catalog-getorgproviderresponse)
  - [cloud.v1.catalog.GetOrgWorkflowRequest](#cloud-v1-catalog-getorgworkflowrequest)
  - [cloud.v1.catalog.GetOrgWorkflowResponse](#cloud-v1-catalog-getorgworkflowresponse)
  - [cloud.v1.catalog.Kind](#cloud-v1-catalog-kind)
  - [cloud.v1.catalog.Level](#cloud-v1-catalog-level)
  - [cloud.v1.catalog.LinkInstanceProviderRequest](#cloud-v1-catalog-linkinstanceproviderrequest)
  - [cloud.v1.catalog.LinkInstanceProviderResponse](#cloud-v1-catalog-linkinstanceproviderresponse)
  - [cloud.v1.catalog.LinkInstanceWorkflowRequest](#cloud-v1-catalog-linkinstanceworkflowrequest)
  - [cloud.v1.catalog.LinkInstanceWorkflowResponse](#cloud-v1-catalog-linkinstanceworkflowresponse)
  - [cloud.v1.catalog.ListInstanceEntriesRequest](#cloud-v1-catalog-listinstanceentriesrequest)
  - [cloud.v1.catalog.ListInstanceEntriesResponse](#cloud-v1-catalog-listinstanceentriesresponse)
  - [cloud.v1.catalog.ListOrgProvidersRequest](#cloud-v1-catalog-listorgprovidersrequest)
  - [cloud.v1.catalog.ListOrgProvidersResponse](#cloud-v1-catalog-listorgprovidersresponse)
  - [cloud.v1.catalog.ListOrgWorkflowsRequest](#cloud-v1-catalog-listorgworkflowsrequest)
  - [cloud.v1.catalog.ListOrgWorkflowsResponse](#cloud-v1-catalog-listorgworkflowsresponse)
  - [cloud.v1.catalog.Origin](#cloud-v1-catalog-origin)
  - [cloud.v1.catalog.UpdateInstanceEntryRequest](#cloud-v1-catalog-updateinstanceentryrequest)
  - [cloud.v1.catalog.UpdateInstanceEntryRequest.FilesEntry](#cloud-v1-catalog-updateinstanceentryrequest-filesentry)
  - [cloud.v1.catalog.UpdateInstanceEntryResponse](#cloud-v1-catalog-updateinstanceentryresponse)
  - [cloud.v1.catalog.UpdateOrgProviderRequest](#cloud-v1-catalog-updateorgproviderrequest)
  - [cloud.v1.catalog.UpdateOrgProviderRequest.FilesEntry](#cloud-v1-catalog-updateorgproviderrequest-filesentry)
  - [cloud.v1.catalog.UpdateOrgProviderResponse](#cloud-v1-catalog-updateorgproviderresponse)
  - [cloud.v1.catalog.UpdateOrgWorkflowRequest](#cloud-v1-catalog-updateorgworkflowrequest)
  - [cloud.v1.catalog.UpdateOrgWorkflowRequest.FilesEntry](#cloud-v1-catalog-updateorgworkflowrequest-filesentry)
  - [cloud.v1.catalog.UpdateOrgWorkflowResponse](#cloud-v1-catalog-updateorgworkflowresponse)

<a name="cloud-v1-catalog-services"></a>
## Services

<a name="cloud-v1-catalog-messages"></a>
## Messages

<a name="cloud-v1-catalog-catalogentry"></a>
### cloud.v1.catalog.CatalogEntry

<pre>
//CatalogEntry is a persisted provider or workflow catalog object, scoped
//by (level, tenant_id) and versioned by (slug, version).
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
entity is the storage envelope (id, tenant_id, name, description, timings).<br>

json_name: entity
go_name: Entity</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-catalog-kind">cloud.v1.catalog.Kind</a></td>
<td><pre>
kind is the catalog object type (provider or workflow).<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>level</td>
<td><a href="#cloud-v1-catalog-level">cloud.v1.catalog.Level</a></td>
<td><pre>
level is the catalog scope this entry lives at.<br>

json_name: level
go_name: Level</pre></td>
</tr><tr>
<td>origin</td>
<td><a href="#cloud-v1-catalog-origin">cloud.v1.catalog.Origin</a></td>
<td><pre>
origin records how this entry came to exist (native, linked, forked).<br>

json_name: origin
go_name: Origin</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
slug is the entry's stable, human-facing identifier within its (level, tenant, kind) scope.<br>

json_name: slug
go_name: Slug</pre></td>
</tr><tr>
<td>source_entry_id</td>
<td>string</td>
<td><pre>
source_entry_id is the CatalogEntry.entity.id this entry was linked or forked from (empty for native).<br>

json_name: sourceEntryId
go_name: SourceEntryId</pre></td>
</tr><tr>
<td>source_ref</td>
<td>string</td>
<td><pre>
source_ref is an opaque bundle-store pointer resolving this entry's content.<br>

json_name: sourceRef
go_name: SourceRef</pre></td>
</tr><tr>
<td>summary</td>
<td><a href="#cloud-v1-catalog-catalogentry-summary">cloud.v1.catalog.CatalogEntry.Summary</a></td>
<td><pre>
summary holds denormalized facets for catalog listing UIs.<br>

json_name: summary
go_name: Summary</pre></td>
</tr><tr>
<td>version</td>
<td>uint32</td>
<td><pre>
version is the monotonic version number of this slug within its scope.<br>

json_name: version
go_name: Version</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-catalogentry-summary"></a>
### cloud.v1.catalog.CatalogEntry.Summary

<pre>
Summary is the flat, indexed projection of the entry used by catalog list views.
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
compiles is the last known Check result: true when the bundle compiled with no error diagnostics.<br>

json_name: compiles
go_name: Compiles</pre></td>
</tr><tr>
<td>machine_group_count</td>
<td>uint32</td>
<td><pre>
machine_group_count is the KIND_WORKFLOW bundle's cluster.yaml machine group count.<br>

json_name: machineGroupCount
go_name: MachineGroupCount</pre></td>
</tr><tr>
<td>provider_slug</td>
<td>string</td>
<td><pre>
provider_slug is the KIND_WORKFLOW bundle's declared provider (cluster.yaml provider.use).<br>

json_name: providerSlug
go_name: ProviderSlug</pre></td>
</tr><tr>
<td>provides</td>
<td>string</td>
<td><pre>
provides is the KIND_PROVIDER manifest's declared capabilities (providers/<name>/manifest.yaml provides:).<br>

json_name: provides
go_name: Provides</pre></td>
</tr><tr>
<td>service_count</td>
<td>uint32</td>
<td><pre>
service_count is the KIND_WORKFLOW bundle's cluster.yaml service count.<br>

json_name: serviceCount
go_name: ServiceCount</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-checkcatalogproviderrequest"></a>
### cloud.v1.catalog.CheckCatalogProviderRequest

<pre>
CheckCatalogProviderRequest/CheckCatalogWorkflowRequest carry tenant_id
purely for all_of/tenant_field RBAC resolution — see the file doc above.
Neither RPC persists anything.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-checkcatalogproviderrequest-filesentry">cloud.v1.catalog.CheckCatalogProviderRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-checkcatalogproviderrequest-filesentry"></a>
### cloud.v1.catalog.CheckCatalogProviderRequest.FilesEntry

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



<a name="cloud-v1-catalog-checkcatalogproviderresponse"></a>
### cloud.v1.catalog.CheckCatalogProviderResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>diagnostics</td>
<td><a href="../dsl/README.md#cloud-v1-dsl-diagnostic">cloud.v1.dsl.Diagnostic</a></td>
<td><pre>
json_name: diagnostics
go_name: Diagnostics</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-checkcatalogworkflowrequest"></a>
### cloud.v1.catalog.CheckCatalogWorkflowRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-checkcatalogworkflowrequest-filesentry">cloud.v1.catalog.CheckCatalogWorkflowRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-checkcatalogworkflowrequest-filesentry"></a>
### cloud.v1.catalog.CheckCatalogWorkflowRequest.FilesEntry

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



<a name="cloud-v1-catalog-checkcatalogworkflowresponse"></a>
### cloud.v1.catalog.CheckCatalogWorkflowResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>diagnostics</td>
<td><a href="../dsl/README.md#cloud-v1-dsl-diagnostic">cloud.v1.dsl.Diagnostic</a></td>
<td><pre>
json_name: diagnostics
go_name: Diagnostics</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-createinstanceentryrequest"></a>
### cloud.v1.catalog.CreateInstanceEntryRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>description</td>
<td>string</td>
<td><pre>
json_name: description
go_name: Description</pre></td>
</tr><tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-createinstanceentryrequest-filesentry">cloud.v1.catalog.CreateInstanceEntryRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>kind</td>
<td><a href="#cloud-v1-catalog-kind">cloud.v1.catalog.Kind</a></td>
<td><pre>
json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
json_name: slug
go_name: Slug</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-createinstanceentryrequest-filesentry"></a>
### cloud.v1.catalog.CreateInstanceEntryRequest.FilesEntry

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



<a name="cloud-v1-catalog-createinstanceentryresponse"></a>
### cloud.v1.catalog.CreateInstanceEntryResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-createorgproviderrequest"></a>
### cloud.v1.catalog.CreateOrgProviderRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>description</td>
<td>string</td>
<td><pre>
json_name: description
go_name: Description</pre></td>
</tr><tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-createorgproviderrequest-filesentry">cloud.v1.catalog.CreateOrgProviderRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
json_name: slug
go_name: Slug</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-createorgproviderrequest-filesentry"></a>
### cloud.v1.catalog.CreateOrgProviderRequest.FilesEntry

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



<a name="cloud-v1-catalog-createorgproviderresponse"></a>
### cloud.v1.catalog.CreateOrgProviderResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-createorgworkflowrequest"></a>
### cloud.v1.catalog.CreateOrgWorkflowRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>description</td>
<td>string</td>
<td><pre>
json_name: description
go_name: Description</pre></td>
</tr><tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-createorgworkflowrequest-filesentry">cloud.v1.catalog.CreateOrgWorkflowRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
json_name: slug
go_name: Slug</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-createorgworkflowrequest-filesentry"></a>
### cloud.v1.catalog.CreateOrgWorkflowRequest.FilesEntry

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



<a name="cloud-v1-catalog-createorgworkflowresponse"></a>
### cloud.v1.catalog.CreateOrgWorkflowResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-deleteinstanceentryrequest"></a>
### cloud.v1.catalog.DeleteInstanceEntryRequest

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
json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-deleteinstanceentryresponse"></a>
### cloud.v1.catalog.DeleteInstanceEntryResponse



<a name="cloud-v1-catalog-deleteorgproviderrequest"></a>
### cloud.v1.catalog.DeleteOrgProviderRequest

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
json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-deleteorgproviderresponse"></a>
### cloud.v1.catalog.DeleteOrgProviderResponse



<a name="cloud-v1-catalog-deleteorgworkflowrequest"></a>
### cloud.v1.catalog.DeleteOrgWorkflowRequest

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
json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-deleteorgworkflowresponse"></a>
### cloud.v1.catalog.DeleteOrgWorkflowResponse



<a name="cloud-v1-catalog-getinstanceentryrequest"></a>
### cloud.v1.catalog.GetInstanceEntryRequest

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
json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-getinstanceentryresponse"></a>
### cloud.v1.catalog.GetInstanceEntryResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-getorgproviderrequest"></a>
### cloud.v1.catalog.GetOrgProviderRequest

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
json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-getorgproviderresponse"></a>
### cloud.v1.catalog.GetOrgProviderResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-getorgworkflowrequest"></a>
### cloud.v1.catalog.GetOrgWorkflowRequest

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
json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-getorgworkflowresponse"></a>
### cloud.v1.catalog.GetOrgWorkflowResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-kind"></a>
### cloud.v1.catalog.Kind

<pre>
Kind is the catalog object type.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>KIND_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>KIND_PROVIDER</td>
<td></td>
</tr><tr>
<td>KIND_WORKFLOW</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-catalog-level"></a>
### cloud.v1.catalog.Level

<pre>
Level scopes a catalog entry: the shared instance catalog or a tenant's own.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>LEVEL_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>LEVEL_INSTANCE</td>
<td><pre>
LEVEL_INSTANCE entries are visible to every tenant (tenant_id unset).
</pre></td>
</tr><tr>
<td>LEVEL_ORG</td>
<td><pre>
LEVEL_ORG entries belong to a single tenant.
</pre></td>
</tr>
</table>

<a name="cloud-v1-catalog-linkinstanceproviderrequest"></a>
### cloud.v1.catalog.LinkInstanceProviderRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>instance_entry_id</td>
<td>string</td>
<td><pre>
json_name: instanceEntryId
go_name: InstanceEntryId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-linkinstanceproviderresponse"></a>
### cloud.v1.catalog.LinkInstanceProviderResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-linkinstanceworkflowrequest"></a>
### cloud.v1.catalog.LinkInstanceWorkflowRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>instance_entry_id</td>
<td>string</td>
<td><pre>
json_name: instanceEntryId
go_name: InstanceEntryId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-linkinstanceworkflowresponse"></a>
### cloud.v1.catalog.LinkInstanceWorkflowResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-listinstanceentriesrequest"></a>
### cloud.v1.catalog.ListInstanceEntriesRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>kind</td>
<td><a href="#cloud-v1-catalog-kind">cloud.v1.catalog.Kind</a></td>
<td><pre>
json_name: kind
go_name: Kind</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-listinstanceentriesresponse"></a>
### cloud.v1.catalog.ListInstanceEntriesResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entries</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entries
go_name: Entries</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-listorgprovidersrequest"></a>
### cloud.v1.catalog.ListOrgProvidersRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-listorgprovidersresponse"></a>
### cloud.v1.catalog.ListOrgProvidersResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entries</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entries
go_name: Entries</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-listorgworkflowsrequest"></a>
### cloud.v1.catalog.ListOrgWorkflowsRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-listorgworkflowsresponse"></a>
### cloud.v1.catalog.ListOrgWorkflowsResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entries</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entries
go_name: Entries</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-origin"></a>
### cloud.v1.catalog.Origin

<pre>
Origin records how a catalog entry came to exist.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>ORIGIN_NATIVE</td>
<td><pre>
ORIGIN_NATIVE entries were authored directly at this level.
</pre></td>
</tr><tr>
<td>ORIGIN_LINKED</td>
<td><pre>
ORIGIN_LINKED entries reference another level's entry without copying it.
</pre></td>
</tr><tr>
<td>ORIGIN_FORKED</td>
<td><pre>
ORIGIN_FORKED entries were copied from another entry (source_entry_id) and diverge independently.
</pre></td>
</tr>
</table>

<a name="cloud-v1-catalog-updateinstanceentryrequest"></a>
### cloud.v1.catalog.UpdateInstanceEntryRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-updateinstanceentryrequest-filesentry">cloud.v1.catalog.UpdateInstanceEntryRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-updateinstanceentryrequest-filesentry"></a>
### cloud.v1.catalog.UpdateInstanceEntryRequest.FilesEntry

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



<a name="cloud-v1-catalog-updateinstanceentryresponse"></a>
### cloud.v1.catalog.UpdateInstanceEntryResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-updateorgproviderrequest"></a>
### cloud.v1.catalog.UpdateOrgProviderRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-updateorgproviderrequest-filesentry">cloud.v1.catalog.UpdateOrgProviderRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-updateorgproviderrequest-filesentry"></a>
### cloud.v1.catalog.UpdateOrgProviderRequest.FilesEntry

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



<a name="cloud-v1-catalog-updateorgproviderresponse"></a>
### cloud.v1.catalog.UpdateOrgProviderResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-updateorgworkflowrequest"></a>
### cloud.v1.catalog.UpdateOrgWorkflowRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-catalog-updateorgworkflowrequest-filesentry">cloud.v1.catalog.UpdateOrgWorkflowRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-catalog-updateorgworkflowrequest-filesentry"></a>
### cloud.v1.catalog.UpdateOrgWorkflowRequest.FilesEntry

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



<a name="cloud-v1-catalog-updateorgworkflowresponse"></a>
### cloud.v1.catalog.UpdateOrgWorkflowResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entry</td>
<td><a href="#cloud-v1-catalog-catalogentry">cloud.v1.catalog.CatalogEntry</a></td>
<td><pre>
json_name: entry
go_name: Entry</pre></td>
</tr>
</table>

