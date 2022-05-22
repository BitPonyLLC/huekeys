package patterns

import (
	"context"
	"time"

	"github.com/bambash/sys76-kb/pkg/keyboard"
)

// InfinitRandom sets the keyboard colors to random values forever
func InfiniteRandom(ctx context.Context, delay time.Duration) error {
	for {
		err := keyboard.ColorFileHandler(keyboard.RandomColor)
		if err != nil {
			return err
		}

		if sleep(ctx, delay) {
			return nil
		}
	}
}
