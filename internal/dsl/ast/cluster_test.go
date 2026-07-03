package ast_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
)

const clusterYAML = `
version: 1
provider:
  use: yandex
  params: { zone: ru-central1-a }
machines:
  db:
    count: 3
    resources: { cpu: 8, ram: 32g, disk: { size: 100g, type: ssd } }
    yandex: { platform_id: standard-v3 }
services:
  postgres:
    on: db
    image: postgres:17
    network: host
    env: { A: "${{ machines.db[0].ip }}" }
    health: { http: ":8008/health", timeout: 120s }
`

func TestDecodeCluster(t *testing.T) {
	doc, diags := ast.DecodeCluster("cluster.yaml", []byte(clusterYAML), "yandex")
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	db := doc.Machines["db"]
	if db.Count != 3 || db.Resources.CPU != 8 || db.Resources.RAM != 32<<30 {
		t.Fatalf("bad machines.db: %+v", db)
	}
	if db.Resources.Disk.Size != 100<<30 || db.Resources.Disk.Type != "ssd" {
		t.Fatalf("bad disk: %+v", db.Resources.Disk)
	}
	if db.Ext["platform_id"] != "standard-v3" {
		t.Fatalf("ext not captured: %+v", db.Ext)
	}
	if doc.Services["postgres"].On != "db" {
		t.Fatal("service.on lost")
	}
}

func TestDecodeClusterUnknownKey(t *testing.T) {
	_, diags := ast.DecodeCluster("cluster.yaml", []byte("version: 1\nmachines:\n  db: { count: 1, resurces: {} }"), "yandex")
	if !diags.HasErrors() {
		t.Fatal("typo key must produce error diagnostic")
	}
}

// --- edge cases implied by the brief's semantics ---

func TestDecodeClusterUnknownKeyHasPosition(t *testing.T) {
	_, diags := ast.DecodeCluster("cluster.yaml", []byte("version: 1\nmachines:\n  db: { count: 1, resurces: {} }"), "yandex")
	if !diags.HasErrors() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range diags {
		if d.Pos.Line > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected at least one diagnostic with a real line position, got: %+v", diags)
	}
}

func TestDecodeClusterExtUnderWrongProviderKey(t *testing.T) {
	// providerKey passed to DecodeCluster is "yandex", but the group has an
	// "aws" block instead: since it doesn't match providerKey, it must be
	// rejected as an unknown key, not silently captured.
	src := []byte("version: 1\nmachines:\n  db: { count: 1, aws: { instance_type: t3.micro } }")
	_, diags := ast.DecodeCluster("cluster.yaml", src, "yandex")
	if !diags.HasErrors() {
		t.Fatal("ext block under a provider key that doesn't match providerKey must be an error")
	}
}

func TestDecodeClusterBadByteSize(t *testing.T) {
	src := []byte("version: 1\nmachines:\n  db: { count: 1, resources: { cpu: 1, ram: notasize } }")
	_, diags := ast.DecodeCluster("cluster.yaml", src, "yandex")
	if !diags.HasErrors() {
		t.Fatal("bad ByteSize string must produce error diagnostic")
	}
}

func TestDecodeClusterBadDuration(t *testing.T) {
	src := []byte("version: 1\nservices:\n  postgres: { on: db, image: x, health: { http: /h, timeout: notaduration } }")
	_, diags := ast.DecodeCluster("cluster.yaml", src, "yandex")
	if !diags.HasErrors() {
		t.Fatal("bad Duration string must produce error diagnostic")
	}
}

func TestDecodeClusterServiceFields(t *testing.T) {
	src := []byte(`
version: 1
services:
  postgres:
    on: db
    image: postgres:17
    network: host
    volumes: ["/data:/var/lib/postgresql/data"]
    env: { PGUSER: admin }
    configs:
      - template: postgresql.conf.tmpl
        dest: /etc/postgresql/postgresql.conf
    health: { http: ":8008/health", timeout: 30s }
`)
	doc, diags := ast.DecodeCluster("cluster.yaml", src, "yandex")
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	svc := doc.Services["postgres"]
	if len(svc.Volumes) != 1 || svc.Volumes[0] != "/data:/var/lib/postgresql/data" {
		t.Fatalf("bad volumes: %+v", svc.Volumes)
	}
	if svc.Env["PGUSER"] != "admin" {
		t.Fatalf("bad env: %+v", svc.Env)
	}
	if len(svc.Configs) != 1 || svc.Configs[0].Template != "postgresql.conf.tmpl" || svc.Configs[0].Dest != "/etc/postgresql/postgresql.conf" {
		t.Fatalf("bad configs: %+v", svc.Configs)
	}
	if svc.Health == nil || svc.Health.HTTP != ":8008/health" {
		t.Fatalf("bad health: %+v", svc.Health)
	}
}

func TestDecodeClusterUnknownTopLevelKey(t *testing.T) {
	_, diags := ast.DecodeCluster("cluster.yaml", []byte("version: 1\nbogus: true"), "yandex")
	if !diags.HasErrors() {
		t.Fatal("unknown top-level key must produce error diagnostic")
	}
}
