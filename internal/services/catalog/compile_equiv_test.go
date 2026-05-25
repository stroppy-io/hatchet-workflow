package catalog

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// sizingByPreset is the per-preset hardware overlay that, combined with each preset's
// Database intent, must reproduce the exact hand-authored topology. It is the bridge
// proving CompileTopology is behavior-identical to the topologies the matrix validated.
func sizingByPreset() map[string]*dag.Sizing {
	f := func(c uint32, m, d uint64) dag.Flavor { return dag.Flavor{Cores: c, MemGb: m, DiskGb: d} }
	proxy := f(2, 4, 50)
	coord := f(2, 4, 50)
	storage := dag.Flavor{Cores: 16, MemGb: 32, DiskGb: 50, DataDisksGb: []uint64{930, 930, 930}}
	return map[string]*dag.Sizing{
		"PostgreSQL Single":   {Database: f(4, 16, 100)},
		"PostgreSQL HA":       {Database: f(4, 16, 200), Coordinator: coord, Proxy: proxy, Proxies: 1},
		"PostgreSQL Scale":    {Database: f(8, 16, 200), Coordinator: coord, Proxy: proxy, Proxies: 2},
		"MySQL Single":        {Database: f(4, 16, 100)},
		"MySQL Replica":       {Database: f(4, 8, 100), Proxy: proxy, Proxies: 1},
		"MySQL Group":         {Database: f(8, 16, 200), Proxy: proxy, Proxies: 2},
		"MariaDB Single":      {Database: f(4, 16, 100)},
		"MariaDB Replica":     {Database: f(4, 8, 100), Proxy: proxy, Proxies: 1},
		"MariaDB Group":       {Database: f(8, 16, 200), Proxy: proxy, Proxies: 2},
		"Picodata Single":     {Database: f(4, 8, 100), Instances: 1},
		"Picodata Cluster":    {Database: f(4, 8, 100), Proxy: proxy, Proxies: 1, Instances: 3},
		"Picodata Scale":      {Database: f(8, 16, 200), Proxy: proxy, Proxies: 2, Instances: 6},
		"YDB Single":          {Database: f(8, 32, 200)},
		"YDB mirror3dc-3x32":  {Storage: storage, Compute: f(32, 64, 50)},
		"YDB mirror3dc-3x64":  {Storage: storage, Compute: f(64, 128, 50)},
		"YDB mirror3dc-9x32":  {Storage: storage, Compute: f(32, 64, 50)},
		"CockroachDB Single":  {Database: f(4, 16, 100)},
		"CockroachDB Cluster": {Database: f(4, 16, 100)},
		"CockroachDB Scale":   {Database: f(8, 16, 200)},
	}
}

// databaseNodesByPreset patches the YDB compute-tier count onto Database.Options for
// the equivalence check: the current hand-authored presets set DatabaseNodes=0, but
// the field exists and CompileTopology reads it. (The conversion sets it for real.)
func databaseNodesByPreset() map[string]uint32 {
	return map[string]uint32{
		"YDB mirror3dc-3x32": 3,
		"YDB mirror3dc-3x64": 3,
		"YDB mirror3dc-9x32": 9,
	}
}

// TestCompileTopology_MatchesHandAuthored proves CompileTopology(Database, Sizing) is
// byte-identical (proto.Equal) to every preset's current hand-authored Topology — the
// no-behavior-change guarantee for the IR refactor.
func TestCompileTopology_MatchesHandAuthored(t *testing.T) {
	sizing := sizingByPreset()
	dbNodes := databaseNodesByPreset()
	for _, sp := range SystemDatabasePresets() {
		sp := sp
		t.Run(sp.Name, func(t *testing.T) {
			size, ok := sizing[sp.Name]
			if !ok {
				t.Fatalf("no sizing defined for preset %q", sp.Name)
			}
			db := proto.Clone(sp.DB.GetDatabase()).(*domain.Database)
			if n, ok := dbNodes[sp.Name]; ok {
				db.GetOptions().GetYdb().GetSelfHosted().DatabaseNodes = n
			}
			got, err := dag.CompileTopology(db, size)
			if err != nil {
				t.Fatalf("CompileTopology: %v", err)
			}
			want := sp.DB.GetTopology()
			if !proto.Equal(got, want) {
				t.Errorf("preset %q: compiled topology != hand-authored\n got=%v\nwant=%v", sp.Name, got, want)
			}
		})
	}
}
