package patterns

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/rs/zerolog"
)

// TypingPattern is used when changing when changing colors from "cold" (blue) to "hot" (red)
// according to the speed of key presses occurring. The "delay" configuration value expresses
// the amount of time to wait between evaluating the number of keys recently pressed.
type TypingPattern struct {
	BasePattern

	eventfile    *os.File
	lastReportAt time.Time
	lastReadAt   time.Time
}

// DefaultIdlePeriod is the amount of time the TypingPattern will wait before
// declaring idle and, if configured, starting another pattern until keys are
// pressed again.
const DefaultIdlePeriod = 30 * time.Second

// InputEventIDLabel is used to get the event ID from configuration.
const InputEventIDLabel = "input-event-id"

// AllKeysLabel is used to get the all keys value from configuration.
const AllKeysLabel = "all-keys"

// IdleLabel is used to get the idle pattern from configuration.
const IdleLabel = "idle"

// IdlePeriodLabel is used to get the idle period from configuration.
const IdlePeriodLabel = "idle-period"

var _ Pattern = (*TypingPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*TypingPattern)(nil) // ensures we conform to the runnable interface

const traceReportPeriod = 10 * time.Second

// String is a customized version of the BasePattern String that also includes
// information about the idle settings.
func (p *TypingPattern) String() string {
	str := p.BasePattern.String()
	idlePattern := p.getIdlePattern()
	if idlePattern != nil {
		str += fmt.Sprintf(" %s=[%s]", IdleLabel, idlePattern)
	}
	return str
}

func init() {
	register("typing", &TypingPattern{}, 300*time.Millisecond)
}

func (p *TypingPattern) run() error {
	inputEventID := config.GetString(p.Name + InputEventIDLabel)
	if inputEventID == "" {
		var err error
		inputEventID, err = lookupInputEventID()
		if err != nil {
			return err
		}
	}

	eventpath := "/dev/input/" + inputEventID

	keyPressCount := int32(0)
	err := keyboard.ColorFileHandler(coldHotColors[0])
	if err != nil {
		return err
	}

	go p.setColor(&keyPressCount)

	err = p.startTypingProcessor(eventpath, &keyPressCount)
	if err != nil {
		return err
	}

	// defer as a closure to make sure we close the most recently set eventfile
	defer func() {
		p.eventfile.Close()
	}()

	wokeWatcher := util.StartWokeWatch(10*time.Second, time.Second, func(diff time.Duration) {
		p.log.Warn().Dur("diff", diff).Msg("woke detected: reopening input")
		err := p.startTypingProcessor(eventpath, &keyPressCount)
		if err != nil {
			p.log.Err(err).Msg("typing processor failed to reopen input")
		}
	})
	defer wokeWatcher.Stop()

	<-p.ctx.Done()
	p.stopRequested = true

	return nil
}

func (p *TypingPattern) setColor(keyPressCount *int32) {
	defer util.LogRecover()

	var idleAt *time.Time
	var cancelFunc context.CancelFunc

	lastIndex := 0
	colorsLen := len(coldHotColors)

	for {
		if p.cancelableSleep() {
			break
		}

		i := int(atomic.LoadInt32(keyPressCount))
		if i >= colorsLen {
			i = colorsLen - 1
		}

		if p.log.GetLevel() == zerolog.TraceLevel && time.Since(p.lastReportAt) > traceReportPeriod {
			p.lastReportAt = time.Now()
			var idleVal time.Time
			if idleAt != nil {
				idleVal = *idleAt
			}
			p.log.Trace().Time("idle-at", idleVal).Time("read-at", p.lastReadAt).
				Int("count", i).Int("last-index", lastIndex).Msg("report")
		}

		// don't bother setting the same value and wait for 2 keypresses to
		// avoid halting the pattern for control-key sequences
		if i > 1 && i != lastIndex {
			color := coldHotColors[i]
			err := keyboard.ColorFileHandler(color)
			if err != nil {
				p.log.Err(err).Msg("can't set typing color")
				break
			}

			lastIndex = i
		}

		if i > 0 {
			if i > 1 {
				idleAt = nil
				if cancelFunc != nil {
					cancelFunc()
					cancelFunc = nil
					p.log.Debug().Msg("no longer idle")
				}
			}

			if atomic.LoadInt32(keyPressCount) > 0 {
				atomic.AddInt32(keyPressCount, -1)
			}

			continue
		}

		if idleAt == nil {
			now := time.Now()
			idleAt = &now
			continue
		}

		if cancelFunc != nil {
			// don't start another background monitor!
			continue
		}

		idlePattern := p.getIdlePattern()
		if idlePattern == nil {
			continue
		}

		diff := time.Since(*idleAt)
		if diff > p.getIdlePeriod() {
			p.log.Debug().Msg("idle")
			var cancelCtx context.Context
			cancelCtx, cancelFunc = context.WithCancel(p.ctx)
			go func() {
				defer util.LogRecover()
				// using the private runner otherwise, we'll get canceled! ;)
				err := idlePattern.GetBase().rawRun(cancelCtx, p.log, "idle")
				if err != nil {
					p.log.Err(err).Str("idle", idlePattern.String()).Msg("pattern failed")
				}
			}()
		}
	}

	if cancelFunc != nil {
		cancelFunc()
	}
}

func (p *TypingPattern) startTypingProcessor(eventpath string, keyPressCount *int32) error {
	if p.eventfile != nil {
		p.eventfile.Close()
	}

	var err error
	p.eventfile, err = os.Open(eventpath)
	if err != nil {
		return fmt.Errorf("can't open input events device (%s): %w", eventpath, err)
	}

	go p.processTypingEvents(keyPressCount)
	return nil
}

func (p *TypingPattern) processTypingEvents(keyPressCount *int32) {
	defer util.LogRecover()

	// https://janczer.github.io/work-with-dev-input/
	buf := make([]byte, 24)
	for !p.stopRequested {
		_, err := p.eventfile.Read(buf)
		if err != nil {
			p.log.Err(err).Msg("can't read input events device")
			return
		}

		if p.log.GetLevel() == zerolog.TraceLevel {
			sec := binary.LittleEndian.Uint64(buf[0:8])
			usec := binary.LittleEndian.Uint64(buf[8:16])
			p.lastReadAt = time.Unix(int64(sec), int64(usec)*1000)
		}

		// typ is the kind of event being reported
		typ := binary.LittleEndian.Uint16(buf[16:18])

		// value is the state of the event being reported (on/off, pressed/unpressed, etc.)
		var value int32
		binary.Read(bytes.NewReader(buf[20:]), binary.LittleEndian, &value)

		// we only care when typ is EV_KEY and value indicates "pressed"
		// https://github.com/torvalds/linux/blob/v5.17/include/uapi/linux/input-event-codes.h#L34-L51
		if typ == 1 && value == 1 {
			// in this context, code indicates what key was pressed
			code := binary.LittleEndian.Uint16(buf[18:20])
			if p.countAllKeys() || isPrintable(code) {
				atomic.AddInt32(keyPressCount, 1)
			}
		}
	}
}

func (p *TypingPattern) countAllKeys() bool {
	return config.GetBool(p.Name + "." + AllKeysLabel)
}

func (p *TypingPattern) getIdlePattern() Pattern {
	name := config.GetString(p.Name + "." + IdleLabel)
	if name == "" {
		return nil
	}

	idlePattern := Get(name)
	if idlePattern == nil {
		p.log.Error().Str("idle", name).Msg("pattern not found")
		return nil
	}

	return idlePattern
}

func (p *TypingPattern) getIdlePeriod() time.Duration {
	return config.GetDuration(p.Name + "." + IdlePeriodLabel)
}

var keyboardEventRE = regexp.MustCompile(`[= ](event\d+)( |$)`)

func lookupInputEventID() (string, error) {
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return "", fmt.Errorf("can't open input devices list: %w", err)
	}
	defer f.Close()

	found := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'N':
			found = strings.Contains(strings.ToLower(line), "keyboard")
		case 'H':
			if found {
				match := keyboardEventRE.FindStringSubmatch(line)
				if match != nil {
					return match[1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("can't find a keyboard input device")
}

func isPrintable(code uint16) bool {
	// https://github.com/torvalds/linux/blob/v5.17/include/uapi/linux/input-event-codes.h#L64
	return (1 < code && code < 14) ||
		(15 < code && code < 29) ||
		(29 < code && code < 42) ||
		(42 < code && code < 54) ||
		(code == 57)
}
