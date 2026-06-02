package database

import (
	"fmt"

	cockroachdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/cockroach"
	mysqldb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/mysql"
	picodatadb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/picodata"
	postgresdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	ydbdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydb"
	ydbmanageddb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydbmanaged"
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
	case domain.Database_KIND_PICODATA:
		if params.GetPicodata() == nil {
			return nil, fmt.Errorf("picodata database requires picodata params")
		}
		return (&picodatadb.Database{}).BuildTopologySpec(params.GetPicodata())
	case domain.Database_KIND_YDB:
		if params.GetYdb() == nil {
			return nil, fmt.Errorf("ydb database requires ydb params")
		}
		return (&ydbdb.Database{}).BuildTopologySpec(params.GetYdb())
	case domain.Database_KIND_YDB_MANAGED:
		if params.GetYdbManaged() == nil {
			return nil, fmt.Errorf("ydb managed database requires ydb_managed params")
		}
		return (&ydbmanageddb.Database{}).BuildTopologySpec(params.GetYdbManaged())
	case domain.Database_KIND_COCKROACH:
		if params.GetCockroach() == nil {
			return nil, fmt.Errorf("cockroach database requires cockroach params")
		}
		return (&cockroachdb.Database{}).BuildTopologySpec(params.GetCockroach())
	case domain.Database_KIND_MYSQL:
		if params.GetMysql() == nil {
			return nil, fmt.Errorf("mysql database requires mysql params")
		}
		return (&mysqldb.Database{}).BuildTopologySpec(params.GetMysql())
	default:
		return nil, fmt.Errorf("database kind %s topology builder is not implemented", input.GetKind())
	}
}
