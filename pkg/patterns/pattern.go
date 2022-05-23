package patterns

import (
	"context"
	"time"

	"github.com/rs/zerolog"
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
