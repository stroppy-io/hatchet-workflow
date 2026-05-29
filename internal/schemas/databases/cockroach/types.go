package cockroach

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the PURE CockroachDB database schema: enum values, field
// names, and the expr paths used by its (DB-internal) gates. This schema is
// provider-agnostic — it knows nothing about machines, disks, providers or
// placement. CockroachDB is a homogeneous, symmetric cluster: every node is
// identical (no master/replica roles), so topology is just shape + node count.
// Those deployment concerns live in the provider schema and the cluster (top)
// schema.
//
// Enum value types are string aliases so they pass straight into
// utils.StrEnum(...).In() / .Default(); each ships a <Name>Values slice.

// =============================================================================
// Enum value types (string) + values
// =============================================================================

// Topology is the cluster shape — the root discriminator. CockroachDB has no
// primary/replica distinction; "cluster" is just N homogeneous, gossip-joined
// nodes, with data replication governed by the zone replication factor.
type Topology = string

const (
	TopologySingle  Topology = "single"  // one node (no --join, replication factor pinned to 1)
	TopologyCluster Topology = "cluster" // N homogeneous nodes, gossip --join, RF >= 3
)

var TopologyValues = []string{TopologySingle, TopologyCluster}

// CrdbMajor is the CockroachDB major version (string so it carries the dotted
// release line, e.g. "24.2"; the exact patch is a deploy-time tarball concern).
type CrdbMajor = string

const (
	CrdbMajor232 CrdbMajor = "23.2"
	CrdbMajor241 CrdbMajor = "24.1"
	CrdbMajor242 CrdbMajor = "24.2"
	CrdbMajor243 CrdbMajor = "24.3"
)

var CrdbMajorValues = []string{CrdbMajor232, CrdbMajor241, CrdbMajor242, CrdbMajor243}

// ReplicationFactor is the default zone-config replica count. 1 = single node;
// 3 is the production minimum for quorum (tolerates 1 failure); 5 tolerates 2.
type ReplicationFactor = int32

const (
	ReplicationFactor1 ReplicationFactor = 1
	ReplicationFactor3 ReplicationFactor = 3
	ReplicationFactor5 ReplicationFactor = 5
)

var ReplicationFactorValues = []int32{ReplicationFactor1, ReplicationFactor3, ReplicationFactor5}

// SecurityMode is the cluster security posture. insecure disables all TLS/auth
// and is bench-only; secure enables node + client certificates.
type SecurityMode = string

const (
	SecurityInsecure SecurityMode = "insecure"
	SecuritySecure   SecurityMode = "secure"
)

var SecurityModeValues = []string{SecurityInsecure, SecuritySecure}

// =============================================================================
// Field names
// =============================================================================

const (
	// root / topology
	FieldTopology  = types.Field("topology")
	FieldVersion   = types.Field("version")
	FieldNodeCount = types.Field("node_count")

	// cluster config (engine knobs that map to `cockroach start` flags)
	FieldCluster           = types.Field("cluster")
	FieldReplicationFactor = types.Field("replication_factor")
	FieldCache             = types.Field("cache")
	FieldMaxSQLMemory      = types.Field("max_sql_memory")
	FieldSecurity          = types.Field("security")
	FieldMaxOffset         = types.Field("max_offset")
	FieldGossipJoin        = types.Field("gossip_join")

	// cluster settings escape hatch (-> SET CLUSTER SETTING k = v)
	FieldClusterSettings = types.Field("cluster_settings")
	FieldSetting         = types.Field("setting")
	FieldKey             = types.Field("key")
	FieldValue           = types.Field("value")
)
