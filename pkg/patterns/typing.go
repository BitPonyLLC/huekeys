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
)

type TypingPattern struct {
	BasePattern

	InputEventID string
	IdlePattern  Pattern
}

const DefaultTypingDelay = 300 * time.Millisecond

var _ Pattern = (*TypingPattern)(nil) // ensures we conform to the Pattern interface

func NewTypingPattern() *TypingPattern {
	return &TypingPattern{BasePattern: BasePattern{
		Name:  "typing",
		Delay: DefaultTypingDelay,
	}}
}

func (p *TypingPattern) Run() error {
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
		if diff > 30*time.Second {
			p.Log.Debug().Msg("idle")
			var cancelCtx context.Context
			cancelCtx, cancelFunc = context.WithCancel(p.Ctx)
			go func() {
				bp := p.IdlePattern.GetBase()
				ilog := p.Log.With().Str("idle", bp.Name).Logger()
				ilog.Info().Msg("starting")
				bp.Ctx = cancelCtx
				bp.Log = &ilog
				p.IdlePattern.Run()
			}()
		}
	}

	if cancelFunc != nil {
		cancelFunc()
	}
}

func (p *TypingPattern) processTypingEvents(eventF io.Reader, keyPressCount *int32) {
	// https://janczer.github.io/work-with-dev-input/
	buf := make([]byte, 24)
	for !p.stopRequested {
		_, err := eventF.Read(buf)
		if err != nil {
			p.Log.Error().Err(err).Msg("can't read input events device")
			return
		}

		typ := binary.LittleEndian.Uint16(buf[16:18])
		// code := binary.LittleEndian.Uint16(buf[18:20])

		var value int32
		binary.Read(bytes.NewReader(buf[20:]), binary.LittleEndian, &value)

		// we only care when typ is EV_KEY and value indicates "pressed"
		// https://github.com/torvalds/linux/blob/v5.17/include/uapi/linux/input-event-codes.h#L34-L51
		if typ == 1 && value == 1 {
			// sec := binary.LittleEndian.Uint64(buf[0:8])
			// usec := binary.LittleEndian.Uint64(buf[8:16])
			// ts := time.Unix(int64(sec), int64(usec)*1000)
			atomic.AddInt32(keyPressCount, 1)
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
