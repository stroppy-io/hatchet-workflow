package recipe

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

func cmdText(c *topology.Component_Strategy) string {
	var b strings.Builder
	for _, cmd := range c.GetDeploymentCommands() {
		b.WriteString(cmd.GetSpec().GetScript().GetText())
		b.WriteString("\n")
	}
	return b.String()
}

func TestBuildDatabase(t *testing.T) {
	refs := Refs{StroppyBinaryURL: "http://minio.local/stroppy", StroppyChecksum: "deadbeef"}
	s := Build("postgres", string(expand.RoleDatabase),
		map[string]any{postgres.FieldPgMajor: float64(16)}, nil, refs)
	if s == nil {
		t.Fatal("nil strategy for database role")
	}
	if len(s.GetConfigurationFiles()) < 1 {
		t.Fatalf("want >=1 config file, got %d", len(s.GetConfigurationFiles()))
	}
	if len(s.GetDeploymentCommands()) < 1 {
		t.Fatalf("want >=1 command, got %d", len(s.GetDeploymentCommands()))
	}

	// At least one config file must carry text content.
	foundText := false
	for _, f := range s.GetConfigurationFiles() {
		if f.GetFile().GetText() != "" {
			foundText = true
		}
	}
	if !foundText {
		t.Fatal("no config file with text content")
	}

	cmds := cmdText(s)
	if !strings.Contains(cmds, "apt-get install") {
		t.Errorf("commands missing apt-get install:\n%s", cmds)
	}
	if !strings.Contains(cmds, "postgresql-16") {
		t.Errorf("commands missing postgresql-16:\n%s", cmds)
	}
}

func TestBuildDatabaseTuning(t *testing.T) {
	s := Build("postgres", string(expand.RoleDatabase),
		map[string]any{
			postgres.FieldPgMajor: float64(16),
			postgres.FieldTuning: map[string]any{
				postgres.FieldSharedBuffers: "256MB",
			},
		}, nil, Refs{})
	conf := s.GetConfigurationFiles()[0].GetFile().GetText()
	if !strings.Contains(conf, "shared_buffers = '256MB'") {
		t.Errorf("postgresql.conf missing tuning value:\n%s", conf)
	}
	if !strings.Contains(conf, "__LISTEN__") {
		t.Errorf("postgresql.conf missing __LISTEN__ placeholder:\n%s", conf)
	}
}

func TestBuildDatabaseDefaultMajor(t *testing.T) {
	s := Build("postgres", string(expand.RoleDatabase), map[string]any{}, nil, Refs{})
	if !strings.Contains(cmdText(s), "postgresql-16") {
		t.Error("default pg_major should be 16")
	}
}

func TestBuildWorkload(t *testing.T) {
	refs := Refs{StroppyBinaryURL: "http://minio.local/bin/stroppy"}
	s := Build("postgres", string(expand.RoleWorkload), nil, nil, refs)
	if s == nil {
		t.Fatal("nil strategy for workload role")
	}
	if len(s.GetDeploymentCommands()) < 1 {
		t.Fatalf("want >=1 command, got %d", len(s.GetDeploymentCommands()))
	}
	cmds := cmdText(s)
	if !strings.Contains(cmds, refs.StroppyBinaryURL) {
		t.Errorf("commands missing stroppy URL:\n%s", cmds)
	}
	if !strings.Contains(cmds, PlaceholderDBHost) {
		t.Errorf("commands missing %s placeholder:\n%s", PlaceholderDBHost, cmds)
	}
	if !strings.Contains(cmds, PlaceholderDBPort) {
		t.Errorf("commands missing %s placeholder:\n%s", PlaceholderDBPort, cmds)
	}
}

func TestBuildUnknownKind(t *testing.T) {
	if Build("nodb", string(expand.RoleDatabase), nil, nil, Refs{}) != nil {
		t.Error("unknown kind should return nil")
	}
}

func TestWire(t *testing.T) {
	conns := Wire("postgres", map[string][]string{
		string(expand.RoleDatabase): {"pg-primary"},
		string(expand.RoleWorkload): {"stroppy-1"},
	})
	if len(conns) != 1 {
		t.Fatalf("want 1 connection, got %d", len(conns))
	}
	c := conns[0]
	if c.GetFrom() != "stroppy-1" || c.GetTo() != "pg-primary" {
		t.Errorf("want workload->database, got %s->%s", c.GetFrom(), c.GetTo())
	}
	if c.GetPort() != pgListenPort {
		t.Errorf("want port %d, got %d", pgListenPort, c.GetPort())
	}
	if c.GetKind() != topology.Connection_KIND_FLOW {
		t.Errorf("want KIND_FLOW, got %v", c.GetKind())
	}
	if c.GetProtocol() != topology.Connection_PROTOCOL_TCP {
		t.Errorf("want PROTOCOL_TCP, got %v", c.GetProtocol())
	}
}

func TestWireMissingRole(t *testing.T) {
	if Wire("postgres", map[string][]string{
		string(expand.RoleDatabase): {"pg-primary"},
	}) != nil {
		t.Error("wire without workload should return nil")
	}
}
