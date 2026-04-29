package types

import "testing"

func TestDefaultProtocol(t *testing.T) {
	cases := []struct {
		kind DatabaseKind
		want Protocol
	}{
		{DatabasePostgres, ProtocolPG},
		{DatabaseMySQL, ProtocolMySQL},
		{DatabaseMariaDB, ProtocolMySQL},
		{DatabasePicodata, ProtocolPicodata},
		{DatabaseYDB, ProtocolYDBGRPC}, // first entry wins → preserves existing-run behaviour
	}
	for _, c := range cases {
		if got := DefaultProtocol(c.kind); got != c.want {
			t.Errorf("DefaultProtocol(%q) = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestKindSupportsProtocol(t *testing.T) {
	if !KindSupportsProtocol(DatabaseYDB, ProtocolYDBGRPC) {
		t.Error("YDB should support ydb-grpc")
	}
	if !KindSupportsProtocol(DatabaseYDB, ProtocolYDBPgwire) {
		t.Error("YDB should support ydb-pgwire")
	}
	if KindSupportsProtocol(DatabasePostgres, ProtocolMySQL) {
		t.Error("Postgres should not support mysql protocol")
	}
}

func TestScriptSupported(t *testing.T) {
	if !ScriptSupported(DatabaseYDB, ProtocolYDBGRPC, "tpcc/tx") {
		t.Error("ydb-grpc should support tpcc/tx")
	}
	if ScriptSupported(DatabaseYDB, ProtocolYDBGRPC, "tpcc/tx-ydb-pgwire") {
		t.Error("ydb-grpc should NOT support the pgwire-only variant")
	}
	if !ScriptSupported(DatabaseYDB, ProtocolYDBPgwire, "tpcc/tx-ydb-pgwire") {
		t.Error("ydb-pgwire should support its own tpcc/tx-ydb-pgwire variant")
	}
	if ScriptSupported(DatabaseYDB, ProtocolYDBPgwire, "tpcc/procs") {
		t.Error("ydb-pgwire should not support stored-procedure variant")
	}
	if !ScriptSupported(DatabaseMariaDB, ProtocolMySQL, "tpcc/procs") {
		t.Error("mariadb on mysql protocol should support tpcc/procs")
	}
}

func TestProtocolMeta_FormatURL(t *testing.T) {
	cases := []struct {
		proto    Protocol
		host     string
		port     string
		want     string
	}{
		{ProtocolPG, "10.0.0.1", "5432", "postgresql://10.0.0.1:5432/postgres?sslmode=disable"},
		{ProtocolMySQL, "10.0.0.1", "3306", "root@tcp(10.0.0.1:3306)/"},
		{ProtocolYDBGRPC, "10.0.0.1", "2136", "grpc://10.0.0.1:2136/Root/testdb"},
		{ProtocolYDBPgwire, "10.0.0.1", "5432", "postgresql://10.0.0.1:5432/local?sslmode=disable"},
		{ProtocolPicodata, "10.0.0.1", "5432", "postgres://10.0.0.1:5432?sslmode=disable"},
	}
	for _, c := range cases {
		meta := Protocols[c.proto]
		if got := meta.FormatURL(c.host, c.port); got != c.want {
			t.Errorf("%s FormatURL(%s, %s) = %q, want %q", c.proto, c.host, c.port, got, c.want)
		}
	}
}
