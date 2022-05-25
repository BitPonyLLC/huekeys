package patterns

import (
	"context"
	"sync"
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

	run           func() error
	stopRequested bool
}

func (p *BasePattern) GetBase() *BasePattern {
	return p
}

// only one allowed to be running at any given time, thus a package global tracker
var running Pattern
var mutex sync.Mutex
var cancel func()

func GetRunning() Pattern {
	return running
}

func (p *BasePattern) Run() error {
	mutex.Lock()
	if cancel != nil {
		cancel()
	}
	var newCtx context.Context
	newCtx, cancel = context.WithCancel(p.Ctx)
	p.Ctx = newCtx
	running = p
	mutex.Unlock()

	p.Log.Info().Str("name", p.Name).Msg("starting")
	defer func() {
		p.Log.Info().Str("name", p.Name).Msg("stopped")
	}()
	return p.run() // this will crash if a pattern is defined and doesn't set it
}

func (p *BasePattern) cancelableSleep() bool {
	wake := time.NewTimer(p.Delay)
	select {
	case <-p.Ctx.Done():
		wake.Stop()
		p.stopRequested = true
		return true
	case <-wake.C:
		return false
	}
}
