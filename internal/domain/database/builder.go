package database

import (
	"fmt"

	cockroachdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/cockroach"
	mysqldb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/mysql"
	orioledbdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/orioledb"
	pgnoopdb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/pgnoop"
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
	case domain.Database_KIND_MARIADB:
		if params.GetMariadb() == nil {
			return nil, fmt.Errorf("mariadb database requires mariadb params")
		}
		return (&mysqldb.Database{}).BuildTopologySpec(params.GetMariadb())
	case domain.Database_KIND_ORIOLEDB:
		if params.GetOrioledb() == nil {
			return nil, fmt.Errorf("orioledb database requires orioledb params")
		}
		return (&orioledbdb.Database{}).BuildTopologySpec(params.GetOrioledb())
	case domain.Database_KIND_PG_NOOP:
		if params.GetPgNoop() == nil {
			return nil, fmt.Errorf("pg_noop database requires pg_noop params")
		}
		return (&pgnoopdb.Database{}).BuildTopologySpec(params.GetPgNoop())
	case domain.Database_KIND_NOOP:
		// The no-DB machine benchmark deploys nothing: stroppy runs on the runner
		// node alone with its internal noop driver. Return an empty spec; the
		// workload builder adds the runner-only topology (PROTOCOL_NOOP).
		return &topology.TopologySpec{}, nil
	default:
		return nil, fmt.Errorf("database kind %s topology builder is not implemented", input.GetKind())
	}
}
