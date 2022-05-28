package patterns

import (
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
)

type RandomPattern struct {
	BasePattern
}

var _ Pattern = (*RandomPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*RandomPattern)(nil) // ensures we conform to the runnable interface

func init() {
	register("random", &RandomPattern{}, 1*time.Second)
}

func (p *RandomPattern) run() error {
	for {
		err := keyboard.ColorFileHandler(keyboard.RandomColor)
		if err != nil {
			return err
		}

		if p.cancelableSleep() {
			return nil
		}
	}
}
