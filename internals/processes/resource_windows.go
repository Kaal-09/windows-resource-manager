//go:build windows

package processes

import (
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// exact fields returned by GetProcessMemoryInfo
type processMemoryCountersEx struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

var psapiDLL = syscall.NewLazyDLL("psapi.dll")
var getProcessMemoryInfo = psapiDLL.NewProc("GetProcessMemoryInfo")

func filetimeToUint64(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

func GetResourceUsage(pid uint32) (ResourceUsage, error) {
	var usage ResourceUsage
	usage.PID = pid
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return usage, fmt.Errorf("OpenProcess failed for PID %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	var creationTime windows.Filetime
	var exitTime windows.Filetime
	var kernelTime windows.Filetime
	var userTime windows.Filetime

	if err = windows.GetProcessTimes(handle, &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return usage, fmt.Errorf("GetProcessTimes failed: %w", err)
	}
	usage.CreationTime100ns = filetimeToUint64(creationTime)
	usage.KernelTime100ns = filetimeToUint64(kernelTime)
	usage.UserTime100ns = filetimeToUint64(userTime)
	usage.TotalCPU100ns = usage.KernelTime100ns + usage.UserTime100ns

	var counters processMemoryCountersEx
	counters.CB = uint32(unsafe.Sizeof(counters))
	result, _, callErr := getProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB))
	if result == 0 {
		if callErr != syscall.Errno(0) && callErr != nil {
			return usage, fmt.Errorf("GetProcessMemoryInfo failed: %w", callErr)
		}
		return usage, errors.New("GetProcessMemoryInfo failed")
	}
	usage.WorkingSetBytes = uint64(counters.WorkingSetSize)
	usage.PrivateBytes = uint64(counters.PrivateUsage)
	usage.PageFaultCount = counters.PageFaultCount
	usage.SampledAt = time.Now()

	return usage, nil
}
