package migrations

import "embed"

// FS contains the SQL migrations for the Postgres adapter.
//
//go:embed *.sql
var FS embed.FS
