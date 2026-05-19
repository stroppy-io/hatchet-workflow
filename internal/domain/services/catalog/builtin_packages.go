package catalog

import (
	"context"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// builtinPkg is the seed-time shape; mirrors main types.Package fields.
type builtinPkg struct {
	name        string
	description string
	kind        catalogpb.Database_Kind
	version     string
	apt         []string
	preInstall  []string
}

// builtinPackages returns the default catalog every tenant should see right
// after creation. 1:1 with `main` types.BuiltinPackages() — keep in sync
// when adding new engines.
func builtinPackages() []builtinPkg {
	return []builtinPkg{
		{
			name: "PostgreSQL 16", description: "Default PostgreSQL 16 from pgdg",
			kind: catalogpb.Database_DATABASE_KIND_POSTGRES, version: "16",
			apt: []string{"postgresql-16", "postgresql-client-16"},
			preInstall: []string{
				`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
				`wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -`,
				`apt-get update`,
			},
		},
		{
			name: "PostgreSQL 17", description: "Default PostgreSQL 17 from pgdg",
			kind: catalogpb.Database_DATABASE_KIND_POSTGRES, version: "17",
			apt: []string{"postgresql-17", "postgresql-client-17"},
			preInstall: []string{
				`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
				`wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -`,
				`apt-get update`,
			},
		},
		{
			name: "MySQL 8.0", description: "Default MySQL 8.0",
			kind: catalogpb.Database_DATABASE_KIND_MYSQL, version: "8.0",
			apt: []string{"mysql-server-8.0", "mysql-client"},
			preInstall: []string{
				`apt-get install -y curl gnupg lsb-release ca-certificates`,
				`install -d /etc/apt/keyrings`,
				`curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 | gpg --dearmor -o /etc/apt/keyrings/mysql.gpg`,
				`bash -c 'echo "deb [signed-by=/etc/apt/keyrings/mysql.gpg] http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) mysql-8.0" > /etc/apt/sources.list.d/mysql.list'`,
				`apt-get update`,
			},
		},
		{
			name: "MySQL 8.4", description: "Default MySQL 8.4 (LTS)",
			kind: catalogpb.Database_DATABASE_KIND_MYSQL, version: "8.4",
			apt: []string{"mysql-server-8.4", "mysql-client"},
			preInstall: []string{
				`apt-get install -y curl gnupg lsb-release ca-certificates`,
				`install -d /etc/apt/keyrings`,
				`curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 | gpg --dearmor -o /etc/apt/keyrings/mysql.gpg`,
				`bash -c 'echo "deb [signed-by=/etc/apt/keyrings/mysql.gpg] http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) mysql-8.4-lts" > /etc/apt/sources.list.d/mysql.list'`,
				`apt-get update`,
			},
		},
		{
			name: "MariaDB 10.11", description: "MariaDB 10.11 LTS",
			kind: catalogpb.Database_DATABASE_KIND_MARIADB, version: "10.11",
			apt: []string{"mariadb-server", "mariadb-client"},
			preInstall: []string{
				`curl -fsSL https://r.mariadb.com/downloads/mariadb_repo_setup -o /tmp/mariadb_repo_setup`,
				`bash /tmp/mariadb_repo_setup --mariadb-server-version=10.11`,
				`apt-get update`,
			},
		},
		{
			name: "MariaDB 11.4", description: "MariaDB 11.4 LTS",
			kind: catalogpb.Database_DATABASE_KIND_MARIADB, version: "11.4",
			apt: []string{"mariadb-server", "mariadb-client"},
			preInstall: []string{
				`curl -fsSL https://r.mariadb.com/downloads/mariadb_repo_setup -o /tmp/mariadb_repo_setup`,
				`bash /tmp/mariadb_repo_setup --mariadb-server-version=11.4`,
				`apt-get update`,
			},
		},
		{
			name: "Picodata 25.3", description: "Default Picodata 25.3",
			kind: catalogpb.Database_DATABASE_KIND_PICODATA, version: "25.3",
			apt: []string{"picodata"},
		},
		{
			name: "Cockroach 24.3", description: "Default CockroachDB 24.3",
			kind: catalogpb.Database_DATABASE_KIND_COCKROACH, version: "24.3",
		},
		{
			name: "YDB Static 24",
			kind: catalogpb.Database_DATABASE_KIND_YDB, version: "24",
		},
		{
			name: "Monitor Agents", description: "Prometheus node_exporter for VM-side metrics",
			kind: catalogpb.Database_DATABASE_KIND_UNSPECIFIED, version: "",
			apt: []string{"prometheus-node-exporter"},
		},
	}
}

// SeedBuiltinPackages inserts the default catalog for a tenant. Idempotent:
// rows already present for this tenant are skipped (matched by name).
func (s *Service) SeedBuiltinPackages(ctx context.Context, tenantID *iampb.TenantId, createdBy *iampb.UserId) error {
	existing, err := s.ListPackages(ctx, tenantID, nil)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, p := range existing {
		if p.GetIdentity() != nil {
			seen[p.GetIdentity().GetName()] = struct{}{}
		}
	}
	for _, bp := range builtinPackages() {
		if _, ok := seen[bp.name]; ok {
			continue
		}
		desc := bp.description
		pkg := &catalogpb.Package{
			Identity:  &commonpb.Identity{Name: bp.name, Description: &desc},
			DbKind:    bp.kind,
			DbVersion: bp.version,
			IsBuiltin: true,
			Source: &catalogpb.Package_PackageSource{
				Source: &catalogpb.Package_PackageSource_Apt{
					Apt: &catalogpb.Package_AptSource{
						AptPackages: bp.apt,
						PreInstall:  bp.preInstall,
					},
				},
			},
		}
		if _, err := s.CreatePackage(ctx, tenantID, createdBy, pkg); err != nil {
			return err
		}
	}
	return nil
}
