package types

import (
	"testing"
)

func TestBuiltinPackages_AllPresent(t *testing.T) {
	pkgs := BuiltinPackages()

	expected := map[string]string{
		"postgres-16":   "PostgreSQL 16",
		"postgres-17":   "PostgreSQL 17",
		"mysql-8.0":     "MySQL 8.0",
		"mysql-8.4":     "MySQL 8.4",
		"picodata-25.3": "Picodata 25.3",
	}

	found := map[string]bool{}
	for _, p := range pkgs {
		key := p.DbKind + "-" + p.DbVersion
		found[key] = true
	}

	for key, name := range expected {
		if !found[key] {
			t.Errorf("missing built-in package: %s (%s)", key, name)
		}
	}
}

func TestBuiltinPackages_PostgresHasApt(t *testing.T) {
	for _, p := range BuiltinPackages() {
		if p.DbKind == "postgres" && p.DbVersion == "16" {
			if len(p.AptPackages) == 0 {
				t.Error("postgres 16 should have apt packages")
			}
			found := false
			for _, a := range p.AptPackages {
				if a == "postgresql-16" {
					found = true
				}
			}
			if !found {
				t.Errorf("postgres 16 should contain postgresql-16, got %v", p.AptPackages)
			}
			if len(p.PreInstall) == 0 {
				t.Error("postgres 16 should have pre-install commands")
			}
			return
		}
	}
	t.Error("postgres 16 not found in builtins")
}

func TestPackage_Fields(t *testing.T) {
	pkg := Package{
		Name:          "test",
		DbKind:        "postgres",
		AptPackages:   []string{"pkg1", "pkg2"},
		PreInstall:    []string{"cmd1"},
		CustomRepo:    "deb https://repo apt main",
		CustomRepoKey: "https://repo/key.gpg",
		DebFilename:   "custom.deb",
	}

	if len(pkg.AptPackages) != 2 {
		t.Errorf("expected 2 apt packages, got %d", len(pkg.AptPackages))
	}
	if pkg.CustomRepo == "" {
		t.Error("CustomRepo should not be empty")
	}
}
