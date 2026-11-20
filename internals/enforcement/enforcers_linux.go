//go:build linux

package enforcement

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const (
	cgroupRoot     = "/sys/fs/cgroup"
	cgroupPeriodUS = 100000
)

type linuxEnforcement struct {
	pid       uint32
	cgroupDir string
	percent   uint32
	active    bool
}

func NewEnforcers() Enforcement {
	return &linuxEnforcement{}
}

func (j *linuxEnforcement) ApplyCPULimit(pid uint32, percent uint32) (bool, error) {
	if err := validatePercent(percent); err != nil {
		return false, err
	}
	if pid == 0 {
		return false, errors.New("invalid PID")
	}
	if j.active {
		return false, errors.New("a CPU limit is already applied")
	}

	if err := ensureCPUController(); err != nil {
		return false, err
	}

	dir := filepath.Join(cgroupRoot, fmt.Sprintf("rm-pid-%d", pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("create cgroup: %w", err)
	}

	if err := writeFile(filepath.Join(dir, "cpu.max"), cpuMaxValue(percent)); err != nil {
		_ = os.Remove(dir)
		return false, fmt.Errorf("set cpu.max: %w", err)
	}

	if err := writeFile(filepath.Join(dir, "cgroup.procs"), strconv.FormatUint(uint64(pid), 10)); err != nil {
		_ = os.Remove(dir)
		return false, fmt.Errorf("move PID %d into cgroup: %w", pid, err)
	}

	j.pid = pid
	j.cgroupDir = dir
	j.percent = percent
	j.active = true
	return true, nil
}

func (j *linuxEnforcement) UpdateCPULimit(percent uint32) (bool, error) {
	if err := validatePercent(percent); err != nil {
		return false, err
	}
	if !j.active {
		return false, errors.New("no active CPU limit to update")
	}

	if err := writeFile(filepath.Join(j.cgroupDir, "cpu.max"), cpuMaxValue(percent)); err != nil {
		return false, fmt.Errorf("update cpu.max: %w", err)
	}
	j.percent = percent
	return true, nil
}

func (j *linuxEnforcement) DisableCPULimit() (bool, error) {
	if !j.active {
		return false, errors.New("no active CPU limit to disable")
	}

	if err := writeFile(filepath.Join(j.cgroupDir, "cpu.max"), cpuMaxDisabled()); err != nil {
		return false, fmt.Errorf("disable cpu.max: %w", err)
	}
	j.percent = 0
	return true, nil
}

func (j *linuxEnforcement) Close() error {
	if !j.active {
		return nil
	}

	var errs []error

	pidStr := strconv.FormatUint(uint64(j.pid), 10)
	if err := writeFile(filepath.Join(cgroupRoot, "cgroup.procs"), pidStr); err != nil &&
		!errors.Is(err, syscall.ESRCH) { // todo: add pid protection mechanism.
		errs = append(errs, fmt.Errorf("move PID %d back to root cgroup: %w", j.pid, err))
	}

	if err := os.Remove(j.cgroupDir); err != nil {
		errs = append(errs, fmt.Errorf("remove cgroup: %w", err))
	}

	j.active = false
	return errors.Join(errs...)
}

func validatePercent(p uint32) error {
	if p < 1 || p > 100 {
		return errors.New("CPU limit must be between 1 and 100")
	}
	return nil
}

func cpuMaxValue(percent uint32) string {
	cpus := uint64(runtime.NumCPU())
	quota := uint64(percent) * cgroupPeriodUS * cpus / 100
	return fmt.Sprintf("%d %d", quota, cgroupPeriodUS)
}

func cpuMaxDisabled() string {
	return fmt.Sprintf("max %d", cgroupPeriodUS)
}

func ensureCPUController() error {
	data, err := os.ReadFile(filepath.Join(cgroupRoot, "cgroup.controllers"))
	if err != nil {
		return fmt.Errorf("cgroup v2 not found at %s: %w", cgroupRoot, err)
	}
	if !hasWord(string(data), "cpu") {
		return errors.New("cpu controller is not available on this system")
	}

	if err := writeFile(filepath.Join(cgroupRoot, "cgroup.subtree_control"), "+cpu"); err != nil {
		return fmt.Errorf("enable cpu controller (requires root): %w", err)
	}
	return nil
}

func hasWord(s, word string) bool {
	for _, f := range strings.Fields(s) {
		if f == word {
			return true
		}
	}
	return false
}

func writeFile(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(value); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
