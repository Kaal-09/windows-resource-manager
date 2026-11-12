package enforcement

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	JobObjectCpuRateControlInformation = 15
	JobObjectCpuRateControlEnable      = 0x1
	JobObjectCpuRateControlHardCap     = 0x4
	ProcessSetQuota                    = 0x0100
	ProcessTerminate                   = 0x0001
)

type cpuRateControllInformation struct {
	ControlFlags uint32
	CpuRate      uint32
}

var kernal32 = syscall.NewLazyDLL("kernel32.dll")
var createJobObject = kernal32.NewProc("CreateJobObjectW")
var setInformationJobObject = kernal32.NewProc("SetInformationJobObject")
var assighnProcessToJobObject = kernal32.NewProc("AssignProcessToJobObject")

type CPUJob struct {
	handle windows.Handle
	pid    uint32
}

func ApplyCPULimit(pid uint32, percent uint32) (*CPUJob, error) {
	if percent < 1 || percent > 100 {
		return nil, errors.New("CPU limit must be between 1 and 100")
	}

	jobHandle, _, callErr := createJobObject.Call(0, 0)
	if jobHandle == 0 {
		return nil, fmt.Errorf("CreateJobObject failed: %w", callErr)
	}
	job := &CPUJob{
		handle: windows.Handle(jobHandle),
		pid:    pid,
	}
	success := false
	defer func() {
		if !success {
			windows.CloseHandle(job.handle)
		}
	}()
	if err := job.UpdateCPULimit(percent); err != nil {
		return nil, err
	}
}

func (j *CPUJob) UpdateCPULimit(percent uint32) error {
	if j == nil || j.handle == 0 {
		return errors.New("invalid Job Object handle")
	}
	if percent < 1 || percent > 100 {
		return errors.New("CPU limit must be between 1 and 100")
	}

	info := cpuRateControllInformation{
		ControlFlags: JobObjectCpuRateControlEnable | JobObjectCpuRateControlHardCap,
		CpuRate:      percent * 100,
	}
	result, _, callErr := setInformationJobObject.Call(
		uintptr(j.handle),
		uintptr(JobObjectCpuRateControlInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	runtime.KeepAlive(info)
	if result == 0 {
		return fmt.Errorf(
			"disabling CPU limit failed: %w",
			callErr,
		)
	}

	return nil
}
