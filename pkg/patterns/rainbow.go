package patterns

import (
	"context"
	"time"

	"github.com/bambash/sys76-kb/pkg/keyboard"
)

// InfiniteRainbow generates... an infinite rainbow
func InfiniteRainbow(ctx context.Context, delay time.Duration) error {
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

			if sleep(ctx, delay) {
				return nil
			}
		}
		currentColorOffset = 0 // only used on first pass
	}
}
