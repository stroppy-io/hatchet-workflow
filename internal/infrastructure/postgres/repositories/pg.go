package repositories

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isUnique reports a unique-violation.
func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func ptrInt64(v int64) *int64 { return &v }
