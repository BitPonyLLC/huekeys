package util

import (
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
