// Pidpath is a helper for managing a PID file to denote when a process might
// already be running.
package pidpath

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"syscall"
	"time"
)

// PidPath is the type for managing a PID file.
type PidPath struct {
	pidpath   string
	perm      fs.FileMode
	checkedAt time.Time
	pid       *int
}

// UnknownPID indicates the PID read from the file wasn't located as a running
// process.
const UnknownPID = -1

// NewPidPath manages a process ID file to coordinate whether a process is
// already running.
func NewPidPath(pathname string, perm fs.FileMode) *PidPath {
	return &PidPath{pidpath: pathname, perm: perm}
}

// String provides the path and other PID info.
func (pp *PidPath) String() string {
	var key string
	if pp.IsOurs() {
		key = "ours"
	} else {
		key = "other"
	}

	return fmt.Sprintf("%s %s=%v", pp.pidpath, key, pp.Getpid())
}

// CheckAndSet evaluates if the process is currently running and, if not, sets
// the current process ID into the file.
func (pp *PidPath) CheckAndSet() error {
	err := pp.Check()
	if err != nil {
		return err
	}

	pid := os.Getpid()

	err = os.WriteFile(pp.pidpath, []byte(fmt.Sprint(pid)), pp.perm)
	if err != nil {
		return fmt.Errorf("unable to write to %s: %w", pp.pidpath, err)
	}

	// wait until _after_ the write succeeds before we declare it "ours"
	pp.pid = &pid
	return nil
}

// Check will determine the state of the process. A `nil` returned indicates no
// other process was found active.
func (pp *PidPath) Check() error {
	return pp.check(true)
}

// IsRunning determines if the pidpath indicates that its process is in the
// process listing (see Getpid's note).
func (pp *PidPath) IsRunning() bool {
	return pp.Getpid() != UnknownPID
}

// IsOurs determines if the pidpath indicates that its process is the currently
// executing one (the caller) of this function.
func (pp *PidPath) IsOurs() bool {
	return pp.Getpid() == os.Getpid()
}

// Getpid retrieves the process ID from the file.
func (pp *PidPath) Getpid() int {
	pp.check(false)

	if pp.pid == nil {
		return UnknownPID
	}

	return *pp.pid
}

// Release will remove the pidpath if it's owned by the current process (i.e.
// this is safe to call if the pidpath is being managed by another process--the
// file will NOT be removed).
func (pp *PidPath) Release() error {
	if pp.IsOurs() {
		pp.checkedAt = time.Time{}
		pp.pid = nil
		return os.Remove(pp.pidpath)
	}

	return nil
}

//--------------------------------------------------------------------------------
// private

func (pp *PidPath) check(forced bool) error {
	if !forced && time.Since(pp.checkedAt) < 1*time.Second {
		return nil
	}

	pp.checkedAt = time.Now()

	pidContent, err := os.ReadFile(pp.pidpath)
	if err == nil {
		var pid int
		pid, err = strconv.Atoi(string(pidContent))
		if err != nil {
			return fmt.Errorf("unable to parse contents of %s: %w", pp.pidpath, err)
		}

		pp.pid = &pid

		if pid == os.Getpid() {
			// just ourselves, probably running an IPC command
			return nil
		}

		err = syscall.Kill(pid, 0)
		if err == nil || err.(syscall.Errno) == syscall.EPERM {
			// if EPERM, process is owned by another user, probably root
			return fmt.Errorf("another process is already running: %d", pid)
		}

		// ESRCH: no such process
		if err.(syscall.Errno) != syscall.ESRCH {
			// can't determine, so assume it is still running
			return fmt.Errorf("unable to check if process %d is still running: %w", pid, err)
		}

		// process is no longer running
		pp.pid = nil
	} else {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to read %s: %w", pp.pidpath, err)
		}
	}

	return nil
}
