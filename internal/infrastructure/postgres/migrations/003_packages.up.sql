CREATE TABLE packages (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    db_kind      TEXT NOT NULL,
    db_version   TEXT NOT NULL DEFAULT '',
    is_builtin   BOOLEAN NOT NULL DEFAULT FALSE,

    apt_packages TEXT[] NOT NULL DEFAULT '{}',
    pre_install  TEXT[] NOT NULL DEFAULT '{}',
    custom_repo  TEXT NOT NULL DEFAULT '',
    custom_repo_key TEXT NOT NULL DEFAULT '',

    deb_data     BYTEA,
    deb_filename TEXT NOT NULL DEFAULT '',

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_packages_tenant ON packages(tenant_id);
CREATE INDEX idx_packages_kind ON packages(tenant_id, db_kind);

-- Seed built-in packages for all existing tenants.

-- PostgreSQL 16
INSERT INTO packages (id, tenant_id, name, description, db_kind, db_version, is_builtin, apt_packages, pre_install)
SELECT
    'builtin-pg16-' || t.id, t.id, 'PostgreSQL 16', 'Default PostgreSQL 16 from pgdg', 'postgres', '16', TRUE,
    ARRAY['postgresql-16', 'postgresql-client-16'],
    ARRAY[
        'sh -c ''echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list''',
        'wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -',
        'apt-get update'
    ]
FROM tenants t;

-- PostgreSQL 17
INSERT INTO packages (id, tenant_id, name, description, db_kind, db_version, is_builtin, apt_packages, pre_install)
SELECT
    'builtin-pg17-' || t.id, t.id, 'PostgreSQL 17', 'Default PostgreSQL 17 from pgdg', 'postgres', '17', TRUE,
    ARRAY['postgresql-17', 'postgresql-client-17'],
    ARRAY[
        'sh -c ''echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list''',
        'wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -',
        'apt-get update'
    ]
FROM tenants t;

-- MySQL 8.0
INSERT INTO packages (id, tenant_id, name, description, db_kind, db_version, is_builtin, apt_packages)
SELECT 'builtin-my80-' || t.id, t.id, 'MySQL 8.0', 'Default MySQL 8.0', 'mysql', '8.0', TRUE,
    ARRAY['mysql-server-8.0', 'mysql-client']
FROM tenants t;

-- MySQL 8.4
INSERT INTO packages (id, tenant_id, name, description, db_kind, db_version, is_builtin, apt_packages)
SELECT 'builtin-my84-' || t.id, t.id, 'MySQL 8.4', 'Default MySQL 8.4', 'mysql', '8.4', TRUE,
    ARRAY['mysql-server-8.4', 'mysql-client']
FROM tenants t;

-- Picodata 25.3
INSERT INTO packages (id, tenant_id, name, description, db_kind, db_version, is_builtin, apt_packages, pre_install)
SELECT
    'builtin-pico253-' || t.id, t.id, 'Picodata 25.3', 'Default Picodata 25.3', 'picodata', '25.3', TRUE,
    ARRAY['picodata'],
    ARRAY[
        'curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg',
        'echo "deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/picodata.list',
        'apt-get update'
    ]
FROM tenants t;
