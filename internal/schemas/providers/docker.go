package providers

import (
	"github.com/stroppy-io/schemapb/schemapb"
)

// DockerProviderSchema is the local Docker provider settings. Machines become
// privileged stroppy-agent containers on one host; "quota" is host capacity.
//
//schemapbgen:name DockerProviderConfig
func DockerProviderSchema() *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, nameDocker, schemaVer).
		Descr("Local Docker provider settings: agent image, network and host capacity limits.").
		Fields(
			schemapb.Str(FieldImage).Default("stroppy-agent:latest").Title("Agent image"),
			schemapb.Str(FieldNetwork).Title("Docker network").
				Desc("Empty = default bridge."),
			schemapb.Bool(FieldPrivileged).Default(true).Title("Privileged").
				Desc("Required for systemd PID 1 + cgroup mounts."),
			schemapb.List(FieldDNS, schemapb.Str(FieldDNS)).
				MaxItems(8).Title("DNS servers"),

			// --- host capacity limits (cluster schema checks machines against these) ---
			schemapb.Object(FieldLimits,
				schemapb.Int32(FieldMaxContainers).Gte(0).Default(0).
					Title("Max containers").Desc("0 = unlimited."),
				schemapb.Int32(FieldMaxCores).Gte(0).Default(0).Title("Host cores"),
				schemapb.Int32(FieldMaxMemoryGB).Gte(0).Default(0).Unit("GB").Title("Host memory"),
				schemapb.Int32(FieldMaxDiskGB).Gte(0).Default(0).Unit("GB").Title("Host disk"),
			).Title("Host limits"),
		).MustBuild()
}
