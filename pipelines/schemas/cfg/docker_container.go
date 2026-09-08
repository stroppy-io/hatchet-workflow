package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// DockerContainer is cfg.docker.container@1 — the runtime knobs applied to
// every container of one role. All databases in this product are deployed as
// docker images (STROPPY.MD §6.1 step 2), so this is the single place where
// restart policy, log rotation, ulimits and cgroup limits are decided.
//
// Rendered as the flags line of `docker run`; the engine's own image/command
// are appended by the pipeline.
//
// doc: https://docs.docker.com/reference/cli/docker/container/run/
// doc: https://docs.docker.com/engine/logging/drivers/json-file/
func DockerContainer() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("docker.container", 1)).
		Descr("Common docker run knobs for the containers of one role.").
		Strict().Coerce().
		Fields(
			// doc: docker run --restart; values no|always|unless-stopped|on-failure[:n]
			schemapb.Choice("restart_policy").Title("Restart policy").Group("Lifecycle").
				Desc("--restart. Benchmarks want a crash to be visible, not silently restarted: the default is no.").
				Opt(schemapb.StrV("no"), "no — never restart").
				Opt(schemapb.StrV("on-failure"), "on-failure").
				Opt(schemapb.StrV("always"), "always").
				Opt(schemapb.StrV("unless-stopped"), "unless-stopped").
				Default(schemapb.StrV("no")),
			schemapb.Int64("restart_max_retries").Title("Restart retries").Group("Lifecycle").
				Desc("Retry count appended to on-failure (--restart on-failure:N). Ignored for other policies.").
				When(`root.restart_policy == "on-failure"`).Gte(1).Lte(100).Default(3),

			// doc: docker run --log-driver / --log-opt
			schemapb.Choice("log_driver").Title("Log driver").Group("Logging").
				Desc("--log-driver. json-file is what the agent tails to ship container logs.").
				Opt(schemapb.StrV("json-file"), "json-file").
				Opt(schemapb.StrV("local"), "local").
				Opt(schemapb.StrV("none"), "none — drop logs").
				Default(schemapb.StrV("json-file")),
			schemapb.Str("log_max_size").Title("Log max size").Group("Logging").
				Desc("--log-opt max-size: rotate a log file at this size (k/m/g suffix).").
				Pattern(`^[0-9]+[kmg]$`).Default("50m"),
			schemapb.Int64("log_max_file").Title("Log files kept").Group("Logging").
				Desc("--log-opt max-file: number of rotated files kept.").
				Gte(1).Lte(100).Default(3),

			// doc: docker run --ulimit <type>=<soft>[:<hard>]
			schemapb.Int64("ulimit_nofile_soft").Title("nofile (soft)").Group("Ulimits").
				Desc("--ulimit nofile soft limit: open file descriptors per process.").
				Gte(1024).Lte(10485760).Default(1048576),
			schemapb.Int64("ulimit_nofile_hard").Title("nofile (hard)").Group("Ulimits").
				Desc("--ulimit nofile hard limit.").
				Gte(1024).Lte(10485760).Default(1048576),
			schemapb.Int64("ulimit_nproc").Title("nproc").Group("Ulimits").
				Desc("--ulimit nproc: max processes/threads. 0 leaves the daemon default.").
				Gte(0).Lte(4194304).Default(0),
			schemapb.Int64("ulimit_memlock").Title("memlock").Group("Ulimits").
				Desc("--ulimit memlock in bytes; -1 = unlimited (needed when the engine locks its buffer pool).").
				Gte(-1).Lte(1099511627776).Default(-1),

			// doc: docker run --shm-size
			schemapb.Int64("shm_size_mb").Title("/dev/shm size").Group("Resources").
				Desc("--shm-size. PostgreSQL parallel query and pg_stat need more than the 64 MB default.").
				Unit("MB").Gte(64).Lte(1048576).Default(1024),
			// doc: docker run --pids-limit (-1 = unlimited)
			schemapb.Int64("pids_limit").Title("PID limit").Group("Resources").
				Desc("--pids-limit; -1 = unlimited.").
				Gte(-1).Lte(4194304).Default(-1),
			schemapb.Bool("limit_cpu").Title("Limit CPU to the machine size").Group("Resources").
				Desc("Emit --cpus from the machine size chosen for this role instead of letting the container use the whole host.").
				Default(false),
			schemapb.Bool("limit_memory").Title("Limit memory to the machine size").Group("Resources").
				Desc("Emit --memory from the machine size chosen for this role.").
				Default(false),
			schemapb.Double("cpus").Title("CPUs").Group("Cluster").
				Desc("--cpus value. Filled by the server from the role's machine size when limit_cpu is on.").
				Gt(0).Lte(1024).Nullable(),
			schemapb.Int64("memory_mb").Title("Memory limit").Group("Cluster").
				Desc("--memory value. Filled by the server from the role's machine size when limit_memory is on.").
				Unit("MB").Gte(64).Lte(16777216).Nullable(),

			// doc: docker run --privileged / --network / --add-host / --sysctl / --cap-add
			schemapb.Bool("privileged").Title("Privileged").Group("Security").
				Desc("--privileged. Needed only for engines that touch raw block devices (YDB pdisks).").
				Default(false),
			schemapb.Choice("network_mode").Title("Network mode").Group("Network").
				Desc("--network. host removes the NAT hop and is what the product uses for database roles.").
				Opt(schemapb.StrV("host"), "host").
				Opt(schemapb.StrV("bridge"), "bridge").
				Default(schemapb.StrV("host")),
			schemapb.List("extra_hosts", schemapb.Str("").Pattern(`^[A-Za-z0-9_.-]+:[0-9A-Fa-f.:]+$`)).
				Title("Extra hosts").Group("Cluster").
				Desc(`--add-host entries "name:ip". Filled by the server from topology so peers resolve without DNS.`).
				MaxItems(64),
			schemapb.MapOf("sysctls", schemapb.Str("value").MaxLen(128)).
				Title("Container sysctls").Group("Security").
				Desc("--sysctl key=value, applied inside the container namespace (net.* only when network_mode is bridge).").
				MaxEntries(32).
				Rules(schemapb.Rule(
					`this.all(k, k.matches("^[a-z0-9_]+([.][a-z0-9_-]+)+$"))`,
					"sysctl keys look like net.core.somaxconn").ID("sysctl-key-shape")),
			schemapb.List("cap_add", schemapb.Str("").Pattern(`^[A-Z_]+$`)).
				Title("Capabilities").Group("Security").
				Desc("--cap-add. IPC_LOCK for engines that mlock memory, SYS_NICE for scheduler priority.").
				MaxItems(32).Unique(),

			// Lists and maps cannot be walked by the one-level render context:
			// the repeated flags are assembled here.
			schemapb.Computed("extra_host_flags",
				`("extra_hosts" in root) ? root.extra_hosts.map(h, " --add-host " + h).join("") : ""`).
				Result(schemapb.ResultString).Group("Cluster").Title("Rendered --add-host flags"),
			schemapb.Computed("sysctl_flags",
				`("sysctls" in root) ? root.sysctls.map(k, " --sysctl " + k + "=" + string(root.sysctls[k])).join("") : ""`).
				Result(schemapb.ResultString).Group("Security").Title("Rendered --sysctl flags"),
			schemapb.Computed("cap_add_flags",
				`("cap_add" in root) ? root.cap_add.map(c, " --cap-add " + c).join("") : ""`).
				Result(schemapb.ResultString).Group("Security").Title("Rendered --cap-add flags"),
			schemapb.Computed("restart_flag",
				`root.restart_policy == "on-failure"
					? "--restart on-failure:" + string(root.restart_max_retries)
					: "--restart " + root.restart_policy`).
				Result(schemapb.ResultString).Group("Lifecycle").Title("Rendered --restart flag"),
			schemapb.Computed("limit_flags",
				`(root.limit_cpu && ("cpus" in root) ? " --cpus " + string(root.cpus) : "") +
				 (root.limit_memory && ("memory_mb" in root) ? " --memory " + string(root.memory_mb) + "m" : "")`).
				Result(schemapb.ResultString).Group("Resources").Title("Rendered --cpus/--memory flags"),
			schemapb.Computed("ulimit_flags",
				`" --ulimit nofile=" + string(root.ulimit_nofile_soft) + ":" + string(root.ulimit_nofile_hard) +
				 (root.ulimit_nproc > 0 ? " --ulimit nproc=" + string(root.ulimit_nproc) : "") +
				 " --ulimit memlock=" + string(root.ulimit_memlock) + ":" + string(root.ulimit_memlock)`).
				Result(schemapb.ResultString).Group("Ulimits").Title("Rendered --ulimit flags"),
			schemapb.Computed("privileged_flag",
				`root.privileged ? " --privileged" : ""`).
				Result(schemapb.ResultString).Group("Security").Title("Rendered --privileged flag"),
			schemapb.Computed("log_flags",
				`root.log_driver == "none"
					? "--log-driver none"
					: "--log-driver " + root.log_driver +
					  " --log-opt max-size=" + root.log_max_size +
					  " --log-opt max-file=" + string(root.log_max_file)`).
				Result(schemapb.ResultString).Group("Logging").Title("Rendered --log-* flags"),
		).
		Rules(
			schemapb.Rule("int(root.ulimit_nofile_soft) <= int(root.ulimit_nofile_hard)",
				"nofile soft limit must be <= hard limit").ID("nofile-soft-le-hard"),
			schemapb.Rule(`root.network_mode == "bridge" || !("sysctls" in root) || root.sysctls.all(k, !k.startsWith("net."))`,
				"net.* container sysctls require network_mode = bridge (host mode shares the host namespace)").ID("net-sysctl-needs-bridge"),
		).
		Template("conf", `{{{values.restart_flag}}} {{{values.log_flags}}}{{{values.ulimit_flags}}} --shm-size {{{values.shm_size_mb}}}m --pids-limit {{{values.pids_limit}}} --network {{{values.network_mode}}}{{{values.limit_flags}}}{{{values.privileged_flag}}}{{{values.extra_host_flags}}}{{{values.sysctl_flags}}}{{{values.cap_add_flags}}}
`).
		MustBuild()
}
