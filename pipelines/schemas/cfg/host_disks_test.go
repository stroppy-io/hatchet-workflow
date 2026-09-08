package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestHostDisks(t *testing.T) {
	schematest.Run(t, HostDisks(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"fstab": true,
				"mounts": []any{
					map[string]any{
						"device":        "/dev/vdb",
						"mount_point":   "/var/lib/postgresql",
						"fs":            "xfs",
						"mount_options": "noatime,nodiratime",
						"owner":         "postgres:postgres",
						"mode":          "0700",
					},
					map[string]any{
						"device": "/dev/disk/by-partlabel/ydb_disk_ssd_01",
						"raw":    true,
					},
				},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"mounts": []any{map[string]any{"mount_point": "/data"}}}, Code: "REQUIRED_MISSING", Path: "mounts[0].device"},
			{Value: map[string]any{"mounts": []any{map[string]any{"device": "/dev/vdb", "fs": "btrfs"}}}, Code: "CHOICE_NOT_ALLOWED", Path: "mounts[0].fs"},
			{Value: map[string]any{"mounts": []any{map[string]any{"device": "/dev/vdb", "mode": "999x"}}}, Code: "PATTERN_MISMATCH", Path: "mounts[0].mode"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"#!/bin/sh",
			"set -eu",
		},
	})
}
