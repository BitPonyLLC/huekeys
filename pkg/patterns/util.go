package patterns

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/bambash/sys76-kb/internal/image_matcher"
	"github.com/bambash/sys76-kb/pkg/keyboard"
)

var stopRequested = false

func setColorFrom(u string) error {
	pictureURL, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("can't parse picture URI (%s): %w", pictureURL, err)
	}

	color, err := image_matcher.GetDominantColorOf(pictureURL.Path)
	if err != nil {
		return fmt.Errorf("can't determine dominant color: %w", err)
	}

	return keyboard.ColorFileHandler(color)
}

func sleep(ctx context.Context, delay time.Duration) bool {
	wake := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		wake.Stop()
		return true
	case <-wake.C:
		return false
	}
}
