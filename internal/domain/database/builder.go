package database

import (
	"fmt"

	postgresdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func BuildTopologySpec(input *domain.Database) (*topology.TopologySpec, error) {
	if input == nil {
		return nil, fmt.Errorf("database is required")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}

	params := input.GetParams()
	if params == nil {
		return nil, fmt.Errorf("database kind %s has no self-deploy params", input.GetKind())
	}

	switch input.GetKind() {
	case domain.Database_KIND_POSTGRES:
		if params.GetPostgres() == nil {
			return nil, fmt.Errorf("postgres database requires postgres params")
		}
		return (&postgresdb.Database{}).BuildTopologySpec(params.GetPostgres())
	default:
		return nil, fmt.Errorf("database kind %s topology builder is not implemented", input.GetKind())
	}
}
