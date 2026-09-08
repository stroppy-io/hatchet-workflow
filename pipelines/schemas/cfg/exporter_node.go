package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// ExporterNode is cfg.exporter.node@1 — the node_exporter process that gives
// every machine of a run its host metrics (the Scrape entries of RunSpec point
// at it). Rendered as the exporter's command-line flags.
//
// doc: https://github.com/prometheus/node_exporter (1.8/1.9)
// doc: https://github.com/prometheus/node_exporter/blob/master/docs/collectors.md
//
// Flag shape: every collector is a kingpin bool pair --collector.<name> /
// --no-collector.<name>; --collector.disable-defaults turns the whole default
// set off so only the explicitly enabled ones run.
func ExporterNode() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("exporter.node", 1)).
		Descr("node_exporter 1.8/1.9 flags: which collectors run and where it listens.").
		Strict().Coerce().
		Fields(
			// doc: --web.listen-address, default ":9100"
			schemapb.Int64("listen_port").Title("Listen port").Group("Server").
				Desc("--web.listen-address port. 9100 is the registered node_exporter port.").
				Gte(1).Lte(65535).Default(9100),
			schemapb.Str("listen_address").Title("Listen address").Group("Server").
				Desc("Interface part of --web.listen-address; empty means all interfaces.").
				MaxLen(64).Default(""),

			// doc: --collector.disable-defaults
			schemapb.Bool("disable_defaults").Title("Disable default collectors").Group("Collectors").
				Desc("--collector.disable-defaults: start from nothing, only `enable` runs. Off keeps the upstream default set and applies enable/disable on top.").
				Default(false),
			// doc: collectors.md — default-on: cpu meminfo diskstats filesystem
			// netdev loadavg stat time uname vmstat netstat textfile;
			// default-off: pressure systemd processes.
			schemapb.List("enable", schemapb.Str("").
				In("arp", "conntrack", "cpu", "cpufreq", "diskstats", "edac", "entropy", "filefd",
					"filesystem", "hwmon", "infiniband", "ipvs", "loadavg", "mdadm", "meminfo",
					"netclass", "netdev", "netstat", "nfs", "nfsd", "os", "powersupplyclass",
					"pressure", "processes", "rapl", "schedstat", "sockstat", "softnet", "stat",
					"systemd", "tapestats", "textfile", "thermal_zone", "time", "timex", "udp_queues",
					"uname", "vmstat", "xfs", "zfs")).
				Title("Enable collectors").Group("Collectors").
				Desc("--collector.<name> for each. pressure (PSI) and systemd are off upstream and worth turning on for a benchmark host.").
				Unique().MaxItems(48),
			schemapb.List("disable", schemapb.Str("").
				In("arp", "bcache", "bonding", "btrfs", "conntrack", "cpu", "cpufreq", "diskstats",
					"dmi", "edac", "entropy", "fibrechannel", "filefd", "filesystem", "hwmon",
					"infiniband", "ipvs", "loadavg", "mdadm", "meminfo", "netclass", "netdev",
					"netstat", "nfs", "nfsd", "nvme", "os", "powersupplyclass", "pressure",
					"rapl", "schedstat", "selinux", "sockstat", "softnet", "stat", "tapestats",
					"textfile", "thermal_zone", "time", "timex", "udp_queues", "uname", "vmstat",
					"xfs", "zfs", "zoneinfo")).
				Title("Disable collectors").Group("Collectors").
				Desc("--no-collector.<name> for each. Turning off the noisy default collectors keeps the scrape cheap on a loaded host.").
				Unique().MaxItems(48),

			// doc: --collector.filesystem.mount-points-exclude (RE2 regex)
			schemapb.Str("filesystem_mount_points_exclude").Title("Exclude mount points").Group("Filters").
				Desc("--collector.filesystem.mount-points-exclude: regex of mount points not to report.").
				MaxLen(1024).
				Default(`^/(dev|proc|run/credentials/.+|sys|var/lib/docker/.+|var/lib/containers/storage/.+)($|/)`),
			// doc: --collector.netdev.device-exclude
			schemapb.Str("netdev_device_exclude").Title("Exclude network devices").Group("Filters").
				Desc("--collector.netdev.device-exclude: regex of interfaces not to report.").
				MaxLen(1024).Default(`^(veth.*|docker[0-9]+|br-.*|lo)$`),
			// doc: --collector.diskstats.device-exclude
			schemapb.Str("diskstats_device_exclude").Title("Exclude block devices").Group("Filters").
				Desc("--collector.diskstats.device-exclude: regex of block devices not to report.").
				MaxLen(1024).Default(`^(ram|loop|fd|(h|s|v|xv)d[a-z]|nvme\d+n\d+p)\d+$`),
			// doc: --collector.textfile.directory
			schemapb.Str("textfile_directory").Title("Textfile directory").Group("Filters").
				Desc("--collector.textfile.directory: directory of *.prom files the agent may drop for run-scoped labels.").
				MaxLen(256).Default(""),

			// doc: promlog flags --log.level / --log.format
			schemapb.Choice("log_level").Title("Log level").Group("Logging").
				Desc("--log.level.").
				Opt(schemapb.StrV("debug"), "debug").
				Opt(schemapb.StrV("info"), "info").
				Opt(schemapb.StrV("warn"), "warn").
				Opt(schemapb.StrV("error"), "error").
				Default(schemapb.StrV("warn")),
			schemapb.Choice("log_format").Title("Log format").Group("Logging").
				Desc("--log.format.").
				Opt(schemapb.StrV("logfmt"), "logfmt").
				Opt(schemapb.StrV("json"), "json").
				Default(schemapb.StrV("logfmt")),

			// Repeated per-collector flags: one level of context is not enough,
			// so the two lists are folded into flag strings here.
			schemapb.Computed("enable_flags",
				`("enable" in root) ? root.enable.map(c, " --collector." + c).join("") : ""`).
				Result(schemapb.ResultString).Group("Collectors").Title("Rendered --collector.* flags"),
			schemapb.Computed("disable_flags",
				`("disable" in root) ? root.disable.map(c, " --no-collector." + c).join("") : ""`).
				Result(schemapb.ResultString).Group("Collectors").Title("Rendered --no-collector.* flags"),
			schemapb.Computed("defaults_flag",
				`root.disable_defaults ? " --collector.disable-defaults" : ""`).
				Result(schemapb.ResultString).Group("Collectors").Title("Rendered --collector.disable-defaults"),
			schemapb.Computed("textfile_flag",
				`root.textfile_directory == "" ? "" : " --collector.textfile.directory=" + root.textfile_directory`).
				Result(schemapb.ResultString).Group("Filters").Title("Rendered --collector.textfile.directory"),
		).
		Rules(
			schemapb.Rule(`!("enable" in root) || !("disable" in root) || root.enable.all(c, !(c in root.disable))`,
				"a collector cannot be both enabled and disabled").ID("enable-disable-disjoint"),
		).
		Template("conf", `--web.listen-address={{{values.listen_address}}}:{{{values.listen_port}}}{{{values.defaults_flag}}}{{{values.enable_flags}}}{{{values.disable_flags}}} --collector.filesystem.mount-points-exclude={{{values.filesystem_mount_points_exclude}}} --collector.netdev.device-exclude={{{values.netdev_device_exclude}}} --collector.diskstats.device-exclude={{{values.diskstats_device_exclude}}}{{{values.textfile_flag}}} --log.level={{{values.log_level}}} --log.format={{{values.log_format}}}
`).
		MustBuild()
}
