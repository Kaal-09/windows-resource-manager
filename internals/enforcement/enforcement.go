package enforcement

import "errors"

var ErrUnsupported = errors.New("process controll enforcement is only supported on Windows/Linux")

type Enforcement interface {
	ApplyCPULimit(pid uint32, percent uint32) (bool, error)
	UpdateCPULimit(percent uint32) (bool, error)
	DisableCPULimit() (bool, error)
	Close() error
}
