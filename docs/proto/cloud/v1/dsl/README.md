

<a name="cloud-v1-dsl"></a>
# cloud.v1.dsl

## Table of Contents
- Messages
  - [cloud.v1.dsl.CheckRequest](#cloud-v1-dsl-checkrequest)
  - [cloud.v1.dsl.CheckRequest.FilesEntry](#cloud-v1-dsl-checkrequest-filesentry)
  - [cloud.v1.dsl.CheckResponse](#cloud-v1-dsl-checkresponse)
  - [cloud.v1.dsl.CompiledJob](#cloud-v1-dsl-compiledjob)
  - [cloud.v1.dsl.CompiledJob.InputGroupsEntry](#cloud-v1-dsl-compiledjob-inputgroupsentry)
  - [cloud.v1.dsl.CompiledJob.MatrixEntry](#cloud-v1-dsl-compiledjob-matrixentry)
  - [cloud.v1.dsl.CompiledJob.ResolvedInputsEntry](#cloud-v1-dsl-compiledjob-resolvedinputsentry)
  - [cloud.v1.dsl.CompiledJob.WithEntry](#cloud-v1-dsl-compiledjob-withentry)
  - [cloud.v1.dsl.CompiledPlan](#cloud-v1-dsl-compiledplan)
  - [cloud.v1.dsl.ComposedSchemaRequest](#cloud-v1-dsl-composedschemarequest)
  - [cloud.v1.dsl.ComposedSchemaRequest.FilesEntry](#cloud-v1-dsl-composedschemarequest-filesentry)
  - [cloud.v1.dsl.ComposedSchemaResponse](#cloud-v1-dsl-composedschemaresponse)
  - [cloud.v1.dsl.ConfigFile](#cloud-v1-dsl-configfile)
  - [cloud.v1.dsl.Diagnostic](#cloud-v1-dsl-diagnostic)
  - [cloud.v1.dsl.DiskSpec](#cloud-v1-dsl-diskspec)
  - [cloud.v1.dsl.DslStep](#cloud-v1-dsl-dslstep)
  - [cloud.v1.dsl.HealthCheck](#cloud-v1-dsl-healthcheck)
  - [cloud.v1.dsl.MachineGroup](#cloud-v1-dsl-machinegroup)
  - [cloud.v1.dsl.PreviewRequest](#cloud-v1-dsl-previewrequest)
  - [cloud.v1.dsl.PreviewRequest.FilesEntry](#cloud-v1-dsl-previewrequest-filesentry)
  - [cloud.v1.dsl.PreviewResponse](#cloud-v1-dsl-previewresponse)
  - [cloud.v1.dsl.ProviderRef](#cloud-v1-dsl-providerref)
  - [cloud.v1.dsl.ServiceSpec](#cloud-v1-dsl-servicespec)
  - [cloud.v1.dsl.ServiceSpec.EnvEntry](#cloud-v1-dsl-servicespec-enventry)
  - [cloud.v1.dsl.Severity](#cloud-v1-dsl-severity)
  - [cloud.v1.dsl.StepList](#cloud-v1-dsl-steplist)
  - [cloud.v1.dsl.WaitStep](#cloud-v1-dsl-waitstep)

<a name="cloud-v1-dsl-services"></a>
## Services

<a name="cloud-v1-dsl-messages"></a>
## Messages

<a name="cloud-v1-dsl-checkrequest"></a>
### cloud.v1.dsl.CheckRequest

<pre>
CheckRequest несёт тот же бандл, что и ComposedSchemaRequest.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-dsl-checkrequest-filesentry">cloud.v1.dsl.CheckRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-checkrequest-filesentry"></a>
### cloud.v1.dsl.CheckRequest.FilesEntry

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



<a name="cloud-v1-dsl-checkresponse"></a>
### cloud.v1.dsl.CheckResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>diagnostics</td>
<td><a href="#cloud-v1-dsl-diagnostic">cloud.v1.dsl.Diagnostic</a></td>
<td><pre>
diagnostics — результат check-режима компиляции. Пустой список значит
"бандл компилируется чисто"; ошибки пользовательского ввода — это ВСЕГДА
диагностика, а не RPC-ошибка (RPC-ошибка — только транспортный сбой).<br>

json_name: diagnostics
go_name: Diagnostics</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-compiledjob"></a>
### cloud.v1.dsl.CompiledJob

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
id включает matrix-инстанс: "bench[workload=insert]".<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>input_groups</td>
<td><a href="#cloud-v1-dsl-compiledjob-inputgroupsentry">cloud.v1.dsl.CompiledJob.InputGroupsEntry</a></td>
<td><pre>
input_groups maps a machine_group-typed input name → the cluster
machine group name it was bound to, so the runtime can bind
`inputs.<name>` to that group's MachineGroupView. Empty for a job that
did not originate from an include component.<br>

json_name: inputGroups
go_name: InputGroups</pre></td>
</tr><tr>
<td>matrix</td>
<td><a href="#cloud-v1-dsl-compiledjob-matrixentry">cloud.v1.dsl.CompiledJob.MatrixEntry</a></td>
<td><pre>
json_name: matrix
go_name: Matrix</pre></td>
</tr><tr>
<td>needs</td>
<td>string</td>
<td><pre>
json_name: needs
go_name: Needs</pre></td>
</tr><tr>
<td>on_group</td>
<td>string</td>
<td><pre>
json_name: onGroup
go_name: OnGroup</pre></td>
</tr><tr>
<td>resolved_inputs</td>
<td><a href="#cloud-v1-dsl-compiledjob-resolvedinputsentry">cloud.v1.dsl.CompiledJob.ResolvedInputsEntry</a></td>
<td><pre>
resolved_inputs are the component's scalar inputs (int/string/bool)
resolved at compile time, bound as CEL `inputs.<name>` at runtime.
Values are the stringified scalar; the runtime rebinds them typed via
the component's InputSpec if needed (v1: string-typed dyn). Empty for
a job that did not originate from an include component.<br>

json_name: resolvedInputs
go_name: ResolvedInputs</pre></td>
</tr><tr>
<td>service</td>
<td>string</td>
<td><pre>
service — имя ServiceSpec: запуск Nomad job + ожидание health.<br>

json_name: service
go_name: Service</pre></td>
</tr><tr>
<td>steps</td>
<td><a href="#cloud-v1-dsl-steplist">cloud.v1.dsl.StepList</a></td>
<td><pre>
json_name: steps
go_name: Steps</pre></td>
</tr><tr>
<td>target_group</td>
<td>string</td>
<td><pre>
target_group is the single machine_group input's group name (the CEL
`target` binding), empty when the component has zero or multiple
machine_group inputs, or the job did not originate from an include
component.<br>

json_name: targetGroup
go_name: TargetGroup</pre></td>
</tr><tr>
<td>when</td>
<td>string</td>
<td><pre>
json_name: when
go_name: When</pre></td>
</tr><tr>
<td>with</td>
<td><a href="#cloud-v1-dsl-compiledjob-withentry">cloud.v1.dsl.CompiledJob.WithEntry</a></td>
<td><pre>
json_name: with
go_name: With</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-compiledjob-inputgroupsentry"></a>
### cloud.v1.dsl.CompiledJob.InputGroupsEntry

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



<a name="cloud-v1-dsl-compiledjob-matrixentry"></a>
### cloud.v1.dsl.CompiledJob.MatrixEntry

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



<a name="cloud-v1-dsl-compiledjob-resolvedinputsentry"></a>
### cloud.v1.dsl.CompiledJob.ResolvedInputsEntry

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



<a name="cloud-v1-dsl-compiledjob-withentry"></a>
### cloud.v1.dsl.CompiledJob.WithEntry

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



<a name="cloud-v1-dsl-compiledplan"></a>
### cloud.v1.dsl.CompiledPlan

<pre>
CompiledPlan — результат компиляции DSL-бандла: запрос машин провайдеру,
сервисы (Nomad jobs) и DAG джобов. Строковые поля могут содержать
невычисленные ${{ CEL }} — их вычисляет исполнитель по рантайм-данным.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>jobs</td>
<td><a href="#cloud-v1-dsl-compiledjob">cloud.v1.dsl.CompiledJob</a></td>
<td><pre>
json_name: jobs
go_name: Jobs</pre></td>
</tr><tr>
<td>machine_groups</td>
<td><a href="#cloud-v1-dsl-machinegroup">cloud.v1.dsl.MachineGroup</a></td>
<td><pre>
json_name: machineGroups
go_name: MachineGroups</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="#cloud-v1-dsl-providerref">cloud.v1.dsl.ProviderRef</a></td>
<td><pre>
json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>services</td>
<td><a href="#cloud-v1-dsl-servicespec">cloud.v1.dsl.ServiceSpec</a></td>
<td><pre>
json_name: services
go_name: Services</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-composedschemarequest"></a>
### cloud.v1.dsl.ComposedSchemaRequest

<pre>
ComposedSchemaRequest несёт весь бандл рецепта (cluster.yaml, workflow.yaml,
components/**, providers/**, files/**, ...) как path -> содержимое файла.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-dsl-composedschemarequest-filesentry">cloud.v1.dsl.ComposedSchemaRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-composedschemarequest-filesentry"></a>
### cloud.v1.dsl.ComposedSchemaRequest.FilesEntry

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



<a name="cloud-v1-dsl-composedschemaresponse"></a>
### cloud.v1.dsl.ComposedSchemaResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>schema_json</td>
<td>string</td>
<td><pre>
schema_json — динамическая JSON Schema (core-схема + provider params/ext
бандла), против которой редактор валидирует cluster.yaml.<br>

json_name: schemaJson
go_name: SchemaJson</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-configfile"></a>
### cloud.v1.dsl.ConfigFile

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>dest</td>
<td>string</td>
<td><pre>
json_name: dest
go_name: Dest</pre></td>
</tr><tr>
<td>template_path</td>
<td>string</td>
<td><pre>
json_name: templatePath
go_name: TemplatePath</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-diagnostic"></a>
### cloud.v1.dsl.Diagnostic

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>col</td>
<td>uint32</td>
<td><pre>
json_name: col
go_name: Col</pre></td>
</tr><tr>
<td>line</td>
<td>uint32</td>
<td><pre>
json_name: line
go_name: Line</pre></td>
</tr><tr>
<td>message</td>
<td>string</td>
<td><pre>
json_name: message
go_name: Message</pre></td>
</tr><tr>
<td>module</td>
<td>string</td>
<td><pre>
module — имя include-компонента/модуля, из которого пришла диагностика
(пусто для диагностик верхнего уровня бандла).<br>

json_name: module
go_name: Module</pre></td>
</tr><tr>
<td>path</td>
<td>string</td>
<td><pre>
path — путь файла внутри бандла, к которому относится диагностика.<br>

json_name: path
go_name: Path</pre></td>
</tr><tr>
<td>severity</td>
<td><a href="#cloud-v1-dsl-severity">cloud.v1.dsl.Severity</a></td>
<td><pre>
json_name: severity
go_name: Severity</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-diskspec"></a>
### cloud.v1.dsl.DiskSpec

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>size_gb</td>
<td>uint64</td>
<td><pre>
json_name: sizeGb
go_name: SizeGb</pre></td>
</tr><tr>
<td>type</td>
<td>string</td>
<td><pre>
type — уже lowered в термины провайдера.<br>

json_name: type
go_name: Type</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-dslstep"></a>
### cloud.v1.dsl.DslStep

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>agent</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-agentstep">cloud.v1.deployment.AgentStep</a></td>
<td><pre>
json_name: agent
go_name: Agent</pre></td>
</tr><tr>
<td>wait</td>
<td><a href="#cloud-v1-dsl-waitstep">cloud.v1.dsl.WaitStep</a></td>
<td><pre>
json_name: wait
go_name: Wait</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-healthcheck"></a>
### cloud.v1.dsl.HealthCheck

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>http</td>
<td>string</td>
<td><pre>
json_name: http
go_name: Http</pre></td>
</tr><tr>
<td>timeout</td>
<td>string</td>
<td><pre>
json_name: timeout
go_name: Timeout</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-machinegroup"></a>
### cloud.v1.dsl.MachineGroup

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>count</td>
<td>uint32</td>
<td><pre>
json_name: count
go_name: Count</pre></td>
</tr><tr>
<td>cpu</td>
<td>uint32</td>
<td><pre>
json_name: cpu
go_name: Cpu</pre></td>
</tr><tr>
<td>disks</td>
<td><a href="#cloud-v1-dsl-diskspec">cloud.v1.dsl.DiskSpec</a></td>
<td><pre>
json_name: disks
go_name: Disks</pre></td>
</tr><tr>
<td>ext_json</td>
<td>string</td>
<td><pre>
ext_json — провайдерский ext-блок после lowering-прецеденса, как JSON.<br>

json_name: extJson
go_name: ExtJson</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>ram_mb</td>
<td>uint64</td>
<td><pre>
json_name: ramMb
go_name: RamMb</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-previewrequest"></a>
### cloud.v1.dsl.PreviewRequest

<pre>
PreviewRequest несёт тот же бандл, что и CheckRequest/ComposedSchemaRequest.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>files</td>
<td><a href="#cloud-v1-dsl-previewrequest-filesentry">cloud.v1.dsl.PreviewRequest.FilesEntry</a></td>
<td><pre>
json_name: files
go_name: Files</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-previewrequest-filesentry"></a>
### cloud.v1.dsl.PreviewRequest.FilesEntry

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



<a name="cloud-v1-dsl-previewresponse"></a>
### cloud.v1.dsl.PreviewResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>diagnostics</td>
<td><a href="#cloud-v1-dsl-diagnostic">cloud.v1.dsl.Diagnostic</a></td>
<td><pre>
diagnostics — как в CheckResponse: ошибки пользовательского ввода
ВСЕГДА диагностика, никогда RPC-ошибка.<br>

json_name: diagnostics
go_name: Diagnostics</pre></td>
</tr><tr>
<td>plan</td>
<td><a href="#cloud-v1-dsl-compiledplan">cloud.v1.dsl.CompiledPlan</a></td>
<td><pre>
plan — резолвленный CompiledPlan (machine_groups/services/jobs) для
панели предпросмотра в редакторе рецепта. Может быть nil, если бандл не
скомпилировался (см. diagnostics) — плана "что будет развёрнуто"
показать нечего.<br>

json_name: plan
go_name: Plan</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-providerref"></a>
### cloud.v1.dsl.ProviderRef

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
json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>params_json</td>
<td>string</td>
<td><pre>
params_json — provider.params как JSON (схема динамическая, в proto не типизируется).<br>

json_name: paramsJson
go_name: ParamsJson</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-servicespec"></a>
### cloud.v1.dsl.ServiceSpec

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>configs</td>
<td><a href="#cloud-v1-dsl-configfile">cloud.v1.dsl.ConfigFile</a></td>
<td><pre>
json_name: configs
go_name: Configs</pre></td>
</tr><tr>
<td>env</td>
<td><a href="#cloud-v1-dsl-servicespec-enventry">cloud.v1.dsl.ServiceSpec.EnvEntry</a></td>
<td><pre>
json_name: env
go_name: Env</pre></td>
</tr><tr>
<td>health</td>
<td><a href="#cloud-v1-dsl-healthcheck">cloud.v1.dsl.HealthCheck</a></td>
<td><pre>
json_name: health
go_name: Health</pre></td>
</tr><tr>
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
</tr><tr>
<td>network</td>
<td>string</td>
<td><pre>
json_name: network
go_name: Network</pre></td>
</tr><tr>
<td>on_group</td>
<td>string</td>
<td><pre>
json_name: onGroup
go_name: OnGroup</pre></td>
</tr><tr>
<td>volumes</td>
<td>string</td>
<td><pre>
json_name: volumes
go_name: Volumes</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-servicespec-enventry"></a>
### cloud.v1.dsl.ServiceSpec.EnvEntry

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



<a name="cloud-v1-dsl-severity"></a>
### cloud.v1.dsl.Severity

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>SEVERITY_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>SEVERITY_ERROR</td>
<td></td>
</tr><tr>
<td>SEVERITY_WARNING</td>
<td></td>
</tr>
</table>

<a name="cloud-v1-dsl-steplist"></a>
### cloud.v1.dsl.StepList

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>steps</td>
<td><a href="#cloud-v1-dsl-dslstep">cloud.v1.dsl.DslStep</a></td>
<td><pre>
json_name: steps
go_name: Steps</pre></td>
</tr>
</table>



<a name="cloud-v1-dsl-waitstep"></a>
### cloud.v1.dsl.WaitStep

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>http</td>
<td>string</td>
<td><pre>
json_name: http
go_name: Http</pre></td>
</tr><tr>
<td>timeout</td>
<td>string</td>
<td><pre>
json_name: timeout
go_name: Timeout</pre></td>
</tr>
</table>

