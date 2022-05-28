package patterns

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Pattern interface {
	GetDefaultDelay() time.Duration
	GetBase() *BasePattern
	Run(context.Context, *zerolog.Logger) error
	String() string
}

type BasePattern struct {
	Name string

	self runnable // used to ensure access to the real (child) pattern type
	ctx  context.Context
	log  *zerolog.Logger

	defaultDelay  time.Duration
	stopRequested bool
}

const DelayLabel = "delay"

func Get(name string) Pattern {
	return registeredPatterns[name]
}

func (p *BasePattern) GetDefaultDelay() time.Duration {
	return p.defaultDelay
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
	running = p.self
	mutex.Unlock()

	plog := log.With().Str("pattern", p.Name).Logger()
	p.log = &plog
	p.log.Info().Msg("started")
	defer p.log.Info().Msg("stopped")
	return p.self.run()
}

func (p *BasePattern) String() string {
	if p.getDelay() == 0 {
		return p.Name
	}
	return fmt.Sprintf("%s %s=%s", p.Name, DelayLabel, p.getDelay())
}

//--------------------------------------------------------------------------------
// private

type runnable interface {
	Pattern
	run() error
}

var registeredPatterns = map[string]Pattern{}

func register(name string, p Pattern, delay time.Duration) {
	base := p.GetBase()
	base.Name = name
	base.defaultDelay = delay

	if r, ok := p.(runnable); ok {
		base.self = r
	}

	registeredPatterns[name] = p
}

func (p *BasePattern) cancelableSleep() bool {
	wake := time.NewTimer(p.getDelay())
	select {
	case <-p.ctx.Done():
		wake.Stop()
		p.stopRequested = true
		return true
	case <-wake.C:
		return false
	}
}

func (p *BasePattern) getDelay() time.Duration {
	return viper.GetDuration(p.Name + "." + DelayLabel)
}
