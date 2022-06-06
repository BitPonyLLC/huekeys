package patterns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"syscall"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"

	"github.com/rs/zerolog"
)

// WatchPattern will report every color, brightness, and pattern change to the Out writer.
type WatchPattern struct {
	BasePattern

	Out io.Writer
}

var _ Pattern = (*WatchPattern)(nil) // ensures we conform to the Pattern interface

func init() {
	register("watch", &WatchPattern{}, 0)
}

// Run is overriding the BasePattern version as a special case and will hang
// forever, waiting for the parent context to interrupt.
func (p *WatchPattern) Run(parent context.Context, _ *zerolog.Logger) error {
	brightness, err := keyboard.GetCurrentBrightness()
	if err != nil {
		return err
	}

	colors, err := keyboard.GetCurrentColors()
	if err != nil {
		return err
	}

	var color string
	for _, v := range colors {
		color = v
		break // all will be set to the same value
	}

	var running string
	pattern := GetRunning()
	if pattern != nil {
		running = pattern.GetBase().Name
	}

	// always produce a report immediately
	err = p.report(brightness, color, running)
	if err != nil {
		return err
	}

	keyboardWatcher := keyboard.Events.Watch()
	patternWatcher := Events.Watch()
	defer func() {
		p.Out.Write([]byte("quit\n"))
		keyboardWatcher.Stop()
		patternWatcher.Stop()
	}()

	for {
		brightness = ""
		color = ""
		running = ""

		select {
		case <-parent.Done():
			p.stopRequested = true
			return nil
		case ev := <-keyboardWatcher.Ch:
			change := ev.(keyboard.ChangeEvent)
			brightness = change.Brightness
			color = change.Color
		case ev := <-patternWatcher.Ch:
			running = ev.(ChangeEvent).Pattern
		}

		err = p.report(brightness, color, running)
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				// client is gone: close up shop!
				return nil
			}

			return err
		}
	}
}

func (p *WatchPattern) report(brightness, color, running string) error {
	msg := ""

	if brightness != "" {
		msg += "b:" + brightness + "\n"
	}

	if color != "" {
		msg += "c:" + color + "\n"
	}

	if running != "" {
		msg += "r:" + running + "\n"
	}

	if msg != "" {
		_, err := p.Out.Write([]byte(msg))
		if err != nil {
			return fmt.Errorf("unable to write to watch output: %w", err)
		}
	}

	return nil
}
