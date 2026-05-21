package types

import "fmt"

// Protocol identifies the wire format stroppy uses to talk to a database.
// Decoupled from DatabaseKind because some engines (YDB, Picodata) speak
// multiple protocols, each with its own driver and SQL feature ceiling.
type Protocol string

const (
	// ProtocolPG is the standard PostgreSQL wire protocol — used by postgres
	// itself and by anything that speaks pg-wire as its primary surface
	// (CockroachDB, YugabyteDB-YSQL).
	ProtocolPG Protocol = "pg"
	// ProtocolMySQL is the MySQL wire protocol — used by mysql, mariadb,
	// percona, vitess (vtgate), tidb.
	ProtocolMySQL Protocol = "mysql"
	// ProtocolPicodata is pg-wire from Picodata routed through stroppy's
	// picodata driver, which knows about Picodata's topology and shard
	// placement. Plain pg-wire works too but loses the routing smarts.
	ProtocolPicodata Protocol = "picodata"
	// ProtocolYDBGRPC is YDB's native gRPC protocol — full feature set,
	// stroppy uses its ydb driver.
	ProtocolYDBGRPC Protocol = "ydb-grpc"
	// ProtocolYDBGRPCS is YDB's native gRPC protocol over TLS, the only
	// surface Yandex Cloud Managed YDB exposes (port 2135). The patched
	// stroppy driver (pkg/driver/ydb/driver.go) falls back to YC metadata
	// for SA token + internal CA, so as long as the client VM has an
	// attached service account with ydb.editor the run authenticates
	// without explicit credentials.
	ProtocolYDBGRPCS Protocol = "ydb-grpcs"
	// ProtocolYDBPgwire is YDB's experimental pg-wire surface. Strict subset
	// of pg-wire — no stored procedures, limited DDL, weaker txn semantics.
	// Stroppy uses its postgres driver against it; benchmark scripts have
	// to be authored to fit the subset (see ScriptCompat).
	ProtocolYDBPgwire Protocol = "ydb-pgwire"
	// ProtocolCockroach is CockroachDB's pg-wire surface on port 26257
	// (NB: not 5432). Stroppy talks to it via the postgres driver — TPC-C
	// works, stored procedures don't (CRDB has limited PL/pgSQL support
	// since v23 but the standard tpcc/procs script doesn't fit).
	ProtocolCockroach Protocol = "cockroach"
)

// ProtocolMeta describes how to connect over a protocol — driver type for
// stroppy's protojson, default port, URL scheme, and any tail the URL needs
// (database path, sslmode, etc.).
type ProtocolMeta struct {
	DriverType string // value for stroppypb.DriverRunConfig.DriverType
	Port       int    // default tcp port; the run can override per-cluster
	URLScheme  string // "postgresql", "mysql", "grpc", "postgres" — stroppy expects scheme-specific URLs
	URLTail    string // appended to "<scheme>://<host>:<port>" — db name + query params
}

// FormatURL builds the connection string stroppy puts in
// DriverRunConfig.Url. Keeps the URL formatting in one place so the agent
// renderer and dry-run preview can't drift.
func (p ProtocolMeta) FormatURL(host, port string) string {
	if p.DriverType == "mysql" {
		// MySQL DSN is "user@tcp(host:port)/" — different shape from URL.
		return fmt.Sprintf("root@tcp(%s:%s)%s", host, port, p.URLTail)
	}
	return fmt.Sprintf("%s://%s:%s%s", p.URLScheme, host, port, p.URLTail)
}

// Protocols is the registry of every protocol the cloud knows how to
// configure. Adding a new entry here is the first step when teaching the
// system about a new engine that speaks an unfamiliar wire format.
var Protocols = map[Protocol]ProtocolMeta{
	ProtocolPG:       {DriverType: "postgres", Port: 5432, URLScheme: "postgresql", URLTail: "/postgres?sslmode=disable"},
	ProtocolMySQL:    {DriverType: "mysql", Port: 3306, URLTail: "/"},
	ProtocolPicodata: {DriverType: "picodata", Port: 5432, URLScheme: "postgres", URLTail: "?sslmode=disable"},
	ProtocolYDBGRPC:  {DriverType: "ydb", Port: 2136, URLScheme: "grpc", URLTail: "/Root/testdb"},
	// Managed YDB endpoints terminate TLS and require the database path as a
	// query parameter. URLTail is left blank because the path is dynamic
	// (only known after terraform apply); task_stroppy splices it in via
	// dbDriverURL.
	ProtocolYDBGRPCS:  {DriverType: "ydb", Port: 2135, URLScheme: "grpcs", URLTail: ""},
	ProtocolYDBPgwire: {DriverType: "postgres", Port: 5432, URLScheme: "postgresql", URLTail: "/local?sslmode=disable"},
	ProtocolCockroach: {DriverType: "postgres", Port: 26257, URLScheme: "postgresql", URLTail: "/defaultdb?sslmode=disable"},
}

// KindProtocols lists the protocols each engine supports, in preference
// order. The first entry is the default when StroppyConfig.Protocol is
// unset — that preserves existing-run behaviour after this lands.
var KindProtocols = map[DatabaseKind][]Protocol{
	DatabasePostgres:   {ProtocolPG},
	DatabaseMySQL:      {ProtocolMySQL},
	DatabaseMariaDB:    {ProtocolMySQL},
	DatabasePicodata:   {ProtocolPicodata},
	DatabaseYDB:        {ProtocolYDBGRPC, ProtocolYDBPgwire},
	DatabaseYDBManaged: {ProtocolYDBGRPCS},
	DatabaseCockroach:  {ProtocolCockroach},
}

// DefaultProtocol returns the first protocol for a kind, or "" if the kind
// isn't registered.
func DefaultProtocol(kind DatabaseKind) Protocol {
	if ps, ok := KindProtocols[kind]; ok && len(ps) > 0 {
		return ps[0]
	}
	return ""
}

// KindSupportsProtocol returns true if the engine speaks this protocol.
func KindSupportsProtocol(kind DatabaseKind, p Protocol) bool {
	for _, x := range KindProtocols[kind] {
		if x == p {
			return true
		}
	}
	return false
}

// KindProtocolKey is the composite key for ScriptCompat. Replaces the old
// scriptDBSupport map that keyed only on DatabaseKind — wrong assumption
// because the same protocol on different engines often has subtly
// different SQL feature ceilings, and engine-specific script variants are
// the rule, not the exception (e.g. tpcc/tx-ydb-pgwire trims features the
// YDB pgwire surface doesn't yet support).
type KindProtocolKey struct {
	Kind     DatabaseKind
	Protocol Protocol
}

// ScriptCompat is the matrix of which stroppy scripts run against which
// (kind, protocol) combination. Script IDs are the stroppy paths verbatim
// — universal where they happen to be, engine-specific variants where
// portability breaks down. The wizard's script picker is a function of
// (kind, protocol); validation rejects anything not in the matrix.
var ScriptCompat = map[KindProtocolKey][]string{
	{DatabasePostgres, ProtocolPG}:         {"tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx"},
	{DatabaseMySQL, ProtocolMySQL}:         {"tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx"},
	{DatabaseMariaDB, ProtocolMySQL}:       {"tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx"},
	{DatabasePicodata, ProtocolPicodata}:   {"tpcc/tx", "tpcb/tx", "tpch/tx"},
	{DatabaseYDB, ProtocolYDBGRPC}:         {"tpcc/tx", "tpcb/tx", "tpch/tx"},
	{DatabaseYDB, ProtocolYDBPgwire}:       {"tpcc/tx-ydb-pgwire", "tpcb/tx-ydb-pgwire"},
	{DatabaseYDBManaged, ProtocolYDBGRPCS}: {"tpcc/tx", "tpcb/tx", "tpch/tx"},
	{DatabaseCockroach, ProtocolCockroach}: {"tpcc/tx", "tpcb/tx", "tpch/tx"},
}

// ScriptSupported returns true if (kind, protocol) is registered in
// ScriptCompat and `script` is in its list.
func ScriptSupported(kind DatabaseKind, p Protocol, script string) bool {
	for _, s := range ScriptCompat[KindProtocolKey{kind, p}] {
		if s == script {
			return true
		}
	}
	return false
}
