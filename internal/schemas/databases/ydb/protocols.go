package ydb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// protocolsSection is the protocol/endpoint surface (pure DB, logical ports).
// These are the LOGICAL port choices written into the ydbd command line /
// config.yaml grpc_config; the actual LB endpoints, firewall rules and host
// addresses are wired at the cluster layer.
func protocolsSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldProtocols,
		schemapb.Int32(FieldGRPCPort).Gte(1).Lte(65535).Default(2136).
			Title("gRPC port").
			Desc("Native YDB gRPC port (ydb-grpc, `grpc://host:port/database`)."),

		schemapb.Bool(FieldEnablePgI).Default(false).
			Title("Enable PostgreSQL wire protocol").
			Desc("Adds --pgwire-port to ydbd, exposing the Postgres-compatible endpoint."),

		schemapb.Int32(FieldPgWirePort).Gte(1).Lte(65535).Default(5432).
			When(utils.IsTrue(rp(pfx, FieldProtocols, FieldEnablePgI))).
			Title("PgWire port").
			Desc("Postgres wire-protocol port (ydb-pgwire). Only when pgwire is enabled."),

		schemapb.Int32(FieldInterconStg).Gte(1).Lte(65535).Default(19001).
			Title("Interconnect port (storage)").
			Desc("ydbd-storage --ic-port (inter-node actor traffic)."),

		schemapb.Int32(FieldInterconDb).Gte(1).Lte(65535).Default(19002).
			When(utils.Eq(rp(pfx, FieldTopology), TopologySplit)).
			Title("Interconnect port (database)").
			Desc("ydbd-database --ic-port. Only meaningful for split topology."),

		schemapb.Int32(FieldMonPortStg).Gte(1).Lte(65535).Default(8765).
			Title("Monitoring port (storage)").
			Desc("ydbd-storage --mon-port (HTTP viewer + /metrics)."),

		schemapb.Int32(FieldMonPortDb).Gte(1).Lte(65535).Default(8766).
			When(utils.Eq(rp(pfx, FieldTopology), TopologySplit)).
			Title("Monitoring port (database)").
			Desc("ydbd-database --mon-port. Only meaningful for split topology."),
	).Title("Protocols & ports")
}
