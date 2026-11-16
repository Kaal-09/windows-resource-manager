//go:build linux

package enforcement

type linuxEnforcement struct {
	pid uint32
}

func NewEnforcers() Enforcement {
	return &linuxEnforcement{}
}

func (j *linuxEnforcement) ApplyCPULimit(pid uint32, percent uint32) (bool, error) {
	return true, nil
}

func (j *linuxEnforcement) UpdateCPULimit(percent uint32) (bool, error) {
	return true, nil
}

func (j *linuxEnforcement) DisableCPULimit() (bool, error) {
	return true, nil
}

func (j *linuxEnforcement) Close() error {
	return nil
}
