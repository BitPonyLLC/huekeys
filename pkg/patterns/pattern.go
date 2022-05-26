package patterns

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Pattern interface {
	GetBase() *BasePattern
	Run(context.Context, *zerolog.Logger) error
	String() string
}

type BasePattern struct {
	Name  string
	Delay time.Duration

	run func() error
	ctx context.Context
	log *zerolog.Logger

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

func (p *BasePattern) Run(parent context.Context, log *zerolog.Logger) error {
	mutex.Lock()
	if cancel != nil {
		cancel()
	}
	p.ctx, cancel = context.WithCancel(parent)
	running = p
	mutex.Unlock()

	plog := log.With().Str("pattern", p.Name).Logger()
	p.log = &plog
	p.log.Info().Msg("started")
	defer p.log.Info().Msg("stopped")
	return p.run() // this will crash if a pattern is defined and doesn't set it
}

func (p *BasePattern) String() string {
	if p.Delay == 0 {
		return p.Name
	}
	return fmt.Sprintf("%s delay=%s", p.Name, p.Delay)
}

func (p *BasePattern) cancelableSleep() bool {
	wake := time.NewTimer(p.Delay)
	select {
	case <-p.ctx.Done():
		wake.Stop()
		p.stopRequested = true
		return true
	case <-wake.C:
		return false
	}
}
