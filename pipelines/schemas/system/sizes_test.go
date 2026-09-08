package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func cell(cpu, mem, disk int64, instance, diskType string) map[string]any {
	return map[string]any{
		"cpu":             cpu,
		"memory_gb":       mem,
		"instance_type":   instance,
		"default_disk_gb": disk,
		"disk_type":       diskType,
	}
}

func yandexFamily() map[string]any {
	return map[string]any{
		"XS": cell(2, 4, 50, "standard-v3", "network-ssd"),
		"S":  cell(4, 16, 100, "standard-v3", "network-ssd"),
		"M":  cell(8, 32, 200, "standard-v3", "network-ssd"),
		"L":  cell(16, 64, 400, "standard-v3", "network-ssd"),
		"XL": cell(32, 128, 800, "standard-v3", "network-ssd-io-m3"),
	}
}

func awsFamily() map[string]any {
	return map[string]any{
		"XS": cell(2, 8, 50, "m7i.large", "gp3"),
		"S":  cell(4, 16, 100, "m7i.xlarge", "gp3"),
		"M":  cell(8, 32, 200, "m7i.2xlarge", "gp3"),
		"L":  cell(16, 64, 400, "m7i.4xlarge", "gp3"),
		"XL": cell(32, 128, 800, "m7i.8xlarge", "io2"),
	}
}

func yandexOnly(family map[string]any) map[string]any {
	return map[string]any{"yandex": map[string]any{"db": family}}
}

func TestSystemSizes(t *testing.T) {
	minimal := yandexOnly(yandexFamily())
	full := map[string]any{
		"yandex": map[string]any{
			"db":          yandexFamily(),
			"proxy":       yandexFamily(),
			"runner":      yandexFamily(),
			"coordinator": yandexFamily(),
		},
		"aws": map[string]any{"db": awsFamily(), "runner": awsFamily()},
	}

	shrinking := yandexFamily()
	shrinking["L"] = cell(4, 64, 400, "standard-v3", "network-ssd")

	missingSize := yandexFamily()
	delete(missingSize, "XL")

	badDisk := yandexFamily()
	badDisk["XS"] = cell(2, 4, 1, "standard-v3", "network-ssd")

	// disk_type carries a Pattern two levels inside the map value schema
	// (provider map → role family → size → cell); it must be enforced.
	badType := yandexFamily()
	badType["XS"] = cell(2, 4, 50, "standard-v3", "Network SSD")

	badInstance := yandexFamily()
	badInstance["XS"] = cell(2, 4, 50, "", "network-ssd")

	unknownCell := yandexFamily()
	unknownCell["XS"] = map[string]any{
		"cpu": int64(2), "memory_gb": int64(4), "instance_type": "standard-v3",
		"default_disk_gb": int64(50), "disk_type": "network-ssd", "gpu": int64(1),
	}

	schematest.Run(t, Sizes(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{Value: yandexOnly(shrinking), Code: "RULE_VIOLATED", Path: "yandex-cpu-monotonic"},
			{Value: yandexOnly(missingSize), Code: "REQUIRED_MISSING", Path: "yandex.db.XL"},
			{Value: yandexOnly(badDisk), Code: "GTE_VIOLATED", Path: "yandex.db.XS.default_disk_gb"},
			{Value: yandexOnly(badType), Code: "PATTERN_MISMATCH", Path: "yandex.db.XS.disk_type"},
			{Value: yandexOnly(badInstance), Code: "MIN_LEN_VIOLATED", Path: "yandex.db.XS.instance_type"},
			{Value: yandexOnly(unknownCell), Code: "UNKNOWN_FIELD", Path: "yandex.db.XS.gpu"},
		},
	})
}
