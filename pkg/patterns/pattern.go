package patterns

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Pattern interface {
	GetBase() *BasePattern
	Run() error
}

type BasePattern struct {
	Name  string
	Ctx   context.Context
	Log   *zerolog.Logger
	Delay time.Duration

	stopRequested bool
}

func (p *BasePattern) GetBase() *BasePattern {
	return p
}

func (p *BasePattern) cancelableSleep() bool {
	wake := time.NewTimer(p.Delay)
	select {
	case <-p.Ctx.Done():
		wake.Stop()
		return true
	case <-wake.C:
		return false
	}
}

func logRecover() {
	if r := recover(); r != nil {
		// wrap these because most (all?) panics and unhandled errors do not carry a stacktrace
		err := errors.Wrap(r.(error), "recovered error")
		log.Error().Stack().Err(err).Msg("")
	}
}
