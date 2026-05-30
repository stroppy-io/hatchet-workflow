package types

type SchemaNs = string

const (
	DatabaseNamespace = SchemaNs("database")
	ProviderNamespace = SchemaNs("provider")
	ClusterNamespace  = SchemaNs("cluster")
	WorkloadNamespace = SchemaNs("workload")
)

type SchemaName = string

type Field = string

//const (
//	Postgres   = SchemaName("postgres")
//	Mysql      = SchemaName("mysql")
//	Mariadb    = SchemaName("mariadb")
//	Picodata   = SchemaName("picodata")
//	Ydb        = SchemaName("ydb")
//	YdbManaged = SchemaName("ydb-managed")
//)
