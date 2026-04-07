package agent

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

func TestInstallPackage_FieldsAccessible(t *testing.T) {
	pkg := types.Package{
		Name:          "test-pkg",
		AptPackages:   []string{"postgresql-16"},
		PreInstall:    []string{"apt-get update"},
		CustomRepo:    "deb https://repo apt main",
		CustomRepoKey: "https://repo/key.gpg",
		DebFilename:   "custom.deb",
	}

	if len(pkg.AptPackages) != 1 {
		t.Errorf("expected 1 apt package, got %d", len(pkg.AptPackages))
	}
	if pkg.DebFilename != "custom.deb" {
		t.Errorf("expected deb_filename custom.deb, got %s", pkg.DebFilename)
	}
}

func TestBuiltinPackages_ContainsPostgres16(t *testing.T) {
	for _, p := range types.BuiltinPackages() {
		if p.DbKind == "postgres" && p.DbVersion == "16" {
			if len(p.AptPackages) == 0 {
				t.Error("postgres 16 should have apt packages")
			}
			return
		}
	}
	t.Error("postgres 16 not found in builtins")
}

func TestResolveMemoryDefaults_Percentages(t *testing.T) {
	m := map[string]string{
		"shared_buffers":       "25%",
		"effective_cache_size": "75%",
		"max_connections":      "200",
	}
	resolveMemoryDefaults(m)

	if m["shared_buffers"] == "25%" {
		t.Error("shared_buffers was not resolved from percentage")
	}
	if m["effective_cache_size"] == "75%" {
		t.Error("effective_cache_size was not resolved from percentage")
	}
	if m["max_connections"] != "200" {
		t.Errorf("max_connections should remain 200, got %s", m["max_connections"])
	}
}

func TestResolveMemoryDefaults_NoPercentage(t *testing.T) {
	m := map[string]string{
		"work_mem": "64MB",
		"listen":   "'*'",
	}
	resolveMemoryDefaults(m)
	if m["work_mem"] != "64MB" {
		t.Errorf("work_mem should remain 64MB, got %s", m["work_mem"])
	}
	if m["listen"] != "'*'" {
		t.Errorf("listen should remain '*', got %s", m["listen"])
	}
}

func TestResolveMemoryDefaults_MinimumFloor(t *testing.T) {
	m := map[string]string{
		"tiny_param": "1%",
	}
	resolveMemoryDefaults(m)
	if m["tiny_param"] == "1%" {
		t.Error("tiny_param was not resolved from percentage")
	}
}

func TestParseConfig_Success(t *testing.T) {
	type testCfg struct {
		Version string `json:"version"`
		Port    int    `json:"port"`
	}

	cmd := Command{
		ID:     "test-1",
		Action: ActionInstallPostgres,
		Config: map[string]any{
			"version": "16",
			"port":    5432,
		},
	}

	var cfg testCfg
	if err := parseConfig(cmd, &cfg); err != nil {
		t.Fatalf("parseConfig failed: %v", err)
	}
	if cfg.Version != "16" {
		t.Errorf("expected version 16, got %s", cfg.Version)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cfg.Port)
	}
}

func TestParseConfig_NilConfig(t *testing.T) {
	type testCfg struct {
		Name string `json:"name"`
	}
	cmd := Command{ID: "test-2", Action: ActionInstallPostgres, Config: nil}
	var cfg testCfg
	if err := parseConfig(cmd, &cfg); err != nil {
		t.Fatalf("parseConfig with nil config should not error: %v", err)
	}
	if cfg.Name != "" {
		t.Errorf("expected empty name, got %q", cfg.Name)
	}
}

func TestParseConfig_NestedStruct(t *testing.T) {
	type inner struct {
		Key string `json:"key"`
	}
	type outer struct {
		Inner inner `json:"inner"`
	}

	cmd := Command{
		ID:     "test-3",
		Action: ActionConfigPostgres,
		Config: map[string]any{
			"inner": map[string]any{"key": "value"},
		},
	}

	var cfg outer
	if err := parseConfig(cmd, &cfg); err != nil {
		t.Fatalf("parseConfig with nested struct failed: %v", err)
	}
	if cfg.Inner.Key != "value" {
		t.Errorf("expected inner key 'value', got %q", cfg.Inner.Key)
	}
}
