package processes

import (
	"errors"
)

var ErrUnsupported = errors.New("process discovery is only supported on Windows")

type Process struct {
	PID       uint32
	ParentPID uint32
	Name      string

	Owner    string
	Domain   string
	OwnerSID string
	OwnerErr string
}
