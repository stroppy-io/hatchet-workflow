package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// HostDisks is cfg.host.disks@1 — the data disks attached to a machine and
// how the host-prep step makes them usable. Rendered as a shell fragment run
// once, before any container starts.
//
// Two shapes:
//   - a normal mount: mkfs + mkdir + mount + chown/chmod (postgres PGDATA,
//     mysql datadir, cockroach --store, picodata instance_dir);
//   - raw: no filesystem at all — the block device is handed to the engine
//     directly. YDB pdisks are used this way (see main-v0-surface §3:
//     pdisks_per_storage_node, /dev/disk/by-partlabel/ydb_disk_*).
//
// doc: mkfs.ext4(8), mkfs.xfs(8), mount(8) options; sgdisk(8) for the label.
func HostDisks() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("host.disks", 1)).
		Descr("Data disks of a machine: filesystem, mount point and ownership, or a raw block device.").
		Strict().Coerce().
		Fields(
			schemapb.Bool("fstab").Title("Persist in /etc/fstab").Group("Mounts").
				Desc("Append an /etc/fstab entry for every non-raw mount so it survives a reboot of the stand.").
				Default(true),

			schemapb.List("mounts",
				schemapb.Object("",
					schemapb.Str("device").Title("Device").Group("Cluster").
						Desc("Block device path. Filled by the server from topology (the provisioned disk of this machine).").
						MinLen(1).MaxLen(256).Required().
						Examples(schemapb.StrV("/dev/vdb"), schemapb.StrV("/dev/disk/by-partlabel/ydb_disk_ssd_01")),
					schemapb.Bool("raw").Title("Raw device").Group("Mounts").
						Desc("No filesystem: the device is given to the engine as a raw block device (YDB pdisk). mount_point/fs/mkfs_options are ignored.").
						Default(false),
					schemapb.Str("mount_point").Title("Mount point").Group("Mounts").
						Desc("Absolute directory the filesystem is mounted at.").
						Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/var/lib/stroppy"),
					schemapb.Choice("fs").Title("Filesystem").Group("Mounts").
						Desc("Filesystem created on the device. xfs is the default for database data volumes.").
						Opt(schemapb.StrV("xfs"), "XFS").
						Opt(schemapb.StrV("ext4"), "ext4").
						Default(schemapb.StrV("xfs")),
					schemapb.Str("mkfs_options").Title("mkfs options").Group("Mounts").
						Desc("Extra flags passed to mkfs.<fs>. Empty means the distro defaults.").
						MaxLen(256).Default(""),
					schemapb.Str("mount_options").Title("Mount options").Group("Mounts").
						Desc("Comma-separated mount(8) options. noatime/nodiratime remove a write per read.").
						Pattern(`^[A-Za-z0-9_,=.:-]*$`).Default("noatime,nodiratime"),
					schemapb.Str("owner").Title("Owner").Group("Mounts").
						Desc(`chown argument for the mount point, "user:group".`).
						Pattern(`^[A-Za-z0-9_.-]+:[A-Za-z0-9_.-]+$`).Default("root:root"),
					schemapb.Str("mode").Title("Mode").Group("Mounts").
						Desc("Octal chmod mode for the mount point.").
						Pattern(`^[0-7]{3,4}$`).Default("0755"),
				).Strict(),
			).Title("Disks").Group("Mounts").
				Desc("One entry per data disk of this machine. Filled by the server from topology.").
				MaxItems(16),

			// A list of objects cannot be walked by a logic-less Mustache
			// template (the render context is one level deep), so the whole
			// shell fragment is assembled here and printed as one block.
			schemapb.Computed("script",
				`("mounts" in root) ? root.mounts.map(m,
					m.raw
						? "# raw device, no filesystem\nblkdiscard -f " + m.device + " || true"
						: "mkfs." + m.fs + " " + m.mkfs_options + " " + m.device + "\n" +
						  "mkdir -p " + m.mount_point + "\n" +
						  "mount -o " + m.mount_options + " " + m.device + " " + m.mount_point + "\n" +
						  "chown " + m.owner + " " + m.mount_point + "\n" +
						  "chmod " + m.mode + " " + m.mount_point +
						  (root.fstab
							? "\necho \"" + m.device + " " + m.mount_point + " " + m.fs + " " + m.mount_options + " 0 2\" >> /etc/fstab"
							: "")
				).join("\n\n") : ""`).
				Result(schemapb.ResultString).Group("Mounts").
				Title("Provisioning script").Desc("Derived: the shell fragment that prepares every disk."),
		).
		Template("conf", `#!/bin/sh
# managed by stroppy-cloud — cfg.host.disks@1
set -eu

{{{values.script}}}
`).
		MustBuild()
}
