package keyboard

import (
	"strconv"
	"time"
)

// BrightnessPulse continuously dials up and down brightness
func BrightnessPulse(delay time.Duration) {
	if delay == 0 {
		delay = 25 * time.Millisecond
	}

	for {
		for i := 255; i >= 0; i-- {
			s := strconv.Itoa(i)
			BrightnessFileHandler(s)
			time.Sleep(delay)
		}
		for i := 1; i <= 255; i++ {
			s := strconv.Itoa(i)
			BrightnessFileHandler(s)
			time.Sleep(delay)
		}
	}
}

// InfiniteRainbow generates... an infinite rainbow
func InfiniteRainbow(delay time.Duration) {
	if delay == 0 {
		delay = time.Nanosecond
	}

	var currentColor string
	var currentColorOffset int

	currentColors := GetCurrentColors()
	for _, v := range currentColors {
		// assume all groups are set to the same color for now and simply grab the first one
		currentColor = v
		break
	}

	colors := make([]string, 0, 6)

	add := func(r, g, b int) {
		c := RGBColor{r, g, b}
		ch := c.GetColorInHex()
		if ch == currentColor {
			currentColorOffset = len(colors)
		}
		colors = append(colors, ch)
	}

	// generate range of rainbow values
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
			ColorFileHandler(c)
			time.Sleep(delay)
		}
		currentColorOffset = 0 // only used on first pass
	}
}

// InfinitRandom sets the keyboard colors to random values forever
func InfiniteRandom(delay time.Duration) {
	if delay == 0 {
		delay = time.Second
	}

	for {
		ColorFileHandler(RandomColor)
		time.Sleep(delay)
	}
}
