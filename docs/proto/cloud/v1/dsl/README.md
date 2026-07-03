

<a name="cloud-v1-dsl"></a>
# cloud.v1.dsl

## Table of Contents
- Messages
  - [cloud.v1.dsl.CheckRequest](#cloud-v1-dsl-checkrequest)
  - [cloud.v1.dsl.CheckRequest.FilesEntry](#cloud-v1-dsl-checkrequest-filesentry)
  - [cloud.v1.dsl.CheckResponse](#cloud-v1-dsl-checkresponse)
  - [cloud.v1.dsl.ComposedSchemaRequest](#cloud-v1-dsl-composedschemarequest)
  - [cloud.v1.dsl.ComposedSchemaRequest.FilesEntry](#cloud-v1-dsl-composedschemarequest-filesentry)
  - [cloud.v1.dsl.ComposedSchemaResponse](#cloud-v1-dsl-composedschemaresponse)
  - [cloud.v1.dsl.Diagnostic](#cloud-v1-dsl-diagnostic)
  - [cloud.v1.dsl.Severity](#cloud-v1-dsl-severity)

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