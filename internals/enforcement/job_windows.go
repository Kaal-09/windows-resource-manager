//go:build windows

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

type jobQueueObjectEnforcement struct {
	handle windows.Handle
	pid    uint32
}

func NewEnforcers() Enforcement {
	return &jobQueueObjectEnforcement{}
}

type cpuRateControllInformation struct {
	ControlFlags uint32
	CpuRate      uint32
}

var kernal32 = syscall.NewLazyDLL("kernel32.dll")
var createJobObject = kernal32.NewProc("CreateJobObjectW")
var setInformationJobObject = kernal32.NewProc("SetInformationJobObject")
var assighnProcessToJobObject = kernal32.NewProc("AssignProcessToJobObject")

func (j *jobQueueObjectEnforcement) ApplyCPULimit(pid uint32, percent uint32) (bool, error) {
	if percent < 1 || percent > 100 {
		return false, errors.New("CPU limit must be between 1 and 100")
	}

	jobHandle, _, callErr := createJobObject.Call(0, 0)
	if jobHandle == 0 {
		return false, fmt.Errorf("CreateJobObject failed: %w", callErr)
	}
	j.handle = windows.Handle(jobHandle)
	j.pid = pid
	success := false
	defer func() {
		if !success {
			windows.CloseHandle(j.handle)
		}
	}()
	if _, err := j.UpdateCPULimit(percent); err != nil {
		return false, err
	}

	processHandle, err := windows.OpenProcess(ProcessSetQuota|ProcessTerminate, false, pid)
	if err != nil {
		return false, fmt.Errorf("OpenProcess failed for PID %d: %w", pid, err)
	}
	defer windows.CloseHandle(processHandle)

	// Call never returns nil, also it forces routine to be multiplexed to same os-thread haha crazy.
	// only for the duration of .Call ofc dumy.
	result, _, callErr := assighnProcessToJobObject.Call(
		uintptr(j.handle),
		uintptr(processHandle),
	)
	if result == 0 {
		return false, fmt.Errorf("AssighnProcessToJobObject failed: %w", callErr)
	}

	success = true
	return true, nil
}

func (j *jobQueueObjectEnforcement) UpdateCPULimit(percent uint32) (bool, error) {
	if j == nil || j.handle == 0 {
		return false, errors.New("invalid Job Object handle")
	}
	if percent < 1 || percent > 100 {
		return false, errors.New("CPU limit must be between 1 and 100")
	}

	info := cpuRateControllInformation{
		ControlFlags: JobObjectCpuRateControlEnable | JobObjectCpuRateControlHardCap,
		CpuRate:      percent * 100,
	}
	result, _, callErr := setInformationJobObject.Call(
		uintptr(j.handle), // this silly call just ask a number.
		uintptr(JobObjectCpuRateControlInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	// not required i keep just for feel.
	runtime.KeepAlive(info)
	if result == 0 {
		return false, fmt.Errorf(
			"disabling CPU limit failed: %w",
			callErr,
		)
	}

	return true, nil
}

func (j *jobQueueObjectEnforcement) DisableCPULimit() (bool, error) {
	if j == nil || j.handle == 0 {
		return false, errors.New("invalid Job Object handle")
	}
	info := cpuRateControllInformation{}

	result, _, callErr := setInformationJobObject.Call(
		uintptr(j.handle),
		uintptr(JobObjectCpuRateControlInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	runtime.KeepAlive(info)
	if result == 0 {
		return false, fmt.Errorf("disabling CPU limit failed: %w", callErr)
	}

	return true, nil
}

func (j *jobQueueObjectEnforcement) Close() error {
	if j == nil || j.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(j.handle)
	if err == nil {
		j.handle = 0
	}

	return err
}
