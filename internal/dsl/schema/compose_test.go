package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
)

const testdataYandexModule = "testdata/yandex-module"

func TestDeriveParamsSchemaZoneAndExt(t *testing.T) {
	params, ext, err := schema.DeriveParamsSchema(testdataYandexModule)
	if err != nil {
		t.Fatalf("DeriveParamsSchema: %v", err)
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatalf("params.properties is not an object: %+v", params)
	}
	zone, ok := props["zone"].(map[string]any)
	if !ok {
		t.Fatalf("params.properties.zone missing or not an object: %+v", props)
	}
	if zone["type"] != "string" {
		t.Fatalf("params.properties.zone.type = %v, want %q", zone["type"], "string")
	}
	if zone["default"] != "ru-central1-a" {
		t.Fatalf("params.properties.zone.default = %v, want %q", zone["default"], "ru-central1-a")
	}
	if params["additionalProperties"] != false {
		t.Fatalf("params.additionalProperties = %v, want false", params["additionalProperties"])
	}

	if ext == nil {
		t.Fatal("ext is nil, want a schema derived from the stroppy_machine_ext variable")
	}
	extProps, ok := ext["properties"].(map[string]any)
	if !ok {
		t.Fatalf("ext.properties is not an object: %+v", ext)
	}
	platformID, ok := extProps["platform_id"].(map[string]any)
	if !ok {
		t.Fatalf("ext.properties.platform_id missing or not an object: %+v", extProps)
	}
	if platformID["type"] != "string" {
		t.Fatalf("ext.properties.platform_id.type = %v, want %q", platformID["type"], "string")
	}
	if platformID["default"] != "standard-v3" {
		t.Fatalf("ext.properties.platform_id.default = %v, want %q", platformID["default"], "standard-v3")
	}
	coreFraction, ok := extProps["core_fraction"].(map[string]any)
	if !ok {
		t.Fatalf("ext.properties.core_fraction missing or not an object: %+v", extProps)
	}
	if coreFraction["type"] != "number" {
		t.Fatalf("ext.properties.core_fraction.type = %v, want %q", coreFraction["type"], "number")
	}
	if ext["additionalProperties"] != false {
		t.Fatalf("ext.additionalProperties = %v, want false", ext["additionalProperties"])
	}
}

// clusterYAMLWithMachineExt mirrors validate_test.go's clusterYAML fixture,
// parameterized on the `machines.db.yandex` block so tests can swap it.
func clusterYAMLWithMachineExt(yandexBlock string) string {
	return `
version: 1
provider:
  use: yandex
  params: { zone: ru-central1-a }
machines:
  db:
    count: 3
    resources: { cpu: 8, ram: 32g, disk: { size: 100g, type: ssd } }
    yandex: ` + yandexBlock + `
services:
  postgres:
    on: db
    image: postgres:17
    network: host
    env: { A: "${{ machines.db[0].ip }}" }
    health: { http: ":8008/health", timeout: 120s }
`
}

func composeYandex(t *testing.T) (*schema.ProviderSchemas, map[string]schema.ProviderSchemas) {
	t.Helper()
	params, ext, err := schema.DeriveParamsSchema(testdataYandexModule)
	if err != nil {
		t.Fatalf("DeriveParamsSchema: %v", err)
	}
	ps := schema.ProviderSchemas{Params: params, Ext: ext}
	return &ps, map[string]schema.ProviderSchemas{"yandex": ps}
}

func TestComposeValidExtCleanValidation(t *testing.T) {
	_, providers := composeYandex(t)
	compiled, raw, err := schema.Compose(providers, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if compiled == nil {
		t.Fatal("Compose returned a nil compiled schema")
	}
	var rawDoc any
	if err := json.Unmarshal(raw, &rawDoc); err != nil {
		t.Fatalf("Compose's []byte is not valid JSON: %v", err)
	}

	src := clusterYAMLWithMachineExt(`{ platform_id: standard-v3, core_fraction: 100, preemptible: false }`)
	diags := schema.Validate(schema.Cluster, "cluster.yaml", []byte(src), compiled)
	if diags.HasErrors() {
		t.Fatalf("valid ext block must validate clean, got: %+v", diags)
	}
}

func TestComposePlatformIDTypeErrors(t *testing.T) {
	_, providers := composeYandex(t)
	compiled, _, err := schema.Compose(providers, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	src := clusterYAMLWithMachineExt(`{ platform_id: 5 }`)
	diags := schema.Validate(schema.Cluster, "cluster.yaml", []byte(src), compiled)
	if !diags.HasErrors() {
		t.Fatal("platform_id: 5 (number instead of string) must produce an error diagnostic")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "platform_id") && strings.Contains(d.Message, "string") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic naming platform_id's string/type mismatch, got: %+v", diags)
	}
}

func TestComposeUnknownExtFieldErrors(t *testing.T) {
	_, providers := composeYandex(t)
	compiled, _, err := schema.Compose(providers, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	src := clusterYAMLWithMachineExt(`{ unknown_field: 1 }`)
	diags := schema.Validate(schema.Cluster, "cluster.yaml", []byte(src), compiled)
	if !diags.HasErrors() {
		t.Fatal("yandex: { unknown_field: 1 } must produce an error diagnostic (additionalProperties: false)")
	}
}

func TestComposeUnknownProviderParamErrors(t *testing.T) {
	_, providers := composeYandex(t)
	compiled, _, err := schema.Compose(providers, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	src := `
version: 1
provider:
  use: yandex
  params: { zone: ru-central1-a, bogus_param: 1 }
machines:
  db:
    count: 1
    resources: { cpu: 8, ram: 32g, disk: { size: 100g, type: ssd } }
`
	diags := schema.Validate(schema.Cluster, "cluster.yaml", []byte(src), compiled)
	if !diags.HasErrors() {
		t.Fatal("provider.params.bogus_param must produce an error diagnostic (additionalProperties: false, derived from variables.tf)")
	}
}

func TestComposeIncludeInputsFragmentsStoredUnderDefs(t *testing.T) {
	_, providers := composeYandex(t)
	fragments := map[string]map[string]any{
		"etcd": {"type": "object", "properties": map[string]any{"nodes": map[string]any{"type": "integer"}}},
	}
	_, raw, err := schema.Compose(providers, fragments)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal composed doc: %v", err)
	}
	defs, ok := doc["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("composed doc has no $defs object: %+v", doc)
	}
	if _, ok := defs["includeInputs_etcd"]; !ok {
		t.Fatalf("expected $defs.includeInputs_etcd, got keys: %v", keysOf(defs))
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
