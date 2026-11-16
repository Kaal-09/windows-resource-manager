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

var psapiDLL = syscall.NewLazyDLL("psapi.dll")
var getProcessMemoryInfo = psapiDLL.NewProc("GetProcessMemoryInfo")

type windowsDiscoverer struct{}

// exact fields returned by GetProcessMemoryInfo bcs there is no type like this already
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

func NewDiscoverer() ProcessDiscoverer {
	return &windowsDiscoverer{}
}

func (wd *windowsDiscoverer) List() ([]Process, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(
		windows.TH32CS_SNAPPROCESS,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("creating process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	err = windows.Process32First(snapshot, &entry)
	if err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return []Process{}, nil
		}
		return nil, fmt.Errorf("reading first process: %w", err)
	}
	var processes []Process
	for {
		p := Process{
			PID:       entry.ProcessID,
			ParentPID: entry.ParentProcessID,
			Name:      windows.UTF16ToString(entry.ExeFile[:]),
		}
		owner, _, _, err := wd.GetOwner(p.PID)
		if err != nil {
			p.OwnerErr = err.Error()
		} else {
			p.Owner = owner
		}
		processes = append(processes, p)
		err = windows.Process32Next(snapshot, &entry)

		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}

			return nil, fmt.Errorf("reading next process: %w", err)
		}
	}

	return processes, nil
}

func (wd *windowsDiscoverer) GetByPID(pid uint32) (*Process, error) {
	processes, err := wd.List()
	if err != nil {
		return nil, err
	}
	for _, p := range processes {
		if p.PID == pid {
			return &p, nil
		}
	}

	return nil, errors.New("process not found")
}

func (wd *windowsDiscoverer) GetOwner(pid uint32) (owner string, domain string, sidString string, err error) {
	processHandle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		pid,
	)
	defer windows.CloseHandle(processHandle)
	if err != nil {
		return "", "", "", fmt.Errorf(
			"OpenProcess failed: %w", err,
		)
	}

	var token windows.Token
	err = windows.OpenProcessToken(
		processHandle,
		windows.TOKEN_QUERY,
		&token,
	)
	if err != nil {
		return "", "", "", fmt.Errorf(
			"OpenProcessToken failed: %w", err,
		)
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return "", "", "", fmt.Errorf(
			"GetTokenUser failed: %w", err,
		)
	}
	sid := tokenUser.User.Sid
	sidString = sid.String()
	owner, domain, _, err = sid.LookupAccount("")
	if err != nil {
		return "", domain, sidString, fmt.Errorf(
			"LookupAccount failed: %w", err,
		)
	}

	return owner, domain, sidString, nil
}

func (wd *windowsDiscoverer) Usage(pid uint32) (*ResourceUsage, error) {
	var usage ResourceUsage
	usage.PID = pid
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return &usage, fmt.Errorf("OpenProcess failed for PID %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	var creationTime windows.Filetime
	var exitTime windows.Filetime
	var kernelTime windows.Filetime
	var userTime windows.Filetime

	if err = windows.GetProcessTimes(handle, &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return &usage, fmt.Errorf("GetProcessTimes failed: %w", err)
	}
	usage.CreationTime = filetimeToUint64(creationTime)
	usage.KernelTime = filetimeToUint64(kernelTime)
	usage.UserTime = filetimeToUint64(userTime)
	usage.TotalCPU = usage.KernelTime + usage.UserTime

	var counters processMemoryCountersEx
	counters.CB = uint32(unsafe.Sizeof(counters))
	result, _, callErr := getProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB))
	if result == 0 {
		if callErr != syscall.Errno(0) && callErr != nil {
			return &usage, fmt.Errorf("GetProcessMemoryInfo failed: %w", callErr)
		}
		return &usage, errors.New("GetProcessMemoryInfo failed")
	}
	usage.WorkingSetBytes = uint64(counters.WorkingSetSize)
	usage.PrivateBytes = uint64(counters.PrivateUsage)
	usage.PageFaultCount = counters.PageFaultCount
	usage.SampledAt = time.Now()

	return &usage, nil
}

func filetimeToUint64(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}
