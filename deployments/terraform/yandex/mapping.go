package yandex

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"

// The Yandex terraform module takes closed-set string vars. These mappers are the
// single source of truth for deployment.Yandex.* enum -> terraform string (the
// mapping documented in deployment/yandex.proto). An unmapped enum returns "" so
// terraform validation rejects it loudly.

// DiskTypeString maps a disk type enum to the yandex_compute_disk.type string.
func DiskTypeString(t deployment.Yandex_DiskType) string {
	switch t {
	case deployment.Yandex_DISK_TYPE_NETWORK_SSD:
		return "network-ssd"
	case deployment.Yandex_DISK_TYPE_NETWORK_HDD:
		return "network-hdd"
	case deployment.Yandex_DISK_TYPE_NETWORK_SSD_NONREPLICATED:
		return "network-ssd-nonreplicated"
	case deployment.Yandex_DISK_TYPE_NETWORK_SSD_IO_M3:
		return "network-ssd-io-m3"
	default:
		return ""
	}
}

// NetworkAccelerationString maps to network_acceleration_type.
func NetworkAccelerationString(a deployment.Yandex_NetworkAcceleration) string {
	switch a {
	case deployment.Yandex_NETWORK_ACCELERATION_STANDARD:
		return "standard"
	case deployment.Yandex_NETWORK_ACCELERATION_SOFTWARE_ACCELERATED:
		return "software_accelerated"
	default:
		return ""
	}
}

// PlatformIDString maps to yandex_compute_instance.platform_id.
func PlatformIDString(p deployment.Yandex_PlatformId) string {
	switch p {
	case deployment.Yandex_PLATFORM_ID_STANDARD_V1:
		return "standard-v1"
	case deployment.Yandex_PLATFORM_ID_STANDARD_V2:
		return "standard-v2"
	case deployment.Yandex_PLATFORM_ID_STANDARD_V3:
		return "standard-v3"
	case deployment.Yandex_PLATFORM_ID_HIGHFREQ_V3:
		return "highfreq-v3"
	default:
		return ""
	}
}

// ZoneString maps to a Yandex Cloud availability zone string.
func ZoneString(z deployment.Yandex_Zone) string {
	switch z {
	case deployment.Yandex_ZONE_RU_CENTRAL1_A:
		return "ru-central1-a"
	case deployment.Yandex_ZONE_RU_CENTRAL1_B:
		return "ru-central1-b"
	case deployment.Yandex_ZONE_RU_CENTRAL1_C:
		return "ru-central1-c"
	case deployment.Yandex_ZONE_RU_CENTRAL1_D:
		return "ru-central1-d"
	default:
		return ""
	}
}
