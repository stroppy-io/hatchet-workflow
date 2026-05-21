package yandexcloud

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

func TestMapQuotaResource(t *testing.T) {
	cases := []struct {
		name    string
		quotaID string
		want    deployment.QuotaResource
	}{
		{"cores", "compute.instanceCores.count", deployment.QuotaResource_QUOTA_RESOURCE_CORES},
		{"memory", "compute.instanceMemory.size", deployment.QuotaResource_QUOTA_RESOURCE_MEMORY_GB},
		{"ssd", "compute.ssdDisks.size", deployment.QuotaResource_QUOTA_RESOURCE_SSD_GB},
		{"hdd", "compute.hddDisks.size", deployment.QuotaResource_QUOTA_RESOURCE_HDD_GB},
		{"instances", "compute.instances.count", deployment.QuotaResource_QUOTA_RESOURCE_INSTANCES},
		{"externalip", "vpc.externalIpAddresses.count", deployment.QuotaResource_QUOTA_RESOURCE_EXTERNAL_IPS},
		{"publicip", "vpc.publicIp.count", deployment.QuotaResource_QUOTA_RESOURCE_EXTERNAL_IPS},
		{"networks", "vpc.networks.count", deployment.QuotaResource_QUOTA_RESOURCE_NETWORKS},
		{"subnets", "vpc.subnets.count", deployment.QuotaResource_QUOTA_RESOURCE_SUBNETS},
		{"unknown -> unspecified", "some.random.quota", deployment.QuotaResource_QUOTA_RESOURCE_UNSPECIFIED},
		{"empty -> unspecified", "", deployment.QuotaResource_QUOTA_RESOURCE_UNSPECIFIED},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, mapQuotaResource(tc.quotaID))
		})
	}

	t.Run("case insensitive", func(t *testing.T) {
		require.Equal(t, deployment.QuotaResource_QUOTA_RESOURCE_CORES, mapQuotaResource("COMPUTE.INSTANCECORES.COUNT"))
		require.Equal(t, deployment.QuotaResource_QUOTA_RESOURCE_SUBNETS, mapQuotaResource("VPC.SUBNETS"))
	})

	t.Run("instance substring matches 'instances'", func(t *testing.T) {
		require.Equal(t, deployment.QuotaResource_QUOTA_RESOURCE_INSTANCES, mapQuotaResource("compute.instance"))
	})
}
