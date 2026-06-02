package database

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestBuildTopologySpecDispatchesPostgres(t *testing.T) {
	spec, err := BuildTopologySpec(&domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Postgres{
					Postgres: &domain.PostgresParams{Replicas: 1},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	if got, want := len(spec.GetNodes()), 2; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
}

func TestBuildTopologySpecRejectsUnsupportedKind(t *testing.T) {
	_, err := BuildTopologySpec(&domain.Database{
		Kind: domain.Database_KIND_MARIADB,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Mariadb{
					Mariadb: &domain.MySqlParams{},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
