package app

import (
	"github.com/gopherex/xlog"
	zapsink "github.com/gopherex/xlog/contrib/loggers/zap"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/ratel/pkg/migrate"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
)

// applyMigrations applies the embedded atlas migrations through ratel's migrator,
// which tracks applied revisions (atlas_schema_revisions) and is a no-op when the
// database is already up to date — so it is safe to run on every server boot.
func applyMigrations(pool *pgxpool.Pool, logger *xlog.Logger) error {
	zl := zap.New(zapsink.NewSink(logger.AppendName("migrate")))
	return migrate.Migrate(pool, zl, "public", migrations.Content)
}
