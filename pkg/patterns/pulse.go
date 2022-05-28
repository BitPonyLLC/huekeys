package patterns

import (
	"strconv"
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
)

type PulsePattern struct {
	BasePattern
}

var _ Pattern = (*PulsePattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*PulsePattern)(nil) // ensures we conform to the runnable interface

func init() {
	register("pulse", &PulsePattern{}, 25*time.Millisecond)
}

func (p *PulsePattern) run() error {
	for {
		for i := 255; i >= 0; i-- {
			s := strconv.Itoa(i)

			err := keyboard.BrightnessFileHandler(s)
			if err != nil {
				return err
			}

			if p.cancelableSleep() {
				return nil
			}
		}
		for i := 1; i <= 255; i++ {
			s := strconv.Itoa(i)

			err := keyboard.BrightnessFileHandler(s)
			if err != nil {
				return err
			}

			if p.cancelableSleep() {
				return nil
			}
		}
	}
}
