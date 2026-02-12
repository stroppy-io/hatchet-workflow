package postgres

import "github.com/stroppy-io/hatchet-workflow/internal/proto/database"

func RunPostgresInstance(instance *database.Postgres_Instance) error {
	installer, err := PostgresInstaller(instance, nil)
	if err != nil {
		return err
	}
	return installer.Start()
}
