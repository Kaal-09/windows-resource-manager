package processes

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrUnsupported = errors.New("process discovery is only supported on Windows")

type Process struct {
	PID       uint32
	ParentPID uint32
	Name      string
}

func List() ([]Process, error) {
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

func GetByPID(pid uint32) (*Process, error) {
	processes, err := List()
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
