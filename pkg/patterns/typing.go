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
)

type TypingPattern struct {
	BasePattern

	InputEventID string
	CountAllKeys bool
	IdlePattern  Pattern
	IdlePeriod   time.Duration
}

const DefaultTypingDelay = 300 * time.Millisecond
const DefaultIdlePeriod = 30 * time.Second

var _ Pattern = (*TypingPattern)(nil) // ensures we conform to the Pattern interface

func NewTypingPattern() *TypingPattern {
	p := &TypingPattern{
		IdlePeriod: DefaultIdlePeriod,
	}
	p.BasePattern = BasePattern{
		Name:  "typing",
		Delay: DefaultTypingDelay,
		run:   p.run,
	}
	return p
}

func (p *TypingPattern) run() error {
	if p.InputEventID == "" {
		var err error
		p.InputEventID, err = getInputEventID()
		if err != nil {
			return err
		}
	}

	eventpath := "/dev/input/" + p.InputEventID
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

	<-p.Ctx.Done()
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

		// don't bother setting the same value
		if i != lastIndex {
			color := coldHotColors[i]
			err := keyboard.ColorFileHandler(color)
			if err != nil {
				p.Log.Error().Err(err).Msg("can't set typing color")
				break
			}

			lastIndex = i
		}

		if i > 0 {
			idleAt = nil

			if cancelFunc != nil {
				cancelFunc()
				cancelFunc = nil
				p.Log.Debug().Msg("no longer idle")
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

		diff := time.Since(*idleAt)
		if diff > p.IdlePeriod {
			p.Log.Debug().Msg("idle")
			var cancelCtx context.Context
			cancelCtx, cancelFunc = context.WithCancel(p.Ctx)
			go func() {
				defer util.LogRecover()
				bp := p.IdlePattern.GetBase()
				ilog := p.Log.With().Str("idle", bp.Name).Logger()
				ilog.Info().Msg("starting")
				bp.Ctx = cancelCtx
				bp.Log = &ilog
				p.IdlePattern.Run()
				ilog.Info().Msg("stopping")
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
			p.Log.Error().Err(err).Msg("can't read input events device")
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
			if p.CountAllKeys || isPrintable(code) {
				// sec := binary.LittleEndian.Uint64(buf[0:8])
				// usec := binary.LittleEndian.Uint64(buf[8:16])
				// ts := time.Unix(int64(sec), int64(usec)*1000)
				atomic.AddInt32(keyPressCount, 1)
			}
		}
	}
}

var keyboardEventRE = regexp.MustCompile(`[= ](event\d+)( |$)`)

func getInputEventID() (string, error) {
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
