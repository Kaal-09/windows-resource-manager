package processes

import (
	"math"
	"time"
)

type ResourceUsage struct {
	PID               uint32
	PageFaultCount    uint32
	CreationTime100ns uint64
	KernelTime100ns   uint64
	UserTime100ns     uint64
	TotalCPU100ns     uint64
	WorkingSetBytes   uint64
	PrivateBytes      uint64
	SampledAt         time.Time
}

func CalculateCPU(previous ResourceUsage, current ResourceUsage, logicalCPUCount int) (float64, float64, bool) {
	if logicalCPUCount <= 0 {
		return 0, 0, false
	}
	if previous.CreationTime100ns != current.CreationTime100ns {
		return 0, 0, false
	}
	elapsed := current.SampledAt.Sub(previous.SampledAt)
	if elapsed <= 0 || current.TotalCPU100ns < previous.TotalCPU100ns {
		return 0, 0, false
	}

	deltaCPU := current.TotalCPU100ns - previous.TotalCPU100ns
	cpuSeconds := float64(deltaCPU) / 10_000_000
	oneCorePercent := cpuSeconds / elapsed.Seconds() * 100
	machinePercent := oneCorePercent / float64(logicalCPUCount)

	return math.Max(0, oneCorePercent), math.Max(0, machinePercent), true
}
