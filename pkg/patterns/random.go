package patterns

import (
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
)

type RandomPattern struct {
	BasePattern
}

const DefaultRandomDelay = 1 * time.Second

var _ Pattern = (*RandomPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*RandomPattern)(nil) // ensures we conform to the runnable interface

func NewRandomPattern() *RandomPattern {
	p := &RandomPattern{}
	p.BasePattern = BasePattern{
		Name:  "random",
		Delay: DefaultRandomDelay,
		self:  p,
	}
	return p
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
