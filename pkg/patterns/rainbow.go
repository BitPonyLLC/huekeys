package patterns

import (
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
)

// RainbowPattern is used when changing colors through the traditional colors of
// a rainbow. The "delay" configuration value expresses the amount of time to
// wait between changes of color.
type RainbowPattern struct {
	BasePattern
}

var _ Pattern = (*RainbowPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*RainbowPattern)(nil) // ensures we conform to the runnable interface

func init() {
	register("rainbow", &RainbowPattern{}, 1*time.Millisecond)
}

func (p *RainbowPattern) run() error {
	var currentColor string
	var currentColorOffset int

	currentColors, err := keyboard.GetCurrentColors()
	if err != nil {
		return err
	}

	for _, v := range currentColors {
		// assume all groups are set to the same color for now and simply grab the first one
		currentColor = v
		break
	}

	colors := make([]string, 0, 6)

	add := func(r, g, b int) {
		c := keyboard.RGBColor{Red: r, Green: g, Blue: b}
		ch := c.GetColorInHex()
		if ch == currentColor {
			currentColorOffset = len(colors)
		}
		colors = append(colors, ch)
	}

	// generate range of rainbow values ("cold" to "hot")
	for i := 0; i <= 255; i++ {
		add(255, i, 0)
	}

	for i := 255; i >= 0; i-- {
		add(i, 255, 0)
	}

	for i := 0; i <= 255; i++ {
		add(0, 255, i)
	}

	for i := 255; i >= 0; i-- {
		add(0, i, 255)
	}

	for i := 0; i <= 255; i++ {
		add(i, 0, 255)
	}

	for i := 255; i >= 0; i-- {
		add(255, 0, i)
	}

	for {
		for i := currentColorOffset; i < len(colors); i++ {
			c := colors[i]

			err := keyboard.ColorFileHandler(c)
			if err != nil {
				return err
			}

			if p.cancelableSleep() {
				return nil
			}
		}

		currentColorOffset = 0 // only used on first pass
	}
}
