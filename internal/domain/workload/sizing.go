package workload

import (
	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

const maxRunnerVusThreshold = 1000

var (
	defaultRunnerSizing = infrastructurebuilder.MachineSizing{CPUCores: 4, MemoryMB: 8192, DiskGB: 50}
	maxRunnerSizing     = infrastructurebuilder.MachineSizing{CPUCores: 16, MemoryMB: 65536, DiskGB: 100}
)

func RunnerSizing(input *domain.Workload) infrastructurebuilder.MachineSizing {
	if input.GetExecution().GetVus() >= maxRunnerVusThreshold {
		return maxRunnerSizing
	}
	return defaultRunnerSizing
}

func ApplyRunnerSizing(options *infrastructurebuilder.BuildOptions, input *domain.Workload) {
	if options.MachineSizing == nil {
		options.MachineSizing = map[string]infrastructurebuilder.MachineSizing{}
	}
	if _, ok := options.MachineSizing[RunnerNodeID]; ok {
		return
	}
	options.MachineSizing[RunnerNodeID] = RunnerSizing(input)
}
