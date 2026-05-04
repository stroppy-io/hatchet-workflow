package run

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

func TestBuildStroppyConfigJSON_SQLAndEnvOverrides(t *testing.T) {
	b, err := BuildStroppyConfigJSON(types.StroppyConfig{
		Script:      "tpch/tx",
		SQL:         "uploaded.sql",
		Duration:    "10m",
		VUs:         20,
		PoolSize:    100,
		ScaleFactor: 1,
		Env: map[string]string{
			"POOL_SIZE":   "250",
			"custom_flag": "enabled",
		},
	}, types.DatabasePostgres, "", 0, types.DefaultStroppySettings(), "run-test", types.DatabaseConfig{Kind: types.DatabasePostgres})
	if err != nil {
		t.Fatalf("BuildStroppyConfigJSON() error = %v", err)
	}

	var rc stroppypb.RunConfig
	if err := protojson.Unmarshal(b, &rc); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}
	if got := rc.GetSql(); got != "uploaded.sql" {
		t.Fatalf("sql = %q, want uploaded.sql", got)
	}
	if got := rc.Env["POOL_SIZE"]; got != "250" {
		t.Fatalf("POOL_SIZE env = %q, want 250", got)
	}
	if got := rc.Env["CUSTOM_FLAG"]; got != "enabled" {
		t.Fatalf("CUSTOM_FLAG env = %q, want enabled", got)
	}
}
