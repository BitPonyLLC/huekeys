package patterns

import (
	"strconv"
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
)

type PulsePattern struct {
	BasePattern
}

const DefaultPulseDelay = 25 * time.Millisecond

var _ Pattern = (*PulsePattern)(nil) // ensures we conform to the Pattern interface

func NewPulsePattern() *PulsePattern {
	return &PulsePattern{BasePattern: BasePattern{
		Name:  "pulse",
		Delay: DefaultPulseDelay,
	}}
}

func (p *PulsePattern) Run() error {
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
