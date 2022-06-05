// Package util provides miscellaneous utility functions.
package util

import (
	"fmt"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// LogRecover helps ensure any unhandled errors are logged.
// Useful as a `defer` function immediately upon entering a goroutine.
func LogRecover() {
	if r := recover(); r != nil {
		// wrap these because most (all?) panics and unhandled errors do not carry a stacktrace
		err := errors.Wrap(r.(error), "recovered error")
		log.Error().Stack().Err(err).Msg("")
	}
}

// BeNice lets a Unix process reduce its own execution priority to avoid impacting other processes.
// Positive values have lower privilege (are nicer) while negative values have a higher privilege (are MEAN!).
func BeNice(priority int) error {
	pid := syscall.Getpid()

	err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, priority)
	if err != nil {
		return fmt.Errorf("unable to set nice level %d: %w", priority, err)
	}

	return nil
}
