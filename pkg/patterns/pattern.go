// Package patterns include types that describe how colors and/or brightness
// should change. All should implement the Pattern interface and the BasePattern
// type will handle common logic across all patterns.
package patterns

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/events"
	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
	"github.com/rs/zerolog"
)

// Pattern is the expected interface all patterns implement.
type Pattern interface {
	GetDefaultDelay() time.Duration
	GetBase() *BasePattern
	Run(context.Context, *zerolog.Logger) error
	Stop()
	String() string
}

// Config is the expected interface for retrieving configuration values.
type Config interface {
	GetBool(string) bool
	GetDuration(string) time.Duration
	GetString(string) string
}

// ChangeEvent is an event that is emitted when the running pattern is changed.
type ChangeEvent struct {
	Pattern string
}

// BasePattern is part of all patterns that provides common attributes and implementation.
type BasePattern struct {
	Name string

	self runnable // used to ensure access to the real (child) pattern type
	ctx  context.Context
	log  *zerolog.Logger

	defaultDelay  time.Duration
	stopRequested bool
}

// DelayLabel is used to get the pattern delay from configuration.
const DelayLabel = "delay"

// Events are where Watchers can be created and ChangeEvents are emitted.
var Events = &events.Manager{}

// SetConfig is used to establish how to retrieve configuration values.
func SetConfig(cfg Config) Config {
	config = cfg
	return config
}

// Get will return a registered Pattern by name.
func Get(name string) Pattern {
	return registeredPatterns[name]
}

// GetDefaultDelay will return the pattern's default delay.
func (p *BasePattern) GetDefaultDelay() time.Duration {
	return p.defaultDelay
}

// GetBase will return the BasePattern of any pattern.
func (p *BasePattern) GetBase() *BasePattern {
	return p
}

// GetRunning will return nil or the currently running pattern.
func GetRunning() Pattern {
	return running
}

// Run will begin executing a pattern. If the context passed in is canceled, the
// running pattern will stop.
func (p *BasePattern) Run(parent context.Context, log *zerolog.Logger) error {
	// first, turn keyboard on if it's off...
	brightness, err := keyboard.GetCurrentBrightness()
	if err != nil {
		return err
	}

	if brightness == "0" {
		keyboard.BrightnessFileHandler("255")
	}

	mutex.Lock()
	if cancel != nil {
		cancel()
	}
	var cancelCtx context.Context
	cancelCtx, cancel = context.WithCancel(parent)
	running = p.self
	mutex.Unlock()

	Events.Emit(ChangeEvent{Pattern: p.Name})
	return p.rawRun(cancelCtx, log, "pattern")
}

// Stop will terminate the currently running pattern.
func (p *BasePattern) Stop() {
	mutex.Lock()
	if cancel != nil {
		cancel()
	}
	cancel = nil
	running = nil
	mutex.Unlock()
}

// String will return a readable representation of the pattern.
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

var config Config
var running Pattern // only one allowed to be running at any given time, thus a package global tracker
var mutex sync.Mutex
var cancel func()

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

func (p *BasePattern) rawRun(parent context.Context, log *zerolog.Logger, logKey string) error {
	plog := log.With().Str(logKey, p.Name).Logger()
	p.ctx = parent
	p.log = &plog
	p.log.Info().Msg("started")
	defer p.log.Info().Msg("stopped")
	return p.self.run()
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
	return config.GetDuration(p.Name + "." + DelayLabel)
}
