package tsgen

import (
	"strings"
	"testing"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

func TestGenerate(t *testing.T) {
	s := schemapb.NewSchema(ids.Cfg("demo.conf", 16)).
		Descr("demo").
		Strict().Coerce().
		Def("endpoint", schemapb.Str("host").Required(), schemapb.Int64("port").Default(5432)).
		Fields(
			schemapb.Int64("shared_buffers").Unit("MB").Title("Shared buffers").Required(),
			schemapb.Bool("autovacuum").Default(true),
			schemapb.Choice("wal_level").Opt(schemapb.StrV("minimal"), "Minimal").Opt(schemapb.StrV("replica"), "Replica"),
			schemapb.Duration("checkpoint_timeout").Default(5*time.Minute),
			schemapb.List("names", schemapb.Str("")),
			schemapb.Object("logging", schemapb.Bool("collector")),
			schemapb.Map("tablespaces", schemapb.Str("location").Required()),
			schemapb.OneOf("backup", "type").Variant("s3", schemapb.Str("bucket").Required()).Variant("none"),
			schemapb.Ref("primary", "endpoint"),
			schemapb.Computed("effective", "root.shared_buffers * 3").Result(schemapb.ResultInt64),
			schemapb.Str("secret").Secret().Nullable(),
			schemapb.JSON("extra"),
		).
		MustBuild()
	out := Generate(s)
	for _, want := range []string{
		"export interface CfgDemoConf16 {",
		"shared_buffers: number | string;",
		"autovacuum?: boolean;",
		`wal_level?: "minimal" | "replica";`,
		"checkpoint_timeout?: string;",
		"names?: Array<string>;",
		"logging?: CfgDemoConf16Logging;",
		"tablespaces?: Record<string, CfgDemoConf16TablespacesValue>;",
		"backup?: CfgDemoConf16BackupNone | CfgDemoConf16BackupS3;",
		`type: "s3";`,
		"primary?: CfgDemoConf16Endpoint;",
		"readonly effective?: number | string;",
		"secret?: string | null;",
		"extra?: unknown;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if TypeName(s.GetId()) != "CfgDemoConf16" {
		t.Errorf("TypeName = %q", TypeName(s.GetId()))
	}
}
