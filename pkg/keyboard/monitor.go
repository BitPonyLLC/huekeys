package keyboard

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

// StartMonitor continuously monitors the currently set brightness and color and
// resets them if changed outside this process. Cancel the provided ctx to stop
// the monitor.
func StartMonitor(ctx context.Context, delay time.Duration) {
	monitorCtx.Store(ctx)
	monitorDelay.Store(delay)

	if monitorMutex.TryLock() {
		go func() {
			defer monitorMutex.Unlock()
			monitor()
		}()
	}
}

//--------------------------------------------------------------------------------
// private

var monitorMutex sync.Mutex
var monitorCtx atomic.Value
var monitorDelay atomic.Duration
var monitoredColor atomic.String
var monitoredBrightness atomic.String

func monitor() {
	log.Debug().Dur("delay", monitorDelay.Load()).Msg("starting color/brightness monitor")
	defer log.Debug().Msg("color/brightness monitor stopped")

	for {
		ctx := monitorCtx.Load().(context.Context)
		check := time.NewTimer(monitorDelay.Load())
		select {
		case <-ctx.Done():
			return
		case <-check.C:
		}

		color := monitoredColor.Load()
		if color != "" {
			cc, err := GetCurrentColors()
			if err != nil {
				log.Err(err).Msg("monitor")
				return
			}

			for _, c := range cc {
				if color != c {
					log.Trace().Str("want", color).Str("have", c).Msg("resetting color")
					err = ColorFileHandler(color)
					if err != nil {
						log.Err(err).Msg("monitor")
						return
					}

					break
				}
			}
		}

		brightness := monitoredBrightness.Load()
		if brightness != "" {
			cb, err := GetCurrentBrightness()
			if err != nil {
				log.Err(err).Msg("monitor")
				return
			}

			if brightness != cb {
				log.Trace().Str("want", brightness).Str("have", cb).Msg("resetting brightness")
				err = BrightnessFileHandler(brightness)
				if err != nil {
					log.Err(err).Msg("monitor")
					return
				}
			}
		}
	}
}
