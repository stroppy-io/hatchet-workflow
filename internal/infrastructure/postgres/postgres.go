package postgres

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/gopherex/pgtx"
	otelpgxext "github.com/gopherex/pgtx/contrib/otel"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	pgxlog "github.com/gopherex/xlog/contrib/libs/pgx"
	"github.com/gopherex/xprobe"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host     string `mapstructure:"host" default:"localhost" validate:"required,hostname|ip"`
	Port     int    `mapstructure:"port" default:"5432" validate:"required,min=1,max=65535"`
	Username string `mapstructure:"username" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	Database string `mapstructure:"database" validate:"required"`
}

func (c *Config) String() string {
	return fmt.Sprintf("postgres://%s:*****@%s:%d/%s",
		c.Username, c.Host, c.Port, c.Database,
	)
}

func (c *Config) Parse() (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(c.String())
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func NewPostgresDb(cf *Config, logger *xlog.Logger) (*pgxpool.Pool, tx.Trm, tx.DB, xprobe.Probe, error) {
	cfg, err := cf.Parse()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	tr, err := pgxlog.NewTracer(logger)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	cfg.ConnConfig.Tracer = multitracer.New(
		otelpgx.NewTracer(
			otelpgx.WithTrimSQLInSpanName(),
			otelpgx.WithIncludeQueryParameters(),
		), tr,
	)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	err = otelpgxext.RegisterMetrics(pool)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	trm, err := pgtx.NewTxManager(pool, tx.ReadCommitted())
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return pool, trm, pgtx.NewTxDB(pool), xprobe.FromError(func(ctx context.Context) error {
		logger.Trace("Pinging Postgres server")
		return pool.Ping(ctx)
	}), nil
}
