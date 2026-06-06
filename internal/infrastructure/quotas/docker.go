package quotas

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

type DockerSource struct {
	SnapshotTTL time.Duration
}

func NewDockerSource(snapshotTTL time.Duration) *DockerSource {
	return &DockerSource{SnapshotTTL: snapshotTTL}
}

func (s *DockerSource) ListQuotas(_ context.Context, req SourceRequest) ([]Snapshot, error) {
	now := time.Now().UTC()
	staleAfter := now.Add(s.SnapshotTTL)
	if s.SnapshotTTL <= 0 {
		staleAfter = now.Add(5 * time.Minute)
	}

	memoryMiB := readMemTotalMiB()
	if memoryMiB == 0 {
		memoryMiB = 1024 * 1024
	}
	if memoryMiB < 256*1024 {
		memoryMiB = 256 * 1024
	}
	diskGiB := readDiskTotalGiB(".")
	if diskGiB == 0 {
		diskGiB = 1024 * 1024
	}

	scope := Scope{
		TenantID:     req.TenantID,
		Provider:     deploymentpb.Provider_PROVIDER_DOCKER,
		ResourceType: ResourceTypeDockerHost,
		ResourceID:   "local",
		Service:      "docker",
	}
	cpuCores := float64(runtime.NumCPU())
	if cpuCores < 64 {
		cpuCores = 64
	}
	rows := []struct {
		name  string
		units string
		limit float64
	}{
		{name: "host.containers.count", units: "count", limit: 1000000},
		{name: "host.cpuCores", units: "cores", limit: cpuCores},
		{name: "host.memory.size", units: "MiB", limit: float64(memoryMiB)},
		{name: "host.disk.size", units: "GiB", limit: float64(diskGiB)},
		{name: "host.ports.count", units: "count", limit: 65535},
	}
	out := make([]Snapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, Snapshot{
			Scope:             scope,
			QuotaName:         row.name,
			Units:             row.units,
			ProviderUsed:      0,
			Limit:             row.limit,
			ProviderAvailable: row.limit,
			ObservedAt:        now,
			StaleAfter:        staleAfter,
		})
	}
	return out, nil
}

func readMemTotalMiB() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kib / 1024
	}
	return 0
}

func readDiskTotalGiB(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	bytes := uint64(stat.Blocks) * uint64(stat.Bsize)
	gib := bytes / 1024 / 1024 / 1024
	if gib < 500 {
		gib = 500
	}
	return gib
}
