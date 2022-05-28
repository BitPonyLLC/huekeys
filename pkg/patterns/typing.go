package patterns

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/spf13/viper"
)

type TypingPattern struct {
	BasePattern
}

const DefaultIdlePeriod = 30 * time.Second

const InputEventIDLabel = "input-event-id"
const AllKeysLabel = "all-keys"
const IdleLabel = "idle"
const IdlePeriodLabel = "idle-period"

var _ Pattern = (*TypingPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*TypingPattern)(nil) // ensures we conform to the runnable interface

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
	inputEventID := viper.GetString(p.Name + InputEventIDLabel)
	if inputEventID == "" {
		var err error
		inputEventID, err = lookupInputEventID()
		if err != nil {
			return err
		}
	}

	eventpath := "/dev/input/" + inputEventID
	f, err := os.Open(eventpath)
	if err != nil {
		return fmt.Errorf("can't open input events device (%s): %w", eventpath, err)
	}

	keyPressCount := int32(0)
	err = keyboard.ColorFileHandler(coldHotColors[0])
	if err != nil {
		return err
	}

	go p.setColor(&keyPressCount)
	go p.processTypingEvents(f, &keyPressCount)

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

		// don't bother setting the same value and wait for 2 keypresses to
		// avoid halting the pattern for control-key sequences
		if i > 1 && i != lastIndex {
			color := coldHotColors[i]
			err := keyboard.ColorFileHandler(color)
			if err != nil {
				p.log.Error().Err(err).Msg("can't set typing color")
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

			atomic.AddInt32(keyPressCount, -1)
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
				bp := idlePattern.GetBase()
				ilog := p.log.With().Str("idle", bp.Name).Logger()
				bp.ctx = cancelCtx
				bp.log = &ilog
				// using the private runner otherwise, we'll get canceled! ;)
				bp.self.run()
			}()
		}
	}

	if cancelFunc != nil {
		cancelFunc()
	}
}

func (p *TypingPattern) processTypingEvents(eventF io.Reader, keyPressCount *int32) {
	defer util.LogRecover()

	// https://janczer.github.io/work-with-dev-input/
	buf := make([]byte, 24)
	for !p.stopRequested {
		_, err := eventF.Read(buf)
		if err != nil {
			p.log.Error().Err(err).Msg("can't read input events device")
			return
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
				// sec := binary.LittleEndian.Uint64(buf[0:8])
				// usec := binary.LittleEndian.Uint64(buf[8:16])
				// ts := time.Unix(int64(sec), int64(usec)*1000)
				atomic.AddInt32(keyPressCount, 1)
			}
		}
	}
}

func (p *TypingPattern) countAllKeys() bool {
	return viper.GetBool(p.Name + "." + AllKeysLabel)
}

func (p *TypingPattern) getIdlePattern() Pattern {
	name := viper.GetString(p.Name + "." + IdleLabel)
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
	return viper.GetDuration(p.Name + "." + IdlePeriodLabel)
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
