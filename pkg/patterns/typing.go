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

	"github.com/bambash/sys76-kb/pkg/keyboard"
	"github.com/rs/zerolog/log"
)

// MonitorTyping sets the keyboard colors acccording to rate of typing
func MonitorTyping(ctx context.Context, delay time.Duration, inputEventID string, idleCB func(context.Context)) error {
	if inputEventID == "" {
		var err error
		inputEventID, err = getInputEventID()
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

	go setTypingColor(ctx, delay, &keyPressCount, idleCB)
	go processTypingEvents(f, &keyPressCount)

	<-ctx.Done()
	stopRequested = true

	return nil
}

func setTypingColor(ctx context.Context, delay time.Duration, keyPressCount *int32, idleCB func(context.Context)) {
	var idleAt *time.Time
	var cancelFunc context.CancelFunc

	lastIndex := 0
	colorsLen := len(coldHotColors)

	for {
		if sleep(ctx, delay) {
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
				log.Error().Err(err).Msg("can't set typing color")
				break
			}
			lastIndex = i
		}

		if i > 0 {
			if idleAt != nil {
				idleAt = nil
				log.Debug().Msg("no longer idle")
			}
			if cancelFunc != nil {
				cancelFunc()
				cancelFunc = nil
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
			log.Debug().Msg("idle")
			var cancelCtx context.Context
			cancelCtx, cancelFunc = context.WithCancel(ctx)
			go func() { idleCB(cancelCtx) }()
		}
	}

	if cancelFunc != nil {
		cancelFunc()
	}
}

func processTypingEvents(eventF io.Reader, keyPressCount *int32) {
	// https://janczer.github.io/work-with-dev-input/
	buf := make([]byte, 24)
	for !stopRequested {
		_, err := eventF.Read(buf)
		if err != nil {
			log.Error().Err(err).Msg("can't read input events device")
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
