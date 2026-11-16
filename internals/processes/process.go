package processes

import (
	"errors"
	"math"
	"time"
)

var ErrUnsupported = errors.New("process discovery is only supported on Windows/Linux")

type ProcessDiscoverer interface {
	List() ([]Process, error)
	GetByPID(pid uint32) (*Process, error)
	Usage(pid uint32) (*ResourceUsage, error)
}

type Process struct {
	PID       uint32
	Name      string
	ParentPID uint32
	Owner     string
	OwnerErr  string
}

type ResourceUsage struct {
	PID             uint32
	PageFaultCount  uint32
	CreationTime    uint64
	KernelTime      uint64
	UserTime        uint64
	TotalCPU        uint64
	WorkingSetBytes uint64
	PrivateBytes    uint64
	SampledAt       time.Time
}

func CalculateCPU(previous ResourceUsage, current ResourceUsage, logicalCPUCount int) (float64, float64, bool) {
	if logicalCPUCount <= 0 {
		return 0, 0, false
	}
	if previous.CreationTime != current.CreationTime {
		return 0, 0, false
	}
	elapsed := current.SampledAt.Sub(previous.SampledAt)
	if elapsed <= 0 || current.TotalCPU < previous.TotalCPU {
		return 0, 0, false
	}

	deltaCPU := current.TotalCPU - previous.TotalCPU
	cpuSeconds := float64(deltaCPU) / 10_000_000
	oneCorePercent := cpuSeconds / elapsed.Seconds() * 100
	machinePercent := oneCorePercent / float64(logicalCPUCount)

	return math.Max(0, oneCorePercent), math.Max(0, machinePercent), true
}
