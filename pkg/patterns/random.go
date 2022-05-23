package patterns

import (
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
)

type RandomPattern struct {
	BasePattern
}

const DefaultRandomDelay = 1 * time.Second

var _ Pattern = (*RandomPattern)(nil) // ensures we conform to the Pattern interface

func NewRandomPattern() *RandomPattern {
	return &RandomPattern{BasePattern: BasePattern{
		Name:  "random",
		Delay: DefaultRandomDelay,
	}}
}

func (p *RandomPattern) Run() error {
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
