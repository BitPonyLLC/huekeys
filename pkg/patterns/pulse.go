package patterns

import (
	"context"
	"strconv"
	"time"

	"github.com/bambash/sys76-kb/pkg/keyboard"
)

// BrightnessPulse continuously dials up and down brightness
func BrightnessPulse(ctx context.Context, delay time.Duration) error {
	for {
		for i := 255; i >= 0; i-- {
			s := strconv.Itoa(i)

			err := keyboard.BrightnessFileHandler(s)
			if err != nil {
				return err
			}

			if sleep(ctx, delay) {
				return nil
			}
		}
		for i := 1; i <= 255; i++ {
			s := strconv.Itoa(i)

			err := keyboard.BrightnessFileHandler(s)
			if err != nil {
				return err
			}

			if sleep(ctx, delay) {
				return nil
			}
		}
	}
}
