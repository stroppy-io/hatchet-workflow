package types

// Package describes a database package that agents install on target machines.
// Stored in the packages table; one row = one installable unit.
type Package struct {
	ID            string   `json:"id"`
	TenantID      string   `json:"tenant_id,omitempty"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	DbKind        string   `json:"db_kind"`
	DbVersion     string   `json:"db_version,omitempty"`
	IsBuiltin     bool     `json:"is_builtin"`
	AptPackages   []string `json:"apt_packages"`
	PreInstall    []string `json:"pre_install,omitempty"`
	CustomRepo    string   `json:"custom_repo,omitempty"`
	CustomRepoKey string   `json:"custom_repo_key,omitempty"`
	DebFilename   string   `json:"deb_filename,omitempty"`
	// DebToken is the auth token for downloading the .deb file. Injected at run start.
	DebToken string `json:"deb_token,omitempty"`
	// DebData is not serialized to JSON — served via a dedicated endpoint.
}

// BuiltinPackages returns the default packages for all supported databases.
// Used to seed new tenants.
func BuiltinPackages() []Package {
	return []Package{
		{
			Name: "PostgreSQL 16", Description: "Default PostgreSQL 16 from pgdg",
			DbKind: "postgres", DbVersion: "16", IsBuiltin: true,
			AptPackages: []string{"postgresql-16", "postgresql-client-16"},
			PreInstall: []string{
				`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
				"wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
				"apt-get update",
			},
		},
		{
			Name: "PostgreSQL 17", Description: "Default PostgreSQL 17 from pgdg",
			DbKind: "postgres", DbVersion: "17", IsBuiltin: true,
			AptPackages: []string{"postgresql-17", "postgresql-client-17"},
			PreInstall: []string{
				`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
				"wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
				"apt-get update",
			},
		},
		{
			Name: "MySQL 8.0", Description: "Default MySQL 8.0",
			DbKind: "mysql", DbVersion: "8.0", IsBuiltin: true,
			AptPackages: []string{"mysql-server-8.0", "mysql-client"},
		},
		{
			Name: "MySQL 8.4", Description: "Default MySQL 8.4",
			DbKind: "mysql", DbVersion: "8.4", IsBuiltin: true,
			AptPackages: []string{"mysql-server-8.4", "mysql-client"},
		},
		{
			Name: "Picodata 25.3", Description: "Default Picodata 25.3",
			DbKind: "picodata", DbVersion: "25.3", IsBuiltin: true,
			AptPackages: []string{"picodata"},
			PreInstall: []string{
				`curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg`,
				`echo "deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/picodata.list`,
				"apt-get update",
			},
		},
	}
}
